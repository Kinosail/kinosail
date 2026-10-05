"""Failure-first exact Go test and Owner diagnostic accounting."""
import json
import unittest
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
