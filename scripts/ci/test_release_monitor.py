"""Regression checks for the release signing monitor."""
import datetime
import json
import unittest
from pathlib import Path

from release_monitor import WORKFLOWS, audit_tags, channel_problems, found_subjects, unexpected_identities


def entry(subject, index=7):
    return json.dumps({"CertSubject": subject, "Issuer": "https://token.actions.githubusercontent.com", "Index": index, "UUID": "abc"})


class ReleaseMonitorTests(unittest.TestCase):
    def test_only_official_tags_and_main_workflows_may_sign(self):
        official = {"player-v0.1.0"}
        lines = "\n".join([
            entry(WORKFLOWS + "release.yml@refs/tags/player-v0.1.0"),
            entry(WORKFLOWS + "release-trust.yml@refs/heads/main"),
            entry(WORKFLOWS + "publish.yml@refs/heads/main"),
            "",
        ])
        self.assertEqual(unexpected_identities(found_subjects(lines), official.__contains__), [])
        for subject in (WORKFLOWS + "release.yml@refs/tags/player-v0.2.0", WORKFLOWS + "release.yml@refs/heads/main",
                        WORKFLOWS + "ci.yml@refs/heads/main", WORKFLOWS + "release-trust.yml@refs/heads/feature",
                        WORKFLOWS + "release.yml@refs/tags/player-v0.1.0-rc1", "https://github.com/Kinosail/kinosail-fork/x", ""):
            with self.subTest(subject=subject):
                problems = unexpected_identities(found_subjects(entry(subject, 99)), official.__contains__)
                self.assertEqual(len(problems), 1)
                self.assertIn("Rekor index 99", problems[0])
        with self.assertRaises(ValueError):
            found_subjects("[]")

    def test_tags_must_be_on_main_and_published(self):
        self.assertEqual(audit_tags(["player-v0.1.0"], lambda tag: True, lambda tag: True, lambda tag: False), [])
        self.assertEqual(audit_tags(["player-v0.2.0"], lambda tag: True, lambda tag: False, lambda tag: True), [])
        cases = {
            "player-v0.2.0": (lambda tag: False, lambda tag: True, lambda tag: False, "not on main"),
            "player-v0.3.0": (lambda tag: True, lambda tag: False, lambda tag: False, "no published release"),
            "player-latest": (lambda tag: True, lambda tag: True, lambda tag: True, "Unexpected release-like tag"),
        }
        for tag, (on_main, released, running, message) in cases.items():
            with self.subTest(tag=tag):
                problems = audit_tags([tag], on_main, released, running)
                self.assertEqual(len(problems), 1)
                self.assertIn(message, problems[0])

    def test_channel_must_stay_fresh(self):
        now = datetime.datetime(2026, 11, 22, 12, tzinfo=datetime.timezone.utc)
        fresh = {"app": "player", "latest": "v0.2.0", "expires": "2026-11-29T11:00:00Z"}
        self.assertEqual(channel_problems(fresh, now), [])
        self.assertIn("not refreshing", channel_problems(dict(fresh, expires="2026-11-25T11:00:00Z"), now)[0])
        self.assertIn("invalid release", channel_problems(dict(fresh, latest="latest"), now)[0])
        self.assertIn("cannot be read", channel_problems({"app": "player"}, now)[0])

    def test_only_the_main_channel_job_can_write_releases(self):
        source = (Path(__file__).resolve().parents[2] / ".github/workflows/release-trust.yml").read_text()
        channel, monitor = source.split("\n  channel:\n", 1)[1].split("\n  monitor:\n", 1)
        self.assertNotIn("pull_request", source)
        self.assertNotIn("push:", source)
        self.assertIn("github.ref == 'refs/heads/main'", channel)
        self.assertIn("contents: write", channel)
        self.assertIn("id-token: write", channel)
        self.assertNotIn("contents: write", monitor)
        self.assertNotIn("id-token", monitor)
        self.assertIn("ref: fbc5fc49d94db706999e29ac53acac8ec8e1fdea", monitor)


if __name__ == "__main__":
    unittest.main()
