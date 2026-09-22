"""A skipped selected job must never make a protected check green."""
import copy
import json
import unittest

from affected import FLAGS
from required import verify

SCOPES = {
    "player": ("static", "race", "security", "system", "images", "tooling", "scheduled", "client"),
    "subtitles": ("static", "race", "security", "system", "images", "tooling", "scheduled"),
    "dashboard": ("static", "race", "security", "system"),
    "repository": ("static", "tooling", "packages", "web"),
    "security": ("secrets", "supply-chain", "codeql", "swift", "findings"),
}


def results(scope, selected):
    plan = dict.fromkeys((*FLAGS, "deep"), selected)
    needs = {"changes": {"result": "success", "outputs": {"plan": json.dumps(plan)}}}
    needs.update({job: {"result": "success" if selected else "skipped"} for job in SCOPES[scope]})
    if scope == "repository":
        needs["static"]["result"] = "success"
    if scope == "security":
        needs["secrets"]["result"] = "success"
    return needs


class RequiredTests(unittest.TestCase):
    def test_all_selected_or_intentionally_unselected_pass(self):
        for scope in SCOPES:
            for selected in (True, False):
                verify(scope, results(scope, selected))

    def test_reusable_workflows_validate_the_shared_plan_and_exact_job_inventory(self):
        for scope in SCOPES:
            for selected in (True, False):
                needs = results(scope, selected)
                plan = needs.pop("changes")["outputs"]["plan"]
                verify(scope, needs, plan)
                for job in SCOPES[scope]:
                    broken = copy.deepcopy(needs)
                    broken[job]["result"] = "cancelled"
                    with self.assertRaises(ValueError):
                        verify(scope, broken, plan)
                for invalid in ("", "[]", "{}", "x" * 16385, json.dumps(dict.fromkeys((*FLAGS, "deep"), 1))):
                    with self.assertRaises(ValueError):
                        verify(scope, needs, invalid)

    def test_selected_failure_cancellation_skip_and_missing_job_fail(self):
        for scope, jobs in SCOPES.items():
            for job in ("changes", *jobs):
                for state in ("failure", "cancelled", "skipped", "", "neutral", "missing"):
                    with self.subTest(scope=scope, job=job, state=state):
                        needs = results(scope, True)
                        if state == "missing":
                            needs.pop(job)
                        else:
                            needs[job]["result"] = state
                        with self.assertRaises((ValueError, KeyError)):
                            verify(scope, needs)

    def test_unknown_jobs_and_malformed_plans_fail(self):
        needs = results("player", True)
        needs["surprise"] = {"result": "success"}
        with self.assertRaises(ValueError):
            verify("player", needs)
        for plan in ({}, {"player": True}, dict.fromkeys((*FLAGS, "deep"), "false")):
            needs = results("player", True)
            needs["changes"]["outputs"]["plan"] = json.dumps(plan)
            with self.assertRaises(ValueError):
                verify("player", needs)
        with self.assertRaises(ValueError):
            verify("unknown", {})

    def test_swift_and_actions_scans_cannot_be_skipped(self):
        for language in ("swift", "actions"):
            needs = results("security", False)
            plan = json.loads(needs["changes"]["outputs"]["plan"])
            plan[language] = True
            needs["changes"]["outputs"]["plan"] = json.dumps(plan)
            for job in (("swift" if language == "swift" else "codeql"), "findings"):
                needs[job]["result"] = "success"
            verify("security", needs)
            for job in (("swift" if language == "swift" else "codeql"), "findings"):
                broken = copy.deepcopy(needs)
                broken[job]["result"] = "skipped"
                with self.assertRaises(ValueError):
                    verify("security", broken)

    def test_native_only_requires_client_and_skips_server(self):
        needs = results("player", False)
        plan = json.loads(needs["changes"]["outputs"]["plan"])
        plan["client"] = True
        needs["changes"]["outputs"]["plan"] = json.dumps(plan)
        needs["client"]["result"] = "success"
        verify("player", needs)
        broken = copy.deepcopy(needs)
        broken["client"]["result"] = "skipped"
        with self.assertRaises(ValueError):
            verify("player", broken)


if __name__ == "__main__":
    unittest.main()
