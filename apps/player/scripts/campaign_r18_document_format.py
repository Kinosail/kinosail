#!/usr/bin/env python3
"""Hosted canonical proposals for two frozen Go inputs; never adopt or compile."""
import argparse
import base64
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import re
import sys
import time
import types

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parent))
from campaign_r18_document_inputs import Budget, read_small
from campaign_r18_document_sources import APP, BASE, BASE_TREE, ROOT, digest, fingerprint, import_execute, safe_process, save_new
from campaign_r18_document_format_process import (
    CONFIG, CONFIG_SHA, GO_FILES, SOURCE_SHA, PacketProjection, bridge, formatter_binary, formatter_command,
)

DRIVER_PATH = "apps/player/scripts/campaign-r18-document-contract.py"
DRIVER_SHA = "257736615cfca467874775c6b562f5059bb797c9848945e8ab6d4911415b8ca4"
DRIVER_OID = "cd3994a3b83677d84f81e708c32d081267154570"
IDENTITY_FIELDS = {"sourceUnchanged", "toolUnchanged", "trackedUnchanged", "historyAdmitted", "exactInvocation", "sourcesPinned"}
RECORD_FIELDS = {"path", "original", "formatted", "formattedSourceBase64", "changed", "autoAdoption", "semanticsVerified"}
SOURCE_BYTES = (4465, 7997)
ARTIFACTS = ("receipt.json", "results.json", "source-manifest.json", "artifact-manifest.json")


def format_record(path, original, output, exit_code, stderr_bytes):
    if path not in GO_FILES or type(original) is not bytes or not 0 < len(original) <= 16384:
        raise ValueError("source-boundary")
    if hashlib.sha256(original).hexdigest() != SOURCE_SHA[GO_FILES.index(path)]:
        raise ValueError("source-pin")
    if type(exit_code) is not int or exit_code != 0 or type(stderr_bytes) is not int or stderr_bytes != 0:
        raise ValueError("formatter-failure")
    if type(output) is not bytes or not 0 < len(output) <= 32768:
        raise ValueError("output-boundary")
    output.decode("utf-8")
    if b"\0" in output or not output.startswith(b"package server_test\n") or len(output.splitlines()) > 300:
        raise ValueError("canonical-source-boundary")
    def metadata(data):
        return {"sha256": hashlib.sha256(data).hexdigest(), "bytes": len(data)}
    return {"path": path, "original": metadata(original), "formatted": metadata(output),
            "formattedSourceBase64": base64.b64encode(output).decode(), "changed": original != output,
            "autoAdoption": False, "semanticsVerified": False}


def history_kind(shallow, anchor_exit, ancestor_exit):
    if type(shallow) is not bool or type(anchor_exit) is not int:
        return "history-incomplete"
    if shallow:
        return "history-shallow"
    if anchor_exit != 0:
        return "history-anchor-unavailable"
    if type(ancestor_exit) is not int:
        return "history-incomplete"
    return "history-admitted" if ancestor_exit == 0 else "history-not-proven-ancestor"


def settled(status):
    return (type(status) is dict and type(status.get("exitCode")) is int and status["exitCode"] == 0
            and status.get("stopReason") is None and all(status.get(k) is True for k in
                ("ownedProcessExited", "ownedGroupSettled", "captureSettled")))


def admit_format(records, processes, identity):
    blocked = {"classification": "prerequisite-blocked", "count": 0, "autoAdoption": False, "semanticsVerified": False}
    if type(identity) is not dict or set(identity) != IDENTITY_FIELDS or not all(identity[k] is True for k in identity):
        return blocked
    if type(records) is not list or len(records) != 2 or type(processes) is not list or len(processes) != 2:
        return blocked
    if not all(settled(status) for status in processes):
        return blocked
    seen = set()
    try:
        for row in records:
            if type(row) is not dict or set(row) != RECORD_FIELDS or row["path"] not in GO_FILES or row["path"] in seen:
                return blocked
            seen.add(row["path"])
            index = GO_FILES.index(row["path"])
            original = {"sha256": SOURCE_SHA[index], "bytes": SOURCE_BYTES[index]}
            if row["original"] != original or type(row["formattedSourceBase64"]) is not str or len(row["formattedSourceBase64"]) > 44000:
                return blocked
            output = base64.b64decode(row["formattedSourceBase64"], validate=True)
            output.decode("utf-8")
            metadata = {"sha256": hashlib.sha256(output).hexdigest(), "bytes": len(output)}
            if row["formatted"] != metadata or not 0 < len(output) <= 32768 or b"\0" in output:
                return blocked
            if not output.startswith(b"package server_test\n") or len(output.splitlines()) > 300:
                return blocked
            if row["autoAdoption"] is not False or row["semanticsVerified"] is not False or type(row["changed"]) is not bool:
                return blocked
            if row["changed"] != (metadata != original):
                return blocked
    except (ValueError, TypeError, UnicodeError):
        return blocked
    return {"classification": "canonical-source-proposals", "count": 2, "autoAdoption": False, "semanticsVerified": False}


def load_driver(deadline):
    raw = read_small(ROOT / DRIVER_PATH, 16384, deadline)
    oid = hashlib.sha1(("blob " + str(len(raw)) + "\0").encode() + raw).hexdigest()
    if oid != DRIVER_OID or hashlib.sha256(raw).hexdigest() != DRIVER_SHA:
        raise ValueError("driver-import-pin")
    module = types.ModuleType("r18_verified_contract_driver")
    module.__file__ = str(ROOT / DRIVER_PATH)
    exec(compile(raw, module.__file__, "exec"), module.__dict__)
    return module


class DiagnosticCapture:
    """Only fixed true/false is retained; all other bytes are count/hash."""
    def __init__(self, boolean=False):
        self.boolean, self.value, self.count, self.bytes = boolean, None, 0, 0
        self.hash, self.prerequisite = hashlib.sha256(), False

    def consume(self, raw):
        self.count += 1
        self.bytes += len(raw)
        self.hash.update(raw)
        if self.count > 32 or self.bytes > 8192:
            self.prerequisite = True
        if self.boolean:
            if self.count != 1 or raw.strip() not in (b"true", b"false"):
                self.prerequisite = True
            else:
                self.value = raw.strip() == b"true"


def history_probe(execute, deadline, receipt):
    outcomes, shallow = [], None
    commands = (["git", "rev-parse", "--is-shallow-repository"],
                ["git", "cat-file", "-e", BASE + "^{commit}"],
                ["git", "merge-base", "--is-ancestor", BASE, "HEAD"])
    for index, argv in enumerate(commands):
        if index == 2 and (len(outcomes) != 2 or outcomes[1] != 0):
            break
        bound = min(5, deadline - time.monotonic() - 7)
        if bound <= 0:
            raise ValueError("history-deadline")
        capture, status = DiagnosticCapture(index == 0), {}
        execute(argv, bound, capture, status, cwd=APP)
        receipt["processes"].append({"purpose": ("history-shallow", "history-anchor", "history-ancestor")[index],
                                    **safe_process(status), "captureBytes": capture.bytes, "captureSHA256": capture.hash.hexdigest()})
        outcomes.append(status.get("exitCode"))
        if capture.prerequisite or not all(status.get(k) is True for k in ("ownedProcessExited", "ownedGroupSettled", "captureSettled")) or status.get("stopReason") is not None:
            return "history-incomplete"
        if index == 0:
            if status.get("exitCode") != 0:
                return "history-incomplete"
            shallow = capture.value
    return history_kind(shallow, outcomes[1] if len(outcomes) > 1 else None, outcomes[2] if len(outcomes) > 2 else None)


def combined_tools(rows, deadline):
    budget = Budget(deadline)
    for row in rows:
        budget.reserve(row["bytes"])
    tool, state = formatter_binary(deadline, budget=budget)
    return tool, state, sorted([*rows, {"name": "golangci-lint", **state}], key=lambda row: row["name"])


def artifacts(output, receipt, results, state, rows, tools):
    save_new(output / ARTIFACTS[0], receipt)
    save_new(output / ARTIFACTS[1], results)
    save_new(output / ARTIFACTS[2], {"schemaVersion": 1, "state": state, "trackedInputs": rows, "tools": tools})
    records = []
    for name in ARTIFACTS[:3]:
        raw = read_small(output / name, 16 * 1024 * 1024)
        records.append({"name": name, "bytes": len(raw), "sha256": hashlib.sha256(raw).hexdigest()})
    save_new(output / ARTIFACTS[3], {"schemaVersion": 1, "artifacts": records})


def arguments():
    parser = argparse.ArgumentParser()
    parser.add_argument("--expected-revision", required=True)
    parser.add_argument("--output")
    parser.add_argument("--child", type=int, choices=(0, 1))
    parser.add_argument("--tool-sha256")
    return parser.parse_args()


def main():
    args, started = arguments(), time.monotonic()
    try:
        driver = load_driver(started + 60)
        driver.hosted(args)
    except (OSError, ValueError, KeyError, ImportError, TypeError, KeyboardInterrupt):
        print(json.dumps({"phase": "canonical-format", "classification": "prerequisite-blocked",
                          "failureKind": "formatter-driver-prerequisite", "autoAdoption": False}, sort_keys=True))
        return 2
    if args.child is not None:
        if not isinstance(args.tool_sha256, str) or not re.fullmatch(r"[a-f0-9]{64}", args.tool_sha256):
            return 2
        print(json.dumps(bridge(args.child, args.tool_sha256), sort_keys=True, allow_nan=False))
        return 0
    receipt = {"schemaVersion": 1, "phase": "canonical-format", "expectedRevision": args.expected_revision,
               "baseRevision": BASE, "baseTree": BASE_TREE, "invocation": "golangci-fmt-stdin-player-two-v1",
               "startedUTC": datetime.now(timezone.utc).isoformat(), "classification": "prerequisite-blocked",
               "autoAdoption": False, "semanticsVerified": False, "processes": [], "formatterPackets": []}
    output, state, rows, tools, records, executions = None, None, [], [], [], []
    deadline = started + 60
    checks = {name: False for name in IDENTITY_FIELDS}
    try:
        output = driver.private_directory(args.output, True)
        execute = import_execute(deadline)
        receipt["historyKind"] = history_probe(execute, deadline, receipt)
        if receipt["historyKind"] != "history-admitted":
            raise ValueError("history-prerequisite")
        before, rows, base_tools, _go, _gofmt = driver.inputs(execute, "baseline", args.expected_revision, deadline, receipt)
        tool, tool_before, tools = combined_tools(base_tools, deadline)
        if hashlib.sha256(read_small(ROOT / CONFIG, 16384, deadline)).hexdigest() != CONFIG_SHA:
            raise ValueError("formatter-config-pin")
        state = {**before, "toolchainSHA256": digest(tools)}
        receipt["beforeState"] = state
        receipt["formatterBefore"] = tool_before
        for index, name in enumerate(GO_FILES):
            original = read_small(ROOT / name, 16384, deadline)
            projection, status = PacketProjection(index), {}
            bound = min(17, deadline - time.monotonic() - 7)
            if bound <= 0:
                raise ValueError("formatter-phase-deadline")
            argv = [sys.executable, "-B", "-I", str(ROOT / "apps/player/scripts/campaign_r18_document_format.py"),
                    "--expected-revision", args.expected_revision, "--child", str(index), "--tool-sha256", tool_before["sha256"]]
            execute(argv, bound, projection, status, cwd=APP)
            executions.append(safe_process(status))
            receipt["processes"].append({"purpose": "format-source-" + str(index), **safe_process(status)})
            packet = projection.result()
            if packet is not None:
                receipt["formatterPackets"].append(packet | {"outputBase64": None})
            if not settled(status) or packet is None or packet["failureKind"] is not None:
                raise ValueError("formatter-prerequisite")
            records.append(format_record(name, original, base64.b64decode(packet["outputBase64"], validate=True),
                                         packet["formatterExitCode"], packet["stderrBytes"]))
        after, _rows, after_tools, _go, _gofmt = driver.inputs(execute, "baseline", args.expected_revision, deadline, receipt)
        tool_after, state_after, tools_after = combined_tools(after_tools, deadline)
        receipt["afterState"] = {**after, "toolchainSHA256": digest(tools_after)}
        receipt["formatterAfter"] = state_after
        unchanged = before == after and tools == tools_after and tool == tool_after and tool_before == state_after
        checks = {name: unchanged and time.monotonic() < deadline for name in IDENTITY_FIELDS}
        receipt.update(admit_format(records, executions, checks))
    except (OSError, ValueError, KeyError, ImportError, TypeError, KeyboardInterrupt) as failure:
        receipt["failureClass"] = type(failure).__name__
    receipt.pop("_sourceState", None)
    receipt.pop("_trackedInputs", None)
    receipt.pop("_tools", None)
    receipt.update(identity=checks, seconds=round(time.monotonic() - started, 3),
                   finishedUTC=datetime.now(timezone.utc).isoformat())
    if output is not None:
        artifacts(output, receipt, {"schemaVersion": 1, "records": records, "autoAdoption": False}, state, rows, tools)
    print(json.dumps({"phase": receipt["phase"], "classification": receipt["classification"], "autoAdoption": False}, sort_keys=True))
    return 0 if receipt["classification"] == "canonical-source-proposals" else 2


if __name__ == "__main__":
    sys.exit(main())
