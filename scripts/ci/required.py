#!/usr/bin/env python3
"""Fail closed when a selected check is missing, skipped, cancelled, or red."""

import json
import os
import sys

from affected import FLAGS
from plan import expected

SCOPES = {"repository": ("repository", "docs"),
          "app": ("go", "e2e", "proofs", "arm", "tools", "client", "android"),
          "security": ("security",)}


def parse_plan(raw):
    if not isinstance(raw, str) or len(raw) > 16384:
        raise ValueError("invalid selection plan")
    plan = json.loads(raw)
    if (not isinstance(plan, dict) or set(plan) != {*FLAGS, "deep", "verified"}
            or any(type(value) is not bool for value in plan.values())):
        raise ValueError("invalid selection plan")
    return plan


def verify(scope, needs, event_name):
    if scope not in SCOPES or not isinstance(needs, dict) or not isinstance(event_name, str):
        raise ValueError("invalid required-check scope or results")
    if not isinstance(needs.get("plan"), dict) or needs["plan"].get("result") != "success":
        raise ValueError("selection did not succeed")
    outputs = needs["plan"].get("outputs")
    plan = parse_plan(outputs.get("plan") if isinstance(outputs, dict) else None)
    wanted = expected(plan, event_name)
    jobs = SCOPES[scope]
    if set(needs) != {"plan", *jobs}:
        raise ValueError("required job inventory does not match workflow")
    for job in jobs:
        value = needs[job]
        result = value.get("result") if isinstance(value, dict) else None
        target = "success" if wanted[job] else "skipped"
        if result != target:
            raise ValueError(f"{job}: expected {target}, got {result}")
    return plan


def main():
    if len(sys.argv) != 2:
        raise ValueError("expected one required-check scope")
    raw = os.environ["RESULTS"]
    if len(raw) > 65536:
        raise ValueError("job results too large")
    plan = verify(sys.argv[1], json.loads(raw), os.environ["GITHUB_EVENT_NAME"])
    print("Every selected check passed; all skips match the selection plan"
          + (" (merge queue verified this commit)." if plan["verified"] else "."))


if __name__ == "__main__":
    main()
