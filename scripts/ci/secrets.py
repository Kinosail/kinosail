#!/usr/bin/env python3
"""Scan all newly introduced commits; scheduled runs audit complete history."""
import json
import os
from pathlib import Path
import re
import subprocess


def log_range(name, event, head):
    if not isinstance(event, dict) or not re.fullmatch(r"[0-9a-f]{40}", head):
        raise ValueError("invalid secret scan event")
    if name in ("schedule", "workflow_dispatch"):
        return "--all"
    if name == "pull_request":
        base, head = event["pull_request"]["base"]["sha"], event["pull_request"]["head"]["sha"]
    elif name == "push":
        base = event["before"]
    elif name == "merge_group":
        base, head = event["merge_group"]["base_sha"], event["merge_group"]["head_sha"]
    else:
        raise ValueError("unsupported secret scan event")
    if any(not isinstance(ref, str) or not re.fullmatch(r"[0-9a-f]{40}", ref) for ref in (base, head)):
        raise ValueError("invalid secret scan commits")
    return "--all" if base == "0" * 40 else base + ".." + head


def main():
    path = Path(os.environ["GITHUB_EVENT_PATH"])
    if path.stat().st_size > 16 * 1024 * 1024:
        raise ValueError("CI event too large")
    selected = log_range(os.environ["GITHUB_EVENT_NAME"], json.loads(path.read_text()), os.environ["GITHUB_SHA"])
    subprocess.run(["gitleaks", "git", "--redact", "--no-banner", "--log-opts=" + selected], check=True)


if __name__ == "__main__":
    main()
