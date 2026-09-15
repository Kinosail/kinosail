#!/usr/bin/env python3
"""Summarize one explicitly matched cohort of exported native playback samples."""
from __future__ import annotations

import argparse
import json
import math
import os
from pathlib import Path
import re
import stat
import sys

MAX_MS = 31_622_400_000
FIELDS = {"engine", "outcome", "firstFrameMs", "firstProgressMs", "seekProgressMs", "seeks", "incompleteSeeks", "stalls", "stallMs", "elapsedMs"}
COHORT = {"device", "os", "network", "outputRoute", "cache", "fixtureSha256", "appRevision", "serverRevision"}


def number(value: object, maximum: float, integer: bool = False) -> bool:
    return type(value) in (float, int) and 0 <= value <= maximum and math.isfinite(value) and (not integer or int(value) == value)


def exact(value: object, fields: set[str]) -> dict:
    if not isinstance(value, dict) or set(value) != fields:
        raise ValueError("missing or unknown report fields")
    return value


def validate(value: object) -> dict:
    document = exact(value, {"cohort", "samples"})
    cohort = exact(document["cohort"], COHORT)
    for key, field in cohort.items():
        pattern = r"[a-f0-9]{64}" if key == "fixtureSha256" else r"[A-Za-z0-9][A-Za-z0-9_.-]{0,79}"
        if not isinstance(field, str) or not re.fullmatch(pattern, field):
            raise ValueError("invalid cohort identity")
    if cohort["cache"] not in {"cold", "warm"}:
        raise ValueError("invalid cache cohort")
    samples = document["samples"]
    if not isinstance(samples, list) or not 1 <= len(samples) <= 10_000:
        raise ValueError("sample count must be 1..10000")
    for raw in samples:
        sample = exact(raw, FIELDS)
        if sample["engine"] not in ("platform", "vlc") or sample["outcome"] not in ("ended", "error", "closed"):
            raise ValueError("invalid playback outcome")
        for key in ("elapsedMs", "stallMs"):
            if not number(sample[key], MAX_MS):
                raise ValueError("invalid playback duration")
        for key in ("firstFrameMs", "firstProgressMs"):
            if sample[key] is not None and not number(sample[key], sample["elapsedMs"]):
                raise ValueError("invalid startup measurement")
        for key in ("seeks", "incompleteSeeks", "stalls"):
            if not number(sample[key], 1_000_000, True):
                raise ValueError("invalid event count")
        seeks = sample["seekProgressMs"]
        if not isinstance(seeks, list) or len(seeks) > 128 or any(not number(seek, sample["elapsedMs"]) for seek in seeks):
            raise ValueError("invalid seek measurements")
        if sample["incompleteSeeks"] + len(seeks) > sample["seeks"] or sample["stallMs"] > sample["elapsedMs"]:
            raise ValueError("conflicting playback measurements")
    return document


def distribution(values: list[float]) -> dict:
    ordered = sorted(values)
    def quantile(fraction: float) -> float | None:
        if not ordered:
            return None
        position = (len(ordered) - 1) * fraction
        lower, upper = math.floor(position), math.ceil(position)
        return ordered[lower] + (ordered[upper] - ordered[lower]) * (position - lower)
    return {"samples": len(ordered), "medianMs": quantile(.5), "p95Ms": quantile(.95), "p95ScreeningOnly": len(ordered) < 200}


def summarize(value: object) -> dict:
    document = validate(value)
    engines = {}
    for engine in ("platform", "vlc"):
        samples = [sample for sample in document["samples"] if sample["engine"] == engine]
        if not samples:
            continue
        engines[engine] = {
            "attempts": len(samples),
            "errors": sum(sample["outcome"] == "error" for sample in samples),
            "closed": sum(sample["outcome"] == "closed" for sample in samples),
            "withoutFirstFrame": sum(sample["firstFrameMs"] is None for sample in samples),
            "firstFrame": distribution([sample["firstFrameMs"] for sample in samples if sample["firstFrameMs"] is not None]),
            "firstTimelineProgress": distribution([sample["firstProgressMs"] for sample in samples if sample["firstProgressMs"] is not None]),
            "seekTimelineProgress": distribution([seek for sample in samples for seek in sample["seekProgressMs"]]),
            "seeks": sum(sample["seeks"] for sample in samples),
            "incompleteSeeks": sum(sample["incompleteSeeks"] for sample in samples),
            "stalls": sum(sample["stalls"] for sample in samples),
            "stallMs": sum(sample["stallMs"] for sample in samples),
        }
    return {"cohort": document["cohort"], "engines": engines,
            "limitations": "Timeline progress is not rendered-frame or audible-output proof. Native controls can issue unobserved seeks. Frame drops, A/V sync and energy require device instrumentation. Keep failures alongside timing distributions."}


def unique_object(pairs: list[tuple[str, object]]) -> dict:
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate report field")
        result[key] = value
    return result


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("input", type=Path, help="one cohort plus exported playbackMetrics() samples")
    args = parser.parse_args(argv)
    try:
        with os.fdopen(os.open(args.input, os.O_RDONLY | os.O_NONBLOCK), "rb") as source:
            if not stat.S_ISREG(os.fstat(source.fileno()).st_mode):
                raise ValueError("report must be a regular file")
            raw = source.read(4 * 1024 * 1024 + 1)
        if len(raw) > 4 * 1024 * 1024:
            raise ValueError("report exceeds 4 MiB")
        report = summarize(json.loads(raw, object_pairs_hook=unique_object))
    except (OSError, ValueError, TypeError, OverflowError, RecursionError):
        print("Invalid playback report; no summary was produced.", file=sys.stderr)
        return 2
    print(json.dumps(report, indent=2, allow_nan=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
