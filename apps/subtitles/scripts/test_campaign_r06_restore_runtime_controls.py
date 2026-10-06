"""Failure-first exact Go test and Owner diagnostic accounting."""
import json
import unittest
import campaign_r06_restore_runtime_controls as controls
from campaign_r06_restore_runtime_artifacts import control_shape
from campaign_r06_restore_runtime_controls import ControlProjection, NAMES, PACKAGE

class RestoreControlProjectionTests(unittest.TestCase):
    def event(self, projection, action, name=None, output=None):
        value = {"Package": PACKAGE, "Action": action}
        if name is not None: value["Test"] = name
        if output is not None: value["Output"] = output
        projection.consume(json.dumps(value).encode())

    def owner(self, projection, name, **changes):
        lines = [(35, "R06_RESTORE_OWNER_REQUEST setup 200 true"),
                 (52, "R06_RESTORE_OWNER_SETUP 200 true true true true"),
                 (35, "R06_RESTORE_OWNER_REQUEST enrollment 200 true"),
                 (85, "R06_RESTORE_OWNER_ENROLLMENT 200 true true true"),
                 (35, "R06_RESTORE_OWNER_REQUEST mfa 303 true"),
                 (35, "R06_RESTORE_OWNER_REQUEST current 200 true"),
                 (121, "R06_RESTORE_OWNER_CURRENT 200 true true")]
        if changes.get("setup_csrf") is False: lines[1] = (52, "R06_RESTORE_OWNER_SETUP 200 true true true false")
        if changes.get("false_csrf") == "enrollment": lines[3] = (85, "R06_RESTORE_OWNER_ENROLLMENT 200 true true false")
        if changes.get("false_csrf") == "current": lines[6] = (121, "R06_RESTORE_OWNER_CURRENT 200 true false")
        for line, text in lines:
            self.event(projection, "output", name,
                       "    restore_owner_enrollment_test.go:"+str(changes.get("line", line))+": "+text+"\n")

    def complete(self):
        value = ControlProjection()
        for name in NAMES:
            self.event(value, "run", name); self.owner(value, name); self.event(value, "pass", name)
        self.event(value, "pass")
        return value

    def test_four_actual_public_names_and_package_are_fixed(self):
        self.assertEqual(NAMES, ("TestRestoreLegacySwapPublicControl", "TestRestorePreparedReceiptPublicControl",
                                "TestRestoreHeldHeadersPublicControl", "TestRestoreHeldInspectionBodyPublicControl"))
        self.assertEqual(PACKAGE, "github.com/MikeO7/kinosail-subtitles/engineering/qa/2026-10-05-restore-recovery/fixture")
        self.assertTrue(self.complete().result()["green"])

    def test_missing_named_test_or_package_terminal_blocks(self):
        value = ControlProjection()
        for name in NAMES[:-1]:
            self.event(value, "run", name); self.owner(value, name); self.event(value, "pass", name)
        self.event(value, "pass")
        self.assertFalse(value.result()["green"])
        value = self.complete(); value.package_terminal = None
        self.assertFalse(value.result()["green"])

    def test_skipped_failed_duplicate_and_foreign_test_never_green(self):
        for action in ("skip", "fail"):
            value = self.complete(); self.event(value, action, NAMES[0])
            self.assertFalse(value.result()["green"])
        for action, name in (("run", NAMES[0]), ("pass", NAMES[0]), ("run", "TestForeign")):
            value = self.complete(); self.event(value, action, name)
            self.assertFalse(value.result()["green"])

    def test_owner_flow_uses_account_csrf_mfa303_current_token(self):
        value = ControlProjection()
        for name in NAMES:
            self.event(value, "run", name); self.event(value, "pass", name)
        self.event(value, "pass")
        self.assertFalse(value.result()["green"])

    def test_wrong_marker_location_wrong_boolean_and_duplicate_block(self):
        for output in ("    restore_owner_enrollment_test.go:34: R06_RESTORE_OWNER_REQUEST setup 200 true\n",
                       "    restore_owner_enrollment_test.go:35: R06_RESTORE_OWNER_REQUEST setup 200 1\n",
                       "    foreign.go:35: R06_RESTORE_OWNER_REQUEST setup 200 true\n"):
            value = self.complete(); self.event(value, "output", NAMES[0], output)
            self.assertFalse(value.result()["green"])
        value = self.complete(); self.owner(value, NAMES[0])
        self.assertFalse(value.result()["green"])

    def test_malformed_wrong_package_and_oversized_events_block(self):
        for line in (b"not-json", b'{"Action":"pass","Package":"foreign"}',
                     b'{"Action":"pass","Action":"pass"}', b"x"*65537):
            value = self.complete(); value.consume(line)
            self.assertFalse(value.result()["green"])

    def test_raw_private_output_is_not_exported(self):
        value = self.complete()
        self.event(value, "output", NAMES[0], "fictional private cookie, URL and body\n")
        result = json.dumps(value.result())
        self.assertNotIn("fictional private", result)
        self.assertNotIn("cookie", result)

    def test_output_without_started_case_and_repeated_package_terminal_block(self):
        value = ControlProjection()
        self.event(value, "output", NAMES[0], "    restore_owner_enrollment_test.go:35: R06_RESTORE_OWNER_REQUEST setup 200 true\n")
        self.assertFalse(value.result()["green"])
        value = self.complete(); self.event(value, "pass")
        self.assertFalse(value.result()["green"])

    def test_initial_cookieless_setup_csrf_false_is_valid_and_recorded(self):
        value = ControlProjection()
        for name in NAMES:
            self.event(value, "run", name); self.owner(value, name, setup_csrf=False); self.event(value, "pass", name)
        self.event(value, "pass")
        result = value.result()
        self.assertTrue(result["green"])
        self.assertTrue(all(row["setupPageCSRF"] is False for row in result["owner"].values()))

    def test_setup_csrf_diagnostic_preserves_both_boolean_values(self):
        result = self.complete().result()
        self.assertTrue(all(row["setupPageCSRF"] is True for row in result["owner"].values()))

    def test_authenticated_enrollment_and_current_csrf_false_block(self):
        for stage in ("enrollment", "current"):
            value = ControlProjection()
            for name in NAMES:
                self.event(value, "run", name); self.owner(value, name, false_csrf=stage); self.event(value, "pass", name)
            self.event(value, "pass")
            self.assertFalse(value.result()["green"])

    def test_package_terminal_cannot_precede_all_named_terminals(self):
        value = ControlProjection()
        self.event(value,"pass")
        for name in NAMES:
            self.event(value,"run",name);self.owner(value,name);self.event(value,"pass",name)
        self.assertFalse(value.result()["green"])
        self.assertGreater(value.result()["invalidEventCount"],0)

    def test_named_output_and_runs_after_terminal_are_accounted_invalid(self):
        for action in ("output","run"):
            value=self.complete();self.event(value,action,NAMES[0],"fictional private output" if action=="output" else None)
            self.assertFalse(value.result()["green"])
            self.assertGreater(value.result()["invalidEventCount"],0)
        value=ControlProjection();self.event(value,"run",NAMES[0]);self.event(value,"pass",NAMES[0])
        self.event(value,"output",NAMES[0],"fictional private output")
        self.assertGreater(value.result()["invalidEventCount"],0)

    def invalid_counts(self, value):
        reasons = value.result().get("invalidReasons")
        expected = set(("control-json control-invalid-type control-output control-marker-location "
                        "control-owner-shape control-marker-duplicate control-owner-prerequisite "
                        "control-package control-event control-after-package-terminal control-name "
                        "control-output-order control-action control-early-package-terminal "
                        "control-package-action control-package-marker").split())
        self.assertIsInstance(reasons, dict)
        self.assertEqual(set(reasons), expected)
        self.assertTrue(all(type(count) is int and 0 <= count <= 16777216 for count in reasons.values()))
        self.assertEqual(sum(reasons.values()), value.result()["invalidEventCount"])
        return reasons

    def test_invalid_reason_inventory_is_fixed_closed_and_zero_for_green(self):
        value = self.complete(); reasons = self.invalid_counts(value)
        self.assertTrue(value.result()["green"]); self.assertEqual(set(reasons.values()), {0})
        self.assertEqual(set(controls.INVALID_REASONS), set(reasons))
        self.assertEqual(controls.INVALID_LIMIT, 16777216)

    def test_private_malformed_duplicate_and_nonfinite_json_only_count_static_json(self):
        for line in (b"fictional private cookie URL body", b'{"private":"fictional-private",',
                     b'{"Action":"run","Action":"pass"}', b'{"n":NaN}', b'{"n":1e999}', b"x"*65537):
            value = ControlProjection(); value.consume(line)
            self.assertEqual(self.invalid_counts(value)["control-json"], 1)
            self.assertNotIn("fictional-private", json.dumps(value.result()))
            self.assertFalse(value.result()["green"])

    def test_package_unknown_fields_foreign_names_and_actions_count_exact_reasons(self):
        cases = [({"Package": "fictional-private", "Action": "start"}, "control-package"),
                 ({"Package": PACKAGE, "Action": "start", "fictional-private": "fictional-private"}, "control-event"),
                 ({"Package": PACKAGE, "Action": "run", "Test": "fictional-private"}, "control-name"),
                 ({"Package": PACKAGE, "Action": "pause", "Test": NAMES[0]}, "control-action"),
                 ({"Package": PACKAGE, "Action": "pause"}, "control-package-action"),
                 ({"Package": PACKAGE, "Action": "pass"}, "control-early-package-terminal"),
                 ({"Package": PACKAGE, "Action": "output", "Output": "R06_RESTORE_OWNER_private"}, "control-package-marker"),
                 ({"Package": PACKAGE, "Action": "output", "Test": NAMES[0], "Output": "private"}, "control-output-order")]
        for event, reason in cases:
            value = ControlProjection(); value.consume(json.dumps(event).encode())
            self.assertEqual(self.invalid_counts(value)[reason], 1)
            self.assertNotIn("fictional-private", json.dumps(value.result()))
            self.assertFalse(value.result()["green"])

    def test_owner_output_location_shape_duplicate_and_prerequisite_count_static_reasons(self):
        cases = [(None, "control-output"),
                 ("    foreign.go:35: R06_RESTORE_OWNER_REQUEST setup 200 true\n", "control-marker-location"),
                 ("    restore_owner_enrollment_test.go:35: R06_RESTORE_OWNER_REQUEST private 200 true\n", "control-owner-shape"),
                 ("    restore_owner_enrollment_test.go:35: R06_RESTORE_OWNER_REQUEST setup 200 false\n", "control-owner-prerequisite")]
        for output, reason in cases:
            value = ControlProjection(); self.event(value, "run", NAMES[0])
            self.event(value, "output", NAMES[0], output)
            self.assertEqual(self.invalid_counts(value)[reason], 1)
        value = ControlProjection(); self.event(value, "run", NAMES[0]); self.owner(value, NAMES[0])
        self.event(value, "output", NAMES[0], "    restore_owner_enrollment_test.go:35: R06_RESTORE_OWNER_REQUEST setup 200 true\n")
        self.assertEqual(self.invalid_counts(value)["control-marker-duplicate"], 1)

    def test_named_and_package_terminal_order_counts_remain_strict(self):
        value = self.complete(); self.event(value, "output", NAMES[0], "fictional-private")
        self.assertEqual(self.invalid_counts(value)["control-after-package-terminal"], 1)
        self.assertFalse(value.result()["green"])
        value = ControlProjection(); self.event(value, "run", NAMES[0]); self.event(value, "pass", NAMES[0])
        self.event(value, "output", NAMES[0], "fictional-private")
        self.assertEqual(self.invalid_counts(value)["control-output-order"], 1)

    def test_unexpected_invalid_type_maps_to_fixed_fallback_without_private_exception(self):
        value = ControlProjection(); self.event(value, "run", NAMES[0])
        self.event(value, "output", NAMES[0], "\ud800fictional-private")
        self.assertEqual(self.invalid_counts(value)["control-invalid-type"], 1)
        self.assertNotIn("fictional-private", json.dumps(value.result()))
        self.assertFalse(value.result()["green"])

    def test_duplicate_accounting_stays_separate_from_invalid_reasons(self):
        value = ControlProjection(); self.event(value, "run", NAMES[0]); self.event(value, "run", NAMES[0])
        self.assertEqual(sum(self.invalid_counts(value).values()), 0)
        self.assertEqual(value.result()["duplicateTerminalCount"], 1)
        self.assertFalse(value.result()["green"])

    def test_invalid_reason_overflow_preserves_accounting_and_artifact_rejects(self):
        value = ControlProjection()
        self.invalid_counts(value)
        value.invalid = 16777215; value.invalid_reasons = {key: 0 for key in controls.INVALID_REASONS}
        value.invalid_reasons["control-json"] = 16777215
        value.consume(b"invalid"); value.consume(b"invalid")
        result = value.result()
        self.assertEqual(result["invalidReasons"]["control-json"], 16777217)
        self.assertEqual(sum(result["invalidReasons"].values()), result["invalidEventCount"])
        self.assertEqual(result["invalidEventCount"], 16777217)
        self.assertFalse(result["green"])
        with self.assertRaises(ValueError): control_shape(result)

    def test_diagnostic_artifact_schema_accepts_exact_green_and_blocked_counts(self):
        value = self.complete(); self.invalid_counts(value); control_shape(value.result())
        value = ControlProjection(); value.consume(b"invalid"); value.consume(b"invalid")
        self.assertEqual(self.invalid_counts(value)["control-json"], 2)
        control_shape(value.result()); self.assertFalse(value.result()["green"])

    def test_diagnostic_artifact_schema_rejects_unknown_omitted_and_foreign_keys(self):
        from copy import deepcopy
        base = self.complete().result(); self.invalid_counts(self.complete())
        variants = []
        value = deepcopy(base); value["invalidReasons"]["fictional-private"] = 0; variants.append(value)
        value = deepcopy(base); value["invalidReasons"].pop("control-json"); variants.append(value)
        value = deepcopy(base); value.pop("invalidReasons"); variants.append(value)
        value = deepcopy(base); value["fictional-private"] = "fictional-private"; variants.append(value)
        for value in variants:
            with self.assertRaises(ValueError): control_shape(value)

    def test_diagnostic_artifact_schema_rejects_bool_negative_fraction_overflow_and_sum(self):
        from copy import deepcopy
        base = self.complete().result(); self.invalid_counts(self.complete())
        for count in (True, -1, 0.5, 16777217, 1):
            value = deepcopy(base); value["invalidReasons"]["control-json"] = count
            with self.assertRaises(ValueError): control_shape(value)
        value = deepcopy(base); value["invalidReasons"] = []
        with self.assertRaises(ValueError): control_shape(value)

    def test_counter_payload_or_tampered_total_cannot_make_blocked_events_green(self):
        from copy import deepcopy
        value = ControlProjection(); value.consume(b"invalid"); base = value.result(); self.invalid_counts(value)
        forged = deepcopy(base); forged["invalidEventCount"] = 0
        with self.assertRaises(ValueError): control_shape(forged)
        forged = deepcopy(base); forged["invalidReasons"]["control-json"] = "fictional-private"
        with self.assertRaises(ValueError): control_shape(forged)
