#!/usr/bin/env python3
"""Hosted source-pair format projection; never rewrite or adopt source."""
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import selectors
import shutil
import signal
import stat
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[3]
OUTPUT = ROOT / ".verification/campaign-proof/Q47"
GO_FILES = (
    "apps/player/e2e/compose-template-fixture.go",
    "apps/player/e2e/compose-template-peer.go",
)
SAFE_NAMES = ("receipt.json", "results.json", "source-manifest.json", "artifact-manifest.json")
INPUT_CAP, OUTPUT_CAP, STDERR_CAP = 256 * 1024, 512 * 1024, 64 * 1024
SOURCE_PINS = {
    "apps/player/e2e/compose-template-fixture.go": {"bytes": 2869, "sha256": "1078f61a5cff0926d0a4a592d337ce64751e232b6162adde1548da2c5c064a80"},
    "apps/player/e2e/compose-template-peer.go": {"bytes": 5718, "sha256": "245ec58619efa6e17a230c39dfaacad796691412418586880e63f7dab7caab45"},
}


def pin(data):
    return {"bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()}


def format_record(*, path, original, output, exit_code, stderr):
    if type(path) is not str or path not in GO_FILES:
        raise ValueError("format-path")
    if type(original) is not bytes or pin(original) != SOURCE_PINS[path]:
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
    original_lines, formatted_lines = len(original.splitlines()), len(output.splitlines())
    return {"path": path, "original": pin(original), "formatted": pin(output),
            "originalLines": original_lines, "formattedLines": formatted_lines,
            "adoptionAllowed": original_lines <= 300 and formatted_lines <= 300,
            "reviewRequired": True, "changed": original != output,
            "formattedSourceBase64": base64.b64encode(output).decode("ascii")}


def git(*arguments):
    result = subprocess.run(["git", *arguments], cwd=ROOT, capture_output=True,
                            check=True, timeout=5)
    if len(result.stdout) > 65536 or len(result.stderr) > 65536:
        raise ValueError("checkout-output-bound")
    return result.stdout.decode("utf-8").strip()


def read_source(name):
    path = ROOT / name
    info = path.lstat()
    if path.resolve(strict=True) != path or not stat.S_ISREG(info.st_mode) or not 0 < info.st_size <= INPUT_CAP:
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


def stopped(group):
    try:
        os.killpg(group, 0)
    except ProcessLookupError:
        return True
    return False


def format_owned(tool, source):
    started = time.monotonic()
    process = subprocess.Popen([str(tool)], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                               stderr=subprocess.PIPE, start_new_session=True)
    selector = selectors.DefaultSelector()
    captures = {"output": bytearray(), "stderr": bytearray()}
    caps = {"output": OUTPUT_CAP, "stderr": STDERR_CAP}
    offset, timed_out, overflow, forced = 0, False, False, False
    deadline, shutdown = started + 10, None
    try:
        for stream, event, key in ((process.stdin, selectors.EVENT_WRITE, "input"),
                                  (process.stdout, selectors.EVENT_READ, "output"),
                                  (process.stderr, selectors.EVENT_READ, "stderr")):
            os.set_blocking(stream.fileno(), False)
            selector.register(stream, event, key)
        while selector.get_map():
            now = time.monotonic()
            if shutdown is None and (now >= deadline or overflow):
                timed_out = now >= deadline
                forced = True
                if not stopped(process.pid):
                    os.killpg(process.pid, signal.SIGKILL)
                shutdown = min(started + 12, now + 2)
            if shutdown is not None and now >= shutdown:
                break
            for key, _ in selector.select(min(0.05, max(0, (shutdown or deadline) - now))):
                stream, kind = key.fileobj, key.data
                if kind == "input":
                    try:
                        offset += os.write(stream.fileno(), source[offset:offset+4096])
                    except BrokenPipeError:
                        offset = len(source)
                    if offset == len(source):
                        selector.unregister(stream); stream.close()
                    continue
                chunk = os.read(stream.fileno(), 65536)
                if not chunk:
                    selector.unregister(stream); stream.close()
                    continue
                remaining = caps[kind] - len(captures[kind])
                captures[kind].extend(chunk[:remaining])
                overflow = overflow or len(chunk) > remaining
        try:
            process.wait(timeout=max(0.001, (shutdown or deadline) - time.monotonic()))
        except subprocess.TimeoutExpired:
            timed_out, forced = True, True
            if not stopped(process.pid):
                os.killpg(process.pid, signal.SIGKILL)
            shutdown = shutdown or min(started + 12, time.monotonic() + 2)
            try:
                process.wait(timeout=max(0, shutdown - time.monotonic()))
            except subprocess.TimeoutExpired:
                pass
        settling = shutdown or min(started + 12, time.monotonic() + 2)
        if not stopped(process.pid):
            forced = True
            os.killpg(process.pid, signal.SIGKILL)
        while not stopped(process.pid) and time.monotonic() < settling:
            time.sleep(0.01)
        capture_settled = not selector.get_map() and process.poll() is not None
    finally:
        if not stopped(process.pid):
            os.killpg(process.pid, signal.SIGKILL)
        shutdown = shutdown or min(started + 12, time.monotonic() + 2)
        try:
            process.wait(timeout=max(0, shutdown - time.monotonic()))
        except subprocess.TimeoutExpired:
            pass
        for stream in (process.stdin, process.stdout, process.stderr):
            stream.close()
        selector.close()
    output, errors = bytes(captures["output"]), bytes(captures["stderr"])
    receipt = {"exitCode": process.returncode, "timedOut": timed_out,
               "ownedGroupStopped": stopped(process.pid), "forcedShutdown": forced,
               "captureSettled": capture_settled,
               "captureBoundExceeded": overflow, "captureHashesComplete": capture_settled and not overflow,
               "durationSeconds": round(time.monotonic() - started, 3),
               "output": pin(output), "stderr": pin(errors)}
    return output, errors, receipt


def fresh_output():
    cursor = ROOT
    for part in (".verification", "campaign-proof", "Q47"):
        cursor = cursor / part
        if cursor.is_symlink() or (cursor.exists() and not cursor.is_dir()):
            return False
    return not OUTPUT.exists() or not any(OUTPUT.iterdir())


def main():
    if (sys.argv[1:] or os.environ.get("CAMPAIGN_Q47_SUITE") != "source-format" or
            os.environ.get("GITHUB_ACTIONS") != "true" or os.environ.get("RUNNER_OS") != "Linux"):
        return 2
    os.umask(0o077)
    receipt = {"id": "Q47", "schemaVersion": 1, "mode": "source-format",
               "result": "prerequisite-blocked", "sourceUnchanged": False,
               "compilerExecuted": False, "applicationExecuted": False, "browserExecuted": False,
               "limits": {"files": 2, "perFileSeconds": 10, "shutdownSeconds": 2,
                          "inputBytes": INPUT_CAP, "outputBytes": OUTPUT_CAP, "stderrBytes": STDERR_CAP,
                          "maxAdoptedLines": 300}, "phases": []}
    results = {"schemaVersion": 1, "mode": "source-format", "files": [],
               "adoptionBoundary": "Line gate only; exact source review is mandatory before adoption."}
    manifest = {"mode": "source-format", "status": "prerequisite-blocked"}
    inputs, revision, tree, identity, tool, exit_code = {}, None, None, None, None, 2
    try:
        revision, tree = git("rev-parse", "HEAD"), git("rev-parse", "HEAD^{tree}")
        if (not re.fullmatch("[a-f0-9]{40}", revision) or os.environ.get("GITHUB_SHA") != revision or
                not re.fullmatch("[a-f0-9]{40}", tree) or git("status", "--porcelain") or
                (ROOT / ".gates-disabled").exists() or (ROOT / ".gates-disabled").is_symlink()):
            raise ValueError("checkout-identity")
        if not fresh_output():
            raise ValueError("fresh-output-required")
        names = (*GO_FILES, "apps/player/scripts/campaign_q47_format.py",
                 "apps/player/scripts/test_campaign_q47_format.py",
                 ".github/workflows/layout-stability.yml", "scripts/ci/run-campaign-proof.sh", "go.work")
        inputs = {name: read_source(name) for name in names}
        if any(pin(inputs[name]) != SOURCE_PINS[name] for name in GO_FILES):
            raise ValueError("fixture-identity")
        formatter = shutil.which("gofmt")
        if formatter is None:
            raise ValueError("formatter-unavailable")
        tool = Path(formatter).resolve(strict=True)
        identity = tool_pin(tool)
        manifest = {"mode": "source-format", "revision": revision, "trackedTree": tree,
                    "sources": {name: pin(data) for name, data in inputs.items()},
                    "boundary": "Known static source pair only; no credentials/runtime/test acceptance."}
        receipt.update(revision=revision, trackedTree=tree, formatter=identity, command=["gofmt"])
        for name in GO_FILES:
            output, errors, phase = format_owned(tool, inputs[name])
            receipt["phases"].append({"path": name, **phase})
            if (phase["timedOut"] or phase["forcedShutdown"] or not phase["ownedGroupStopped"] or
                    not phase["captureSettled"] or phase["captureBoundExceeded"]):
                raise ValueError("formatter-settlement")
            record = format_record(path=name, original=inputs[name], output=output,
                                   exit_code=phase["exitCode"], stderr=errors)
            results["files"].append(record)
        allowed = all(row["adoptionAllowed"] for row in results["files"])
        receipt["result"] = "formatted-source-projection" if allowed else "formatted-source-over-line-limit"
        exit_code = 0 if allowed else 2
    except (OSError, ValueError, subprocess.SubprocessError):
        receipt["errorClass"] = "source-format-prerequisite"
    try:
        receipt["sourceUnchanged"] = bool(inputs and revision and tree and
            git("rev-parse", "HEAD") == revision and git("rev-parse", "HEAD^{tree}") == tree and
            not git("status", "--porcelain") and all(read_source(name) == data for name, data in inputs.items()) and
            (identity is None or tool_pin(tool) == identity))
    except (OSError, ValueError, subprocess.SubprocessError):
        receipt["sourceUnchanged"] = False
    if not receipt["sourceUnchanged"]:
        receipt["result"], exit_code = "prerequisite-blocked", 2
        results["files"] = []
    if not fresh_output():
        return 2
    OUTPUT.mkdir(parents=True, exist_ok=True, mode=0o700)
    values = {"receipt.json": receipt, "results.json": results, "source-manifest.json": manifest}
    bodies = {name: (json.dumps(value, sort_keys=True, indent=2)+"\n").encode()
              for name, value in values.items()}
    bodies["artifact-manifest.json"] = (json.dumps({name: pin(body) for name, body in bodies.items()},
                                                    sort_keys=True, indent=2)+"\n").encode()
    if any(len(body) > 4*1024*1024 for body in bodies.values()):
        return 2
    for name in SAFE_NAMES:
        with (OUTPUT / name).open("xb") as stream:
            stream.write(bodies[name])
    print("Q47 source formatting: " + receipt["result"])
    return exit_code


if __name__ == "__main__":
    sys.exit(main())
