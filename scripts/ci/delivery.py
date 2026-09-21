#!/usr/bin/env python3
"""Wait for exact-commit sibling CI; emit only validated deployment targets."""
import json
import os
import re
import subprocess
import time

from affected import APPS, FLAGS

WORKFLOWS = ("player-hygiene.yml", "subtitles-hygiene.yml", "dashboard-hygiene.yml", "security.yml")


def targets(raw):
    if len(raw) > 16384:
        raise ValueError("delivery plan too large")
    plan = json.loads(raw)
    if not isinstance(plan, dict) or set(plan) != {*FLAGS, "deep"} or any(type(v) is not bool for v in plan.values()):
        raise ValueError("invalid delivery plan")
    return [app for app in APPS if plan[app]]


def conclusion(payload, commit):
    if not isinstance(payload, dict) or not isinstance(payload.get("workflow_runs"), list):
        raise ValueError("invalid workflow response")
    runs = payload["workflow_runs"]
    if not runs:
        return False
    if len(runs) > 5 or any(type(run.get("id")) is not int for run in runs):
        raise ValueError("invalid workflow inventory")
    run = max(runs, key=lambda item: item["id"])
    if run.get("head_sha") != commit or run.get("event") != "push" or run.get("head_branch") != "main":
        raise ValueError("workflow is not for this main commit")
    if run.get("status") in ("queued", "in_progress", "waiting", "pending", "requested"):
        return False
    if run.get("status") != "completed" or run.get("conclusion") != "success":
        raise ValueError("required workflow did not succeed")
    return True


def main():
    apps = targets(os.environ["CI_PLAN"])
    commit, repo = os.environ["GITHUB_SHA"], os.environ["GITHUB_REPOSITORY"]
    if (not re.fullmatch(r"[0-9a-f]{40}", commit) or repo != "Kinosail/kinosail"
            or os.environ["GITHUB_EVENT_NAME"] != "push" or os.environ["GITHUB_REF"] != "refs/heads/main"):
        raise ValueError("delivery requires a protected main push")
    subprocess.run(["git", "merge-base", "--is-ancestor", commit, "origin/main"], check=True)
    deadline = time.monotonic() + 1200
    pending = set(WORKFLOWS)
    while pending and time.monotonic() < deadline:
        for workflow in sorted(pending):
            response = subprocess.run([
                "gh", "api", "--method", "GET", f"repos/{repo}/actions/workflows/{workflow}/runs",
                "-f", "head_sha=" + commit, "-f", "event=push", "-f", "branch=main", "-f", "per_page=5",
            ], capture_output=True, check=True, timeout=60)
            if len(response.stdout) > 4 * 1024 * 1024:
                raise ValueError("workflow response too large")
            if conclusion(json.loads(response.stdout), commit):
                pending.remove(workflow)
        if pending:
            print("Waiting for " + ", ".join(sorted(pending)), flush=True)
            time.sleep(15)
    if pending:
        raise ValueError("timed out waiting for required CI")
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        output.write("apps=" + json.dumps(apps) + "\n")


if __name__ == "__main__":
    main()
