#!/usr/bin/env python3
"""Bound the reviewed R06 public tests and export only safe, pinned evidence."""

import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import threading
import time

from campaign_r06_sources import (APP, ROOT, SCOPE_SHA, checked_scope, current_revision,
    fingerprint, prepare_overlay, r16_state, save, source_matches, source_state,
    tool_identity, tracked_clean, tracked_tree)
from campaign_r06_projection import Projection


def kill_owned(process):
    if process is not None:
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass


def execute(scope, overlay, projection, receipt):
    command = ["go", "test", "-p", "1", "-parallel", "1", "-overlay=" + str(overlay),
               "./internal/server", "-run", scope["runPattern"], "-count=1", "-timeout=80s", "-json"]
    environment = dict(os.environ, GOMAXPROCS="2", GOPROXY="off",
                       GOTOOLCHAIN="local", GOWORK=str(ROOT / "go.work"))
    process, timer, reason, total = None, None, {"stop": None}, 0
    started = time.monotonic()
    def deadline():
        reason["stop"] = "external-timeout"
        kill_owned(process)
    previous = {s: signal.getsignal(s) for s in (signal.SIGINT, signal.SIGTERM)}
    def interrupted(_signum, _frame):
        reason["stop"] = "interrupted"
        kill_owned(process)
    try:
        for signum in previous:
            signal.signal(signum, interrupted)
        process = subprocess.Popen(command, cwd=APP, env=environment, stdout=subprocess.PIPE,
                                   stderr=subprocess.STDOUT, start_new_session=True)
        receipt["goWasLaunched"] = True
        timer = threading.Timer(90, deadline)
        timer.start()
        while line := process.stdout.readline(65537):
            total += len(line)
            if len(line) > 65536 or total > 16 * 1024 * 1024:
                reason["stop"] = "output-bound"
                kill_owned(process)
                break
            projection.consume(line)
            if projection.prerequisite:
                reason["stop"] = "prerequisite-failure"
                kill_owned(process)
                break
    finally:
        if timer:
            timer.cancel()
            timer.join(timeout=5)
        if process:
            kill_owned(process)
            process.wait(timeout=5)
            receipt.update({"exitCode": process.returncode, "seconds": round(time.monotonic() - started, 3),
                            "stopReason": reason["stop"], "ownedProcessExited": True})
        for signum, handler in previous.items():
            signal.signal(signum, handler)


def complete_green(results, execution, unchanged):
    return (unchanged and execution.get("exitCode") == 0 and not execution.get("stopReason")
            and results["packageStatus"] == "pass" and results["packageTerminalComplete"] and results["runIdentityComplete"]
            and not any(results[k] for k in ("invalidEventCount", "unallowlistedTestEventCount", "duplicateTerminalCount", "duplicatePackageTerminalCount"))
            and results["counts"]["topLevel"]["pass"] == 33 and results["counts"]["subtests"]["pass"] == 45)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", default=str(ROOT / ".verification/campaign-proof/R06"))
    parser.add_argument("--static-check", action="store_true")
    args = parser.parse_args()
    output = Path(args.output).resolve()
    output.mkdir(parents=True, exist_ok=True)
    if any(output.iterdir()):
        parser.error("refusing to reuse nonempty evidence directory")
    scope = checked_scope()
    projection = Projection(scope)
    sources, before, overlay, stage = source_state(scope), {}, None, "source-preflight"
    tree_before = None
    receipt = {"schemaVersion": 1, "protocolSourceSnapshot": scope["protocolSourceSnapshot"],
               "startedUTC": datetime.now(timezone.utc).isoformat(), "scopeSHA256": SCOPE_SHA,
               "driver": fingerprint(Path(__file__)), "mode": "static-only" if args.static_check else "public-go",
               "bounds": {"goSeconds": 80, "externalSeconds": 90}, "goWasLaunched": False,
               "workingDirectory": "apps/subtitles", "selector": scope["runPattern"],
               "environment": {"GOMAXPROCS": "2", "GOPROXY": "off", "GOTOOLCHAIN": "local", "GOWORK": "repository/go.work"},
               "exportPolicy": "Only fixed names/status/counts/phases and source/artifact hashes; no raw Go output or environment dump."}
    try:
        revision = current_revision()
        if not re.fullmatch(r"[0-9a-f]{40}", revision) or not source_matches(scope, sources):
            raise RuntimeError("source-input-integrity")
        receipt["checkoutRevision"] = revision
        stage = "hosted-revision-preflight"
        expected = os.environ.get("GITHUB_SHA", "")
        if not args.static_check:
            if not re.fullmatch(r"[0-9a-f]{40}", expected) or expected != revision:
                raise RuntimeError("hosted-revision-integrity")
            receipt["expectedHostedRevision"] = expected
        stage = "tracked-source-preflight"
        if not tracked_clean(scope):
            raise RuntimeError("unreviewed-tracked-drift")
        tree_before = tracked_tree(scope)
        receipt["entireTrackedTreeBefore"] = tree_before
        overlay, before = prepare_overlay(scope, output / "work")
        receipt["sourceProvenanceHelper"] = fingerprint(Path(__file__).with_name("campaign_r06_sources.py"))
        receipt["projectionHelper"] = fingerprint(Path(__file__).with_name("campaign_r06_projection.py"))
        receipt["checksumPolicy"] = "Driver does not override GOSUMDB; normal integrity policy retained."
        receipt["compiledTestBinarySHA256"] = None
        receipt["compiledEvidenceLimit"] = "Transient go-test executable digest not captured; 53 pins identify selected protocol inputs only. Entire tracked tree, locks, tool, and driver are separately bound."
        receipt["overlay"] = {"sha256": fingerprint(overlay)["sha256"], "baselineSHA256": scope["overlay"]["baselineSHA256"],
                              "replace": scope["overlay"]["replace"], "exclude": scope["overlay"]["exclude"]}
        if not args.static_check:
            stage = "toolchain-preflight"
            receipt["goToolBinary"] = tool_identity()
            version = subprocess.check_output(["go", "version"], cwd=APP, timeout=5, stderr=subprocess.DEVNULL,
                                              env=dict(os.environ, GOTOOLCHAIN="local")).decode()
            match = re.fullmatch(r"go version (go1\.27(?:\.\d+)?) [a-z0-9]+/[a-z0-9]+\n?", version)
            if not match:
                raise RuntimeError("toolchain-version")
            receipt["goVersion"] = match[1]
            stage = "public-go"
            execute(scope, overlay, projection, receipt)
        stage = "integrity-settlement"
        receipt["sourcesUnchanged"] = source_state(scope) == sources
        receipt["endedRevision"] = current_revision()
        receipt["revisionUnchanged"] = receipt["endedRevision"] == revision
        receipt["entireTrackedTreeAfter"] = tracked_tree(scope)
        receipt["trackedTreeUnchanged"] = receipt["entireTrackedTreeAfter"] == tree_before and tracked_clean(scope)
        receipt["R16Before"], receipt["R16After"] = before, r16_state(scope)
        receipt["R16Preserved"] = receipt["R16Before"] == receipt["R16After"]
        if not all(receipt[key] for key in ("sourcesUnchanged", "revisionUnchanged", "trackedTreeUnchanged", "R16Preserved")):
            raise RuntimeError("execution-integrity-drift")
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError) as error:
        receipt["harnessFailure"] = {"stage": stage, "class": type(error).__name__}
    results = projection.results()
    green = complete_green(results, receipt, receipt.get("sourcesUnchanged") and receipt.get("R16Preserved") and receipt.get("revisionUnchanged") and receipt.get("trackedTreeUnchanged"))
    receipt["classification"] = "focused-public-protocol-green" if green else "static-source-only" if args.static_check and "harnessFailure" not in receipt else "incomplete-or-failed"
    receipt["productBoundary"] = "33 public Go contracts only; browser/playback/lint/full required gates/integration/merge remain separate."
    save(output / "results.json", results)
    save(output / "source-manifest.json", {"schemaVersion": 1, "protocolSourceSnapshot": scope["protocolSourceSnapshot"], "sources": sources})
    save(output / "receipt.json", receipt)
    save(output / "artifact-manifest.json", {"artifacts": [{"path": name, **fingerprint(output / name)} for name in scope["exportAllowlist"] if name != "artifact-manifest.json"]})
    print(json.dumps({"classification": receipt["classification"], "counts": results["counts"]}))
    return 0 if green or receipt["classification"] == "static-source-only" else 124 if receipt.get("stopReason") == "external-timeout" else 1


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError) as error:
        print(json.dumps({"classification": "harness-failed", "class": type(error).__name__, "goLaunchStatus": "unconfirmed; inspect persisted receipt"}))
        raise SystemExit(1) from None
