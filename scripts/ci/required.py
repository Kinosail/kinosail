#!/usr/bin/env python3
"""Reject failed, cancelled, missing, or unexpectedly skipped required jobs."""

import json
import os
import sys

from affected import APPS, FLAGS, LANGUAGES


def verify(scope, needs, raw_plan=None):
    if scope not in (*APPS, "repository", "security") or not isinstance(needs, dict):
        raise ValueError("invalid required-check scope or results")
    selection = {"changes"} if raw_plan is None else set()
    if raw_plan is None:
        if needs.get("changes", {}).get("result") != "success":
            raise ValueError("change detection did not succeed")
        raw_plan = needs["changes"]["outputs"]["plan"]
    if not isinstance(raw_plan, str) or len(raw_plan) > 16384:
        raise ValueError("invalid selection plan")
    plan = json.loads(raw_plan)
    if (not isinstance(plan, dict) or set(plan) != {*FLAGS, "deep"}
            or any(type(value) is not bool for value in plan.values())):
        raise ValueError("invalid selection plan")
    if scope in APPS:
        expected = dict.fromkeys(("static", "race", "security", "system"), plan[scope])
        if scope != "dashboard":
            expected["images"] = plan[scope]
            expected["tooling"] = plan[f"{scope}_tools"]
            expected["scheduled"] = plan["deep"]
        if scope == "player":
            expected["client"] = plan["client"]
    elif scope == "repository":
        expected = {"static": True, "tooling": plan["tooling"], "packages": plan["packages"],
                    "web": plan["web"]}
    else:
        codeql = any(plan[language] for language in LANGUAGES if language != "swift")
        expected = {"secrets": True, "supply-chain": plan["supply"], "codeql": codeql, "swift": plan["swift"], "findings": codeql or plan["swift"]}
    if set(needs) != {*selection, *expected}:
        raise ValueError("required job inventory does not match workflow")
    for job, selected in expected.items():
        result = needs[job].get("result")
        if result != ("success" if selected else "skipped"):
            raise ValueError(f"{job}: expected {'success' if selected else 'skipped'}, got {result}")


def main():
    if len(sys.argv) != 2:
        raise ValueError("expected one required-check scope")
    raw = os.environ["RESULTS"]
    if len(raw) > 65536:
        raise ValueError("job results too large")
    verify(sys.argv[1], json.loads(raw), os.environ.get("CI_PLAN"))
    print("Every selected check passed; all skips match the selection plan.")


if __name__ == "__main__":
    main()
