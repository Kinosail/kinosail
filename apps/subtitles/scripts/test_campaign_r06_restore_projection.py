"""Failure-first controls for unadmitted Restore formatter source only."""
import base64
import unittest

import campaign_r06_restore_format as formatter
import campaign_r06_restore_projection as projection


class RestoreRejectedProjectionControls(unittest.TestCase):
    original = b"package main\nvar x=1\n"
    changed = b"package main\nvar x=2\n"

    def record(self, **changes):
        values = dict(path=formatter.GO_FILES[5], original=self.original,
            output=self.changed, expected=(formatter.blob(self.original),
                formatter.pin(self.original)["sha256"], len(self.original)),
            phase=dict(exitCode=0, timedOut=False, ownedGroupStopped=True,
                       captureSettled=True, captureFailed=False),
            stderr=b"", source_unchanged=True, failure_code="format-token-change")
        values.update(changes)
        return projection.unadmitted_record(**values)

    def test_only_closed_source_diagnostic_is_exported(self):
        record = self.record()
        self.assertEqual(set(record), {"path", "admitted", "sourceOnly", "failureCode",
            "tokensPreserved", "original", "formatted", "formattedLines",
            "formattedSourceBase64"})
        self.assertIs(record["admitted"], False)
        self.assertIs(record["sourceOnly"], True)
        self.assertIs(record["tokensPreserved"], False)
        self.assertEqual(record["failureCode"], "format-token-change")
        self.assertEqual(record["original"],
            {"gitBlob": formatter.blob(self.original), **formatter.pin(self.original)})
        self.assertEqual(record["formatted"], formatter.pin(self.changed))
        self.assertEqual(record["formattedLines"], len(self.changed.splitlines()))
        self.assertEqual(base64.b64decode(record["formattedSourceBase64"], validate=True),
                         self.changed)

    def test_all_fourteen_paths_and_unchanged_projection_bindings_are_exact(self):
        self.assertEqual(projection.GO_FILES, formatter.GO_FILES)
        self.assertEqual(len(projection.GO_FILES), 14)
        self.assertEqual(projection.INPUT_CAP, formatter.INPUT_CAP)
        self.assertEqual(projection.OUTPUT_CAP, formatter.OUTPUT_CAP)
        self.assertIs(projection.format_record, formatter.format_record)
        self.assertIs(projection.complete_files, formatter.complete_files)
        self.assertIs(projection.pin, formatter.pin)
        self.assertIs(projection.blob, formatter.blob)
        self.assertIs(projection.matches_pin, formatter.matches_pin)

    def test_foreign_missing_or_malformed_path_is_not_projected(self):
        for path in ("../private.go", "/tmp/file.go", "main.go", None,
                     formatter.GO_FILES[5] + "\n"):
            with self.subTest(path=path):
                self.assertIsNone(self.record(path=path))

    def test_exact_input_identity_is_required(self):
        expected = (formatter.blob(self.original),
                    formatter.pin(self.original)["sha256"], len(self.original))
        for altered in (("0" * 40, expected[1], expected[2]),
                        (expected[0], "0" * 64, expected[2]),
                        (expected[0], expected[1], expected[2] + 1),
                        (expected[0], expected[1], True),
                        list(expected), None):
            with self.subTest(kind=type(altered).__name__):
                self.assertIsNone(self.record(expected=altered))
        self.assertIsNone(self.record(original=self.changed))

    def test_malformed_or_unbounded_bytes_never_publish_source(self):
        for field, cap in (("original", formatter.INPUT_CAP),
                           ("output", formatter.OUTPUT_CAP)):
            for data in (None, "package main", b"", b"\xff", b"package\0main",
                         b"x" * (cap + 1)):
                with self.subTest(field=field, kind=type(data).__name__):
                    self.assertIsNone(self.record(**{field: data}))
        self.assertIsNone(self.record(output=self.changed + b"\n" * 301))

    def test_only_existing_token_change_code_can_publish(self):
        for code in ("format-result", "format-file-lines", "formatter-settlement",
                     "unclassified", None, True):
            with self.subTest(code=code):
                self.assertIsNone(self.record(failure_code=code))

    def test_nonzero_or_boolean_exit_never_publish(self):
        for code in (1, -9, None, True, False, "0"):
            phase = dict(exitCode=code, timedOut=False, ownedGroupStopped=True,
                         captureSettled=True, captureFailed=False)
            with self.subTest(code=code):
                self.assertIsNone(self.record(phase=phase))

    def test_timeout_capture_or_group_failure_never_publish(self):
        valid = dict(exitCode=0, timedOut=False, ownedGroupStopped=True,
                     captureSettled=True, captureFailed=False)
        for field, values in (("timedOut", (True, 0, None)),
                              ("ownedGroupStopped", (False, 1, None)),
                              ("captureSettled", (False, 1, None)),
                              ("captureFailed", (True, 0, None))):
            for value in values:
                with self.subTest(field=field, value=value):
                    self.assertIsNone(self.record(phase={**valid, field: value}))
        self.assertIsNone(self.record(phase=None))

    def test_stderr_and_final_drift_never_publish(self):
        for errors in (b"fictional private formatter error", None, ""):
            self.assertIsNone(self.record(stderr=errors))
        for unchanged in (False, 1, None):
            self.assertIsNone(self.record(source_unchanged=unchanged))

    def test_process_private_fields_are_not_exported(self):
        phase = dict(exitCode=0, timedOut=False, ownedGroupStopped=True,
                     captureSettled=True, captureFailed=False,
                     privateDiagnostic="fictional private sentinel")
        record = self.record(phase=phase)
        self.assertNotIn("privateDiagnostic", record)
        self.assertNotIn("fictional private sentinel", str(record))

    def test_accepted_source_never_becomes_unadmitted_diagnostic(self):
        self.assertIsNone(self.record(output=self.original))

    def test_diagnostic_never_counts_as_an_accepted_file(self):
        records = [{"path": path, "tokensPreserved": True}
                   for path in formatter.GO_FILES[:5]]
        diagnostic = self.record()
        self.assertFalse(formatter.complete_files(records))
        self.assertFalse(formatter.complete_files(records + [diagnostic]))
        self.assertEqual(len(records), 5)
        self.assertEqual([row["path"] for row in records], list(formatter.GO_FILES[:5]))
        self.assertEqual(diagnostic["path"], formatter.GO_FILES[5])
