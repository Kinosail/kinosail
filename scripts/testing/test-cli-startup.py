#!/usr/bin/env python3
"""Exercise a real app's read-only version command and retain private evidence."""
import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import platform
import subprocess
import time


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--app", choices=("player", "subtitles"), required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[2]
    output = args.output.resolve()
    output.mkdir(mode=0o700, parents=True, exist_ok=False)
    revision = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip()
    patch = subprocess.check_output(["git", "diff", "HEAD"], cwd=root)
    binary = output / "kinosail"
    version = "qa-cli-startup"
    build = ["go", "-C", f"apps/{args.app}", "build", "-trimpath",
             "-ldflags", f"-X main.version={version}", "-o", str(binary), "./cmd/kinosail"]
    state = output / "state"
    environment = {key: value for key, value in os.environ.items() if not key.startswith("KINOSAIL_")}
    for key, folder in (("DATA", "config"), ("CACHE", "cache"), ("BACKUP", "backups"), ("MEDIA", "media")):
        path = state / folder
        path.mkdir(parents=True)
        environment[f"KINOSAIL_{key}_DIR"] = str(path)
    command = [str(binary), "version"]
    checksum = lambda path: hashlib.sha256(path.read_bytes()).hexdigest()
    receipt = {
        "revision": revision, "workingDiffSHA256": hashlib.sha256(patch).hexdigest(),
        "app": args.app, "environment": platform.platform(), "build": build,
        "command": command, "data": "Disposable empty state; no user configuration or credentials",
        "startedUTC": datetime.now(timezone.utc).isoformat(),
        "budgetSeconds": 15, "elapsedSeconds": None, "timedOut": False,
        "exitCode": None, "passed": False,
        "runnerSHA256": checksum(Path(__file__)),
    }
    try:
        receipt["goVersion"] = subprocess.check_output(["go", "version"], text=True).strip()
        with (output / "build.log").open("w") as log:
            subprocess.run(build, cwd=root, stdout=log, stderr=log, check=True, timeout=600)
        receipt["binarySHA256"] = checksum(binary)
        started = time.monotonic()
        with (output / "version.log").open("w") as log:
            try:
                result = subprocess.run(command, cwd=output, env=environment,
                                        stdout=log, stderr=log, timeout=15)
                receipt["exitCode"] = result.returncode
            except subprocess.TimeoutExpired:
                receipt["timedOut"] = True
        receipt["elapsedSeconds"] = time.monotonic() - started
        receipt["passed"] = not receipt["timedOut"] and receipt["exitCode"] == 0 and (output / "version.log").read_text() == version + "\n"
    finally:
        receipt["logsSHA256"] = {name: checksum(output / name) for name in ("build.log", "version.log") if (output / name).exists()}
        receipt["sourceUnchanged"] = revision == subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip() and patch == subprocess.check_output(["git", "diff", "HEAD"], cwd=root)
        (output / "receipt.json").write_text(json.dumps(receipt, indent=2) + "\n")
        print(json.dumps({"app": args.app, "passed": receipt["passed"], "elapsedSeconds": receipt["elapsedSeconds"],
                          "receipt": str(output / "receipt.json")}))
    return 0 if receipt["passed"] and receipt["sourceUnchanged"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
