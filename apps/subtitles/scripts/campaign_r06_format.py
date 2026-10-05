#!/usr/bin/env python3
"""Hosted source formatting only; preserve inputs and export four bounded JSONs."""
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import stat
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[3]
OUTPUT = ROOT / ".verification/campaign-proof/R06"
GO_FILES = tuple("apps/subtitles/engineering/qa/2026-10-05-save-browser/fixture/" + name
                 for name in (
                     "main_test.go", "transport_test.go", "witness_test.go", "fixture_test.go", "owner_test.go",
                     "filesystem_test.go", "http_test.go", "routing_test.go", "transport_hold_test.go",
                     "witness_effects_test.go", "owner_enrollment_test.go", "control_requests_test.go",
                     "control_owned_test.go", "control_faults_test.go",
                 )) + (
    "apps/subtitles/internal/server/assets.go", "apps/subtitles/internal/server/subtitle_inspector.go",
)
FORMAT_FILE_LIMIT = len(GO_FILES)
SAFE_NAMES = ("receipt.json", "results.json", "source-manifest.json", "artifact-manifest.json")
INPUT_CAP, OUTPUT_CAP = 256 * 1024, 512 * 1024


def pin(data):
    return {"bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()}


def format_record(*, path, original, output, exit_code, stderr):
    if type(path) is not str or path not in GO_FILES:
        raise ValueError("format-path")
    if type(original) is not bytes or not 0 < len(original) <= INPUT_CAP:
        raise ValueError("format-input")
    if type(output) is not bytes or not 0 < len(output) <= OUTPUT_CAP:
        raise ValueError("format-output")
    if type(exit_code) is not int or exit_code != 0 or type(stderr) is not bytes or stderr:
        raise ValueError("format-result")
    try:
        original.decode("utf-8"); output.decode("utf-8")
    except UnicodeDecodeError:
        raise ValueError("format-encoding") from None
    if b"\0" in original or b"\0" in output:
        raise ValueError("format-encoding")
    if len(output.splitlines()) > 300:
        raise ValueError("format-file-lines")
    return {"path": path, "original": pin(original), "formatted": pin(output),
            "changed": original != output,
            "formattedSourceBase64": base64.b64encode(output).decode("ascii")}


def git(*arguments):
    return subprocess.run(["git", *arguments], cwd=ROOT, capture_output=True,
                          check=True, timeout=5).stdout.decode().strip()


def read_source(name):
    path = ROOT / name
    info = path.lstat()
    if not stat.S_ISREG(info.st_mode) or not 0 < info.st_size <= INPUT_CAP:
        raise ValueError("source-shape")
    return path.read_bytes()


def tool_pin(path):
    info = path.stat()
    if not stat.S_ISREG(info.st_mode) or not 0 < info.st_size <= 64 * 1024 * 1024 or not info.st_mode & 0o111:
        raise ValueError("formatter-shape")
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(65536), b""):
            digest.update(block)
    return {"bytes": info.st_size, "sha256": digest.hexdigest()}


def format_owned(tool, source):
    started = time.monotonic()
    process = subprocess.Popen([str(tool)], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                               stderr=subprocess.PIPE, start_new_session=True)
    timed_out = False
    try:
        output, errors = process.communicate(source, timeout=10)
    except subprocess.TimeoutExpired:
        timed_out = True
        os.killpg(process.pid, signal.SIGKILL)
        output, errors = process.communicate(timeout=2)
    stopped = False
    try:
        os.killpg(process.pid, 0)
    except ProcessLookupError:
        stopped = True
    if not stopped:
        os.killpg(process.pid, signal.SIGKILL)
    receipt = {"exitCode": process.returncode, "timedOut": timed_out,
               "ownedGroupStopped": stopped, "captureSettled": process.poll() is not None,
               "durationSeconds": round(time.monotonic() - started, 3),
               "output": pin(output), "stderr": pin(errors)}
    if timed_out or not stopped or not receipt["captureSettled"]:
        raise ValueError("formatter-settlement")
    return output, errors, receipt


def main():
    # Source-format has no application, browser, compiler, dependency or network operation.
    if (sys.argv[1:] or os.environ.get("CAMPAIGN_R06_SUITE") != "source-format" or
            os.environ.get("GITHUB_ACTIONS") != "true" or os.environ.get("RUNNER_OS") != "Linux"):
        return 2
    os.umask(0o077)
    receipt = {"id": "R06", "schemaVersion": 1, "mode": "source-format",
               "result": "prerequisite-blocked", "sourceUnchanged": False,
               "compilerExecuted": False, "applicationExecuted": False, "browserExecuted": False,
               "limits": {"files": FORMAT_FILE_LIMIT, "perFileSeconds": 10, "shutdownSeconds": 2,
                          "inputBytes": INPUT_CAP, "outputBytes": OUTPUT_CAP}, "phases": []}
    results = {"schemaVersion": 1, "mode": "source-format", "files": []}
    manifest = {"mode": "source-format", "status": "prerequisite-blocked"}
    exit_code = 2
    try:
        revision, tree = git("rev-parse", "HEAD"), git("rev-parse", "HEAD^{tree}")
        if (not re.fullmatch("[a-f0-9]{40}", revision) or os.environ.get("GITHUB_SHA") != revision or
                not re.fullmatch("[a-f0-9]{40}", tree) or git("status", "--porcelain") or
                (ROOT / ".gates-disabled").exists()):
            raise ValueError("checkout-identity")
        if OUTPUT.exists() and any(OUTPUT.iterdir()):
            raise ValueError("fresh-output-required")
        names = (*GO_FILES, str(Path(__file__).relative_to(ROOT)),
                 "apps/subtitles/scripts/test_campaign_r06_format.py",
                 ".github/workflows/layout-stability.yml", "scripts/ci/run-campaign-proof.sh", "go.work")
        inputs = {name: read_source(name) for name in names}
        formatter = shutil.which("gofmt")
        if formatter is None:
            raise ValueError("formatter-unavailable")
        tool = Path(formatter).resolve(strict=True)
        identity = tool_pin(tool)
        manifest = {"mode": "source-format", "revision": revision, "trackedTree": tree,
                    "sources": {name: pin(data) for name, data in inputs.items()},
                    "boundary": "Known static source projection only; no credentials, runtime state or test result."}
        receipt.update(revision=revision, trackedTree=tree, formatter=identity, command=["gofmt"])
        for name in GO_FILES:
            output, errors, phase = format_owned(tool, inputs[name])
            receipt["phases"].append({"path": name, **phase})
            results["files"].append(format_record(path=name, original=inputs[name], output=output,
                                                  exit_code=phase["exitCode"], stderr=errors))
        receipt["sourceUnchanged"] = (git("rev-parse", "HEAD") == revision and
                                      git("rev-parse", "HEAD^{tree}") == tree and
                                      not git("status", "--porcelain") and tool_pin(tool) == identity and
                                      all(read_source(name) == data for name, data in inputs.items()))
        if not receipt["sourceUnchanged"]:
            raise ValueError("source-drift")
        receipt["result"] = "formatted-source-projection"
        exit_code = 0
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        receipt["errorClass"] = type(error).__name__
    if OUTPUT.exists() and any(OUTPUT.iterdir()):
        return 2
    OUTPUT.mkdir(parents=True, exist_ok=True)
    values = {"receipt.json": receipt, "results.json": results, "source-manifest.json": manifest}
    bodies = {name: (json.dumps(value, sort_keys=True, indent=2)+"\n").encode()
              for name, value in values.items()}
    bodies["artifact-manifest.json"] = (json.dumps({name: pin(body) for name, body in bodies.items()},
                                                    sort_keys=True, indent=2)+"\n").encode()
    if any(len(body) > 4*1024*1024 for body in bodies.values()):
        return 2
    for name in SAFE_NAMES:
        (OUTPUT / name).write_bytes(bodies[name])
    print("R06 source formatting: " + receipt["result"])
    return exit_code


if __name__ == "__main__":
    sys.exit(main())
