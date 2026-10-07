#!/usr/bin/env python3
"""Run one fixed real-Go BrowseReturn profile and require its complete safe proof."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import signal
import stat
import subprocess
import sys

from campaign_q14_admission import admit, complete

SPEC = importlib.util.spec_from_file_location("q14_runtime", Path(__file__).with_name("campaign-q14-public.py"))
driver = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(driver)
ROOT = Path(__file__).resolve().parents[3]
MODES = ("primary", "navigation", "safety", "home")
PROJECTS = ("chromium", "firefox", "webkit")
INPUTS = ("apps/player/scripts/run-browse-return.py", "apps/player/scripts/campaign-q14-public.py",
          "apps/player/scripts/campaign_q14_admission.py", "apps/player/scripts/campaign_q14_suites.py",
          "apps/player/internal/server/browse_return_browser_test.go", "apps/player/e2e/browse-return-proof-reporter.ts",
          "apps/player/e2e/browse-return-helpers.ts", "apps/player/e2e/pnpm-lock.yaml", "apps/player/e2e/playwright.config.ts")


def safe_path(value):
    if not isinstance(value, str) or not 1 <= len(value) <= 1024 or any(ord(c) < 32 or ord(c) == 127 for c in value):
        raise ValueError("bounded fixture path required")
    path = Path(value)
    if not path.is_absolute() or ".." in path.parts:
        raise ValueError("absolute contained fixture path required")
    for parent in (path, *path.parents):
        if parent.is_symlink():
            raise ValueError("linked fixture path")
    return path


def prepare(arguments):
    if len(arguments) != 4 or arguments[0] not in PROJECTS or arguments[1] not in MODES:
        raise ValueError("fixed project and mode required")
    project, mode = arguments[:2]
    media, output = map(safe_path, arguments[2:])
    if output.exists():
        raise ValueError("fresh output required")
    parent = next(path for path in output.parents if path.exists())
    if not parent.is_dir() or parent.stat().st_uid != os.getuid():
        raise ValueError("owned output parent required")
    with os.fdopen(os.open(media, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK), "rb") as stream:
        info = os.fstat(stream.fileno())
        if not stat.S_ISREG(info.st_mode) or not 1 <= info.st_size <= 8 * 1024 * 1024:
            raise ValueError("bounded regular fixture required")
        data = stream.read(8 * 1024 * 1024 + 1)
    if len(data) != info.st_size:
        raise ValueError("fixture changed during admission")
    return project, mode, media, output, {"bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()}


def source_inputs(mode):
    from campaign_q14_suites import SUITES
    paths = (*INPUTS, *("apps/player/e2e/" + file for file, _title in SUITES[mode]))
    return {path: hashlib.sha256((ROOT / path).read_bytes()).hexdigest() for path in sorted(set(paths))}


def main(arguments=None):
    try:
        project, mode, media, output, fixture = prepare(sys.argv[1:] if arguments is None else arguments)
        inputs = source_inputs(mode)
        revision = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, timeout=5, text=True).strip()
    except (OSError, ValueError, StopIteration, subprocess.SubprocessError):
        print("BrowseReturn fixture admission rejected", file=sys.stderr)
        return 2
    os.umask(0o077)
    output.mkdir(parents=True, exist_ok=False)
    command = ["../../scripts/tooling/with-go-module.sh", "go", "test", "-v", "-p", "1", "./internal/server",
               "-run", "^TestBrowseReturnBrowserJourney$", "-count=1", "-timeout=6m"]
    environment = dict(os.environ, GOMAXPROCS="2", KINOSAIL_BROWSE_RETURN_BROWSER="1", KINOSAIL_BROWSE_RETURN_PROOF="1",
                       KINOSAIL_BROWSE_RETURN_CASES=mode, KINOSAIL_BROWSE_RETURN_MEDIA=str(media), KINOSAIL_BROWSER_PROJECT=project,
                       KINOSAIL_E2E_OUTPUT_DIR=str(output / "browser-results"), KINOSAIL_E2E_ARTIFACT_DIR=str(output / "browser-artifacts"))
    receipt = {"schemaVersion": 1, "revision": revision, "mode": mode, "project": project, "media": fixture,
               "sourceInputs": inputs, "workers": 1, "retries": 0, "passedCases": 0, "result": "failed"}
    result = 1
    try:
        phase, proof = driver.run(command, ROOT / "apps/player", environment, 370, "journeys", 365, mode)
        receipt["phase"] = phase
        valid = (phase.get("exitCode") == 0 and phase.get("ownedGroupStopped") is True and phase.get("captureSettled") is True
                 and not phase.get("outputOverflow") and not phase.get("timeout") and phase.get("goBoundary") == "completed-pass"
                 and phase.get("reportAdmitted") is True and proof is not None and proof.get("schemaVersion") == 3 and admit(proof, suite=mode, project=project) is not None
                 and complete(proof, suite=mode, project=project) and proof["status"] == "passed" and source_inputs(mode) == inputs)
        if valid:
            (output / "proof.json").write_text(json.dumps(proof, indent=2) + "\n")
            receipt.update(result="passed", passedCases=len(proof["cases"]))
            result = 0
    except (OSError, ValueError, TypeError, KeyError, subprocess.SubprocessError):
        receipt["errorClass"] = "fixture-or-proof-failure"
    finally:
        receipt["exitCode"] = result
        (output / "receipt.json").write_text(json.dumps(receipt, indent=2) + "\n")
    return result


if __name__ == "__main__":
    signal.signal(signal.SIGTERM, driver.stop)
    signal.signal(signal.SIGINT, driver.stop)
    raise SystemExit(main())
