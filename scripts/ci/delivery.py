#!/usr/bin/env python3
"""Validate the completed CI graph before emitting deployment targets."""
import json
import os
import re
import subprocess

from affected import APPS, FLAGS

GATES = ("changes", "required", "player-required", "subtitles-required", "dashboard-required", "security-required")


def targets(raw):
    if len(raw) > 16384:
        raise ValueError("delivery plan too large")
    plan = json.loads(raw)
    if not isinstance(plan, dict) or set(plan) != {*FLAGS, "deep"} or any(type(v) is not bool for v in plan.values()):
        raise ValueError("invalid delivery plan")
    return [app for app in APPS if plan[app]]


def verify_results(raw, plan):
    if len(raw) > 65536:
        raise ValueError("CI results too large")
    results = json.loads(raw)
    if not isinstance(results, dict) or set(results) != set(GATES):
        raise ValueError("invalid CI gate inventory")
    for name in GATES:
        result = results[name]
        if not isinstance(result, dict) or result.get("result") != "success":
            raise ValueError(f"{name}: required CI did not succeed")
    if results["changes"].get("outputs", {}).get("plan") != plan:
        raise ValueError("delivery plan differs from verified selection")


def main():
    plan = os.environ["CI_PLAN"]
    apps = targets(plan)
    verify_results(os.environ["RESULTS"], plan)
    commit, repo = os.environ["GITHUB_SHA"], os.environ["GITHUB_REPOSITORY"]
    if (not apps or not re.fullmatch(r"[0-9a-f]{40}", commit) or repo != "Kinosail/kinosail"
            or os.environ["GITHUB_EVENT_NAME"] != "push" or os.environ["GITHUB_REF"] != "refs/heads/main"):
        raise ValueError("delivery requires a protected main push with affected apps")
    subprocess.run(["git", "merge-base", "--is-ancestor", commit, "origin/main"], check=True, timeout=60)
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        output.write("apps=" + json.dumps(apps) + "\n")


if __name__ == "__main__":
    main()
