#!/usr/bin/env python3
import importlib.util
import sys
sys.dont_write_bytecode = True
from pathlib import Path
import tempfile
import unittest
from unittest import mock


MODULE_PATH = Path(__file__).with_name("check-halstead.py")
SPEC = importlib.util.spec_from_file_location("check_halstead", MODULE_PATH)
CHECK_HALSTEAD = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(CHECK_HALSTEAD)


class CheckHalsteadTest(unittest.TestCase):
    def test_source_files_ignores_deleted_and_nonproduction_files(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            repo = Path(directory)
            (repo / "existing.go").write_text("package example\n")
            with mock.patch.object(
                CHECK_HALSTEAD.subprocess,
                "check_output",
                return_value="existing.go\ndeleted.go\nvalue_test.go\npath/third_party/copy.go\n",
            ):
                self.assertEqual(CHECK_HALSTEAD.source_files(repo), ["existing.go"])

    def test_accepts_report_without_functions(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            with (
                mock.patch.object(CHECK_HALSTEAD, "source_files", return_value=["empty.go"]),
                mock.patch.object(CHECK_HALSTEAD, "report", return_value={"empty.go": {"functions": None}}),
                mock.patch.object(CHECK_HALSTEAD.sys, "argv", ["check-halstead.py", "tool", directory]),
            ):
                self.assertEqual(CHECK_HALSTEAD.main(), 0)

    def test_groups_only_same_package_and_target(self) -> None:
        paths = ["pkg/a.go", "pkg/b.go", "pkg/os_linux.go", "other/a.go"]
        calls = []
        def report(_tool, _repo, group):
            calls.append(group)
            return {path: {"functions": []} for path in group}
        with (
            tempfile.TemporaryDirectory() as directory,
            mock.patch.object(CHECK_HALSTEAD, "source_files", return_value=paths),
            mock.patch.object(CHECK_HALSTEAD, "report", side_effect=report),
            mock.patch.object(CHECK_HALSTEAD.sys, "argv", ["check-halstead.py", "tool", directory]),
        ):
            self.assertEqual(CHECK_HALSTEAD.main(), 0)
        self.assertCountEqual(calls, [["pkg/a.go", "pkg/b.go"], ["pkg/os_linux.go"], ["other/a.go"]])

    def test_failure_in_any_file_fails_batch(self) -> None:
        with (
            tempfile.TemporaryDirectory() as directory,
            mock.patch.object(CHECK_HALSTEAD, "source_files", return_value=["a.go", "b.go"]),
            mock.patch.object(CHECK_HALSTEAD, "report", return_value={
                "a.go": {"functions": []},
                "b.go": {"functions": [{"name": "bad", "start": {"line": 2}, "metrics": {"difficulty": 80}}]},
            }),
            mock.patch.object(CHECK_HALSTEAD.sys, "argv", ["check-halstead.py", "tool", directory]),
            mock.patch.object(CHECK_HALSTEAD.sys, "stderr"),
        ):
            self.assertEqual(CHECK_HALSTEAD.main(), 1)


if __name__ == "__main__":
    unittest.main()
