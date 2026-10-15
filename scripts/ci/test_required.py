"""Protected checks fail closed on skipped, missing, or cancelled work."""

import copy
import json
import re
import subprocess
import tempfile
import unittest
from pathlib import Path

from affected import FLAGS
from required import verify

WORKFLOWS = Path(__file__).resolve().parents[2] / ".github/workflows"


def case(scope, selected=True, app="player"):
    plan = dict.fromkeys((*FLAGS, "deep"), selected)
    raw = json.dumps(plan)
    needs = {} if scope == "app" else {"plan": {"result": "success", "outputs": {"plan": raw}}}
    if scope == "repository":
        jobs = ("static", "tooling", "packages", "web", "docs")
    elif scope == "security":
        jobs = ("secrets", "codeql", "supply-chain", "findings")
    else:
        jobs = ("static", "race", "security", "system", "browser", "tooling", "client", "android", "wear")
    needs.update({job: {"result": "success" if selected else "skipped"} for job in jobs})
    if scope == "repository":
        needs["static"]["result"] = "success"
    if scope == "security":
        needs["secrets"]["result"] = "success"
    if scope == "app" and app != "player":
        needs["client"]["result"] = "skipped"
        needs["android"]["result"] = "skipped"
        needs["wear"]["result"] = "skipped"
    return raw, needs


class RequiredTests(unittest.TestCase):
    def test_selected_and_intentionally_unselected_jobs(self):
        for scope in ("repository", "security"):
            for selected in (True, False):
                raw, needs = case(scope, selected)
                verify(scope, needs)
        for app in ("player", "subtitles"):
            raw, needs = case("app", app=app)
            verify("app", needs, raw, app)

    def test_failure_cancelled_skip_missing_and_unexpected_job_fail(self):
        for scope, app in (("repository", None), ("security", None), ("app", "player")):
            raw, original = case(scope)
            for job in original:
                for state in ("failure", "cancelled", "skipped", "neutral", "missing"):
                    with self.subTest(scope=scope, job=job, state=state):
                        needs = copy.deepcopy(original)
                        if state == "missing":
                            needs.pop(job)
                        else:
                            needs[job]["result"] = state
                        with self.assertRaises((ValueError, KeyError)):
                            verify(scope, needs, raw, app)
            needs = copy.deepcopy(original)
            needs["surprise"] = {"result": "success"}
            with self.assertRaises(ValueError):
                verify(scope, needs, raw, app)

    def test_malformed_plan_and_app_rejected(self):
        raw, needs = case("app")
        for value in ("", "[]", "x" * 16385, "{}", json.dumps(dict.fromkeys((*FLAGS, "deep"), "false"))):
            with self.subTest(value=value[:20]), self.assertRaises(ValueError):
                verify("app", needs, value, "player")
        for app in (None, "../player", "retired"):
            with self.subTest(app=app), self.assertRaises(ValueError):
                verify("app", needs, raw, app)

    def test_native_only_requires_client_without_server(self):
        plan = dict.fromkeys((*FLAGS, "deep"), False)
        plan["client"] = True
        raw = json.dumps(plan)
        needs = {**{job: {"result": "skipped"} for job in ("static", "race", "security", "system", "browser", "tooling")},
                 "client": {"result": "success"}, "android": {"result": "skipped"}, "wear": {"result": "skipped"}}
        verify("app", needs, raw, "player")
        needs["client"]["result"] = "skipped"
        with self.assertRaises(ValueError):
            verify("app", needs, raw, "player")

    def test_android_only_requires_android_build(self):
        plan = dict.fromkeys((*FLAGS, "deep"), False)
        plan["android"] = True
        raw = json.dumps(plan)
        needs = {job: {"result": "skipped"} for job in
                 ("static", "race", "security", "system", "browser", "tooling", "client", "android", "wear")}
        needs["android"]["result"] = "success"
        verify("app", needs, raw, "player")
        needs["android"]["result"] = "skipped"
        with self.assertRaises(ValueError):
            verify("app", needs, raw, "player")

    def test_deep_android_requires_wear_runtime(self):
        plan = dict.fromkeys((*FLAGS, "deep"), False)
        plan["android"] = plan["deep"] = True
        raw = json.dumps(plan)
        needs = {job: {"result": "skipped"} for job in
                 ("static", "race", "security", "system", "browser", "tooling", "client", "android", "wear")}
        needs["android"]["result"] = needs["wear"]["result"] = "success"
        verify("app", needs, raw, "player")
        for state in ("failure", "cancelled", "skipped", "missing"):
            failed = copy.deepcopy(needs)
            if state == "missing":
                failed.pop("wear")
            else:
                failed["wear"]["result"] = state
            with self.subTest(state=state), self.assertRaises(ValueError):
                verify("app", failed, raw, "player")

    def test_actions_codeql_cannot_be_skipped(self):
        raw, needs = case("security", False)
        plan = json.loads(raw)
        plan["actions"] = True
        needs["plan"]["outputs"]["plan"] = json.dumps(plan)
        needs["codeql"]["result"] = needs["findings"]["result"] = "success"
        verify("security", needs)
        needs["codeql"]["result"] = "skipped"
        with self.assertRaises(ValueError):
            verify("security", needs)

    def test_gate_kill_switch_fails_required_checks_closed(self):
        guard = "test ! -e .gates-disabled"
        ci = (WORKFLOWS / "ci.yml").read_text()
        app = (WORKFLOWS / "app.yml").read_text()

        def job(source, name):
            return re.split(r"\n  [a-z][a-z-]*:\n", source.split(f"\n  {name}:\n", 1)[1], maxsplit=1)[0]

        plan = job(ci, "plan")
        self.assertLess(plan.index(f"      - run: {guard}\n"), plan.index("scripts/ci/affected.py"))
        for name in ("static", "repository-required", "security-required"):
            self.assertIn(f"      - run: {guard}\n", job(ci, name))
        self.assertIn(f"      - run: {guard}\n", job(app, "required"))
        for name in ("repository-required", "player-required", "subtitles-required", "security-required"):
            block = job(ci, name)
            self.assertIn("    if: always()\n", block)
            self.assertRegex(block, r"    needs: \[plan, ")
            self.assertTrue("required.py" in block or "test "$SELECTION" = success" in block)
        with tempfile.TemporaryDirectory() as directory:
            run = lambda: subprocess.run(["bash", "-ec", guard], cwd=directory, check=False).returncode
            self.assertEqual(run(), 0)
            (Path(directory) / ".gates-disabled").touch()
            self.assertNotEqual(run(), 0)


if __name__ == "__main__":
    unittest.main()
