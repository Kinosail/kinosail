#!/usr/bin/env python3
"""Pure reporter-phase controls: mocked IO/processes and exact tracked driver loader."""
from contextlib import ExitStack, contextmanager
import importlib.util
from pathlib import Path
import unittest
from unittest.mock import patch

PATH = Path(__file__).with_name("campaign-q47-public.py")
SPEC = importlib.util.spec_from_file_location("q47_reporter_control_driver", PATH)
if SPEC is None or SPEC.loader is None:
    raise ImportError("fixed_q47_driver_loader")
driver = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(driver)

NODE = Path("/fictional/pinned/node")
COMMAND = [str(NODE), "--test", "apps/player/e2e/compose-template-proof-attachments.controls.cjs"]
ENV = {"CI": "1", "FICTIONAL": "control-only"}


def terminal(exit_code=0):
    return dict(launched=True, exitCode=exit_code, durationMs=1, stopReason=None, leaderExited=True,
                groupAbsent=True, captureComplete=True, captureBytes=0, captureLimit=driver.processes.CAPTURE_LIMIT)


@contextmanager
def main_fixture(preflight_failure=False, control_success=False):
    events, outputs = [], []
    temporary = Path("/fictional/temporary/q47")
    tools = {"node": NODE, "go": Path("/fictional/pinned/go")}
    def preflight(_tools):
        events.append("preflight")
        if preflight_failure:
            raise ValueError("fictional_preflight")
    def execute(command, bound, cwd, env, hook=None, label=None):
        events.append(label)
        if label == "reporter-controls":
            if command != COMMAND or bound != 10 or cwd != driver.ROOT or env != ENV or hook is not None:
                raise AssertionError("fixed_control_contract")
            return terminal(0 if control_success else 1), b"fictional-private-canary"
        return terminal(), b""
    def browser(name, *args):
        events.append(name)
        return {"classification": "collection", "report": None}
    class Peer:
        def __init__(self, *args):
            pass
        def start(self):
            events.append("peer-start")
            return False
        def stop(self):
            return {"ready": False, "stopped": True, "requestedStop": True, "exitCode": 0,
                    "leaderExited": True, "groupAbsent": True, "captureComplete": True, "interrupted": False}
    with ExitStack() as stack:
        for target, name, value in (
                (driver, "output_boundary", lambda: None),
                (driver, "source_snapshot", lambda: {"fixture": True}),
                (driver, "temporary_boundary", lambda: temporary),
                (driver, "dependencies", lambda: (tools, Path("/fictional/cli"), {"inventory": True})),
                (driver, "preflight", preflight), (driver, "execute", execute),
                (driver, "environment", lambda: ENV),
                (driver, "generated", lambda _site: ({"built": True}, {"pins": True})),
                (driver, "fingerprint", lambda *args: {"binary": True}),
                (driver, "browser_phase", browser), (driver, "Peer", Peer),
                (driver, "artifacts", lambda *args: outputs.append(args)),
                (driver, "selector_valid", lambda *args: True),
                (driver.signal, "getsignal", lambda _number: None),
                (driver.signal, "signal", lambda *args: None),
                (driver.time, "monotonic", lambda: 1),
                (driver.processes, "PROCESS_RECORDS", []),
                (driver.processes, "PROOF_DEADLINE", None)):
            stack.enter_context(patch.object(target, name, value))
        yield events, outputs


class ReporterPhaseControls(unittest.TestCase):
    def test_exact_node_command_uses_pinned_tool_cwd_env_bound_and_zero_exit(self):
        calls = []
        def execute(command, bound, cwd, env, hook=None, label=None):
            calls.append((command, bound, cwd, env, hook, label))
            return terminal(), b"fictional-private-canary"
        with patch.object(driver, "execute", execute), patch.object(driver, "environment", lambda: ENV):
            driver.reporter_controls({"node": NODE})
        self.assertEqual(calls, [(COMMAND, 10, driver.ROOT, ENV, None, "reporter-controls")])

    def test_every_incomplete_process_record_blocks_controls(self):
        mutations = [
            ("exitCode", 1), ("exitCode", None), ("exitCode", False), ("launched", False),
            ("leaderExited", False), ("groupAbsent", False), ("captureComplete", False),
            ("captureBytes", driver.processes.CAPTURE_LIMIT + 1), ("captureLimit", 1),
            ("stopReason", "timeout"), ("stopReason", "interrupted"), ("durationMs", float("nan")),
        ]
        for key, value in mutations:
            with self.subTest(field=key, value=value):
                record = terminal(); record[key] = value
                with patch.object(driver, "execute", lambda *args, **kwargs: (record, b"private-canary")):
                    with self.assertRaises(ValueError):
                        driver.reporter_controls({"node": NODE})

    def test_control_failure_records_fixed_phase_before_any_build_or_primary(self):
        with main_fixture() as (events, outputs):
            self.assertEqual(driver.main(), 2)
        self.assertEqual(events, ["preflight", "reporter-controls"])
        receipt, results, sources = outputs[0]
        self.assertEqual(receipt["blockedPhase"], "reporter-controls")
        self.assertEqual(receipt["result"], "incomplete")
        self.assertIsNone(receipt["dependencyStage"])
        self.assertEqual(results["attemptedRuns"], 0)
        self.assertIsNone(results["collection"])
        self.assertEqual(results["runs"], [])
        self.assertIsNone(sources["generated"])
        self.assertIsNone(sources["binary"])
        self.assertNotIn("fictional-private-canary", repr(outputs))

    def test_preflight_failure_cannot_start_reporter_controls(self):
        with main_fixture(preflight_failure=True) as (events, outputs):
            self.assertEqual(driver.main(), 2)
        self.assertEqual(events, ["preflight"])
        self.assertEqual(outputs[0][0]["blockedPhase"], "dependency-check")
        self.assertEqual(outputs[0][1]["attemptedRuns"], 0)

    def test_successful_controls_precede_docs_compile_collection_and_primary(self):
        with main_fixture(control_success=True) as (events, outputs):
            self.assertEqual(driver.main(), 2)
        self.assertEqual(events, ["preflight", "reporter-controls", "docs-build",
                                  "fixture-compile", "collection", "peer-start"])
        self.assertEqual(outputs[0][0]["blockedPhase"], "primary-1")
        self.assertEqual(outputs[0][1]["attemptedRuns"], 0)

    def test_closed_bounds_and_command_description_preserve_existing_contracts(self):
        self.assertEqual(driver.BOUNDS, {"dependency-check": 10, "reporter-controls": 10,
                         "docs-build": 120, "fixture-compile": 90, "collection": 15,
                         "primary-1": 75, "primary-2": 75})
        self.assertEqual(driver.COMMANDS, {
            "reporter-controls": "node --test apps/player/e2e/compose-template-proof-attachments.controls.cjs",
            "docs-build": "python3 engineering/documentation/build.py --output <new RUNNER_TEMP site>",
            "fixture-compile": "go build -p 1 -tags q47proof -o <new private binary> <declared fixture files>",
            "collection": "node <installed pinned Playwright CLI> test --config compose-template-recovery.config.ts --list --workers=1 --retries=0",
            "primary": "node <installed pinned Playwright CLI> test --config compose-template-recovery.config.ts --workers=1 --retries=0"})
        self.assertEqual(driver.LIMITS["admissionBudgetSeconds"], 540)
        self.assertEqual(driver.LIMITS["commandCleanupSeconds"], 7)

    def test_missing_pinned_node_never_falls_back_to_global_resolution(self):
        with patch.object(driver, "execute", side_effect=AssertionError("must_not_launch")):
            with self.assertRaises(KeyError):
                driver.reporter_controls({})


if __name__ == "__main__":
    unittest.main()
