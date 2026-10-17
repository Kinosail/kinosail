"""The fast-path plan derives every matrix from one selection and only trusts a successful merge queue run."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

from affected import APPS, FLAGS
import plan
from plan import KEYS, build, derive, expected, tested_run

SHA = "a" * 40
REPO = "Kinosail/kinosail"


def flags(selected=False, **overrides):
    result = dict.fromkeys(FLAGS, selected)
    result.update(overrides)
    return result


class DeriveTests(unittest.TestCase):
    def test_every_matrix_follows_the_selection(self):
        full = derive(flags(True))
        self.assertEqual([entry["module"] for entry in full["modules"]], ["player", "subtitles", "packages"])
        self.assertEqual([(entry["app"], entry["phase"]) for entry in full["apps"]],
                         [("player", "journeys"), ("player", "populated"), ("subtitles", "all")])
        self.assertEqual({entry["system"] for entry in full["apps"] if entry["phase"] != "populated"}, {"true"})
        self.assertEqual(full["arm"], list(APPS))
        self.assertEqual(full["tools"], list(APPS))
        self.assertEqual(full["codeql"], "go,java-kotlin,javascript-typescript,python,actions")
        self.assertTrue(full["shards"])
        empty = derive(flags(False))
        self.assertEqual(empty, {"codeql": "", "modules": [], "apps": [], "shards": [], "arm": [], "tools": []})
        subtitles = derive(flags(False, subtitles=True, go=True, subtitles_arm=True))
        self.assertEqual([entry["module"] for entry in subtitles["modules"]], ["subtitles"])
        self.assertEqual([entry["app"] for entry in subtitles["apps"]], ["subtitles"])
        self.assertEqual(subtitles["arm"], ["subtitles"])
        self.assertEqual(subtitles["shards"], [])
        self.assertEqual(subtitles["codeql"], "go")
        for value in ({"player": "true"}, [], {"player": 1}):
            with self.subTest(value=value), self.assertRaises((ValueError, TypeError, KeyError)):
                derive(value)

    def test_expected_jobs_match_selection_event_and_verification(self):
        live = expected({**flags(True), "deep": False, "verified": False}, "pull_request")
        self.assertTrue(all(live.values()))
        nothing = expected({**flags(False), "deep": False, "verified": False}, "pull_request")
        self.assertEqual({job for job, run in nothing.items() if run}, {"repository", "security"})
        verified = expected({**flags(True), "deep": False, "verified": True}, "push")
        self.assertEqual({job for job, run in verified.items() if run}, {"docs", "security"})
        self.assertFalse(any(expected({**flags(True), "deep": False, "verified": True}, "pull_request").values()))


class TestedRunTests(unittest.TestCase):
    def completed(self, stdout, code=0):
        return subprocess.CompletedProcess([], code, stdout, "")

    def test_only_a_completed_successful_merge_queue_run_for_the_exact_commit_counts(self):
        good = [{"databaseId": 42, "headSha": SHA, "status": "completed", "conclusion": "success"}]
        with patch("plan.subprocess.run", return_value=self.completed(json.dumps(good))) as run:
            self.assertEqual(tested_run(SHA, REPO), 42)
            command = run.call_args.args[0]
            self.assertEqual(command[:3], ["gh", "run", "list"])
            self.assertIn("merge_group", command)
            self.assertIn(SHA, command)
            self.assertIn("ci.yml", command)
        for rows in ([], [{**good[0], "headSha": "b" * 40}], [{**good[0], "conclusion": "failure"}],
                     [{**good[0], "status": "in_progress"}], [{**good[0], "databaseId": "42"}], {"x": 1}, "garbage"):
            stdout = rows if isinstance(rows, str) else json.dumps(rows)
            with self.subTest(rows=str(rows)[:40]), patch("plan.subprocess.run", return_value=self.completed(stdout)):
                self.assertIsNone(tested_run(SHA, REPO))
        with patch("plan.subprocess.run", return_value=self.completed(json.dumps(good), code=1)):
            self.assertIsNone(tested_run(SHA, REPO))
        with patch("plan.subprocess.run", return_value=self.completed("[" + "x" * 70000)):
            self.assertIsNone(tested_run(SHA, REPO))
        for sha, repo in (("--help", REPO), (SHA, "evil;rm"), ("", REPO)):
            with self.subTest(sha=sha, repo=repo), patch("plan.subprocess.run") as run:
                with self.assertRaises(ValueError):
                    tested_run(sha, repo)
                run.assert_not_called()


class BuildTests(unittest.TestCase):
    def test_main_push_tested_by_the_queue_is_verified_and_other_events_never_ask(self):
        with patch("plan.changed_paths", return_value=["apps/player/internal/server/home.go"]), \
                patch("plan.tested_run", return_value=7) as tested:
            built, derived, run = build("push", {"ref": "refs/heads/main", "before": "b" * 40}, SHA, REPO, False, False)
            self.assertTrue(built["verified"])
            self.assertEqual(run, 7)
            self.assertTrue(built["player"])
            self.assertFalse(built["deep"])
            tested.assert_called_once_with(SHA, REPO)
        for event_name, event, full, deep in (("pull_request", {"ref": "refs/heads/main"}, False, False),
                                               ("merge_group", {"ref": "refs/heads/main"}, False, False),
                                               ("push", {"ref": "refs/heads/other"}, False, False),
                                               ("push", {"ref": "refs/heads/main"}, True, False),
                                               ("workflow_dispatch", {}, True, True)):
            with self.subTest(event=event_name, full=full, deep=deep), \
                    patch("plan.changed_paths", return_value=None), patch("plan.tested_run") as tested:
                built, derived, run = build(event_name, event, SHA, REPO, full, deep)
                tested.assert_not_called()
                self.assertFalse(built["verified"])
                self.assertIsNone(run)
                self.assertEqual(built["deep"], deep)
                self.assertTrue(all(built[flag] for flag in FLAGS))
        with patch("plan.changed_paths", return_value=["README.md"]), patch("plan.tested_run", return_value=None):
            built, derived, run = build("push", {"ref": "refs/heads/main"}, SHA, REPO, False, False)
            self.assertFalse(built["verified"])
            self.assertEqual(derived["apps"], [])

    def test_cli_writes_every_output_once_and_rejects_bad_flags_without_side_effects(self):
        with tempfile.TemporaryDirectory() as directory:
            event = Path(directory) / "event.json"
            output = Path(directory) / "output"
            event.write_text(json.dumps({"pull_request": {"base": {"sha": "b" * 40}, "head": {"sha": SHA}}}))
            env = {"GITHUB_EVENT_PATH": str(event), "GITHUB_OUTPUT": str(output), "GITHUB_EVENT_NAME": "pull_request",
                   "GITHUB_SHA": SHA, "GITHUB_REPOSITORY": REPO}
            with patch.dict(os.environ, env | {"CI_FULL": "maybe"}), self.assertRaises(ValueError):
                plan.main()
            self.assertFalse(output.exists())
            with patch.dict(os.environ, env), patch("plan.changed_paths", return_value=["apps/subtitles/cmd/kinosail/main.go"]):
                plan.main()
            lines = output.read_text().splitlines()
            self.assertEqual([line.split("=", 1)[0] for line in lines], list(KEYS))
            values = dict(line.split("=", 1) for line in lines)
            self.assertEqual(values["verified"], "false")
            self.assertEqual(values["codeql"], "go")
            self.assertEqual([entry["app"] for entry in json.loads(values["apps"])], ["subtitles"])
            self.assertEqual(json.loads(values["shards"]), [])
            selection = json.loads(values["plan"])
            self.assertEqual(set(selection), {*FLAGS, "deep", "verified"})


if __name__ == "__main__":
    unittest.main()
