#!/usr/bin/env python3
"""Hosted-only, two-phase R18 public contract proof; no media or overlay."""
import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import sys
import time

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parent))
from campaign_r18_document_admission import admit, blocked, settled
from campaign_r18_document_inputs import read_small
from campaign_r18_document_events import PACKAGE, Projection, pairs, constant
from campaign_r18_document_sources import (
    APP, BASE, BASE_TREE, Capture, GO_FILES, ROOT, binary_state, digest,
    import_execute, safe_process, save_new, snapshot, tool_state, valid_handoff,
)

RUN_PATTERN = "^(TestHomeAssistantDocumentTargetsRemainIndependentThroughPublicApp|TestHomeAssistantDocumentSiblingCannotOverwriteReleaseOrDrain|TestHomeAssistantDocumentReleaseKeepsCandidateAndRejectsRetiredClaim|TestHomeAssistantDocumentNativeStrictContractRemainsUnchanged)$"


def arguments():
    parser = argparse.ArgumentParser()
    parser.add_argument("phase", choices=("compile", "run"))
    parser.add_argument("--mode", required=True, choices=("baseline", "candidate"))
    parser.add_argument("--expected-revision", required=True)
    parser.add_argument("--work", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--handoff-sha256")
    return parser.parse_args()


def hosted(args):
    if os.environ.get("GITHUB_ACTIONS") != "true" or os.environ.get("RUNNER_OS") != "Linux":
        raise ValueError("hosted-only")
    if not re.fullmatch(r"[a-f0-9]{40}", args.expected_revision) or os.environ.get("GITHUB_SHA") != args.expected_revision:
        raise ValueError("exact-hosted-revision")
    # Preserve runner/cache locations; remove ambient app/provider and Go/Git overrides.
    for name in tuple(os.environ):
        if (name.startswith(("KINOSAIL_", "AWS_", "AZURE_", "CGO_", "GIT_CONFIG_"))
                or name in ("GOFLAGS", "GOENV", "GOOS", "GOARCH", "GOROOT", "GOWORK", "GOTOOLCHAIN",
                            "GOPROXY", "GONOPROXY", "GOSUMDB", "GONOSUMDB", "GOPRIVATE", "CC", "CXX", "AR")):
            os.environ.pop(name, None)
    os.environ.update(GOMAXPROCS="2", GOPROXY="off", GOTOOLCHAIN="local",
                      GOWORK=str(ROOT / "go.work"), GOENV="off", GOFLAGS="",
                      GIT_OPTIONAL_LOCKS="0", GIT_TERMINAL_PROMPT="0")


def private_directory(raw, fresh):
    root = Path(os.environ["RUNNER_TEMP"]).resolve(strict=True)
    path = Path(raw)
    if not path.is_absolute() or path.parent != root or not re.fullmatch(r"r18-document-[a-z0-9-]{1,48}", path.name):
        raise ValueError("private-directory-boundary")
    if fresh:
        path.mkdir(mode=0o700)
    if path.resolve(strict=True) != path or not path.is_dir() or path.stat().st_mode & 0o077:
        raise ValueError("private-directory-shape")
    return path


def command(execute, argv, kind, deadline, receipt, bound=5):
    remaining = min(bound, deadline - time.monotonic() - 7)
    if remaining <= 0:
        raise ValueError("phase-deadline")
    capture, status = Capture(kind), {}
    execute(argv, remaining, capture, status, cwd=APP)
    receipt["processes"].append({"purpose": kind, **safe_process(status),
                                 "captureLines": capture.count, "captureSHA256": capture.hash.hexdigest()})
    if capture.prerequisite or not settled(status, (0,)):
        raise ValueError("preflight-prerequisite")
    return capture.lines


def inputs(execute, mode, revision, deadline, receipt):
    metadata = command(execute, ["git", "rev-parse", "HEAD", "HEAD^{tree}"], "hash", deadline, receipt)
    if len(metadata) != 2 or metadata[0] != revision:
        raise ValueError("revision-prerequisite")
    command(execute, ["git", "merge-base", "--is-ancestor", BASE, "HEAD"], "silent", deadline, receipt)
    command(execute, ["git", "status", "--porcelain=v1", "--untracked-files=all"], "silent", deadline, receipt)
    listing = command(execute, ["git", "-C", str(ROOT), "-c", "core.quotepath=false", "ls-files", "--stage"], "files", deadline, receipt)
    rows = snapshot(listing, mode, deadline)
    go, formatter, tools = tool_state(deadline)
    state = {"revision": metadata[0], "tree": metadata[1], "sourceSHA256": digest(rows),
             "toolchainSHA256": digest(tools), "inputCount": len(rows)}
    receipt.setdefault("_sourceState", state)
    receipt.setdefault("_trackedInputs", rows)
    receipt.setdefault("_tools", tools)
    return state, rows, tools, go, formatter


def compile_phase(args, work, execute, deadline, receipt):
    before, rows, tools, go, formatter = inputs(execute, args.mode, args.expected_revision, deadline, receipt)
    command(execute, [str(formatter), "-l", *(str(ROOT / name) for name in GO_FILES)], "silent", deadline, receipt)
    command(execute, [str(go), "mod", "verify"], "modules", deadline, receipt, bound=15)
    command(execute, [str(go), "test", "-c", "-p=1", "-o", str(work / "r18-document-targets.test"),
                      "./internal/server"], "silent", deadline, receipt, bound=90)
    product = binary_state(work / "r18-document-targets.test", deadline)
    after, _rows, _tools, _go, _formatter = inputs(execute, args.mode, args.expected_revision, deadline, receipt)
    if after != before or time.monotonic() >= deadline:
        raise ValueError("compile-input-drift")
    handoff = {"schemaVersion": 1, "baseRevision": BASE, "baseTree": BASE_TREE,
               "mode": args.mode, **before, "binary": product,
               "formattingVerified": True, "modulesVerified": True, "compileSettled": True,
               "compileInvocation": "go-test-c-player-r18-v1",
               "runInvocation": "go-test2json-player-r18-four-twice-v1"}
    if not valid_handoff(handoff):
        raise ValueError("compile-handoff-shape")
    save_new(work / "compile-handoff.json", handoff)
    receipt["handoffSHA256"] = hashlib.sha256(read_small(work / "compile-handoff.json", 4096, deadline)).hexdigest()
    receipt["classification"] = "compile-prerequisites-complete"
    return before, rows, tools, None


def read_handoff(args, work, deadline):
    if not isinstance(args.handoff_sha256, str) or not re.fullmatch(r"[a-f0-9]{64}", args.handoff_sha256):
        raise ValueError("handoff-pin-required")
    path = work / "compile-handoff.json"
    raw = read_small(path, 4096, deadline)
    if hashlib.sha256(raw).hexdigest() != args.handoff_sha256:
        raise ValueError("handoff-integrity")
    handoff = json.loads(raw, object_pairs_hook=pairs, parse_constant=constant)
    if not valid_handoff(handoff) or handoff["mode"] != args.mode or handoff["revision"] != args.expected_revision:
        raise ValueError("handoff-prerequisite")
    return handoff


def run_phase(args, work, execute, deadline, receipt):
    handoff = read_handoff(args, work, deadline)
    before, rows, tools, go, _formatter = inputs(execute, args.mode, args.expected_revision, deadline, receipt)
    if any(before[key] != handoff[key] for key in before):
        raise ValueError("handoff-input-drift")
    product = binary_state(work / "r18-document-targets.test", deadline)
    if product != handoff["binary"]:
        raise ValueError("handoff-product-drift")
    argv = [str(go), "tool", "test2json", "-t", "-p", PACKAGE, str(work / "r18-document-targets.test"),
            "-test.v=test2json", "-test.run=" + RUN_PATTERN, "-test.count=2",
            "-test.parallel=1", "-test.timeout=35s"]
    remaining = min(38, deadline - time.monotonic() - 7)
    if remaining <= 0:
        raise ValueError("runtime-deadline")
    projection, status = Projection(), {}
    execute(argv, remaining, projection, status, cwd=APP)
    receipt["processes"].append({"purpose": "eight-public-records", **safe_process(status)})
    data = projection.result()
    receipt["_capturedResults"] = data
    after, _rows, _tools, _go, _formatter = inputs(execute, args.mode, args.expected_revision, deadline, receipt)
    product_after = binary_state(work / "r18-document-targets.test", deadline)
    checks = {"sourceUnchanged": before == after, "toolchainUnchanged": before["toolchainSHA256"] == after["toolchainSHA256"],
              "binaryUnchanged": product == product_after, "exactInvocation": True,
              "formattingVerified": handoff["formattingVerified"], "handoffVerified": True}
    result = admit(data, status, checks, args.mode)
    receipt.update({"classification": result["classification"], "admission": result,
                    "handoffSHA256": args.handoff_sha256, "binary": product, "identity": checks})
    return before, rows, tools, data


def artifacts(output, receipt, state, rows, tools, data):
    save_new(output / "receipt.json", receipt)
    save_new(output / "source-manifest.json", {"state": state, "trackedInputs": rows, "tools": tools})
    save_new(output / "results.json", data)
    records = []
    for name in ("receipt.json", "source-manifest.json", "results.json"):
        raw = read_small(output / name, 16 * 1024 * 1024)
        records.append({"name": name, "bytes": len(raw), "sha256": hashlib.sha256(raw).hexdigest()})
    save_new(output / "artifact-manifest.json", {"schemaVersion": 1, "artifacts": records})


def main():
    args = arguments()
    started = time.monotonic()
    receipt = {"schemaVersion": 1, "phase": args.phase, "mode": args.mode,
               "expectedRevision": args.expected_revision, "baseRevision": BASE, "baseTree": BASE_TREE,
               "startedUTC": datetime.now(timezone.utc).isoformat(),
               "classification": "prerequisite-blocked", "incumbentDefectRED": False,
               "r18FeatureAccepted": False, "processes": []}
    output, state, rows, tools, data = None, None, [], [], None
    try:
        hosted(args)
        output = private_directory(args.output, True)
        work = private_directory(args.work, args.phase == "compile")
        if work == output:
            raise ValueError("separate-output-directory")
        deadline = started + (90 if args.phase == "compile" else 45)
        execute = import_execute(deadline)
        if args.phase == "compile":
            state, rows, tools, data = compile_phase(args, work, execute, deadline, receipt)
        else:
            state, rows, tools, data = run_phase(args, work, execute, deadline, receipt)
        if time.monotonic() >= deadline:
            receipt["classification"] = "prerequisite-blocked"
            receipt["admission"] = blocked()
    except (OSError, ValueError, KeyError, ImportError, KeyboardInterrupt) as failure:
        receipt["failureClass"] = type(failure).__name__
        receipt["admission"] = blocked()
        receipt["classification"] = "prerequisite-blocked"
    data = receipt.pop("_capturedResults", data)
    state = receipt.pop("_sourceState", state)
    rows = receipt.pop("_trackedInputs", rows)
    tools = receipt.pop("_tools", tools)
    receipt["seconds"] = round(time.monotonic() - started, 3)
    receipt["finishedUTC"] = datetime.now(timezone.utc).isoformat()
    if output is not None:
        artifacts(output, receipt, state, rows, tools, data)
    print(json.dumps({"phase": args.phase, "classification": receipt["classification"],
                      "incumbentDefectRED": False, "r18FeatureAccepted": False}, sort_keys=True))
    return 0 if receipt["classification"] in ("compile-prerequisites-complete", "route-prerequisite-absence",
                                              "public-document-contract-green") else 2


if __name__ == "__main__":
    sys.exit(main())
