#!/usr/bin/env python3
"""Fourteen pinned Restore fixture inputs; hosted stock source formatting only."""
import base64
import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import time

from campaign_r06_format import formatter_command, read_source, tool_pin
from campaign_r06_browser_sources import identity, tracked_sources, git
from campaign_r06_restore_tokens import equivalent

ROOT = Path(__file__).resolve().parents[3]
OUTPUT = ROOT / ".verification/campaign-proof/R06"
BASE_COMMIT = "bd1ea2e148787ae8d4a0a46640bc2a965e10fe6a"
MANIFEST_PATH = "apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture-source-manifest.json"
MANIFEST_BLOB = "bf9ae2b849da6cb83d9f7b87780f06353596d1ad"
MANIFEST_SHA256 = "8e46680fd34692d1be5090726d16833a1693d276d10c5f677c2aa74e6909a604"
PREFIX = "apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture/"
GO_PINS = {
    PREFIX + "restore_assertions_test.go": ("0c72a93cf599390ecfa42a1b08ebbbe7e0c8bd6b", "82261355e73118d9cad07c3f6fa289ac29a0d6343efb8db669926b96087191c0", 6490),
    PREFIX + "restore_controls_test.go": ("ef0599a3a672d09603f5619b69cb1eee3fd2a770", "d7be68372506e6813b40b7a5b2c9080a53af634a8a0ab95e8369c1b18bddacdb", 4697),
    PREFIX + "restore_exchange_test.go": ("ddf89b521e9884a06a9e479acb17ecfb7322dff0", "d809df7ca579c2c612616d0448cdc056d7dba5c32cddea8d15adb276e8380414", 4659),
    PREFIX + "restore_filesystem_test.go": ("95e46d54ad38b19aae445e7886a9a10579e3e6ce", "14ad292f5db92043f44cdba9f62abcdd6a2f875da9e616508aa7ae04da8db0b1", 1960),
    PREFIX + "restore_http_test.go": ("7b108781419303977eccd33e28241f4023766bba", "5efa10db3ffbcf70296fe0389e866de415e7ed6c365bb72b71b703325eab7798", 6018),
    PREFIX + "restore_main_test.go": ("5b71ecc1bc3e94be6b516595bea89ac487f88500", "b808cdca52ea70579a5550a1c049e9f14e5213efafcef85ea8575630b7354f49", 4725),
    PREFIX + "restore_owner_enrollment_test.go": ("8c570ea7cf23f1c60b01950fee8ba5122b64ccf0", "d856b68f17840df1c7b8ccd857dbb4cd2e8397ae89307ef73401ba3fefb4913b", 5019),
    PREFIX + "restore_owner_test.go": ("ce0d824a454b4dda4aff3e714f57fd214740deff", "b4d54051e32adc3ad26f14d61f9b223f10cdd5ae5fd9f1e96769126a4212df43", 1374),
    PREFIX + "restore_requests_test.go": ("ba48bfa05ec50dfb01bcfa223f4b1b6ca9ad93c0", "4e2a98772c1d468a16c94c7da6044866d0a972d50209ede130cad0a400c349be", 7174),
    PREFIX + "restore_routes_test.go": ("0f3c734bc5fa27c2c9d7d33086e3c9a076aecaef", "c18118c650b89746167032518b3add2dac3a7d08ce7da3d6a35d3063091e84e8", 3109),
    PREFIX + "restore_routing_test.go": ("6bc8920c88227466f3c1185dfcc94e41fd48d20b", "e4bc645a6e7d1c2705b09dc4b9a62a4def57208198574b253212ccabc880d1c3", 6144),
    PREFIX + "restore_target_test.go": ("5cffb6dc125fc4827e3926a05f6a7a8dc232e00a", "a2a985b832eb447129f7223737c1c083399b03478ee8ab992923d13e92928fe9", 4606),
    PREFIX + "restore_transport_test.go": ("bef80ffe1957e026697b8ff738c125a0e20f9ae9", "8641139e8acd6fcf7f6b553f06d1cb0a6443c4d1d78b29c5413579d96319aaf6", 4928),
    PREFIX + "restore_witness_test.go": ("055dc7a72a1944d34379157aa5ce02102a84ae3a", "328020a75bb82707eedc78dd2951b0d8504f3f85e9b2cf07712fa990f3f468fd", 8214),
}
GO_FILES = tuple(GO_PINS)
DEPENDENCIES = {
    "apps/subtitles/scripts/campaign_r06_format.py": "0627baac73b256fc6ec6a2788b06ce97f7a0e0faede8abbbedbb70f6fc93d1c2",
    "apps/subtitles/scripts/campaign_r06_browser_sources.py": "20a342625421d6ec7bc90df15bfa79c1ef2d3ef2deb915b0bab4cd334944e258",
    "apps/subtitles/.golangci.yml": "389f5d87f20f6ea354dc36be94150e4057a09e1fd31a5cfb6aa14de96953eafd",
}
SAFE_NAMES = ("receipt.json", "results.json", "source-manifest.json", "artifact-manifest.json")
INPUT_CAP, OUTPUT_CAP = 256 * 1024, 512 * 1024
PER_FILE_SECONDS, SHUTDOWN_SECONDS = 10, 2
FAILURE_CODES = frozenset(("format-path format-bytes format-result format-file-lines format-token-change "
    "manifest-identity manifest-base fixture-input-identity dependency-identity checkout-identity "
    "fresh-output-required formatter-unavailable formatter-settlement").split())

COMMAND_FAILURES = {
    ("git", "rev-parse", "HEAD"): "git-head-read-failed",
    ("git", "rev-parse", "HEAD^{tree}"): "git-tree-read-failed",
    ("git", "diff", "HEAD", "--name-only", "-z"): "git-tracked-diff-failed",
    ("git", "status", "--porcelain"): "git-worktree-status-failed",
    ("git", "ls-files", "--stage", "-z"): "git-source-index-failed",
    ("git", "cat-file", "-e", BASE_COMMIT + "^{commit}"): "git-base-object-unavailable",
    ("git", "merge-base", "--is-ancestor", BASE_COMMIT, "HEAD"): "git-base-ancestry-failed",
}


def failure_code(error):
    if type(error) is subprocess.CalledProcessError:
        command = error.cmd
        if type(command) in (list, tuple) and all(type(part) is str for part in command):
            return COMMAND_FAILURES.get(tuple(command), "unclassified")
        return "unclassified"
    return str(error) if type(error) is ValueError and str(error) in FAILURE_CODES else "unclassified"


def require_base_ancestry():
    git("cat-file", "-e", BASE_COMMIT + "^{commit}")
    git("merge-base", "--is-ancestor", BASE_COMMIT, "HEAD")


def pin(data):
    return {"bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()}


def blob(data):
    return hashlib.sha1(("blob " + str(len(data)) + "\0").encode() + data).hexdigest()


def matches_pin(data, expected):
    return type(data) is bytes and type(expected) is dict and pin(data) == expected


def phase_accepted(exit_code, timed_out, group_stopped, capture_settled, capture_failed=False):
    return (type(exit_code) is int and exit_code == 0 and type(timed_out) is bool and not timed_out
            and group_stopped is True and capture_settled is True and capture_failed is False)


def format_record(*, path, original, output, exit_code, stderr):
    if type(path) is not str or path not in GO_FILES:
        raise ValueError("format-path")
    for data, cap in ((original, INPUT_CAP), (output, OUTPUT_CAP)):
        if type(data) is not bytes or not 0 < len(data) <= cap or b"\0" in data:
            raise ValueError("format-bytes")
        data.decode("utf-8")
    if type(exit_code) is not int or exit_code != 0 or type(stderr) is not bytes or stderr:
        raise ValueError("format-result")
    if len(output.splitlines()) > 300:
        raise ValueError("format-file-lines")
    if not equivalent(original, output):
        raise ValueError("format-token-change")
    return {"path": path, "original": pin(original), "formatted": pin(output),
            "formattedLines": len(output.splitlines()), "changed": original != output,
            "tokensPreserved": True, "formattedSourceBase64": base64.b64encode(output).decode("ascii")}


def complete_files(records):
    return (type(records) is list and len(records) == len(GO_FILES)
            and all(type(row) is dict and row.get("tokensPreserved") is True for row in records)
            and [row.get("path") for row in records] == list(GO_FILES))


def signal_owned(pid):
    try:
        os.killpg(pid, signal.SIGKILL)
        return "sent"
    except ProcessLookupError:
        return "absent"
    except PermissionError:
        return "denied"
    except OSError:
        return "failed"


def group_absent(pid):
    try:
        os.killpg(pid, 0)
        return False
    except ProcessLookupError:
        return True
    except OSError:
        return False


def format_owned(tool, source):
    started = time.monotonic()
    process = subprocess.Popen(formatter_command(tool), cwd=ROOT,
        env=dict(os.environ, GOPROXY="off", GOTOOLCHAIN="local", GOMAXPROCS="2"),
        stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
    output, errors, timed_out, capture, capture_failed = b"", b"", False, False, False
    stop = "not-needed"
    try:
        output, errors = process.communicate(source, timeout=PER_FILE_SECONDS)
        capture = True
    except subprocess.TimeoutExpired:
        timed_out = True
        stop = signal_owned(process.pid)
        try:
            output, errors = process.communicate(timeout=SHUTDOWN_SECONDS)
            capture = True
        except (OSError, ValueError, subprocess.SubprocessError):
            capture_failed = True
            stop = "capture-unsettled"
    except (OSError, ValueError, subprocess.SubprocessError):
        capture_failed = True
        stop = signal_owned(process.pid)
        try:
            output, errors = process.communicate(timeout=SHUTDOWN_SECONDS)
            capture = True
        except (OSError, ValueError, subprocess.SubprocessError):
            stop = "capture-unsettled"
    stopped = group_absent(process.pid)
    if not stopped:
        stop = signal_owned(process.pid)
        end = time.monotonic() + SHUTDOWN_SECONDS
        while not stopped and time.monotonic() < end:
            stopped = group_absent(process.pid)
            if not stopped:
                time.sleep(0.02)
    return output, errors, {
        "exitCode": process.returncode, "timedOut": timed_out, "ownedGroupStopped": stopped,
        "captureSettled": capture and process.poll() is not None,
        "captureFailed": capture_failed, "stopClass": stop,
        "durationSeconds": round(time.monotonic() - started, 3),
        "output": pin(output), "stderr": pin(errors)}


def input_sources():
    raw = read_source(MANIFEST_PATH)
    if hashlib.sha256(raw).hexdigest() != MANIFEST_SHA256 or blob(raw) != MANIFEST_BLOB:
        raise ValueError("manifest-identity")
    manifest = json.loads(raw.decode("utf-8"))
    if manifest.get("baseCommit") != BASE_COMMIT:
        raise ValueError("manifest-base")
    inputs = {}
    for name, (expected_blob, digest, size) in GO_PINS.items():
        data = read_source(name)
        if blob(data) != expected_blob or not matches_pin(data, {"bytes": size, "sha256": digest}):
            raise ValueError("fixture-input-identity")
        inputs[name] = data
    for name, digest in DEPENDENCIES.items():
        data = read_source(name)
        if hashlib.sha256(data).hexdigest() != digest:
            raise ValueError("dependency-identity")
        inputs[name] = data
    names = (MANIFEST_PATH, "apps/subtitles/scripts/campaign_r06_restore_format.py",
             "apps/subtitles/scripts/campaign_r06_restore_tokens.py",
             "apps/subtitles/scripts/test_campaign_r06_restore_format.py",
             ".github/workflows/layout-stability.yml", "scripts/ci/run-campaign-proof.sh",
             "go.work", "apps/subtitles/go.mod")
    inputs.update({name: read_source(name) for name in names})
    return inputs


def safe_write(values):
    if OUTPUT.exists() and (OUTPUT.is_symlink() or any(OUTPUT.iterdir())):
        return False
    for directory in (ROOT / ".verification", ROOT / ".verification/campaign-proof", OUTPUT):
        if directory.is_symlink():
            return False
    OUTPUT.mkdir(parents=True, exist_ok=True)
    bodies = {name: (json.dumps(value, sort_keys=True, indent=2) + "\n").encode()
              for name, value in values.items()}
    bodies["artifact-manifest.json"] = (json.dumps({name: pin(data) for name, data in bodies.items()},
                                                  sort_keys=True, indent=2) + "\n").encode()
    if set(bodies) != set(SAFE_NAMES) or any(len(data) > 4 * 1024 * 1024 for data in bodies.values()):
        return False
    for name in SAFE_NAMES:
        descriptor = os.open(OUTPUT / name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        with os.fdopen(descriptor, "wb") as stream:
            stream.write(bodies[name])
    return True


def main():
    if (sys.argv[1:] or os.environ.get("CAMPAIGN_R06_SUITE") != "restore-source-format"
            or os.environ.get("GITHUB_ACTIONS") != "true" or os.environ.get("RUNNER_OS") != "Linux"):
        return 2
    os.umask(0o077)
    receipt = {"id": "R06", "schemaVersion": 1, "mode": "restore-source-format",
               "result": "prerequisite-blocked", "sourceUnchanged": False,
               "compilerExecuted": False, "applicationExecuted": False, "browserExecuted": False,
               "limits": {"files": 14, "perFileSeconds": PER_FILE_SECONDS,
                          "captureShutdownSeconds": SHUTDOWN_SECONDS, "groupShutdownSeconds": SHUTDOWN_SECONDS,
                          "inputBytes": INPUT_CAP, "outputBytes": OUTPUT_CAP, "canonicalLines": 300},
               "phases": []}
    results = {"schemaVersion": 1, "mode": "restore-source-format", "files": []}
    manifest = {"mode": "restore-source-format", "status": "prerequisite-blocked"}
    before = tracked = inputs = tool = tool_identity = None
    complete, stage = False, "admission"
    try:
        before = identity()
        if before["revision"] != os.environ.get("GITHUB_SHA") or git("status", "--porcelain"):
            raise ValueError("checkout-identity")
        require_base_ancestry()
        if OUTPUT.exists() and any(OUTPUT.iterdir()):
            raise ValueError("fresh-output-required")
        inputs = input_sources()
        tracked = tracked_sources()
        import shutil
        executable = shutil.which("golangci-lint")
        if executable is None:
            raise ValueError("formatter-unavailable")
        tool = Path(executable).resolve(strict=True)
        tool_identity = tool_pin(tool)
        receipt.update(**before, formatter=tool_identity,
                       command=["golangci-lint", "fmt", "--stdin", "--config", "apps/subtitles/.golangci.yml"],
                       baseCommit=BASE_COMMIT,
                       fixtureManifest={"gitBlob": MANIFEST_BLOB, "sha256": MANIFEST_SHA256})
        manifest = {"mode": "restore-source-format", **before, "baseCommit": BASE_COMMIT,
                    "fixtureManifest": {"gitBlob": MANIFEST_BLOB, "sha256": MANIFEST_SHA256},
                    "sources": {name: {"gitBlob": blob(data), **pin(data)} for name, data in inputs.items()},
                    "trackedSources": tracked, "boundary": "Static source projection; no public runtime acceptance."}
        stage = "formatter"
        for name in GO_FILES:
            output, errors, phase = format_owned(tool, inputs[name])
            receipt["phases"].append({"path": name, **phase})
            if not phase_accepted(phase["exitCode"], phase["timedOut"],
                                  phase["ownedGroupStopped"], phase["captureSettled"], phase["captureFailed"]):
                raise ValueError("formatter-settlement")
            results["files"].append(format_record(path=name, original=inputs[name], output=output,
                                                  exit_code=phase["exitCode"], stderr=errors))
        complete = complete_files(results["files"])
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        receipt.update(errorClass=type(error).__name__, failureStage=stage, failureCode=failure_code(error))
    finally:
        if before is not None and tracked is not None and inputs is not None and tool_identity is not None:
            try:
                receipt["sourceUnchanged"] = (identity() == before and not git("status", "--porcelain")
                    and tracked_sources() == tracked and tool_pin(tool) == tool_identity
                    and all(read_source(name) == data for name, data in inputs.items()))
            except (OSError, ValueError, subprocess.SubprocessError):
                receipt["sourceUnchanged"] = False
    if complete and receipt["sourceUnchanged"]:
        receipt["result"] = "formatted-source-projection"
    if not safe_write({"receipt.json": receipt, "results.json": results, "source-manifest.json": manifest}):
        return 2
    print("R06 Restore source formatting: " + receipt["result"])
    return 0 if receipt["result"] == "formatted-source-projection" else 2


if __name__ == "__main__":
    sys.exit(main())
