#!/usr/bin/env python3
"""Validate the selection contract of one reusable app suite."""

import json

from affected import APPS, FLAGS


def select(app, raw):
    if app not in APPS or not isinstance(raw, str) or len(raw) > 16384:
        raise ValueError("invalid app suite input")
    plan = json.loads(raw)
    if not isinstance(plan, dict) or set(plan) != {*FLAGS, "deep"} or any(type(value) is not bool for value in plan.values()):
        raise ValueError("invalid app suite plan")
    if not (plan[app] or (app == "player" and plan["client"])):
        raise ValueError("unselected app suite")
    for suffix in ("tools", "browsers", "arm"):
        if plan[f"{app}_{suffix}"] and not plan[app]:
            raise ValueError("app check selected without its app")
    return {"selected": plan[app], "tools": plan[f"{app}_tools"],
            "browsers": plan[f"{app}_browsers"], "arm": plan[f"{app}_arm"],
            "client": app == "player" and plan["client"]}
