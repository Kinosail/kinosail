"""Publication is limited to successful main builds and validated digests."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

from affected import FLAGS
from delivery import GATES, main as deliver, targets, verify_results
from promote import main as promote
from secrets import log_range

ROOT = Path(__file__).resolve().parents[2]
SHA = "a" * 40
DIGEST = "sha256:" + "b" * 64
ENV = {"APP": "player", "IMAGE": "ghcr.io/kinosail/kinosail-player", "DIGEST": DIGEST,
       "GITHUB_SHA": SHA, "GITHUB_EVENT_NAME": "push", "GITHUB_REF": "refs/heads/main",
       "GITHUB_REPOSITORY": "Kinosail/kinosail"}


def delivery_results(plan):
    results = {name: {"result": "success"} for name in GATES}
    results["changes"]["outputs"] = {"plan": plan}
    return results


class DeliveryTests(unittest.TestCase):
    def test_every_gate_must_succeed_before_delivery(self):
        plan = json.dumps(dict.fromkeys((*FLAGS, "deep"), True))
        verify_results(json.dumps(delivery_results(plan)), plan)
        for gate in GATES:
            for state in ("failure", "cancelled", "skipped", "neutral", "", "missing", None):
                with self.subTest(gate=gate, state=state), tempfile.TemporaryDirectory() as directory:
                    results = delivery_results(plan)
                    if state == "missing":
                        del results[gate]
                    else:
                        results[gate]["result"] = state
                    output = Path(directory) / "output"
                    env = ENV | {"CI_PLAN": plan, "RESULTS": json.dumps(results), "GITHUB_OUTPUT": str(output)}
                    with patch.dict(os.environ, env), patch("delivery.subprocess.run") as run:
                        with self.assertRaises(ValueError):
                            deliver()
                        run.assert_not_called()
                    self.assertFalse(output.exists())

    def test_malformed_results_and_conflicting_plan_fail_without_side_effects(self):
        plan = json.dumps(dict.fromkeys((*FLAGS, "deep"), True))
        results = delivery_results(plan)
        for raw in ("", "[]", "{}", "x" * 65537, json.dumps(results | {"unknown": {"result": "success"}}),
                    json.dumps(results | {"required": None}),
                    json.dumps(results | {"changes": {"result": "success", "outputs": {"plan": "{}"}}})):
            with self.subTest(raw=raw[:40]), patch.dict(os.environ, ENV | {"CI_PLAN": plan, "RESULTS": raw}), \
                    patch("delivery.subprocess.run") as run:
                with self.assertRaises(ValueError):
                    deliver()
                run.assert_not_called()

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
            env = ENV | {"CI_PLAN": plan, "RESULTS": json.dumps(delivery_results(plan))} | replacement
            with patch.dict(os.environ, env), patch("delivery.subprocess.run") as run:
                with self.assertRaises(ValueError):
                    deliver()
                run.assert_not_called()

    def test_success_emits_only_selected_apps_without_polling(self):
        plan = dict.fromkeys((*FLAGS, "deep"), False) | {"player": True, "dashboard": True}
        encoded = json.dumps(plan)
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "output"
            env = ENV | {"CI_PLAN": encoded, "RESULTS": json.dumps(delivery_results(encoded)), "GITHUB_OUTPUT": str(output)}
            with patch.dict(os.environ, env), patch("delivery.subprocess.run") as run:
                deliver()
                run.assert_called_once_with(["git", "merge-base", "--is-ancestor", SHA, "origin/main"], check=True, timeout=60)
            self.assertEqual(json.loads(output.read_text().removeprefix("apps=")), ["player", "dashboard"])

    def test_non_ancestor_and_empty_plan_cannot_emit_matrix(self):
        for selected in (True, False):
            plan = json.dumps(dict.fromkeys((*FLAGS, "deep"), selected))
            with tempfile.TemporaryDirectory() as directory:
                output = Path(directory) / "output"
                env = ENV | {"CI_PLAN": plan, "RESULTS": json.dumps(delivery_results(plan)), "GITHUB_OUTPUT": str(output)}
                with patch.dict(os.environ, env), patch("delivery.subprocess.run", side_effect=subprocess.CalledProcessError(1, "git")) as run:
                    with self.assertRaises((ValueError, subprocess.CalledProcessError)):
                        deliver()
                    self.assertEqual(run.call_count, int(selected))
                self.assertFalse(output.exists())

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
