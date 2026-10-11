#!/usr/bin/env python3
"""Pure Q47 preflight controls; mocked commands never launch tools or read files."""
from contextlib import ExitStack
from pathlib import PurePosixPath
import unittest
from unittest import mock
import importlib.util
from pathlib import Path

PATH = Path(__file__).with_name("campaign-q47-public.py")
SPEC = importlib.util.spec_from_file_location("q47_preflight_driver", PATH)
if SPEC is None or SPEC.loader is None:
    raise ImportError("q47_driver_loader")
driver = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(driver)

TOOLS = {name: PurePosixPath("/fictional/bin") / name for name in ("go", "node", "ruby", "bundle")}
OUTPUTS = {"go": b"go version go1.27.0 linux/amd64\n", "node": b"v26.0.0\n",
           "ruby": b"ruby 3.4.0 (fictional)\n"}
PREFIXED = b"Bundler version 4.0.22\n"
PLAIN = b"4.0.22\n"


def terminal(output):
    return {"launched": True, "exitCode": 0, "durationMs": 1, "stopReason": None,
            "leaderExited": True, "groupAbsent": True, "captureComplete": True,
            "captureBytes": len(output), "captureLimit": 16 * 1024 * 1024}


def commands(bundle_output=PREFIXED, changes=None, probe="bundle", check_changes=None, other=None):
    context = ExitStack()
    def execute(argv, timeout, cwd, env, label):
        name = PurePosixPath(argv[0]).name
        output = b"" if argv[1:] == ["check"] else bundle_output if name == "bundle" else OUTPUTS[name]
        if other and name in other: output = other[name]
        result = terminal(output)
        if changes and name == probe and argv[1:] == ["--version"]: result.update(changes)
        if check_changes and argv[1:] == ["check"]: result.update(check_changes)
        return result, output
    calls = context.enter_context(mock.patch.object(driver, "execute", side_effect=execute))
    context.enter_context(mock.patch.object(driver, "environment", return_value={"CI": "1"}))
    return context, calls


class PreflightControls(unittest.TestCase):
    def test_plain_pinned_bundler_output_reaches_unchanged_locked_check(self):
        for output in (PLAIN, b"4.0.22", b"4.0.22\r\n"):
            with self.subTest(form=len(output)):
                context, calls = commands(output)
                with context: driver.preflight(TOOLS)
                self.assertEqual(calls.call_count, 5)
                self.assertEqual(calls.call_args_list[-1].args[0], [str(TOOLS["bundle"]), "check"])

    def test_existing_prefixed_output_and_probe_contracts_are_preserved(self):
        context, calls = commands()
        with context: driver.preflight(TOOLS)
        expected = [[str(TOOLS[name]), "version" if name == "go" else "--version"]
                    for name in ("go", "node", "ruby", "bundle")] + [[str(TOOLS["bundle"]), "check"]]
        self.assertEqual([call.args[0] for call in calls.call_args_list], expected)
        for call in calls.call_args_list:
            self.assertEqual(call.args[1], 10)
            self.assertEqual(call.kwargs, {"label": "dependency-check"})
        self.assertTrue(all(call.args[2] == driver.ROOT for call in calls.call_args_list[:4]))
        check = calls.call_args_list[-1]
        self.assertEqual(check.args[2], driver.ROOT / "engineering/documentation")
        self.assertEqual(check.args[3], {"CI": "1", "BUNDLE_GEMFILE":
                                       str(driver.ROOT / "engineering/documentation/Gemfile")})

    def test_wrong_versions_extra_text_and_simulation_are_rejected(self):
        values = (b"4.0.15\n", b"4.0.17\n", b"4.0.220\n", b"4.0.22.pre\n", b"v4.0.22\n",
                  b"Bundler version 4.0.15\n", b"bundler version 4.0.22\n",
                  b"warning\n4.0.22\n", b"4.0.22\nextra\n", b"4.0.22 (simulating Bundler 5)\n",
                  b"4.0.22 (build data)\n", b"Bundler version 4.0.22 extra\n", b"",
                  b"\x1b[32m4.0.22\x1b[0m\n", b"4x0x22\n", b"4.0.22\x00\n")
        for output in values:
            with self.subTest(form=len(output)):
                context, calls = commands(output)
                with context, self.assertRaises(ValueError): driver.preflight(TOOLS)
                self.assertEqual(calls.call_count, 4)

    def test_ascii_and_512_byte_cap_remain_exact(self):
        output = b" " * (512 - len(PREFIXED)) + PREFIXED
        context, calls = commands(output)
        with context: driver.preflight(TOOLS)
        self.assertEqual(len(output), 512)
        self.assertEqual(calls.call_count, 5)
        for output in (b" " + output, b"\xff" + PREFIXED):
            context, calls = commands(output)
            with context, self.assertRaises((ValueError, UnicodeError)): driver.preflight(TOOLS)
            self.assertEqual(calls.call_count, 4)

    def test_every_completed_process_prerequisite_is_mandatory(self):
        changes = ({"launched": False}, {"exitCode": 1}, {"leaderExited": False},
                   {"groupAbsent": False}, {"groupAbsent": None}, {"captureComplete": False},
                   {"stopReason": "interrupted"}, {"stopReason": "timeout"},
                   {"captureLimit": 512}, {"captureBytes": -1})
        for change in changes:
            with self.subTest(field=next(iter(change))):
                context, calls = commands(changes=change)
                with context, self.assertRaises(ValueError): driver.preflight(TOOLS)
                self.assertEqual(calls.call_count, 4)

    def test_locked_bundle_check_failure_still_blocks_preflight(self):
        for change in ({"exitCode": 1}, {"groupAbsent": False}, {"stopReason": "interrupted"}):
            context, calls = commands(check_changes=change)
            with context, self.assertRaises(ValueError): driver.preflight(TOOLS)
            self.assertEqual(calls.call_count, 5)

    def test_other_tool_pins_still_fail_before_bundler_probe(self):
        for name, output, count in (("go", b"go version go1.26.0 linux/amd64\n", 1),
                                    ("node", b"v25.0.0\n", 2), ("ruby", b"ruby 3.3.0\n", 3)):
            context, calls = commands(other={name: output})
            with context, self.assertRaises(ValueError): driver.preflight(TOOLS)
            self.assertEqual(calls.call_count, count)


if __name__ == "__main__":
    unittest.main()
