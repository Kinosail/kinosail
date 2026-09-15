#!/usr/bin/env python3
"""Generate the Player architecture snapshot through the shared generator."""

import subprocess
import sys
from pathlib import Path

tool = Path(__file__).resolve().parents[4] / "scripts" / "tooling" / "generate-architecture-explorer.py"
if len(sys.argv) != 1:
    raise SystemExit(f"usage: {sys.argv[0]}")
subprocess.run([sys.executable, str(tool), "player"], check=True)
