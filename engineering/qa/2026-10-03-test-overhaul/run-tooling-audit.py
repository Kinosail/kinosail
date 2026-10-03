#!/usr/bin/env python3
"""Reproduce the lightweight tooling audit without deployments or large builds."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[3]
OUTPUT = Path(__file__).resolve().parent


def commands(runtime_workflows=None):
    ci = [sys.executable, "-m", "unittest", "discover", "-s", "scripts/ci", "-p", "test_*.py", "-v"]
    if runtime_workflows:
        code = ("import sys,unittest; from pathlib import Path; sys.path.insert(0,'scripts/ci'); "
                "suite=unittest.defaultTestLoader.discover('scripts/ci',pattern='test_*.py'); "
                "import test_runtime_contracts as runtime; "
                f"runtime.WORKFLOWS=Path({str(runtime_workflows)!r}); "
                "result=unittest.TextTestRunner(verbosity=2).run(suite); "
                "raise SystemExit(not result.wasSuccessful())")
        ci = [sys.executable, "-c", code]
    yield "scripts/ci/test_*.py", ci
    for path in sorted((ROOT / "scripts/quality").glob("test_*.py")):
        yield str(path.relative_to(ROOT)), [sys.executable, str(path.relative_to(ROOT))]
    for path in sorted((ROOT / "scripts/tooling").glob("test-*.py")):
        yield str(path.relative_to(ROOT)), [sys.executable, str(path.relative_to(ROOT))]
    for path in ("scripts/quality/browser-script-bundles.test.mjs", "scripts/ci/prepare-codeql-js.test.mjs", "scripts/tooling/test-script-lint.mjs"):
        yield path, ["node", "--test", path]
    for path in ("scripts/tooling/test-check-go-loc.sh", "scripts/tooling/test-go-coverage-input.sh", "scripts/tooling/test-quality-controls.sh", "scripts/tooling/test-scan-deployment-image.sh", "scripts/tooling/test-source-tools.sh", "scripts/tooling/test-deploy-nox-app.sh", "scripts/tooling/test-deploy-nox-remote.sh", "scripts/tooling/test-nox-autodeploy.sh"):
        yield path, ["bash", path]
    for app in ("player", "subtitles"):
        for name in ("test-installer.sh", "test-remote-setup.sh", "test-deploy-nox.sh", "test-verify-changed.sh"):
            path = f"apps/{app}/scripts/{name}"
            yield path, ["bash", path]
        path = f"apps/{app}/scripts/test-native-contract.py"
        yield path, [sys.executable, path, f"apps/{app}/packaging/native-installation.json"]
    for name in ("test-package-installer.py", "test_media_server_benchmark.py", "test_summarize_playback.py"):
        path = f"apps/player/scripts/{name}"
        yield path, [sys.executable, path]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("phase", choices=("baseline", "final"))
    parser.add_argument("--runtime-workflows", type=Path,
                        help="Validate pending parent CI evidence wiring without editing this checkout")
    args = parser.parse_args()
    destination = OUTPUT / f"tooling-{args.phase}"
    destination.mkdir(exist_ok=True)
    records = []
    for name, command in commands(args.runtime_workflows):
        start = time.monotonic()
        try:
            result = subprocess.run(command, cwd=ROOT, capture_output=True, timeout=120,
                                    env=os.environ | {"PYTHONDONTWRITEBYTECODE": "1"})
            body = result.stdout + result.stderr
            status = "pass" if result.returncode == 0 else "fail"
            code = result.returncode
        except subprocess.TimeoutExpired as error:
            body = (error.stdout or b"") + (error.stderr or b"")
            status, code = "timeout", None
        log = destination / (name.replace("/", "_").replace("*", "all") + ".log")
        log.write_bytes(body)
        records.append({"file": name, "command": command, "status": status, "exit_code": code,
                        "duration_seconds": round(time.monotonic() - start, 3),
                        "log": str(log.relative_to(ROOT)), "log_sha256": hashlib.sha256(body).hexdigest()})
        print(f"{status.upper()} {name}", flush=True)
    receipt = {"revision": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip(),
               "phase": args.phase, "platform": platform.platform(), "machine": platform.machine(),
               "python": sys.version, "working_changes": subprocess.check_output(["git", "status", "--short"], cwd=ROOT, text=True),
               "runtime_workflow": ({"path": str(args.runtime_workflows / "app.yml"),
                                     "sha256": hashlib.sha256((args.runtime_workflows / "app.yml").read_bytes()).hexdigest()}
                                    if args.runtime_workflows else None),
               "environment": {name: os.environ.get(name) for name in ("CI", "GOWORK", "KINOSAIL_VERIFY_WORKTREE", "KINOSAIL_BROWSER_TEST")},
               "results": records,
               "limits": ["Isolated fixture checks only; no real Nox deployment or cleanup.",
                          "Container, populated-browser, native release cross-builds and Go metrics run separately.",
                          "These results are not product E2E evidence."]}
    (OUTPUT / f"tooling-{args.phase}.json").write_text(json.dumps(receipt, indent=2) + "\n")
    return int(any(record["status"] != "pass" for record in records))


if __name__ == "__main__":
    raise SystemExit(main())
