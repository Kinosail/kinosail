"""Home readiness privacy controls: native E2E cannot inject malformed dumps.

Missing, ambiguous, truncated, or per-display observations must not imply global
readiness. Raw component names must never escape into the uploaded receipt.
"""
import importlib.util
import json
from pathlib import Path
import unittest

PATH = Path(__file__).resolve().parents[2] / "apps/player/scripts/wear_home_readiness.py"
SPEC = importlib.util.spec_from_file_location("wear_home_readiness", PATH)
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class WearHomeReadinessTests(unittest.TestCase):
    def test_home_readiness_requires_one_current_follow_on_in_home_package(self):
        # Historical events and per-display records cannot establish the global
        # current Activity. Malformed snapshots cannot be injected in native E2E.
        home = b"Activity: private.home/.Entry\n"
        entry = b"  ResumedActivity: ActivityRecord{abc123 u0 private.home/.Entry t12}\n"
        ready = b"  ResumedActivity: ActivityRecord{abc123 u0 private.home/.FollowOn t12}\n"
        for snapshot, expected in (
            (ready, True), (entry, False), (b"", False),
            (ready.replace(b"private.home/", b"private.other/"), False),
            (ready.replace(b"ResumedActivity:", b"Resumed:"), False),
            (ready + ready, False), (ready.replace(b"abc123", b"invalid!"), False),
            (b"private-secret\n" * 100_000 + ready, False),
        ):
            with self.subTest(expected=expected, size=len(snapshot)):
                state = MODULE.home_state(snapshot, home)
                self.assertEqual(state["followOnResumed"], expected)
                self.assertNotIn("private", json.dumps(state))
                self.assertNotIn("FollowOn", json.dumps(state))
        self.assertFalse(MODULE.home_state(ready, b"Activity: invalid!\n")["followOnResumed"])


if __name__ == "__main__":
    unittest.main()
