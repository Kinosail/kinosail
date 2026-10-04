#!/usr/bin/env python3
"""Bound the reviewed R06 public tests and export only safe, pinned evidence."""

import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import re
import selectors
import signal
import subprocess
import time

from campaign_r06_sources import (APP, ROOT, SCOPE_SHA, checked_scope, current_revision,
    compiled_binary, fingerprint, prepare_overlay, r16_state, save, source_matches, source_state,
    tool_identity, tracked_clean, tracked_tree)
from campaign_r06_projection import PACKAGE, Projection


def signal_owned(pid, signum):
    try:
        os.killpg(pid, signum)
        return "present", None
    except ProcessLookupError:
        return "absent", None
    except OSError as error:
        return "unconfirmed", type(error).__name__


def settle_owned(pid):
    # The leader has already been reaped. Prove the whole owned group absent.
    for signum in (signal.SIGTERM, signal.SIGKILL):
        state, error = signal_owned(pid, 0)
        if state != "present":
            return state == "absent", error
        state, error = signal_owned(pid, signum)
        if state != "present":
            return state == "absent", error
        limit = time.monotonic() + 2
        while time.monotonic() < limit:
            state, error = signal_owned(pid, 0)
            if state != "present":
                return state == "absent", error
            time.sleep(0.025)
    return False, "TimeoutExpired"


def stop_owned(process, receipt):
    # The leader may have exited while a descendant still holds stdout.
    _state, error = signal_owned(process.pid, signal.SIGKILL)
    if error:
        receipt["processStopFailureClass"] = error
        if process.poll() is None:
            try:
                process.kill()
            except ProcessLookupError:
                pass
            except OSError as failure:
                receipt["leaderStopFailureClass"] = type(failure).__name__


def execute(command, bound, projection, receipt, cwd=APP):
    environment = dict(os.environ, GOMAXPROCS="2", GOPROXY="off",
                       GOTOOLCHAIN="local", GOWORK=str(ROOT / "go.work"))
    process, selector, reason, total, pending = None, None, {"stop": None}, 0, b""
    started = time.monotonic()
    deadline = started + bound
    previous = {s: signal.getsignal(s) for s in (signal.SIGINT, signal.SIGTERM)}
    def interrupted(_signum, _frame):
        reason["stop"] = "interrupted"
    receipt.update({"exitCode": None, "ownedProcessExited": False, "ownedGroupSettled": False})
    try:
        for signum in previous:
            signal.signal(signum, interrupted)
        process = subprocess.Popen(command, cwd=cwd, env=environment, stdout=subprocess.PIPE,
                                   stderr=subprocess.STDOUT, start_new_session=True)
        receipt["goWasLaunched"] = True
        os.set_blocking(process.stdout.fileno(), False)
        selector = selectors.DefaultSelector()
        selector.register(process.stdout, selectors.EVENT_READ)
        eof = False
        while not reason["stop"] and not eof:
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                reason["stop"] = "external-timeout"
                break
            if not selector.select(min(remaining, 0.25)):
                continue
            data = os.read(process.stdout.fileno(), 65536)
            if not data:
                eof = True
                if pending and projection is not None:
                    projection.consume(pending)
                    if projection.prerequisite:
                        reason["stop"] = "prerequisite-failure"
                continue
            total += len(data)
            pending += data
            if total > 16 * 1024 * 1024:
                reason["stop"] = "output-bound"
                break
            while b"\n" in pending:
                line, _separator, pending = pending.partition(b"\n")
                if len(line) > 65536:
                    reason["stop"] = "output-bound"
                    break
                if projection is not None:
                    projection.consume(line)
                    if projection.prerequisite:
                        reason["stop"] = "prerequisite-failure"
                        break
            if len(pending) > 65536:
                reason["stop"] = "output-bound"
        # EOF is not proof of process exit. Preserve its natural result first.
        while eof and not reason["stop"]:
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                reason["stop"] = "external-timeout"
                break
            try:
                process.wait(timeout=min(remaining, 0.25))
                break
            except subprocess.TimeoutExpired:
                continue
    except OSError as error:
        receipt["processBoundaryFailureClass"] = type(error).__name__
        reason["stop"] = "process-boundary-failure"
    finally:
        if process:
            if reason["stop"] or process.returncode is None:
                stop_owned(process, receipt)
            try:
                process.wait(timeout=3)
                receipt["ownedProcessExited"] = True
            except subprocess.TimeoutExpired:
                receipt["settlementFailureClass"] = "TimeoutExpired"
                receipt["settlementFailurePhase"] = "leader-reap"
            if receipt["ownedProcessExited"]:
                settled, error = settle_owned(process.pid)
                receipt["ownedGroupSettled"] = settled
                if error:
                    receipt["settlementFailureClass"] = error
                    receipt["settlementFailurePhase"] = "owned-group"
            if not receipt["ownedGroupSettled"] and not reason["stop"]:
                reason["stop"] = "process-settlement-failure"
            receipt["exitCode"] = process.returncode
            process.stdout.close()
        if selector:
            selector.close()
        receipt.update({"seconds": round(time.monotonic() - started, 3), "stopReason": reason["stop"]})
        for signum, handler in previous.items():
            signal.signal(signum, handler)


def complete_process(execution):
    return (execution.get("exitCode") == 0 and not execution.get("stopReason")
            and execution.get("ownedProcessExited") and execution.get("ownedGroupSettled"))


def run_protocol(scope, overlay, projection, receipt, work, before, tree_before):
    binary = work / "public-server.test"
    compilation = {"goWasLaunched": False, "boundSeconds": 120}
    receipt["compilation"] = compilation
    command = ["go", "test", "-c", "-p", "1", "-overlay=" + str(overlay), "-o", str(binary), "./internal/server"]
    execute(command, 120, None, compilation)
    receipt["goWasLaunched"] = compilation["goWasLaunched"]
    receipt["compilationComplete"] = complete_process(compilation)
    if not receipt["compilationComplete"]:
        receipt["stopReason"] = "compile-" + (compilation["stopReason"] or "failed")
        return
    if not (source_matches(scope, source_state(scope)) and r16_state(scope) == before
            and current_revision() == receipt["checkoutRevision"] and tracked_tree(scope) == tree_before and tracked_clean(scope)):
        receipt["stopReason"] = "compile-source-integrity-drift"
        return
    try:
        compiled_binary(binary)
        binary.chmod(0o700)
        receipt["compiledBinaryBefore"] = compiled_binary(binary)
    except (OSError, ValueError) as error:
        receipt["compiledBinaryFailureClass"] = type(error).__name__
        receipt["stopReason"] = "compile-artifact-failure"
        return
    receipt["compiledTestBinarySHA256"] = receipt["compiledBinaryBefore"]["sha256"]
    command = ["go", "tool", "test2json", "-t", "-p", PACKAGE, str(binary), "-test.v=test2json",
               "-test.run=" + scope["runPattern"], "-test.parallel=1", "-test.count=1", "-test.timeout=80s"]
    execution = {"goWasLaunched": False}
    execute(command, 90, projection, execution, cwd=APP / "internal/server")
    receipt.update(execution)
    receipt["publicRuntimeWasLaunched"] = execution["goWasLaunched"]
    receipt["goWasLaunched"] = compilation["goWasLaunched"] or execution["goWasLaunched"]
    try:
        receipt["compiledBinaryAfter"] = compiled_binary(binary)
        receipt["compiledBinaryUnchanged"] = receipt["compiledBinaryAfter"] == receipt["compiledBinaryBefore"]
    except (OSError, ValueError) as error:
        receipt["compiledBinaryFailureClass"] = type(error).__name__


def complete_green(results, execution, unchanged):
    return (unchanged and complete_process(execution) and execution.get("compilationComplete")
            and execution.get("publicRuntimeWasLaunched") and execution.get("compiledBinaryUnchanged")
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
               "bounds": {"compileSeconds": 120, "goSeconds": 80, "externalSeconds": 90, "leaderReapSeconds": 3, "groupSettlementSeconds": 4},
               "goWasLaunched": False, "publicRuntimeWasLaunched": False, "compiledBinaryUnchanged": False,
               "workingDirectory": "apps/subtitles", "compilationWorkingDirectory": "apps/subtitles",
               "publicRuntimeWorkingDirectory": "apps/subtitles/internal/server", "selector": scope["runPattern"],
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
        receipt["compiledEvidenceLimit"] = "Compiled executable stays private; digest/size/mode are compared before and after direct execution through Go test2json. 53 pins identify selected protocol inputs only."
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
            stage = "compile-and-public-runtime"
            run_protocol(scope, overlay, projection, receipt, output / "work", before, tree_before)
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
