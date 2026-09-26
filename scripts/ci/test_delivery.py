"""Publication needs complete same-run checks and validated image inputs."""

import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

from affected import FLAGS
from delivery import main as deliver, targets, verify_results
from promote import main as promote
from secrets import log_range

ROOT = Path(__file__).resolve().parents[2]
SHA = "a" * 40
DIGEST = "sha256:" + "b" * 64
ENV = {"APP": "player", "IMAGE": "ghcr.io/kinosail/kinosail-player", "DIGEST": DIGEST,
       "GITHUB_SHA": SHA, "GITHUB_EVENT_NAME": "push", "GITHUB_REF": "refs/heads/main",
       "GITHUB_REPOSITORY": "Kinosail/kinosail"}
REQUIRED = ("repository-required", "player-required", "subtitles-required", "security-required")


def results(plan):
    return {"plan": {"result": "success", "outputs": {"plan": plan}},
            **{job: {"result": "success"} for job in REQUIRED}}


class DeliveryTests(unittest.TestCase):
    def test_delivery_plan_rejects_unknown_missing_and_non_boolean_values(self):
        plan = dict.fromkeys((*FLAGS, "deep"), False)
        plan["player"] = True
        self.assertEqual(targets(json.dumps(plan)), ["player"])
        for value in ("", "[]", "{}", "x" * 16385, json.dumps(plan | {"x": True}),
                      json.dumps(plan | {"player": "false"})):
            with self.subTest(value=value[:40]), self.assertRaises(ValueError):
                targets(value)

    def test_untrusted_context_has_no_side_effect(self):
        plan = json.dumps(dict.fromkeys((*FLAGS, "deep"), True))
        for replacement in ({"GITHUB_SHA": "--help"}, {"GITHUB_REPOSITORY": "other/repo"},
                            {"GITHUB_EVENT_NAME": "pull_request"}, {"GITHUB_REF": "refs/tags/v1"}):
            with patch.dict(os.environ, ENV | {"CI_PLAN": plan, "CI_RESULTS": json.dumps(results(plan))} | replacement), \
                    patch("delivery.subprocess.run") as run:
                with self.assertRaises(ValueError):
                    deliver()
                run.assert_not_called()

    def test_missing_failed_cancelled_stale_and_malformed_checks_do_not_publish(self):
        plan = json.dumps(dict.fromkeys((*FLAGS, "deep"), True))
        for job in ("plan", *REQUIRED):
            for state in ("missing", "failure", "cancelled", "skipped"):
                with self.subTest(job=job, state=state), tempfile.TemporaryDirectory() as directory:
                    output = Path(directory) / "output"
                    needs = results(plan)
                    if state == "missing":
                        needs.pop(job)
                    else:
                        needs[job]["result"] = state
                    with patch.dict(os.environ, ENV | {"CI_PLAN": plan, "CI_RESULTS": json.dumps(needs),
                                                     "GITHUB_OUTPUT": str(output)}), patch("delivery.subprocess.run") as run:
                        with self.assertRaises(ValueError):
                            deliver()
                        run.assert_not_called()
                    self.assertFalse(output.exists())
        for invalid in ("", "[]", "x" * 65537, json.dumps(results("{}"))):
            with self.subTest(invalid=invalid[:20]), self.assertRaises(ValueError):
                verify_results(invalid, plan)
        malformed = results(plan)
        malformed["plan"]["outputs"] = []
        with self.assertRaises(ValueError):
            verify_results(json.dumps(malformed), plan)

    def test_no_selected_app_causes_no_side_effect(self):
        plan = json.dumps(dict.fromkeys((*FLAGS, "deep"), False))
        with patch.dict(os.environ, ENV | {"CI_PLAN": plan, "CI_RESULTS": json.dumps(results(plan))}), \
                patch("delivery.subprocess.run") as run:
            with self.assertRaises(ValueError):
                deliver()
            run.assert_not_called()

    def test_success_emits_selected_apps_after_ancestry_proof(self):
        plan = dict.fromkeys((*FLAGS, "deep"), False)
        plan["player"] = plan["subtitles"] = True
        raw = json.dumps(plan)
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "output"
            with patch.dict(os.environ, ENV | {"CI_PLAN": raw, "CI_RESULTS": json.dumps(results(raw)),
                                             "GITHUB_OUTPUT": str(output)}), patch("delivery.subprocess.run") as run:
                deliver()
                run.assert_called_once_with(["git", "merge-base", "--is-ancestor", SHA, "origin/main"], check=True)
            self.assertEqual(json.loads(output.read_text().removeprefix("apps=")), ["player", "subtitles"])

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
