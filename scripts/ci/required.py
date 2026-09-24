#!/usr/bin/env python3
"""Fail closed when a selected check is missing, skipped, cancelled, or red."""

import json
import os
import sys

from affected import FLAGS, LANGUAGES
from app_plan import select


def parse_plan(raw):
    if not isinstance(raw, str) or len(raw) > 16384:
        raise ValueError("invalid selection plan")
    plan = json.loads(raw)
    if not isinstance(plan, dict) or set(plan) != {*FLAGS, "deep"} or any(type(value) is not bool for value in plan.values()):
        raise ValueError("invalid selection plan")
    return plan


def verify(scope, needs, raw_plan=None, app=None):
    if scope not in ("repository", "security", "app") or not isinstance(needs, dict):
        raise ValueError("invalid required-check scope or results")
    if scope != "app" and (not isinstance(needs.get("plan"), dict)
                           or needs["plan"].get("result") != "success"):
        raise ValueError("selection did not succeed")
    if scope != "app":
        outputs = needs["plan"].get("outputs")
        raw_plan = outputs.get("plan") if isinstance(outputs, dict) else None
    plan = parse_plan(raw_plan)
    if scope == "repository":
        expected = {"static": True, "tooling": plan["tooling"], "packages": plan["packages"],
                    "web": plan["web"], "docs": plan["docs"]}
    elif scope == "security":
        codeql = any(plan[language] for language in LANGUAGES)
        expected = {"secrets": True, "codeql": codeql, "supply-chain": plan["supply"],
                    "findings": codeql}
    else:
        select(app, raw_plan)
        expected = dict.fromkeys(("static", "race", "security", "system", "browser"), plan[app])
        expected["tooling"] = app != "dashboard" and plan[f"{app}_tools"]
        expected["client"] = app == "player" and plan["client"]
        expected["android"] = app == "player" and plan["android"]
    if set(needs) != ({"plan", *expected} if scope != "app" else set(expected)):
        raise ValueError("required job inventory does not match workflow")
    for job, selected in expected.items():
        value = needs[job]
        result = value.get("result") if isinstance(value, dict) else None
        if result != ("success" if selected else "skipped"):
            raise ValueError(f"{job}: expected {'success' if selected else 'skipped'}, got {result}")


def main():
    if len(sys.argv) != 2:
        raise ValueError("expected one required-check scope")
    raw = os.environ["RESULTS"]
    if len(raw) > 65536:
        raise ValueError("job results too large")
    verify(sys.argv[1], json.loads(raw), os.environ.get("CI_PLAN"), os.environ.get("CI_APP"))
    print("Every selected check passed; all skips match the selection plan.")


if __name__ == "__main__":
    main()
