"""Pure failure-first fixed stage/reason controls; no filesystem or processes."""
import builtins
import json
from pathlib import Path
import sys
import unittest
from unittest.mock import Mock, patch

import r18_format_public_controls as controls
import test_r18_format_public_controls as baseline

STAGES = ("selection", "root", "reader-definitions", "wrapper-capture", "test-capture",
          "wrapper-definitions", "package-load", "package-validation", "test-definitions",
          "test-selection", "test-execution", "complete")
REASONS = ("none", "selection-prerequisite", "revision-prerequisite", "root-prerequisite",
           "input-deadline", "controls-deadline", "regular-size-boundary", "opened-input-changed",
           "input-growth", "descriptor-input-changed", "named-input-changed",
           "descriptor-close-unconfirmed", "captured-source-size", "captured-source-identity",
           "captured-module-boundary", "uncaptured-definition-import", "package-pins",
           "package-closure", "package-module", "package-byte-bound", "test-class", "test-alias",
           "test-method-closure", "mock-only-boundary", "unexpected-test", "duplicate-test",
           "unexpected-terminal", "unexpected-stop", "directory-path-boundary",
           "regular-path-boundary", "input-limit", "os-error", "import-error", "type-error",
           "value-error", "runtime-error", "recursion-error", "interrupted", "unclassified",
           "control-results")
BASE_FIELDS = {"schemaVersion", "id", "suite", "phase", "classification", "count", "passed",
               "failed", "errors", "skipped", "records", "formatterExecuted", "goExecuted",
               "autoAdoption", "semanticsVerified"}


def main_result(prepare=None, select=None, run=None, environment=None):
    with patch.object(controls.os, "environ", environment or baseline.ENV), \
            patch.object(sys, "argv", ["fixed"]), \
            patch.object(controls, "prepare", prepare or Mock()), \
            patch.object(controls, "select_tests", select or Mock()), \
            patch.object(controls, "run_controls", run or Mock()), \
            patch.object(controls.time, "monotonic", return_value=0), \
            patch.object(builtins, "print") as output:
        code = controls.main()
    output.assert_called_once()
    report = json.loads(output.call_args.args[0])
    return code, report


class StageControls(unittest.TestCase):
    def test_python312_pathlib_lazy_import_is_preloaded_and_guarded(self):
        self.assertIn("ntpath", sys.modules)
        with controls.definition_imports():
            self.assertIs(builtins.__import__("ntpath"), sys.modules["ntpath"])
            for name in ("urllib", "r18_unlisted_definition"):
                with self.subTest(name=name), self.assertRaises(ImportError):
                    builtins.__import__(name)
            with self.assertRaises(ImportError):
                builtins.__import__("ntpath", level=1)

    def test_all_stages_and_reason_enums_are_exact_and_closed(self):
        self.assertEqual(tuple(controls.STAGES), STAGES)
        self.assertEqual(tuple(controls.REASONS), REASONS)
        for stage in STAGES:
            state = {"stage": "selection"}
            controls.advance(state, stage)
            self.assertEqual(state, {"stage": stage})
        for stage in ("private-path", "__main__", None, 19):
            state = {"stage": "selection"}
            with self.assertRaises(ValueError):
                controls.advance(state, stage)
            self.assertEqual(state, {"stage": "selection"})

    def test_exact_prepare_markers_and_captures_retain_original_protocol(self):
        reader, public, target = baseline.prepared()
        state, seen = {"stage": "selection"}, []
        def create(name, relative, raw, root):
            seen.append(state["stage"])
            return {controls.READER_ALIAS: reader, controls.PUBLIC_ALIAS: public,
                    controls.TEST_ALIAS: target}[name]
        def read(path, limit, deadline):
            seen.append(state["stage"])
            return baseline.WRAPPER if path == baseline.ROOT / controls.WRAPPER_PIN[1] else baseline.TEST
        reader.read_small.side_effect = read
        public.load_package.side_effect = lambda deadline: (seen.append(state["stage"]) or baseline.package())
        with patch.object(controls, "module_from", side_effect=create):
            self.assertIs(controls.prepare(baseline.ROOT, 20, state), target)
        self.assertEqual(seen, ["reader-definitions", "wrapper-capture", "test-capture",
                                "wrapper-definitions", "package-load", "test-definitions"])
        self.assertEqual(state["stage"], "test-definitions")

    def test_every_prepare_fault_is_distinct_before_any_later_boundary(self):
        for stage in STAGES[1:9]:
            reader, public, target = baseline.prepared()
            state = {"stage": "selection"}
            def create(name, relative, raw, root):
                if stage == state["stage"]:
                    raise ValueError("captured-module-boundary")
                return {controls.READER_ALIAS: reader, controls.PUBLIC_ALIAS: public,
                        controls.TEST_ALIAS: target}[name]
            def read(path, limit, deadline):
                if stage == state["stage"]:
                    raise ValueError("descriptor-close-unconfirmed")
                return baseline.WRAPPER if path == baseline.ROOT / controls.WRAPPER_PIN[1] else baseline.TEST
            def load(deadline):
                if stage == "package-load":
                    raise ValueError("uncaptured-definition-import")
                return baseline.package()
            def validate(*args):
                if stage == "package-validation":
                    raise ValueError("package-closure")
            reader.read_small.side_effect, public.load_package.side_effect = read, load
            root = Path("relative") if stage == "root" else baseline.ROOT
            with self.subTest(stage=stage), patch.object(controls, "module_from", side_effect=create), \
                    patch.object(controls, "validate_package", side_effect=validate):
                with self.assertRaises(ValueError):
                    controls.prepare(root, 20, state)
                self.assertEqual(state["stage"], stage)

    def test_known_codes_are_fixed_but_private_messages_or_classes_never_export(self):
        for reason in REASONS[1:31]:
            self.assertEqual(controls.error_reason(ValueError(reason)), reason)
        for error, reason in ((OSError("private-cookie/path"), "os-error"),
                              (ImportError("private-token"), "import-error"),
                              (TypeError("private-environment"), "type-error"),
                              (ValueError("private-body"), "value-error"),
                              (RuntimeError("private-captured-output"), "runtime-error"),
                              (RecursionError("private-output"), "recursion-error"),
                              (KeyboardInterrupt("private-path"), "interrupted"),
                              (Exception("private-secret"), "unclassified")):
            self.assertEqual(controls.error_reason(error), reason)
        self.assertEqual(controls.error_reason(ValueError("input-deadline", "private-extra")), "value-error")

    def test_selection_failure_reports_only_fixed_stage_and_reason(self):
        prepare = Mock()
        code, report = main_result(prepare=prepare, environment=baseline.ENV | {"CAMPAIGN_PROOF": "wrong"})
        prepare.assert_not_called()
        self.assertEqual((code, report["diagnostic"]["stage"], report["diagnostic"]["reason"]), (2, "selection", "selection-prerequisite"))
        self.assertEqual((report["count"], report["records"]), (0, []))
        self.assertEqual(set(report), BASE_FIELDS | {"diagnostic"})
        self.assertEqual(set(report["diagnostic"]), {"stage", "reason"})

    def test_main_exports_fixed_prepare_stage_without_private_exception_payload(self):
        for stage in STAGES[1:9]:
            def failing(_root, _deadline, state):
                controls.advance(state, stage)
                raise OSError("private-cookie-secret-url-environment-output")
            select, run = Mock(), Mock()
            code, report = main_result(prepare=failing, select=select, run=run)
            select.assert_not_called()
            run.assert_not_called()
            self.assertEqual((code, report["diagnostic"]["stage"], report["diagnostic"]["reason"]), (2, stage, "os-error"))
            self.assertEqual((report["count"], report["records"]), (0, []))
            self.assertNotIn("private-", json.dumps(report))
            self.assertFalse(report["formatterExecuted"])
            self.assertFalse(report["goExecuted"])

    def test_selection_and_run_faults_cannot_masquerade_as_zero_count_root_failure(self):
        select, run = Mock(side_effect=ValueError("test-method-closure")), Mock()
        code, report = main_result(select=select, run=run)
        self.assertEqual((code, report["diagnostic"]["stage"], report["diagnostic"]["reason"]), (2, "test-selection", "test-method-closure"))
        run.assert_not_called()
        code, report = main_result(run=Mock(side_effect=RuntimeError("mock-only-boundary")))
        self.assertEqual((code, report["diagnostic"]["stage"], report["diagnostic"]["reason"]), (2, "test-execution", "mock-only-boundary"))
        self.assertEqual((report["count"], report["records"]), (0, []))

    def test_existing_nineteen_result_schema_is_unchanged_and_only_completed_green_exits_zero(self):
        with patch.object(controls.time, "monotonic", return_value=0):
            original = controls.run_controls(baseline.passing_class(), 20)
        self.assertEqual(set(original), BASE_FIELDS)
        code, report = main_result(run=Mock(return_value=original))
        self.assertEqual(code, 0)
        self.assertNotIn("diagnostic", report)
        self.assertEqual(set(report), BASE_FIELDS)
        self.assertEqual({key: report[key] for key in BASE_FIELDS}, original)
        for classification in ("controls-failed", "prerequisite-blocked"):
            changed = original | {"classification": classification}
            code, report = main_result(run=Mock(return_value=changed))
            self.assertEqual(code, 2)
            if classification == "prerequisite-blocked":
                self.assertEqual(report["diagnostic"], {"stage": "complete", "reason": "control-results"})
                self.assertEqual(set(report), BASE_FIELDS | {"diagnostic"})
            else:
                self.assertEqual(set(report), BASE_FIELDS)
                self.assertNotIn("diagnostic", report)

    def test_stage_reporting_preserves_sys_path_bytecode_and_all_frozen_source_pins(self):
        before = list(sys.path), sys.dont_write_bytecode
        def failing(_root, _deadline, state):
            controls.advance(state, "wrapper-definitions")
            sys.path.insert(0, "private-placeholder")
            sys.dont_write_bytecode = not before[1]
            raise ImportError("private-module")
        code, report = main_result(prepare=failing)
        self.assertEqual((list(sys.path), sys.dont_write_bytecode), before)
        self.assertEqual((code, report["diagnostic"]["stage"], report["diagnostic"]["reason"]), (2, "wrapper-definitions", "import-error"))
        self.assertEqual(controls.WRAPPER_PIN[2], "b7ad9f7c311c4bdc1e8f5ba3fea8bb13b2ae1d42")
        self.assertEqual(controls.TEST_PIN[2], "ff3be3283cc8c5e35d23b697592fa5571f2acc89")
        self.assertEqual(tuple(controls.NAMES), baseline.NAMES)
