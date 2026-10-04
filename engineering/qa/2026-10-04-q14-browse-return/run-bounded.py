#!/usr/bin/env python3
"""Run one parent-scheduled fictional Go/browser slice with preserved receipts."""

import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import time

sys.dont_write_bytecode = True

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--cases", choices=["primary", "cold", "bfcache", "htmx", "shows", "search"], required=True)
parser.add_argument("--label", required=True, choices=["primary-red-1", "primary-red-2", "cold-red", "bfcache-control", "htmx-red", "shows-red", "search-red"])
parser.add_argument("--seconds", type=int, default=75)
args = parser.parse_args()
if not 30 <= args.seconds <= 120:
    parser.error("external bound must be 30..120 seconds")

root = Path(__file__).resolve().parents[3]
clip = root.parent / "r03-progress/.verification/r03-progress/20261004T075204Z/media/R03 Example.mp4"
clip_hash = hashlib.sha256(clip.read_bytes()).hexdigest()
if clip_hash != "9dbd85e7863d921e209978c8349992e5f8505b0d7ffa468c37b8caf97af3f0a4":
    raise SystemExit("authorized fictional clip checksum differs; do not run")

stamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%S.%fZ")
output = root / ".verification/q14-browse-return" / f"{stamp}-{args.label}"
output.mkdir(parents=True, exist_ok=False)
command = ["../../scripts/tooling/with-go-module.sh", "go", "test", "-v", "-p", "1", "./internal/server", "-run", "^TestBrowseReturnBrowserJourney$", "-count=1", f"-timeout={args.seconds - 5}s"]
selected = {
    "GOMAXPROCS": "2", "PLAYWRIGHT_CHANNEL": "", "KINOSAIL_BROWSER_PROJECT": "chromium",
    "KINOSAIL_BROWSER_WORKERS": "1", "KINOSAIL_E2E_VIDEO": "off",
    "KINOSAIL_BROWSE_RETURN_BROWSER": "1", "KINOSAIL_BROWSE_RETURN_CASES": args.cases,
    "KINOSAIL_BROWSE_RETURN_MEDIA": str(clip),
    "KINOSAIL_E2E_OUTPUT_DIR": str(output / "browser"),
    "KINOSAIL_E2E_ARTIFACT_DIR": str(output / "report"),
}
environment = os.environ.copy()
environment.update(selected)
revision = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip()
receipt = {"sourceRevision": revision, "cases": args.cases, "command": command, "cwd": str(root / "apps/player"), "environmentSelection": selected, "fixtureSHA256": clip_hash, "externalSeconds": args.seconds, "cleanupGraceSeconds": 5, "startedUTC": stamp}
receipt_path = output / "receipt.json"
receipt_path.write_text(json.dumps(receipt, indent=2) + "\n")
process = None
interrupted = None


def stop_owned_group():
    if process is None:
        return
    try:
        os.killpg(process.pid, signal.SIGTERM)
    except ProcessLookupError:
        return
    try:
        process.wait(timeout=3)
    except subprocess.TimeoutExpired:
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        process.wait(timeout=2)


def interrupted_signal(number, _frame):
    global interrupted
    interrupted = number
    stop_owned_group()


signal.signal(signal.SIGINT, interrupted_signal)
signal.signal(signal.SIGTERM, interrupted_signal)
start = time.monotonic()
timed_out = False
with (output / "go-browser.log").open("wb") as log:
    if interrupted:
        raise SystemExit(128 + interrupted)
    process = subprocess.Popen(command, cwd=root / "apps/player", env=environment, stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
    try:
        if interrupted:
            stop_owned_group()
        exit_code = process.wait(timeout=args.seconds)
    except subprocess.TimeoutExpired:
        timed_out = True
        stop_owned_group()
        exit_code = process.returncode
    finally:
        # This fresh process group belongs only to this one disposable run.
        stop_owned_group()

receipt.update({"exitCode": exit_code, "externalTimeout": timed_out, "interruptedSignal": interrupted, "elapsedSeconds": round(time.monotonic() - start, 3)})
report = output / "report/results-chromium.json"
if report.is_file():
    try:
        result = json.loads(report.read_text())
        receipt["rawStats"] = result.get("stats")
        receipt["globalErrorCount"] = len(result.get("errors", []))
    except (ValueError, OSError) as error:
        receipt["reportReadFailure"] = type(error).__name__
receipt["logSHA256"] = hashlib.sha256((output / "go-browser.log").read_bytes()).hexdigest()
receipt_path.write_text(json.dumps(receipt, indent=2) + "\n")
print(json.dumps({"receipt": str(receipt_path), "exitCode": exit_code, "externalTimeout": timed_out, "interruptedSignal": interrupted, "rawStats": receipt.get("rawStats")}, indent=2))
raise SystemExit(124 if timed_out else 128 + interrupted if interrupted else exit_code if exit_code is not None else 1)
