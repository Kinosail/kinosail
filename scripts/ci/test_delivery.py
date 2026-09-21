"""Publication is limited to successful main builds and validated digests."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

from affected import FLAGS
from delivery import conclusion, main as deliver, targets
from promote import main as promote
from secrets import log_range

ROOT = Path(__file__).resolve().parents[2]
SHA = "a" * 40
DIGEST = "sha256:" + "b" * 64
ENV = {"APP": "player", "IMAGE": "ghcr.io/kinosail/kinosail-player", "DIGEST": DIGEST,
       "GITHUB_SHA": SHA, "GITHUB_EVENT_NAME": "push", "GITHUB_REF": "refs/heads/main",
       "GITHUB_REPOSITORY": "Kinosail/kinosail"}


def workflow(**overrides):
    return {"id": 1, "head_sha": SHA, "event": "push", "head_branch": "main",
            "status": "completed", "conclusion": "success"} | overrides


class DeliveryTests(unittest.TestCase):
    def test_exact_commit_success_is_required(self):
        self.assertTrue(conclusion({"workflow_runs": [workflow()]}, SHA))
        self.assertFalse(conclusion({"workflow_runs": []}, SHA))
        self.assertFalse(conclusion({"workflow_runs": [workflow(status="in_progress", conclusion=None)]}, SHA))
        invalid = ({"head_sha": "c" * 40}, {"event": "pull_request"}, {"head_branch": "feature"},
                   {"status": "unknown"}, {"conclusion": "failure"}, {"conclusion": "cancelled"},
                   {"conclusion": "skipped"}, {"conclusion": "neutral"}, {"id": "1"})
        for values in invalid:
            with self.subTest(values=values), self.assertRaises(ValueError):
                conclusion({"workflow_runs": [workflow(**values)]}, SHA)
        # A newer failed rerun must not reuse an older successful run.
        with self.assertRaises(ValueError):
            conclusion({"workflow_runs": [workflow(), workflow(id=2, conclusion="failure")]}, SHA)
        for payload in ({}, [], {"workflow_runs": None}, {"workflow_runs": [workflow()] * 6}):
            with self.assertRaises(ValueError):
                conclusion(payload, SHA)

    def test_delivery_plan_rejects_unknown_missing_and_non_boolean_values(self):
        plan = dict.fromkeys((*FLAGS, "deep"), False)
        plan["player"] = True
        self.assertEqual(targets(json.dumps(plan)), ["player"])
        for value in ("", "[]", "{}", "x" * 16385, json.dumps(plan | {"x": True}),
                      json.dumps(plan | {"player": "false"})):
            with self.subTest(value=value[:40]), self.assertRaises(ValueError):
                targets(value)

    def test_delivery_rejects_untrusted_context_before_any_side_effect(self):
        plan = json.dumps(dict.fromkeys((*FLAGS, "deep"), True))
        for replacement in ({"GITHUB_SHA": "--help"}, {"GITHUB_REPOSITORY": "other/repo"},
                            {"GITHUB_EVENT_NAME": "pull_request"}, {"GITHUB_REF": "refs/tags/v1"}):
            with patch.dict(os.environ, ENV | {"CI_PLAN": plan} | replacement), patch("delivery.subprocess.run") as run:
                with self.assertRaises(ValueError):
                    deliver()
                run.assert_not_called()

    def test_failed_ci_cannot_emit_deployment_matrix(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "output"
            plan = json.dumps(dict.fromkeys((*FLAGS, "deep"), True))
            response = subprocess.CompletedProcess([], 0, json.dumps({"workflow_runs": [workflow(conclusion="failure")]}).encode())
            with patch.dict(os.environ, ENV | {"CI_PLAN": plan, "GITHUB_OUTPUT": str(output)}), patch("delivery.subprocess.run", return_value=response):
                with self.assertRaises(ValueError):
                    deliver()
            self.assertFalse(output.exists())

    def test_delivery_waits_for_cold_swift_but_remains_bounded(self):
        for status, clock, succeeds in (("completed", [0, 3660], True),
                                         ("in_progress", [0, 3660, 3901], False)):
            with self.subTest(status=status), tempfile.TemporaryDirectory() as directory:
                output = Path(directory) / "output"
                plan = json.dumps(dict.fromkeys((*FLAGS, "deep"), True))
                payload = {"workflow_runs": [workflow(status=status, conclusion="success" if succeeds else None)]}
                response = subprocess.CompletedProcess([], 0, json.dumps(payload).encode())
                with patch.dict(os.environ, ENV | {"CI_PLAN": plan, "GITHUB_OUTPUT": str(output)}), \
                        patch("delivery.subprocess.run", return_value=response), \
                        patch("delivery.time.monotonic", side_effect=clock), patch("delivery.time.sleep"):
                    if succeeds:
                        deliver()
                    else:
                        with self.assertRaisesRegex(ValueError, "timed out"):
                            deliver()
                self.assertEqual(output.exists(), succeeds)
                if succeeds:
                    self.assertEqual(json.loads(output.read_text().removeprefix("apps=")),
                                     ["player", "subtitles", "dashboard"])

    def test_promotion_allows_docs_and_unrelated_apps_but_never_rolls_back_affected_app(self):
        for paths, expected in ((b"README.md\0", True), (b"apps/subtitles/main.go\0", True),
                                (b"apps/player/main.go\0", False), (b"packages/catalog.go\0", False),
                                (b"unknown\0", False), (b"", True)):
            with self.subTest(paths=paths), patch.dict(os.environ, ENV), patch("promote.subprocess.run") as run:
                run.return_value = subprocess.CompletedProcess([], 0, paths)
                promote()
                mutations = [call for call in run.call_args_list if call.args[0][0] == "docker"]
                self.assertEqual(bool(mutations), expected)
                if expected:
                    self.assertIn(ENV["IMAGE"] + "@" + DIGEST, mutations[0].args[0])

    def test_promotion_rejects_input_before_any_side_effect(self):
        for replacement in ({"APP": "../bad"}, {"IMAGE": "ghcr.io/other/image"}, {"DIGEST": "latest"},
                            {"GITHUB_SHA": ""}, {"GITHUB_EVENT_NAME": "pull_request"}, {"GITHUB_REF": "other"}):
            with patch.dict(os.environ, ENV | replacement), patch("promote.subprocess.run") as run:
                with self.assertRaises(ValueError):
                    promote()
                run.assert_not_called()

    def test_failed_fetch_and_invalid_git_diff_cannot_publish(self):
        with patch.dict(os.environ, ENV), patch("promote.subprocess.run", side_effect=subprocess.CalledProcessError(1, "git")) as run:
            with self.assertRaises(subprocess.CalledProcessError):
                promote()
            self.assertEqual(run.call_count, 1)
        with patch.dict(os.environ, ENV), patch("promote.subprocess.run") as run:
            run.return_value = subprocess.CompletedProcess([], 0, b"unterminated")
            with self.assertRaises(ValueError):
                promote()
            self.assertFalse(any(call.args[0][0] == "docker" for call in run.call_args_list))

    def test_invalid_shell_inputs_do_not_invoke_tools(self):
        with tempfile.TemporaryDirectory() as directory:
            marker = Path(directory) / "executed"
            for tool in ("go", "docker", "podman"):
                binary = Path(directory) / tool
                binary.write_text(f'#!/bin/sh\ntouch "{marker}"\n')
                binary.chmod(0o755)
            env = os.environ | {"PATH": directory + ":" + os.environ["PATH"]}
            commands = []
            for args in ([], [""], ["unknown"], ["../player"], ["player", "packages"], ["x" * 10000]):
                commands.append(["bash", str(ROOT / "scripts/ci/test-go.sh"), *args])
            for args in ([], ["player"], ["unknown", "image"], ["player", "ghcr.io/kinosail/kinosail-player:latest"],
                         ["player", "ghcr.io/other/image@" + DIGEST], ["player", "x" * 10000]):
                commands.append(["bash", str(ROOT / "scripts/ci/test-image.sh"), *args])
            for command in commands:
                result = subprocess.run(command, capture_output=True, env=env, cwd=directory)
                self.assertEqual(result.returncode, 2, result.stderr)
                self.assertFalse(marker.exists())
            for flag in ("true", "2", "x" * 10000):
                result = subprocess.run(["bash", str(ROOT / "apps/dashboard/scripts/test-container.sh")],
                    capture_output=True, env=env | {"KINOSAIL_TEST_IMAGE_READY": flag}, cwd=ROOT)
                self.assertEqual(result.returncode, 2, result.stderr)
                self.assertFalse(marker.exists())

    def test_secret_scan_covers_all_introduced_commits(self):
        base = "b" * 40
        self.assertEqual(log_range("push", {"before": base}, SHA), base + ".." + SHA)
        self.assertEqual(log_range("push", {"before": "0" * 40}, SHA), "--all")
        self.assertEqual(log_range("schedule", {}, SHA), "--all")
        pr = {"pull_request": {"base": {"sha": base}, "head": {"sha": SHA}}}
        self.assertEqual(log_range("pull_request", pr, "c" * 40), base + ".." + SHA)
        for event, data, head in (("push", {"before": "--all"}, SHA), ("push", [], SHA),
                                  ("unknown", {}, SHA), ("schedule", {}, "")):
            with self.assertRaises(ValueError):
                log_range(event, data, head)


if __name__ == "__main__":
    unittest.main()
