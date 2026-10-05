#!/usr/bin/env python3
"""Fixed dependency inspection stages only; never retain exception details or paths."""
STAGES = (
    "go-resolution", "node-resolution", "ruby-resolution", "bundle-resolution",
    "go-fingerprint", "node-fingerprint", "ruby-fingerprint", "bundle-fingerprint",
    "playwright-test-metadata", "playwright-test-tree", "playwright-metadata", "playwright-tree",
    "playwright-core-metadata", "playwright-core-tree", "cli-resolution", "cli-fingerprint",
    "registry-read", "cache-resolution", "cache-validation", "chromium-registry-entry",
    "chromium-executable", "chromium-fingerprint", "headless-registry-entry",
    "headless-executable", "headless-fingerprint", "inspection-complete",
)
_current = None


def reset_stage():
    global _current
    _current = None


def record_stage(stage):
    global _current
    if type(stage) is not str or stage not in STAGES:
        _current = None
        raise ValueError("dependency_stage_invalid")
    _current = stage


def current_stage():
    return _current if type(_current) is str and _current in STAGES else None


def receipt_stage(blocked_phase):
    return current_stage() if blocked_phase == "dependency-check" else None
