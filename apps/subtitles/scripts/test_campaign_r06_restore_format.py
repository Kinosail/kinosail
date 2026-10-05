"""Failure-first controls for static Restore formatting; no tool or app runs."""
import base64
import hashlib
import unittest
from unittest.mock import patch
import signal
import subprocess

import campaign_r06_restore_format as formatter
from campaign_r06_restore_tokens import equivalent


class RestoreFormatControls(unittest.TestCase):
    original = b'package main\nimport ("net/http"; "bytes")\nfunc f(){ return }\n'
    canonical = b'package main\n\nimport (\n "bytes"\n "net/http"\n)\n\nfunc f() { return }\n'

    def record(self, **changes):
        values = dict(path=formatter.GO_FILES[0], original=self.original,
                      output=self.canonical, exit_code=0, stderr=b"")
        values.update(changes)
        return formatter.format_record(**values)

    def test_exact_fourteen_fixture_paths_only(self):
        prefix = "apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture/"
        names = ("restore_assertions_test.go", "restore_controls_test.go",
                 "restore_exchange_test.go", "restore_filesystem_test.go",
                 "restore_http_test.go", "restore_main_test.go",
                 "restore_owner_enrollment_test.go", "restore_owner_test.go",
                 "restore_requests_test.go", "restore_routes_test.go",
                 "restore_routing_test.go", "restore_target_test.go",
                 "restore_transport_test.go", "restore_witness_test.go")
        self.assertEqual(formatter.GO_FILES, tuple(prefix + name for name in names))
        self.assertEqual(len(formatter.GO_PINS), 14)
        self.assertEqual(set(formatter.GO_PINS), set(formatter.GO_FILES))

    def test_pins_original_and_canonical_without_runtime(self):
        value = self.record()
        self.assertEqual(value["original"]["sha256"], hashlib.sha256(self.original).hexdigest())
        self.assertEqual(value["formatted"]["sha256"], hashlib.sha256(self.canonical).hexdigest())
        self.assertEqual(base64.b64decode(value["formattedSourceBase64"], validate=True), self.canonical)
        self.assertTrue(value["tokensPreserved"])

    def test_foreign_path_fails_closed(self):
        for path in ("../secret.go", "/tmp/file.go", "main.go",
                     formatter.GO_FILES[0] + "\n", "apps/subtitles/internal/server/assets.go"):
            with self.subTest(path=path), self.assertRaises(ValueError):
                self.record(path=path)

    def test_input_shape_and_encoding_fail_closed(self):
        for source in (None, "package main", b"", b"\xff", b"package\0main",
                       b"x" * (formatter.INPUT_CAP + 1)):
            with self.subTest(kind=type(source).__name__), self.assertRaises(ValueError):
                self.record(original=source)

    def test_output_shape_and_encoding_fail_closed(self):
        for output in (None, "package main", b"", b"\xff", b"package\0main",
                       b"x" * (formatter.OUTPUT_CAP + 1)):
            with self.subTest(kind=type(output).__name__), self.assertRaises(ValueError):
                self.record(output=output)

    def test_failed_boolean_exit_and_stderr_are_rejected(self):
        for code in (True, False, None, "0", 1, -9):
            with self.subTest(code=code), self.assertRaises(ValueError):
                self.record(exit_code=code)
        for errors in (b"private diagnostic", None, ""):
            with self.subTest(kind=type(errors).__name__), self.assertRaises(ValueError):
                self.record(stderr=errors)

    def test_overflow_is_incomplete_not_adopted(self):
        with self.assertRaisesRegex(ValueError, "^format-file-lines$"):
            self.record(output=self.canonical + b"\n" * 301)

    def test_literals_comments_identifiers_and_order_are_exact(self):
        source = b'package main\n// Keep this bound.\nconst n=0o600\nconst s="safe"\nfunc a(){}\nfunc b(){}\n'
        changes = (
            source.replace(b"0o600", b"0o700"), source.replace(b"0o600", b"0600"),
            source.replace(b'"safe"', b'"other"'), source.replace(b"this bound", b"new bound"),
            source.replace(b"func a(){}\nfunc b(){}", b"func b(){}\nfunc a(){}"),
            source.replace(b"func a", b"func c"),
        )
        for changed in changes:
            with self.subTest(change=hashlib.sha256(changed).hexdigest()):
                self.assertFalse(equivalent(source, changed))

    def test_import_binding_and_membership_cannot_change(self):
        for changed in (
            self.canonical.replace(b'"bytes"', b'b "bytes"'),
            self.canonical.replace(b'"bytes"', b'"strings"'),
            self.canonical.replace(b' "net/http"\n', b""),
        ):
            with self.subTest(change=hashlib.sha256(changed).hexdigest()):
                self.assertFalse(equivalent(self.original, changed))

    def test_semicolon_insertion_preserves_go_statement_boundaries(self):
        self.assertTrue(equivalent(b"package main; func f(){x:=1; x++; return}",
                                   b"package main\nfunc f() {\nx := 1\nx++\nreturn\n}\n"))
        self.assertFalse(equivalent(b"package main\nfunc f(){return 1}\n",
                                    b"package main\nfunc f(){return\n1}\n"))
        self.assertFalse(equivalent(b"package main\nfunc f(){for ;; {}}\n",
                                    b"package main\nfunc f(){for ; {}}\n"))

    def test_only_recognized_trailing_list_commas_are_optional(self):
        self.assertTrue(equivalent(b"package main\nvar x=[]string{\"a\", \"b\"}\n",
                                   b"package main\nvar x=[]string{\n\"a\", \"b\",\n}\n"))
        self.assertTrue(equivalent(b"package main\nfunc f(){g(1,2)}\n",
                                   b"package main\nfunc f(){g(\n1,2,\n)}\n"))
        self.assertFalse(equivalent(b"package main\nfunc f(){return}\n",
                                    b"package main\nfunc f(){return,}\n"))
        self.assertFalse(equivalent(b"package main\nvar x=(1)\n",
                                    b"package main\nvar x=(1,)\n"))

    def test_unrecognized_or_malformed_lexical_input_fails_closed(self):
        for source in (b"package main\nvar x=\"unfinished", b"package main\n/* unfinished",
                       b"package main\nfunc f(){]", b"package main\nvar \xc3\xa9=1\n"):
            with self.subTest(kind=hashlib.sha256(source).hexdigest()):
                self.assertFalse(equivalent(source, source))

    def test_comments_and_raw_string_contents_cannot_be_relocated_or_changed(self):
        source = b"package main\n// before a\nfunc a(){}\nfunc b(){}\nvar s=`a\nb`\n"
        for changed in (source.replace(b"// before a\n", b"").replace(b"func b", b"// before a\nfunc b"),
                        source.replace(b"`a\nb`", b"`a b`")):
            self.assertFalse(equivalent(source, changed))

    def test_group_timeout_and_unsettled_capture_never_admit_output(self):
        self.assertTrue(formatter.phase_accepted(0, False, True, True))
        for args in ((0, True, True, True), (0, False, False, True),
                     (0, False, True, False), (-9, False, True, True),
                     (True, False, True, True), (0, 0, True, True)):
            with self.subTest(args=args):
                self.assertFalse(formatter.phase_accepted(*args))

    def test_natural_formatter_eof_keeps_zero_exit_and_proves_group_absence(self):
        process = FakeFormatterProcess([(self.canonical, b"")], 0)
        with patch.object(formatter.subprocess, "Popen", return_value=process), \
                patch.object(formatter.os, "killpg", side_effect=ProcessLookupError):
            output, errors, receipt = formatter.format_owned("/owned/tool", self.original)
        self.assertEqual(output, self.canonical)
        self.assertEqual(errors, b"")
        self.assertEqual(receipt["exitCode"], 0)
        self.assertTrue(receipt["ownedGroupStopped"])
        self.assertTrue(receipt["captureSettled"])

    def test_capture_error_still_settles_group_and_never_admits_output(self):
        process = FakeFormatterProcess([OSError("fictional private capture failure"),
                                        (self.canonical, b"")], 0)
        signals = []
        def kill_group(pid, signum):
            signals.append(signum)
            if signum == 0:
                raise ProcessLookupError
        with patch.object(formatter.subprocess, "Popen", return_value=process), \
                patch.object(formatter.os, "killpg", side_effect=kill_group):
            _, _, receipt = formatter.format_owned("/owned/tool", self.original)
        self.assertIn(signal.SIGKILL, signals)
        self.assertTrue(receipt["captureFailed"])
        self.assertFalse(formatter.phase_accepted(receipt["exitCode"], receipt["timedOut"],
            receipt["ownedGroupStopped"], receipt["captureSettled"], receipt["captureFailed"]))

    def test_timeout_signals_group_even_when_leader_already_exited(self):
        process = FakeFormatterProcess([subprocess.TimeoutExpired("fictional", 10),
                                        (self.canonical, b"")], 0)
        signals = []
        def kill_group(pid, signum):
            self.assertEqual(pid, 4321)
            signals.append(signum)
            if signum == 0:
                raise ProcessLookupError
        with patch.object(formatter.subprocess, "Popen", return_value=process), \
                patch.object(formatter.os, "killpg", side_effect=kill_group):
            _, _, receipt = formatter.format_owned("/owned/tool", self.original)
        self.assertIn(signal.SIGKILL, signals)
        self.assertTrue(receipt["timedOut"])
        self.assertFalse(formatter.phase_accepted(receipt["exitCode"], receipt["timedOut"],
                                                  receipt["ownedGroupStopped"], receipt["captureSettled"]))

    def test_git_admission_failure_codes_are_exact_and_private(self):
        commands = (
            (("git", "rev-parse", "HEAD"), "git-head-read-failed"),
            (("git", "rev-parse", "HEAD^{tree}"), "git-tree-read-failed"),
            (("git", "diff", "HEAD", "--name-only", "-z"), "git-tracked-diff-failed"),
            (("git", "status", "--porcelain"), "git-worktree-status-failed"),
            (("git", "ls-files", "--stage", "-z"), "git-source-index-failed"),
            (("git", "cat-file", "-e", formatter.BASE_COMMIT + "^{commit}"), "git-base-object-unavailable"),
            (("git", "merge-base", "--is-ancestor", formatter.BASE_COMMIT, "HEAD"), "git-base-ancestry-failed"),
        )
        for command, code in commands:
            with self.subTest(code=code):
                error = subprocess.CalledProcessError(128, list(command),
                    output=b"fictional private output", stderr=b"fictional private target")
                self.assertEqual(formatter.failure_code(error), code)
        for command in ("git show private", ["git", "show", "private"],
                        ["git", ["malformed"]], ("git", "merge-base", "--is-ancestor", "foreign", "HEAD")):
            self.assertEqual(formatter.failure_code(subprocess.CalledProcessError(1, command)), "unclassified")

    def test_missing_base_blocks_before_unchanged_ancestry_check(self):
        calls = []
        def missing(*arguments):
            calls.append(arguments)
            raise subprocess.CalledProcessError(128, ["git", *arguments])
        with patch.object(formatter, "git", side_effect=missing), self.assertRaises(subprocess.CalledProcessError) as raised:
            formatter.require_base_ancestry()
        self.assertEqual(calls, [("cat-file", "-e", formatter.BASE_COMMIT + "^{commit}")])
        self.assertEqual(formatter.failure_code(raised.exception), "git-base-object-unavailable")
        with patch.object(formatter, "git", return_value=b"") as valid:
            formatter.require_base_ancestry()
        self.assertEqual(valid.call_args_list, [
            unittest.mock.call("cat-file", "-e", formatter.BASE_COMMIT + "^{commit}"),
            unittest.mock.call("merge-base", "--is-ancestor", formatter.BASE_COMMIT, "HEAD")])

    def test_manifest_base_and_raw_identity_are_fixed(self):
        self.assertEqual(formatter.BASE_COMMIT, "bd1ea2e148787ae8d4a0a46640bc2a965e10fe6a")
        self.assertEqual(formatter.MANIFEST_BLOB, "8a2b3396e1c372b6b5732eb034f4ae0da8016760")
        self.assertEqual(formatter.MANIFEST_SHA256,
                         "776916da74d6a165ab157f975b85e668c917552572dfec3a91ec9a7869321aa3")
        expected = formatter.pin(b"source")
        self.assertTrue(formatter.matches_pin(b"source", expected))
        self.assertFalse(formatter.matches_pin(b"changed", expected))

    def test_missing_duplicate_or_unvalidated_output_is_incomplete(self):
        records = [{"path": path, "tokensPreserved": True} for path in formatter.GO_FILES]
        self.assertTrue(formatter.complete_files(records))
        for incomplete in (records[:-1], records + [records[0]],
                           [{**records[0], "tokensPreserved": False}, *records[1:]],
                           [{**records[0], "tokensPreserved": 1}, *records[1:]],
                           [{**records[0], "path": "foreign.go"}, *records[1:]]):
            self.assertFalse(formatter.complete_files(incomplete))

    def test_stock_formatter_has_only_stdin_existing_app_configuration(self):
        self.assertEqual(formatter.formatter_command("/owned/tool"),
                         ["/owned/tool", "fmt", "--stdin", "--config", "apps/subtitles/.golangci.yml"])
        self.assertEqual(formatter.PER_FILE_SECONDS, 10)
        self.assertEqual(formatter.SHUTDOWN_SECONDS, 2)


class FakeFormatterProcess:
    pid = 4321

    def __init__(self, replies, returncode):
        self.replies = list(replies)
        self.returncode = returncode

    def communicate(self, input=None, timeout=None):
        reply = self.replies.pop(0)
        if isinstance(reply, Exception):
            raise reply
        return reply

    def poll(self):
        return self.returncode


if __name__ == "__main__":
    unittest.main()
