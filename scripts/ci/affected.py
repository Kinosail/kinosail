#!/usr/bin/env python3
"""Select complete app suites from Git; uncertainty never silently skips tests."""

import json
import os
from pathlib import Path, PurePosixPath
import re
import subprocess

APPS = ("player", "subtitles", "dashboard")
FLAGS = (*APPS, "packages", "tooling", "web", "client", "supply",
         "go", "javascript-typescript", "python",
         *(f"{app}_{kind}" for app in APPS for kind in ("tools", "browsers", "arm")))


def affected(paths):
    selected = dict.fromkeys(FLAGS, False)
    if paths is None:
        return dict.fromkeys(FLAGS, True)
    if not isinstance(paths, list) or len(paths) > 100000:
        raise ValueError("invalid changed path list")
    for path in paths:
        if (not isinstance(path, str) or not 0 < len(path) <= 4096
                or path.startswith("/") or "\0" in path
                or any(part in ("", ".", "..") for part in path.split("/"))):
            raise ValueError("invalid repository path")
    for path in paths:
        parts = PurePosixPath(path).parts
        # Only prose is exempt. Embedded assets and unfamiliar build inputs run CI.
        if (path in ("README.md", "AGENTS.md")
                or (path.endswith(".md") and path.startswith(("engineering/", "docs/")))
                or (len(parts) == 3 and parts[0] == "apps" and parts[-1] in ("README.md", "AGENTS.md", "DESIGN.md", "CONTEXT.md"))
                or (len(parts) > 3 and parts[0] == "apps" and parts[2] == "docs" and path.endswith(".md"))):
            continue
        if path.startswith("apps/player/apps/native/"):
            selected["client"] = True
            selected["supply"] = True
            continue
        if path.startswith("apps/") and len(parts) > 2 and parts[1] in APPS:
            app = parts[1]
            if parts[-1] == "go.mod":
                return dict.fromkeys(FLAGS, True)  # Workspace MVS affects every module.
            selected[app] = selected["go"] = True
            relative = "/".join(parts[2:])
            if relative.startswith(("internal/server/", "e2e/")):
                selected[f"{app}_browsers"] = selected["web"] = True
                selected["javascript-typescript"] = True
            if (relative.startswith("scripts/") or parts[-1].startswith(("Containerfile", "compose"))
                    or relative in ("Makefile", "go.sum")):
                selected[f"{app}_tools"] = selected[f"{app}_arm"] = True
                selected["supply"] = True
            if app == "player" and relative == "Makefile":
                selected["client"] = True
            if relative.endswith((".js", ".mjs", ".ts", ".tsx", ".html", ".css")):
                selected[f"{app}_browsers"] = selected["web"] = True
                selected["javascript-typescript"] = True
            if relative.endswith((".py",)):
                selected["python"] = True
            if parts[-1] in ("package.json", "pnpm-lock.yaml"):
                selected["supply"] = True
        elif path.startswith("packages/"):
            selected["packages"] = selected["go"] = True
            for app in APPS:
                selected[app] = True
            if path.endswith(("go.mod", "go.sum")):
                selected["supply"] = True
            if path.startswith(("packages/webassets/", "packages/playerweb/", "packages/servertest/")):
                selected["web"] = selected["javascript-typescript"] = True
                for app in APPS:
                    selected[f"{app}_browsers"] = True
        else:
            return dict.fromkeys(FLAGS, True)
    return selected


def changed_paths(event_name, event, head, full=False):
    if type(full) is not bool or not isinstance(event, dict):
        raise ValueError("invalid CI event")
    if event_name not in ("pull_request", "push", "schedule", "workflow_dispatch", "workflow_call", "merge_group"):
        raise ValueError("unsupported CI event")
    if full or event_name in ("schedule", "workflow_dispatch", "workflow_call"):
        return None
    if event_name == "pull_request":
        base = event["pull_request"]["base"]["sha"]
        head = event["pull_request"]["head"]["sha"]
        separator = "..."
    elif event_name == "merge_group":
        base, head = event["merge_group"]["base_sha"], event["merge_group"]["head_sha"]
        separator = ".."
    else:
        base, separator = event["before"], ".."
    for ref in (base, head):
        if not isinstance(ref, str) or not re.fullmatch(r"[0-9a-f]{40}|[0-9a-f]{64}", ref):
            raise ValueError("invalid CI commit")
    if set(base) == {"0"}:
        return None
    result = subprocess.run(
        ["git", "diff", "--name-only", "--no-renames", "-z", base + separator + head, "--"],
        check=True, capture_output=True, timeout=60,
    )
    if len(result.stdout) > 32 * 1024 * 1024 or (result.stdout and not result.stdout.endswith(b"\0")):
        raise ValueError("invalid or oversized Git diff")
    return result.stdout.decode("utf-8", errors="surrogateescape").split("\0")[:-1]


def main():
    event_path = Path(os.environ["GITHUB_EVENT_PATH"])
    if event_path.stat().st_size > 16 * 1024 * 1024:
        raise ValueError("CI event too large")
    full = os.environ.get("CI_FULL", "false")
    if full not in ("true", "false"):
        raise ValueError("invalid full-suite selection")
    event = json.loads(event_path.read_text())
    paths = changed_paths(os.environ["GITHUB_EVENT_NAME"], event, os.environ["GITHUB_SHA"], full == "true")
    plan = affected(paths)
    plan["deep"] = os.environ["GITHUB_EVENT_NAME"] in ("schedule", "workflow_dispatch")
    encoded = json.dumps(plan, separators=(",", ":"))
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        output.write("plan=" + encoded + "\n")
        output.write("languages=" + json.dumps([name for name in ("go", "javascript-typescript", "python") if plan[name]]) + "\n")
    print("Selected checks: " + ", ".join(key for key, value in plan.items() if value))
    if os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as summary:
            summary.write("### Affected checks\n\n```json\n" + json.dumps(plan, indent=2) + "\n```\n")


if __name__ == "__main__":
    main()
