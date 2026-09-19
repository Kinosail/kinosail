#!/usr/bin/env python3

from __future__ import annotations

import subprocess
import tempfile
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
FILTER = ROOT / "scripts/quality/filter-duplicates.py"
OUTPUT = "apps/player/internal/server/one.go:1-10: duplicate of apps/subtitles/internal/server/two.go:1-10\n"


def run(changed: str) -> subprocess.CompletedProcess[str]:
    with tempfile.TemporaryDirectory() as directory:
        path = Path(directory) / "changed.txt"
        path.write_text(changed)
        return subprocess.run(["python3", str(FILTER), str(path)], input=OUTPUT, text=True, capture_output=True, check=False)


def main() -> None:
    assert run("packages/other.go\n").returncode == 0
    assert run("apps/player/internal/server/one.go\n").returncode == 1
    malformed = subprocess.run(["python3", str(FILTER), "/dev/null"], input="not dupl output\n", text=True, capture_output=True, check=False)
    assert malformed.returncode == 2


if __name__ == "__main__":
    main()
