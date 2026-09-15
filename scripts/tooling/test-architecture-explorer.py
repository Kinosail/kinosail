#!/usr/bin/env python3
"""Verify the shared Code Atlas generator and its side-effect boundaries."""

from __future__ import annotations

import hashlib
import json
import re
import subprocess
import sys
from pathlib import Path


REPO = Path(__file__).resolve().parents[2]
APPS = ("player", "subtitles")
OUTPUTS = tuple(
    REPO / "apps" / app / area / "architecture-explorer" / "index.html"
    for app in APPS
    for area in ("engineering", "docs")
)


def digest(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


before = {path: digest(path) for path in OUTPUTS}
commands = [
    [sys.executable, str(REPO / "scripts/tooling/generate-architecture-explorer.py")],
    [sys.executable, str(REPO / "scripts/tooling/generate-architecture-explorer.py"), "unknown"],
]
commands.extend(
    [sys.executable, str(REPO / f"apps/{app}/engineering/architecture-explorer/generate.py"), "unknown"]
    for app in APPS
)
for command in commands:
    if subprocess.run(command, cwd=REPO, capture_output=True, check=False).returncode == 0:
        raise SystemExit(f"invalid generator input was accepted: {' '.join(command)}")
if before != {path: digest(path) for path in OUTPUTS}:
    raise SystemExit("invalid generator input changed an architecture snapshot")

for app in APPS:
    engineering = REPO / f"apps/{app}/engineering/architecture-explorer/index.html"
    published = REPO / f"apps/{app}/docs/architecture-explorer/index.html"
    if engineering.read_bytes() != published.read_bytes():
        raise SystemExit(f"{app} architecture snapshots differ")
    html = engineering.read_text(encoding="utf-8")
    match = re.search(r"const snapshot = (\{.*\});\n", html)
    if not match:
        raise SystemExit(f"{app} architecture snapshot data is missing")
    snapshot = json.loads(match.group(1))
    if snapshot.get("commit") != "main" or "blob/main/" not in html:
        raise SystemExit(f"{app} architecture source links do not target main")
    if "graph.parentElement.style.height = `${height}px`" not in html:
        raise SystemExit(f"{app} architecture map does not grow with its package rows")
    if 'simulation.force("link").strength(0)' not in html:
        raise SystemExit(f"{app} compact package grid can drift into overlapping rows")
    for package in snapshot["packages"]:
        for file in package["files"]:
            if not (REPO / file["path"]).is_file():
                raise SystemExit(f"{app} architecture source is missing: {file['path']}")

print("Architecture explorer tests passed")
