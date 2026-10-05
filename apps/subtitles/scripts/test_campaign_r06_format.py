"""Protect bounded hosted formatting projection; no formatter or files execute."""
import base64
import hashlib
import campaign_r06_format
import unittest

from campaign_r06_format import GO_FILES, format_record

class R06FormatAdmissionTests(unittest.TestCase):
    source = b"package main\nfunc main(){}\n"
    formatted = b"package main\n\nfunc main() {}\n"

    def call(self, **changes):
        values = {"path": GO_FILES[0], "original": self.source,
                  "output": self.formatted, "exit_code": 0, "stderr": b""}
        values.update(changes)
        return format_record(**values)

    def test_binds_original_and_formatted_bytes_without_execution(self):
        record = self.call()
        self.assertEqual(record["original"]["sha256"], hashlib.sha256(self.source).hexdigest())
        self.assertEqual(record["formatted"]["sha256"], hashlib.sha256(self.formatted).hexdigest())
        self.assertEqual(base64.b64decode(record["formattedSourceBase64"], validate=True), self.formatted)
        self.assertEqual(record["path"], GO_FILES[0])

    def test_unknown_path_is_never_exported(self):
        for path in ("../private.go", "/tmp/private.go", "main.go", GO_FILES[0]+"\n"):
            with self.subTest(path=path), self.assertRaises(ValueError):
                self.call(path=path)

    def test_only_regular_bounded_byte_inputs_are_admitted(self):
        for original in ("package main", None, b"", b"x"*(256*1024+1)):
            with self.subTest(kind=type(original).__name__), self.assertRaises(ValueError):
                self.call(original=original)

    def test_output_size_and_utf8_are_bounded(self):
        for output in (None, "package main", b"", b"x"*(512*1024+1), b"\xff", b"package\0main"):
            with self.subTest(kind=type(output).__name__), self.assertRaises(ValueError):
                self.call(output=output)

    def test_boolean_or_nonzero_exit_never_counts_as_formatted(self):
        for code in (True, False, None, "0", 1, -9):
            with self.subTest(code=code), self.assertRaises(ValueError):
                self.call(exit_code=code)

    def test_stderr_is_not_included_or_ignored(self):
        for stderr in (b"private fixture diagnostic", None, ""):
            with self.subTest(kind=type(stderr).__name__), self.assertRaises(ValueError):
                self.call(stderr=stderr)


    def test_only_reviewed_save_production_go_paths_extend_fixture_list(self):
        prefix = "apps/subtitles/engineering/qa/2026-10-05-save-browser/fixture/"
        self.assertEqual(GO_FILES, tuple(prefix+name for name in
            (
                     "main_test.go", "transport_test.go", "witness_test.go", "fixture_test.go", "owner_test.go",
                     "filesystem_test.go", "http_test.go", "routing_test.go", "transport_hold_test.go",
                     "witness_effects_test.go", "owner_enrollment_test.go", "control_requests_test.go",
                     "control_owned_test.go", "control_faults_test.go",
                 )) + (
            "apps/subtitles/internal/server/assets.go",
            "apps/subtitles/internal/server/subtitle_inspector.go",
        ))
        self.assertEqual(getattr(campaign_r06_format, "FORMAT_FILE_LIMIT", None), len(GO_FILES))
        for path in GO_FILES[-2:]:
            with self.subTest(path=path):
                self.assertEqual(self.call(path=path)["path"], path)
        for path in ("apps/subtitles/internal/server/server.go",
                     "apps/subtitles/internal/server/auth.go"):
            with self.subTest(path=path), self.assertRaises(ValueError):
                self.call(path=path)

    def test_formatted_file_cannot_exceed_existing_300_line_limit(self):
        with self.assertRaisesRegex(ValueError, "^format-file-lines$"):
            self.call(output=b"package main\n" + b"\n"*300)

if __name__ == "__main__":
    unittest.main()
