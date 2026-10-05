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
from campaign_r06_restore_projection import (pin, blob, matches_pin, format_record,
                                             complete_files, unadmitted_record)

ROOT = Path(__file__).resolve().parents[3]
OUTPUT = ROOT / ".verification/campaign-proof/R06"
BASE_COMMIT = "bd1ea2e148787ae8d4a0a46640bc2a965e10fe6a"
MANIFEST_PATH = "apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture-source-manifest.json"
MANIFEST_BLOB = "62bf1292673af484aa170bb0534d66677e8f250d"
MANIFEST_SHA256 = "b627c147c2a3f569ce6df2f9122a15a9fcefdb6a348216640fc8b5cf15e5a9a7"
PREFIX = "apps/subtitles/engineering/qa/2026-10-05-restore-recovery/fixture/"
GO_PINS = {
    PREFIX + "restore_assertions_test.go": ("0c72a93cf599390ecfa42a1b08ebbbe7e0c8bd6b", "82261355e73118d9cad07c3f6fa289ac29a0d6343efb8db669926b96087191c0", 6490),
    PREFIX + "restore_controls_test.go": ("ef0599a3a672d09603f5619b69cb1eee3fd2a770", "d7be68372506e6813b40b7a5b2c9080a53af634a8a0ab95e8369c1b18bddacdb", 4697),
    PREFIX + "restore_exchange_test.go": ("96dceb7318df112838f2d409d8647e7a5bcfbc18", "b694f5a9f2060daac8fcc08e77076030b6d3a34733b9eb2f38da557763a0bfa1", 4926),
    PREFIX + "restore_filesystem_test.go": ("e330d8bb78d411b32565ccb6c98d626eba9c6230", "37b2c37f8534cc220871f6892c2b0e76be9e77ad7e7683be561d24b322000685", 7649),
    PREFIX + "restore_http_test.go": ("96e2101833c66403eeed164552e696f7d0c52cba", "c1b6b0e0c6fecd90220d03fb14ecf9c5ed83fc656b680a0c194a3faa94462a73", 6127),
    PREFIX + "restore_main_test.go": ("8b53d7a674a4099d5b318ae35d4fd0bff41d549e", "09e1ca5c0277ba72237cfdbb9ad880cca048d13d01ec31ea55ae1b77e946a9d4", 9238),
    PREFIX + "restore_owner_enrollment_test.go": ("8c570ea7cf23f1c60b01950fee8ba5122b64ccf0", "d856b68f17840df1c7b8ccd857dbb4cd2e8397ae89307ef73401ba3fefb4913b", 5019),
    PREFIX + "restore_owner_test.go": ("687b5b542433ad9f91faf2475039dbb1d9926f60", "1fd267854c88a2aa23bb9dabb261033448919f3bf590a636fa88cfd3a5b58cb6", 1373),
    PREFIX + "restore_requests_test.go": ("e789fb0fe27471106ee509b65867a6190229d8d5", "7bbb33bf545f6563cf5eefcc36d1d7fb046a6a1f6ad6c0d858e6b42761d6b697", 7329),
    PREFIX + "restore_routes_test.go": ("f635cfb15f244fe69c8797c7eb0e0342979929f8", "ee742a4ac5decc9d60e60657356560528cc961b957ad4c0dffc46e5058aac434", 3168),
    PREFIX + "restore_routing_test.go": ("44063dde743dd1a43c9b39987388a112bb21ff8d", "df68268c7c7819fce2338590d1c88350285bcd6adf64c3ecd12b18aa6e44dfce", 6305),
    PREFIX + "restore_target_test.go": ("839aa6185bda860cf48a38e99b657b9277742226", "6e996fb624a949dc12fd79d211d3bc38651c635de0d2ab37a1c6e7a01710eea8", 4621),
    PREFIX + "restore_transport_test.go": ("8f090a9f9606bb6fb04092854971b3d542445271", "503f6c1d469dacb67d7eb92df0cf16fabd5b12f07c7c6be7839a6e08edecb4dd", 5001),
    PREFIX + "restore_witness_test.go": ("6bdb0fa8f2c0e4905ef3911f3139eb9571c03988", "b6c9bdcfa25152fc6b2e2a887e0020955adf11ce4e437978016d07f0e33e615a", 8319),
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


def phase_accepted(exit_code, timed_out, group_stopped, capture_settled, capture_failed=False):
    return (type(exit_code) is int and exit_code == 0 and type(timed_out) is bool and not timed_out
            and group_stopped is True and capture_settled is True and capture_failed is False)


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
             "apps/subtitles/scripts/test_campaign_r06_restore_qualified_tokens.py",
             "apps/subtitles/scripts/campaign_r06_restore_projection.py",
             "apps/subtitles/scripts/test_campaign_r06_restore_projection.py",
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
    complete, stage, diagnostic = False, "admission", None
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
            try:
                results["files"].append(format_record(path=name, original=inputs[name], output=output,
                                                      exit_code=phase["exitCode"], stderr=errors))
            except ValueError as error:
                if failure_code(error) == "format-token-change":
                    diagnostic = (name, output, errors, phase)
                raise
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
    if diagnostic is not None:
        name, output, errors, phase = diagnostic
        rejected = unadmitted_record(path=name, original=inputs[name], output=output,
            expected=GO_PINS[name], phase=phase, stderr=errors,
            source_unchanged=receipt["sourceUnchanged"], failure_code="format-token-change")
        if rejected is not None:
            results["unadmittedFormat"] = rejected
    if complete and receipt["sourceUnchanged"]:
        receipt["result"] = "formatted-source-projection"
    if not safe_write({"receipt.json": receipt, "results.json": results, "source-manifest.json": manifest}):
        return 2
    print("R06 Restore source formatting: " + receipt["result"])
    return 0 if receipt["result"] == "formatted-source-projection" else 2


if __name__ == "__main__":
    sys.exit(main())
