"""Protected checks fail closed on skipped, missing, or cancelled work."""

import copy
import json
import re
import subprocess
import tempfile
import unittest
from pathlib import Path

from affected import FLAGS
from plan import expected
from required import SCOPES, verify

WORKFLOWS = Path(__file__).resolve().parents[2] / ".github/workflows"


def make_plan(selected=True, **overrides):
    plan = dict.fromkeys((*FLAGS, "deep", "verified"), selected)
    plan["deep"] = plan["verified"] = False
    plan.update(overrides)
    return plan


def needs_for(scope, plan, event_name="pull_request"):
    wanted = expected(plan, event_name)
    needs = {"plan": {"result": "success", "outputs": {"plan": json.dumps(plan)}}}
    needs.update({job: {"result": "success" if wanted[job] else "skipped"} for job in SCOPES[scope]})
    return needs


class RequiredTests(unittest.TestCase):
    def test_selected_and_intentionally_unselected_jobs(self):
        for scope in SCOPES:
            for selected in (True, False):
                with self.subTest(scope=scope, selected=selected):
                    plan = make_plan(selected)
                    verify(scope, needs_for(scope, plan), "pull_request")

    def test_failure_cancelled_skip_missing_and_unexpected_job_fail(self):
        for scope in SCOPES:
            plan = make_plan()
            original = needs_for(scope, plan)
            for job in original:
                for state in ("failure", "cancelled", "skipped", "neutral", "missing"):
                    with self.subTest(scope=scope, job=job, state=state):
                        needs = copy.deepcopy(original)
                        if state == "missing":
                            needs.pop(job)
                        else:
                            needs[job]["result"] = state
                        with self.assertRaises((ValueError, KeyError)):
                            verify(scope, needs, "pull_request")
            needs = copy.deepcopy(original)
            needs["surprise"] = {"result": "success"}
            with self.assertRaises(ValueError):
                verify(scope, needs, "pull_request")

    def test_unselected_job_that_ran_is_rejected(self):
        plan = make_plan(False)
        wanted = expected(plan, "pull_request")
        self.assertEqual({job for job, run in wanted.items() if run}, {"repository", "security"})
        for scope in SCOPES:
            for job in SCOPES[scope]:
                if wanted[job]:
                    continue
                needs = needs_for(scope, plan)
                needs[job]["result"] = "success"
                with self.subTest(scope=scope, job=job), self.assertRaises(ValueError):
                    verify(scope, needs, "pull_request")

    def test_malformed_plan_scope_and_event_rejected(self):
        plan = make_plan()
        needs = needs_for("app", plan)
        for value in ("", "[]", "x" * 16385, "{}", json.dumps(dict.fromkeys((*FLAGS, "deep", "verified"), "false")),
                      json.dumps(dict.fromkeys((*FLAGS, "deep"), True))):
            with self.subTest(value=value[:20]), self.assertRaises(ValueError):
                broken = copy.deepcopy(needs)
                broken["plan"]["outputs"]["plan"] = value
                verify("app", broken, "pull_request")
        for scope in (None, "../app", "retired", "player"):
            with self.subTest(scope=scope), self.assertRaises(ValueError):
                verify(scope, needs, "pull_request")
        with self.assertRaises(ValueError):
            verify("app", needs, None)

    def test_native_only_requires_client_without_server(self):
        plan = make_plan(False, client=True)
        needs = needs_for("app", plan)
        self.assertEqual(needs["client"]["result"], "success")
        self.assertEqual({needs[job]["result"] for job in ("go", "e2e", "proofs", "arm", "tools", "android")}, {"skipped"})
        verify("app", needs, "pull_request")
        needs["client"]["result"] = "skipped"
        with self.assertRaises(ValueError):
            verify("app", needs, "pull_request")

    def test_android_only_requires_android_build(self):
        plan = make_plan(False, android=True)
        needs = needs_for("app", plan)
        self.assertEqual(needs["android"]["result"], "success")
        self.assertEqual(needs["go"]["result"], "skipped")
        verify("app", needs, "pull_request")
        needs["android"]["result"] = "skipped"
        with self.assertRaises(ValueError):
            verify("app", needs, "pull_request")

    def test_actions_codeql_cannot_be_skipped(self):
        plan = make_plan(False, actions=True)
        needs = needs_for("security", plan)
        self.assertEqual(needs["security"]["result"], "success")
        self.assertEqual(needs["codeql-compiled"]["result"], "skipped")
        verify("security", needs, "pull_request")
        needs["security"]["result"] = "skipped"
        with self.assertRaises(ValueError):
            verify("security", needs, "pull_request")

    def test_merge_queue_verified_push_skips_tests_but_keeps_main_security_and_docs(self):
        plan = make_plan(True, verified=True)
        for scope in SCOPES:
            needs = needs_for(scope, plan, "push")
            states = {job: needs[job]["result"] for job in SCOPES[scope]}
            if scope == "repository":
                self.assertEqual(states, {"repository": "skipped", "docs": "success"})
            elif scope == "security":
                self.assertEqual(states, {"security": "success", "codeql-compiled": "success"})
            else:
                self.assertEqual(set(states.values()), {"skipped"})
            verify(scope, needs, "push")
            for job in SCOPES[scope]:
                flipped = copy.deepcopy(needs)
                flipped[job]["result"] = "success" if states[job] == "skipped" else "skipped"
                with self.subTest(scope=scope, job=job), self.assertRaises(ValueError):
                    verify(scope, flipped, "push")
        pull = needs_for("security", plan, "pull_request")
        self.assertEqual(pull["security"]["result"], "skipped")

    def test_gate_kill_switch_fails_required_checks_closed(self):
        guard = "test ! -e .gates-disabled"
        ci = (WORKFLOWS / "ci.yml").read_text()

        def job(source, name):
            return re.split(r"\n  [a-z][a-z-]*:\n", source.split(f"\n  {name}:\n", 1)[1], maxsplit=1)[0]

        plan = job(ci, "plan")
        self.assertLess(plan.index(f"      - run: {guard}\n"), plan.index("scripts/ci/plan.py"))
        for name in ("repository-required", "player-required", "subtitles-required", "security-required"):
            block = job(ci, name)
            self.assertIn(f"      - run: {guard}\n", block)
            self.assertIn("    if: always()\n", block)
            self.assertRegex(block, r"    needs: \[plan, ")
            self.assertIn("required.py", block)
        with tempfile.TemporaryDirectory() as directory:
            run = lambda: subprocess.run(["bash", "-ec", guard], cwd=directory, check=False).returncode
            self.assertEqual(run(), 0)
            (Path(directory) / ".gates-disabled").touch()
            self.assertNotEqual(run(), 0)


if __name__ == "__main__":
    unittest.main()
