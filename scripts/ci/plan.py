#!/usr/bin/env python3
"""Emit one selection plan for the fast CI path; a main push already tested by the merge queue only publishes."""

import json
import os
from pathlib import Path
import re
import subprocess

from affected import APPS, LANGUAGES, affected, changed_paths
from proof_shard import load, plan as proof_plan

KEYS = ("plan", "codeql", "compiled", "modules", "apps", "shards", "arm", "tools", "verified")
INTERPRETED = ("javascript-typescript", "python", "actions")
COMPILED = ("go", "java-kotlin")
MODULES = ({"module": "player", "directory": "apps/player", "media": "true", "contract": "single-container tidy-check"},
           {"module": "subtitles", "directory": "apps/subtitles", "media": "false", "contract": "single-container tidy-check"},
           {"module": "packages", "directory": "packages", "media": "false", "contract": "tidy-check"})
PHASES = {"player": (("journeys", "true", "3"), ("populated", "false", "1")), "subtitles": (("all", "true", "1"),)}


def derive(plan):
    if not isinstance(plan, dict) or any(type(value) is not bool for value in plan.values()):
        raise ValueError("invalid selection plan")
    selected = {app for app in APPS if plan[app]}
    return {
        "codeql": ",".join(name for name in INTERPRETED if plan[name]),
        "compiled": ",".join(name for name in COMPILED if plan[name]),
        "modules": [entry for entry in MODULES if plan[entry["module"]]],
        "apps": [{"app": app, "phase": phase, "system": system, "workers": workers}
                 for app in APPS if app in selected for phase, system, workers in PHASES[app]],
        "shards": proof_plan("player", load()) if plan["player"] else [],
        "arm": [app for app in APPS if plan[f"{app}_arm"]],
        "tools": [app for app in APPS if plan[f"{app}_tools"]],
    }


def expected(plan, event_name):
    verified = plan["verified"]
    derived = derive({key: value for key, value in plan.items() if key not in ("deep", "verified")})
    live = not verified
    return {
        "repository": live,
        "docs": plan["docs"] and (live or event_name == "push"),
        "go": live and bool(derived["modules"]),
        "e2e": live and bool(derived["apps"]),
        "proofs": live and plan["player"],
        "arm": live and bool(derived["arm"]),
        "tools": live and bool(derived["tools"]),
        "client": live and plan["client"],
        "android": live and plan["android"],
        "security": live or event_name == "push",
        "codeql-compiled": (live or event_name == "push") and bool(derived["compiled"]),
    }


def tested_run(sha, repository):
    if not re.fullmatch(r"[0-9a-f]{40}", sha) or not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repository):
        raise ValueError("invalid commit or repository")
    result = subprocess.run(
        ["gh", "run", "list", "--repo", repository, "--workflow", "ci.yml", "--commit", sha, "--event", "merge_group",
         "--status", "success", "--limit", "5", "--json", "databaseId,headSha,status,conclusion"],
        capture_output=True, text=True, timeout=60, check=False)
    if result.returncode != 0 or len(result.stdout) > 65536:
        return None
    try:
        runs = json.loads(result.stdout)
    except ValueError:
        return None
    for run in runs if isinstance(runs, list) else []:
        if (isinstance(run, dict) and run.get("headSha") == sha and run.get("status") == "completed"
                and run.get("conclusion") == "success" and type(run.get("databaseId")) is int):
            return run["databaseId"]
    return None


def build(event_name, event, sha, repository, full, deep):
    paths = changed_paths(event_name, event, sha, full)
    plan = affected(paths)
    plan["deep"] = deep
    run = None
    if event_name == "push" and event.get("ref") == "refs/heads/main" and not full and not deep:
        run = tested_run(sha, repository)
    plan["verified"] = run is not None
    derived = derive({key: value for key, value in plan.items() if key not in ("deep", "verified")})
    return plan, derived, run


def main():
    event_path = Path(os.environ["GITHUB_EVENT_PATH"])
    if event_path.stat().st_size > 16 * 1024 * 1024:
        raise ValueError("CI event too large")
    flags = {name: os.environ.get(name, "false") for name in ("CI_FULL", "CI_DEEP")}
    if any(value not in ("true", "false") for value in flags.values()):
        raise ValueError("invalid full or deep selection")
    event = json.loads(event_path.read_text())
    if not isinstance(event, dict):
        raise ValueError("invalid CI event")
    plan, derived, run = build(os.environ["GITHUB_EVENT_NAME"], event, os.environ["GITHUB_SHA"],
                               os.environ["GITHUB_REPOSITORY"], flags["CI_FULL"] == "true", flags["CI_DEEP"] == "true")
    outputs = {"plan": json.dumps(plan, separators=(",", ":")), "verified": "true" if plan["verified"] else "false",
               **{key: value if key in ("codeql", "compiled") else json.dumps(value, separators=(",", ":"))
                  for key, value in derived.items()}}
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        output.writelines(f"{key}={outputs[key]}\n" for key in KEYS)
    selected = ", ".join(key for key, value in plan.items() if value)
    print(f"Selected checks: {selected}" + (f"; merge queue run {run} already verified this commit" if run else ""))
    if os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as summary:
            summary.write("### Affected checks\n\n```json\n" + json.dumps(plan, indent=2) + "\n```\n")
            if run:
                summary.write(f"\nMerge queue run {run} verified this exact commit; this run only publishes.\n")


if __name__ == "__main__":
    main()
