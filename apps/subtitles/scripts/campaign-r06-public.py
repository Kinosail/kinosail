#!/usr/bin/env python3
"""Bound the reviewed R06 public tests and export only safe, pinned evidence."""

import argparse
from datetime import datetime, timezone
import hashlib
import json
import math
import os
from pathlib import Path
import re
import signal
import subprocess
import threading
import time

ROOT = Path(__file__).resolve().parents[3]
APP = ROOT / "apps/subtitles"
RECIPE = APP / "engineering/qa/2026-10-04-action-recovery/hosted"
SCOPE_SHA = "7de3b390b051d2053e715674601f52854ae9b1074a2732599607cdd131d69ea5"
PREREQUISITES = {
    "startup admission prerequisite remained busy": "startup-admission-prerequisite",
    "public operation preparation =": "preparation-prerequisite",
    "public operation status =": "private-status-prerequisite",
    "legacy public audio prerequisite =": "audio-prerequisite",
    "legacy public Save setup =": "save-prerequisite",
    "controlled descendant process prerequisite did not become ready": "child-prerequisite",
    "fictional analysis process did not start": "analysis-prerequisite",
    "durable-boundary fixture prerequisite:": "durable-fixture-prerequisite",
}


def fingerprint(path):
    if not path.exists():
        return {"present": False}
    data = path.read_bytes()
    return {"present": True, "bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()}


def save(path, value):
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def source_state(scope):
    return [{"path": item["path"], **fingerprint(ROOT / item["path"])}
            for item in scope["sourcePins"]]


def source_matches(scope, actual):
    return all(observed.get("present") and observed["bytes"] == expected["bytes"]
               and observed["sha256"] == expected["sha256"]
               for expected, observed in zip(scope["sourcePins"], actual, strict=True))


def r16_state(scope):
    return {name: fingerprint(ROOT / name) for name in scope["overlay"]["preservedLocalR16"]}


def checked_scope():
    data = (RECIPE / "protocol-scope.json").read_bytes()
    if hashlib.sha256(data).hexdigest() != SCOPE_SHA:
        raise RuntimeError("scope-integrity")
    scope = json.loads(data)
    names = scope["namedR06Tests"] + scope["incumbentControls"]
    if len(names) != 33 or len(set(names)) != 33 or len(scope["expectedSubtests"]) != 45:
        raise RuntimeError("selector-integrity")
    if scope["runPattern"] != "^(" + "|".join(names) + ")$":
        raise RuntimeError("selector-integrity")
    return scope


def prepare_overlay(scope, work):
    config = scope["overlay"]
    fixture = fingerprint(ROOT / config["baselineFixture"])
    if fixture.get("sha256") != config["baselineSHA256"] or fixture.get("bytes") != config["baselineBytes"]:
        raise RuntimeError("overlay-fixture-integrity")
    before = r16_state(scope)
    for name, state in before.items():
        allowed = {config["preservedLocalR16"][name]["sha256"]}
        if name == config["replace"]:
            allowed.add(config["baselineSHA256"])
        if state.get("present") and state.get("sha256") not in allowed:
            raise RuntimeError("r16-original-integrity")
    if not before[config["replace"]]["present"]:
        raise RuntimeError("r16-baseline-missing")
    work.mkdir()
    overlay = work / "overlay.json"
    save(overlay, {"Replace": {str(ROOT / config["replace"]): str(ROOT / config["baselineFixture"]),
                              str(ROOT / config["exclude"]): ""}})
    return overlay, before


def kill_owned(process):
    if process is not None and process.poll() is None:
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass


class Projection:
    def __init__(self, scope):
        self.top = scope["namedR06Tests"] + scope["incumbentControls"]
        self.sub = scope["expectedSubtests"]
        self.rows, self.runs, self.phases = {}, {}, {}
        self.package = None
        self.invalid = self.unknown = self.duplicates = 0
        self.prerequisite = None

    def consume(self, line):
        try:
            event = json.loads(line)
        except (ValueError, UnicodeError):
            self.invalid += 1
            return
        if not isinstance(event, dict):
            self.invalid += 1
            return
        name, action = event.get("Test"), event.get("Action")
        if event.get("Package") != "github.com/MikeO7/kinosail-subtitles/internal/server":
            if name or action in ("pass", "fail"):
                self.invalid += 1
            return
        if not name:
            if action in ("pass", "fail"):
                self.package = action
            return
        if name not in self.top and name not in self.sub:
            self.unknown += 1
            return
        if action == "run":
            self.runs[name] = self.runs.get(name, 0) + 1
        elif action == "output" and isinstance(event.get("Output"), str):
            for marker, phase in PREREQUISITES.items():
                if marker in event["Output"]:
                    self.phases[name] = phase
        elif action in ("pass", "fail", "skip"):
            elapsed = event.get("Elapsed", 0)
            if isinstance(elapsed, bool) or not isinstance(elapsed, (int, float)) or not math.isfinite(elapsed) or not 0 <= elapsed <= 90:
                self.invalid += 1
                elapsed = None
            if name in self.rows:
                self.duplicates += 1
            self.rows[name] = {"name": name, "status": action, "seconds": elapsed}
            if action == "fail":
                self.rows[name]["failurePhase"] = self.phases.get(name, "public-assertion")
                self.prerequisite = self.phases.get(name)

    def results(self):
        def counts(names):
            statuses = [self.rows.get(n, {}).get("status") for n in names]
            return {"expected": len(names), **{s: statuses.count(s) for s in ("pass", "fail", "skip")},
                    "notCompleted": statuses.count(None)}
        return {"topLevel": [self.rows.get(n, {"name": n, "status": "not-completed"}) for n in self.top],
                "subtests": [self.rows.get(n, {"name": n, "status": "not-completed"}) for n in self.sub],
                "counts": {"topLevel": counts(self.top), "subtests": counts(self.sub)},
                "packageStatus": self.package, "invalidEventCount": self.invalid,
                "unallowlistedTestEventCount": self.unknown, "duplicateTerminalCount": self.duplicates,
                "runIdentityComplete": all(self.runs.get(n) == 1 for n in self.top + self.sub),
                "prerequisiteFailurePhase": self.prerequisite}


def execute(scope, overlay, projection, receipt):
    command = ["go", "test", "-p", "1", "-parallel", "1", "-overlay=" + str(overlay),
               "./internal/server", "-run", scope["runPattern"], "-count=1", "-timeout=80s", "-json"]
    environment = dict(os.environ, GOMAXPROCS="2", GOPROXY="off", GOSUMDB="off",
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
        process.wait(timeout=5)
    finally:
        if timer:
            timer.cancel()
        if process:
            if process.poll() is None:
                kill_owned(process)
            process.wait(timeout=5)
            receipt.update({"exitCode": process.returncode, "seconds": round(time.monotonic() - started, 3),
                            "stopReason": reason["stop"], "ownedProcessExited": True})
        for signum, handler in previous.items():
            signal.signal(signum, handler)


def complete_green(results, execution, unchanged):
    return (unchanged and execution.get("exitCode") == 0 and not execution.get("stopReason")
            and results["packageStatus"] == "pass" and results["runIdentityComplete"]
            and not any(results[k] for k in ("invalidEventCount", "unallowlistedTestEventCount", "duplicateTerminalCount"))
            and results["counts"]["topLevel"]["pass"] == 33 and results["counts"]["subtests"]["pass"] == 45)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True)
    parser.add_argument("--static-check", action="store_true")
    args = parser.parse_args()
    output = Path(args.output).resolve()
    output.mkdir(parents=True, exist_ok=True)
    if any(output.iterdir()):
        parser.error("refusing to reuse nonempty evidence directory")
    scope = checked_scope()
    projection = Projection(scope)
    sources, before, overlay, stage = source_state(scope), {}, None, "source-preflight"
    receipt = {"schemaVersion": 1, "protocolSourceSnapshot": scope["protocolSourceSnapshot"],
               "startedUTC": datetime.now(timezone.utc).isoformat(), "scopeSHA256": SCOPE_SHA,
               "driver": fingerprint(Path(__file__)), "mode": "static-only" if args.static_check else "public-go",
               "bounds": {"goSeconds": 80, "externalSeconds": 90}, "goWasLaunched": False,
               "workingDirectory": "apps/subtitles", "selector": scope["runPattern"],
               "environment": {"GOMAXPROCS": "2", "GOPROXY": "off", "GOSUMDB": "off", "GOTOOLCHAIN": "local", "GOWORK": "repository/go.work"},
               "exportPolicy": "Only fixed names/status/counts/phases and source/artifact hashes; no raw Go output or environment dump."}
    try:
        revision = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, timeout=5, stderr=subprocess.DEVNULL).decode().strip()
        if not re.fullmatch(r"[0-9a-f]{40}", revision) or not source_matches(scope, sources):
            raise RuntimeError("source-input-integrity")
        receipt["checkoutRevision"] = revision
        overlay, before = prepare_overlay(scope, output / "work")
        receipt["overlay"] = {"sha256": fingerprint(overlay)["sha256"], "baselineSHA256": scope["overlay"]["baselineSHA256"],
                              "replace": scope["overlay"]["replace"], "exclude": scope["overlay"]["exclude"]}
        if not args.static_check:
            stage = "toolchain-preflight"
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
        receipt["R16Before"], receipt["R16After"] = before, r16_state(scope)
        receipt["R16Preserved"] = receipt["R16Before"] == receipt["R16After"]
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError) as error:
        receipt["harnessFailure"] = {"stage": stage, "class": type(error).__name__}
    results = projection.results()
    green = complete_green(results, receipt, receipt.get("sourcesUnchanged") and receipt.get("R16Preserved"))
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
