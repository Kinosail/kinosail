"""Isolated privacy contract for the temporary Wear event projection.

The hosted journey cannot safely inject secret-bearing or malformed system logs.
Failures to prevent: raw components, IDs, URLs, or reasons escape into artifacts;
malformed records invent transitions; bounded input or event loss goes unreported.
These checks protect the uploaded projection, not Android lifecycle behavior.
"""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

PATH = Path(__file__).resolve().parents[2] / "apps/player/scripts/wear_event_projection.py"
SPEC = importlib.util.spec_from_file_location("wear_event_projection", PATH)
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class WearEventProjectionTests(unittest.TestCase):
    def test_projection_preserves_transition_order_without_private_fields(self):
        home = b"Status: ok\nActivity: private.launcher/.Home\nComplete\n"
        data = b"\n".join((
            b"I/wm_resume_activity( 523): [0,12345,678,com.kinosail.player.dev/com.kinosail.player.wear.WearActivity]",
            b"I/wm_pause_activity( 523): [0,12345,com.kinosail.player.dev/com.kinosail.player.wear.WearActivity,userLeaving=false,resumeTopActivity]",
            b"I/wm_set_resumed_activity( 523): [0,private.launcher/private.launcher.Home,private-secret-reason]",
            b"I/wm_stop_activity( 523): [0,12345,com.kinosail.player.dev/com.kinosail.player.wear.WearActivity]",
            b"I/wm_create_activity( 523): [0,23456,789,com.kinosail.player.dev/androidx.activity.ComponentActivity,private-action,private-mime,https://private.invalid/token,123]",
        ))
        result = MODULE.project(data, home)
        self.assertEqual(result["events"], [
            {"event": "resume", "role": "wear"},
            {"event": "pause", "role": "wear", "reason": "resumeTopActivity", "userLeaving": False},
            {"event": "set-resumed", "role": "home", "reason": "other"},
            {"event": "stop", "role": "wear"},
            {"event": "create", "role": "fixture"},
        ])
        self.assertFalse(result["eventsTruncated"])
        self.assertEqual(result["malformedEvents"], 0)
        for secret in ("private", "12345", "23456", "https://", "523", "789"):
            self.assertNotIn(secret, json.dumps(result))

    def test_malformed_unknown_and_oversized_logs_do_not_leak_or_invent_events(self):
        data = b"\n".join((
            b"I/wm_pause_activity( 1): [0,1,private.Component,true]",
            b"I/wm_stop_activity( 1): [0,1,private.Component]",
            b"I/wm_pause_activity( 1): [0,1,other.app/.Private,userLeaving=secret,secret-reason]",
            b"I/unrelated( 1): secret=private-token",
        ))
        result = MODULE.project(data, b"Activity: private-token\n")
        self.assertEqual(result["events"], [])
        self.assertEqual(result["malformedEvents"], 3)
        self.assertNotIn("secret", json.dumps(result))
        self.assertNotIn("private", json.dumps(result))
        oversized = MODULE.project(b"secret" * 200_000, b"")
        self.assertTrue(oversized["inputTruncated"])
        self.assertEqual(oversized["events"], [])

    def test_event_limit_reports_loss_and_unknown_components_remain_opaque(self):
        line = b"I/wm_stop_activity( 1): [0,1,private.app/.Secret]\n"
        result = MODULE.project(line * 70, b"")
        self.assertEqual(len(result["events"]), 64)
        self.assertEqual(result["events"][0], {"event": "stop", "role": "other"})
        self.assertTrue(result["eventsTruncated"])
        self.assertEqual(result["matchedEvents"], 70)
        self.assertNotIn("private", json.dumps(result))

    def test_capture_does_not_hide_tail_loss_behind_tag_filtering(self):
        # Android's reader selects the tail before logcat filters tags. The real
        # journey cannot safely create event-log overload to exercise this gap.
        event = b"I/wm_stop_activity( 1): [0,1,private.app/.Secret]\n"
        unfiltered = b"I/other_tag( 1): private-token\n" * 999 + event
        def logcat(*args, **options):
            return event if any(arg.startswith("wm_") for arg in args) else unfiltered
        with tempfile.TemporaryDirectory() as directory:
            result = MODULE.capture_events(logcat, Path(directory), b"")
        self.assertTrue(result["tailLimitReached"])
        self.assertEqual(result["events"], [{"event": "stop", "role": "other"}])
        self.assertNotIn("private", json.dumps(result))

    def test_invalid_integer_fields_cannot_invent_transitions(self):
        for value in ("bad", "2147483648", "-2147483649", "1" * 20):
            for record in (
                f"I/wm_resume_activity( 1): [0,{value},1,private.app/.Secret]",
                f"I/wm_create_activity( 1): [0,1,2,private.app/.Secret,action,mime,uri,{value}]",
            ):
                with self.subTest(record=record):
                    result = MODULE.project(record.encode(), b"")
                    self.assertEqual(result["events"], [])
                    self.assertEqual(result["malformedEvents"], 1)

    def test_other_activities_retain_safe_package_roles_without_names(self):
        for target, expected in (
            ("private.launcher/.Secret", "home-package"),
            ("com.kinosail.player.dev/.Secret", "target-package"),
            ("com.kinosail.player.dev.test/.Secret", "instrumentation-package"),
            ("com.google.android.wearable.app/.Secret", "wearable-app"),
            ("com.google.android.wearable.sysui/.Secret", "wearable-system-ui"),
            ("com.google.android.wearable.setupwizard/.Secret", "wearable-setup"),
            ("com.google.android.gms/.Secret", "play-services"),
            ("com.android.systemui/.Secret", "system-ui"),
            ("com.android.settings/.Secret", "system-settings"),
            ("android/.Secret", "android-system"),
            ("private.package/android.service.dreams.DreamActivity", "system-dream"),
            ("private.package/.Secret", "other"),
        ):
            with self.subTest(role=expected):
                data = f"I/wm_stop_activity( 1): [0,1,{target}]".encode()
                result = MODULE.project(data, b"Activity: private.launcher/.Home\n")
                self.assertEqual(result["events"], [{"event": "stop", "role": expected}])
                for secret in ("private", "Secret", "com.google", "DreamActivity"):
                    self.assertNotIn(secret, json.dumps(result))

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
