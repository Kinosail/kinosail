#!/usr/bin/env python3
"""Run an E2E command and retain a private, checksummed reproduction receipt."""

import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import platform
import subprocess
import sys


def capture(command):
    try:
        result = subprocess.run(command, capture_output=True, text=True, timeout=30, check=False)
        return {"command": command, "exitCode": result.returncode,
                "stdout": result.stdout.strip(), "stderr": result.stderr.strip()}
    except (OSError, subprocess.TimeoutExpired) as error:
        return {"command": command, "error": type(error).__name__}


def digest(path):
    hasher = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            hasher.update(chunk)
    return hasher.hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True)
    parser.add_argument("command", nargs=argparse.REMAINDER)
    args = parser.parse_args()
    command = args.command[1:] if args.command[:1] == ["--"] else args.command
    if not command:
        parser.error("a command is required after --")
    output = Path(args.output).resolve()
    output.mkdir(parents=True, exist_ok=True)
    if any(output.iterdir()):
        parser.error("output directory must be empty; use a separate directory for each run")
    revision = capture(["git", "rev-parse", "HEAD"])
    if revision.get("exitCode") != 0:
        parser.error("the command must run inside a Git checkout")
    root = Path(capture(["git", "rev-parse", "--show-toplevel"])["stdout"])
    status = capture(["git", "status", "--porcelain", "--untracked-files=normal"])
    patch = subprocess.run(["git", "diff", "HEAD", "--binary"], capture_output=True, check=True)
    safe_keys = ("CI", "CONTAINER_ENGINE", "KINOSAIL_BROWSER_TEST", "KINOSAIL_BROWSER_MATRIX",
                 "KINOSAIL_BROWSER_PROJECT", "KINOSAIL_BROWSER_SMOKE", "KINOSAIL_BROWSER_WORKERS",
                 "KINOSAIL_E2E_WORKERS", "KINOSAIL_E2E_VIDEO", "KINOSAIL_TEST_INSTANCE",
                 "KINOSAIL_TEST_IMAGE", "KINOSAIL_TEST_IMAGE_READY", "PLAYWRIGHT_CHANNEL")
    receipt = {
        "schemaVersion": 1, "revision": revision["stdout"],
        "sourceStatus": status, "trackedPatchSHA256": hashlib.sha256(patch.stdout).hexdigest(),
        "command": command, "workingDirectory": str(Path.cwd().relative_to(root)),
        "startedUTC": datetime.now(timezone.utc).isoformat(),
        "environment": {key: os.environ[key] for key in safe_keys if key in os.environ},
        "platform": {"system": platform.system(), "release": platform.release(),
                     "machine": platform.machine(), "python": platform.python_version()},
        "tools": [capture(cmd) for cmd in (["go", "version"], ["node", "--version"],
                                          ["pnpm", "--version"])],
        "inputs": {},
        "testData": "Disposable generated media and UI fixtures; see hashed generators and command log.",
    }
    for app in ("player", "subtitles"):
        for name in ("e2e/pnpm-lock.yaml", "e2e/playwright.config.ts",
                     "scripts/generate-test-media.sh", "scripts/test-container.sh"):
            path = root / "apps" / app / name
            receipt["inputs"][str(path.relative_to(root))] = digest(path)
    image = os.environ.get("KINOSAIL_TEST_IMAGE")
    engine = os.environ.get("CONTAINER_ENGINE", "docker")
    if image and engine in ("docker", "podman"):
        receipt["container"] = capture([engine, "image", "inspect", "--format",
            '{{.Id}} {{index .Config.Labels "org.opencontainers.image.revision"}}', image])
    environment = dict(os.environ, KINOSAIL_E2E_ARTIFACT_DIR=str(output),
                       KINOSAIL_E2E_OUTPUT_DIR=str(output / "browser-results"),
                       PLAYWRIGHT_HTML_OUTPUT_DIR=str(output / "html-report"))
    result = 1
    try:
        with (output / "command.log").open("w", encoding="utf-8") as log:
            process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                                       text=True, errors="replace", env=environment)
            try:
                for line in process.stdout:
                    log.write(line)
                    log.flush()
                    sys.stdout.write(line)
                result = process.wait()
            except BaseException:
                process.terminate()
                try:
                    process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()
                raise
    except (OSError, KeyboardInterrupt) as error:
        receipt["executionError"] = type(error).__name__
    finally:
        receipt["commandExitCode"] = result
        receipt["finalRevision"] = capture(["git", "rev-parse", "HEAD"])
        receipt["finalSourceStatus"] = capture(["git", "status", "--porcelain", "--untracked-files=normal"])
        final_patch = subprocess.run(["git", "diff", "HEAD", "--binary"], capture_output=True, check=True)
        receipt["finalTrackedPatchSHA256"] = hashlib.sha256(final_patch.stdout).hexdigest()
        receipt["sourceUnchanged"] = (
            receipt["finalRevision"].get("stdout") == receipt["revision"]
            and receipt["finalSourceStatus"] == status
            and receipt["finalTrackedPatchSHA256"] == receipt["trackedPatchSHA256"])
        receipt["exactRevisionProof"] = receipt["sourceUnchanged"] and not status.get("stdout")
        if not receipt["sourceUnchanged"]:
            receipt["verificationError"] = "Source changed during the command; start revision is not certified."
            result = result or 1
        receipt["finishedUTC"] = datetime.now(timezone.utc).isoformat()
        receipt["exitCode"] = result
        receipt["result"] = "passed" if result == 0 else "failed"
        receipt["evidence"] = {str(path.relative_to(output)): digest(path)
                               for path in sorted(output.rglob("*")) if path.is_file() and not path.is_symlink()}
        (output / "manifest.json").write_text(json.dumps(receipt, indent=2) + "\n", encoding="utf-8")
        checksums = dict(receipt["evidence"], **{"manifest.json": digest(output / "manifest.json")})
        (output / "SHA256SUMS").write_text("".join(f"{value}  {name}\n" for name, value in sorted(checksums.items())), encoding="utf-8")
    return result if result >= 0 else 128 - result


if __name__ == "__main__":
    sys.exit(main())
