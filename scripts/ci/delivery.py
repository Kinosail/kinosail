#!/usr/bin/env python3
"""Authorize container publication from this main run's required results."""

import json
import os
import re
import subprocess

from affected import APPS, FLAGS

REQUIRED = ("repository-required", "player-required", "subtitles-required",
            "dashboard-required", "security-required")


def targets(raw):
    if not isinstance(raw, str) or len(raw) > 16384:
        raise ValueError("invalid delivery plan")
    plan = json.loads(raw)
    if not isinstance(plan, dict) or set(plan) != {*FLAGS, "deep"} or any(type(v) is not bool for v in plan.values()):
        raise ValueError("invalid delivery plan")
    return [app for app in APPS if plan[app]]


def verify_results(raw, plan):
    if not isinstance(raw, str) or len(raw) > 65536:
        raise ValueError("invalid CI results")
    needs = json.loads(raw)
    if not isinstance(needs, dict) or set(needs) != {"plan", *REQUIRED}:
        raise ValueError("incomplete CI results")
    if any(not isinstance(needs[job], dict) or needs[job].get("result") != "success" for job in needs):
        raise ValueError("required CI did not succeed")
    outputs = needs["plan"].get("outputs")
    if not isinstance(outputs, dict) or outputs.get("plan") != plan:
        raise ValueError("delivery plan differs from verified selection")


def main():
    plan = os.environ["CI_PLAN"]
    apps = targets(plan)
    if not apps:
        raise ValueError("delivery has no selected app")
    verify_results(os.environ["CI_RESULTS"], plan)
    commit, repo = os.environ["GITHUB_SHA"], os.environ["GITHUB_REPOSITORY"]
    if (not re.fullmatch(r"[0-9a-f]{40}", commit) or repo != "Kinosail/kinosail"
            or os.environ["GITHUB_EVENT_NAME"] != "push" or os.environ["GITHUB_REF"] != "refs/heads/main"):
        raise ValueError("delivery requires a protected main push")
    subprocess.run(["git", "merge-base", "--is-ancestor", commit, "origin/main"], check=True)
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        output.write("apps=" + json.dumps(apps) + "\n")


if __name__ == "__main__":
    main()
