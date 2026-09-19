#!/usr/bin/env python3
"""Fail when a duplicate-code finding touches a changed Go file."""

from __future__ import annotations

import re
import sys
from pathlib import Path


CLONE = re.compile(r"^(?P<left>.+?):\d+-\d+: duplicate of (?P<right>.+?):\d+-\d+$")


def main() -> int:
    changed = {Path(line.strip()).as_posix() for line in Path(sys.argv[1]).read_text().splitlines() if line.strip()}
    findings: list[str] = []
    for line in sys.stdin:
        line = line.rstrip("\n")
        if not line:
            continue
        match = CLONE.match(line)
        if match is None:
            print(f"unrecognized duplicate-code output: {line}", file=sys.stderr)
            return 2
        if match.group("left") in changed or match.group("right") in changed:
            findings.append(line)
    if findings:
        print("duplicate code touches changed Go files:", file=sys.stderr)
        for line in dict.fromkeys(findings):
            print(line, file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
