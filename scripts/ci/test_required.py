"""Protected checks fail closed on skipped, missing, or cancelled work."""

import copy
import json
import unittest

from affected import FLAGS
from required import verify


def case(scope, selected=True, app="player"):
    plan = dict.fromkeys((*FLAGS, "deep"), selected)
    raw = json.dumps(plan)
    needs = {} if scope == "app" else {"plan": {"result": "success", "outputs": {"plan": raw}}}
    if scope == "repository":
        jobs = ("static", "tooling", "packages", "web", "docs")
    elif scope == "security":
        jobs = ("secrets", "codeql", "supply-chain", "findings")
    else:
        jobs = ("static", "race", "security", "system", "browser", "tooling", "client")
    needs.update({job: {"result": "success" if selected else "skipped"} for job in jobs})
    if scope == "repository":
        needs["static"]["result"] = "success"
    if scope == "security":
        needs["secrets"]["result"] = "success"
    if scope == "app" and app != "player":
        needs["client"]["result"] = "skipped"
    if scope == "app" and app == "dashboard":
        needs["tooling"]["result"] = "skipped"
    return raw, needs


class RequiredTests(unittest.TestCase):
    def test_selected_and_intentionally_unselected_jobs(self):
        for scope in ("repository", "security"):
            for selected in (True, False):
                raw, needs = case(scope, selected)
                verify(scope, needs)
        for app in ("player", "subtitles", "dashboard"):
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
        for app in (None, "../player", "dashboard"):
            with self.subTest(app=app), self.assertRaises(ValueError):
                verify("app", needs, raw, app)

    def test_native_only_requires_client_without_server(self):
        plan = dict.fromkeys((*FLAGS, "deep"), False)
        plan["client"] = True
        raw = json.dumps(plan)
        needs = {**{job: {"result": "skipped"} for job in ("static", "race", "security", "system", "browser", "tooling")},
                 "client": {"result": "success"}}
        verify("app", needs, raw, "player")
        needs["client"]["result"] = "skipped"
        with self.assertRaises(ValueError):
            verify("app", needs, raw, "player")

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


if __name__ == "__main__":
    unittest.main()
