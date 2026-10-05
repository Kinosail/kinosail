#!/usr/bin/env python3
"""Hosted Q47 primary baseline only. No install, deployment or product mutation."""
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import sys
import tempfile
import time
import campaign_q47_execution as processes
from campaign_q47_execution import Peer, execute, complete, environment
from campaign_q47_admission import admit, selector_valid
from campaign_q47_dependencies import receipt_stage
from campaign_q47_sources import ROOT, APP, BASE, FIXTURE_FILES, canonical, source_snapshot, dependencies, generated, fingerprint, private_report

OUTPUT = ROOT / ".verification/campaign-proof/Q47"
FILES = ("receipt.json", "results.json", "source-manifest.json", "artifact-manifest.json")
BOUNDS = {"dependency-check": 10, "reporter-controls": 10, "docs-build": 120, "fixture-compile": 90, "collection": 15,
          "primary-1": 75, "primary-2": 75}
LIMITS = {"admissionBudgetSeconds": 540, "commandCleanupSeconds": 7, "peerReadySeconds": 5,
          "peerLifetimeSeconds": 105, "privateReportBytes": 262144, "artifactBytes": 4194304}
COMMANDS = {"reporter-controls": "node --test apps/player/e2e/compose-template-proof-attachments.controls.cjs",
            "docs-build": "python3 engineering/documentation/build.py --output <new RUNNER_TEMP site>",
            "fixture-compile": "go build -p 1 -tags q47proof -o <new private binary> <declared fixture files>",
            "collection": "node <installed pinned Playwright CLI> test --config compose-template-recovery.config.ts --list --workers=1 --retries=0",
            "primary": "node <installed pinned Playwright CLI> test --config compose-template-recovery.config.ts --workers=1 --retries=0"}


def output_boundary():
    current = ROOT
    for name in (".verification", "campaign-proof", "Q47"):
        current /= name
        if current.is_symlink() or current.exists() and not current.is_dir():
            raise ValueError("artifact_directory")
    if OUTPUT.exists() and any(OUTPUT.iterdir()):
        raise ValueError("artifact_not_empty")


def temporary_boundary():
    value = os.environ.get("RUNNER_TEMP", "")
    path = Path(value)
    if (not value or len(value) > 4096 or not path.is_absolute() or path == Path("/")
            or str(path.resolve(strict=True)) != value or not path.is_dir() or path.is_relative_to(ROOT)):
        raise ValueError("runner_temporary")
    return Path(tempfile.mkdtemp(prefix="q47-public-", dir=path))


def require_version(command, pattern, cwd):
    result, output = execute(command, 10, cwd, environment(), label="dependency-check")
    if not complete(result, 0) or len(output) > 512 or re.fullmatch(pattern, output.decode("ascii").strip()) is None:
        raise ValueError("tool_version")


def preflight(tools):
    require_version([str(tools["go"]), "version"], r"go version go1\.27(?:\.[0-9]+)? linux/amd64", ROOT)
    require_version([str(tools["node"]), "--version"], r"v26\.[0-9]+\.[0-9]+", ROOT)
    require_version([str(tools["ruby"]), "--version"], r"ruby 3\.4\.[^\n]{1,300}", ROOT)
    require_version([str(tools["bundle"]), "--version"], r"(?:Bundler version )?4\.0\.16", ROOT)
    env = environment() | {"BUNDLE_GEMFILE": str(ROOT / "engineering/documentation/Gemfile")}
    result, _private = execute([str(tools["bundle"]), "check"], 10, ROOT / "engineering/documentation", env,
                              label="dependency-check")
    if not complete(result, 0):
        raise ValueError("locked_gems_missing")


def command_phase(name, command, cwd, env, hook=None):
    result, _private = execute(command, BOUNDS[name], cwd, env, hook, label=name)
    return result


def reporter_controls(tools):
    command = [str(tools["node"]), "--test", "apps/player/e2e/compose-template-proof-attachments.controls.cjs"]
    terminal = command_phase("reporter-controls", command, ROOT, environment())
    if not complete(terminal, 0):
        raise ValueError("reporter_controls")


def browser_phase(name, root, node, cli, pins, peer=None):
    destination = root / name
    destination.mkdir(mode=0o700)
    env = environment() | {"KINOSAIL_Q47_SUITE": "primary", "KINOSAIL_Q47_REPORT_FILE": str(destination / "q47-proof.json"),
                           "KINOSAIL_E2E_OUTPUT_DIR": str(destination / "browser-private")}
    command = [str(node), str(cli), "test", "--config", "compose-template-recovery.config.ts", "--workers=1", "--retries=0"]
    if name == "collection":
        env["KINOSAIL_Q47_COLLECTION_ONLY"] = "1"
        command.append("--list")
    terminal = command_phase(name, command, APP, env, peer.live if peer else None)
    value = private_report(destination / "q47-proof.json")
    admission = admit(value, "collection" if name == "collection" else "journey", "primary", pins)
    expected_exit = 0 if admission["classification"] in ("collection", "green") else 1
    if (admission["classification"] in ("invalid", "incomplete") or not complete(terminal, expected_exit)):
        admission["classification"] = "incomplete"
    return admission


def peer_complete(value):
    return (value.get("interrupted") is False and value["ready"] is True and value["stopped"] is True and value["requestedStop"] is True
            and value["exitCode"] == 0 and value["leaderExited"] is True
            and value["groupAbsent"] is True and value["captureComplete"] is True)


def artifacts(receipt, results, sources):
    values = {"receipt.json": receipt, "results.json": results, "source-manifest.json": sources}
    encoded = {name: canonical(value) for name, value in values.items()}
    if any(len(data) > LIMITS["artifactBytes"] for data in encoded.values()):
        raise ValueError("artifact_size")
    manifest = {"schemaVersion": 1, "campaign": "Q47", "kind": "public-baseline",
                "files": [{"name": name, "bytes": len(encoded[name]), "sha256": hashlib.sha256(encoded[name]).hexdigest()}
                          for name in FILES[:3]]}
    encoded["artifact-manifest.json"] = canonical(manifest)
    # Preserve every private temporary, report and binary; only safe JSON is eligible for upload.
    for name in (".verification", "campaign-proof", "Q47"):
        path = ROOT / name if name == ".verification" else path / name
        path.mkdir(mode=0o700, exist_ok=True)
        if path.is_symlink() or not path.is_dir():
            raise ValueError("artifact_directory")
    for name in FILES:
        with (OUTPUT / name).open("xb") as stream:
            os.chmod(OUTPUT / name, 0o600)
            stream.write(encoded[name])


def main():
    phase = "source-prerequisite"
    before = after = dependency_pins = built = binary_pin = None
    tools = cli = binary = site = temporary = None
    receipt = {"schemaVersion": 1, "campaign": "Q47", "kind": "public-baseline", "base": BASE,
               "result": "incomplete", "blockedPhase": None, "dependencyStage": None, "sourceUnchanged": None,
               "dependenciesUnchanged": None, "generatedUnchanged": None, "binaryUnchanged": None,
               "ownedProcessesSettled": None, "limits": LIMITS, "bounds": BOUNDS, "commands": COMMANDS, "processes": []}
    results = {"schemaVersion": 1, "campaign": "Q47", "suite": "primary", "collection": None, "runs": [],
               "classification": "incomplete", "attemptedRuns": 0,
               "scope": "Two primary public generated-docs cases repeated twice; later modes remain unrun."}
    sources = {"schemaVersion": 1, "campaign": "Q47", "before": None, "after": None,
               "dependencies": None, "generated": None, "binary": None, "fixtureFiles": list(FIXTURE_FILES)}
    previous = {}
    output_safe = False
    try:
        if not selector_valid(sys.argv, os.environ):
            raise ValueError("fixed_selector")
        output_boundary(); output_safe = True
        for number in (signal.SIGINT, signal.SIGTERM):
            previous[number] = signal.getsignal(number)
            signal.signal(number, lambda _number, _frame: (_ for _ in ()).throw(KeyboardInterrupt()))
        processes.PROOF_DEADLINE = time.monotonic() + LIMITS["admissionBudgetSeconds"]
        before = source_snapshot(); sources["before"] = before
        temporary = temporary_boundary()
        phase = "dependency-check"
        tools, cli, dependency_pins = dependencies()
        sources["dependencies"] = dependency_pins
        preflight(tools)
        phase = "reporter-controls"
        reporter_controls(tools)
        phase = "docs-build"
        site = temporary / "site"
        terminal = command_phase(phase, [sys.executable, str(ROOT / "engineering/documentation/build.py"), "--output", str(site)],
                                 ROOT, environment())
        if not complete(terminal, 0):
            raise ValueError("docs_build")
        built, pins = generated(site); sources["generated"] = built
        phase = "fixture-compile"
        binary = temporary / "q47-peer"
        # Explicit declared reviewed paths, never a glob or an arbitrary environment filename.
        command = [str(tools["go"]), "build", "-p", "1", "-tags", "q47proof", "-o", str(binary),
                   *[str(ROOT / name) for name in FIXTURE_FILES]]
        terminal = command_phase(phase, command, ROOT, environment())
        if not complete(terminal, 0):
            raise ValueError("fixture_compile")
        binary_pin = fingerprint(binary, 64 * 1024 * 1024); sources["binary"] = binary_pin
        phase = "collection"
        results["collection"] = browser_phase(phase, temporary, tools["node"], cli, pins)
        if results["collection"]["classification"] != "collection":
            raise ValueError("collection")
        for index in (1, 2):
            phase = "primary-" + str(index)
            peer = Peer(binary, site, ROOT)
            admission = {"classification": "incomplete", "report": None}
            settlement = None
            try:
                if not peer.start():
                    raise ValueError("fixture_ready")
                results["attemptedRuns"] += 1
                admission = browser_phase(phase, temporary, tools["node"], cli, pins, peer)
            finally:
                settlement = peer.stop()
                results["runs"].append({"repeat": index, **admission, "peer": settlement})
            if not peer_complete(settlement) or admission["classification"] not in ("green", "deadline-contract-red"):
                raise ValueError("primary_incomplete")
        classes = [value["classification"] for value in results["runs"]]
        if classes not in (["green", "green"], ["deadline-contract-red", "deadline-contract-red"]):
            raise ValueError("repeat_disagreement")
        phase = "final-provenance"
        after = source_snapshot()
        sources["after"] = {**after, "tracked": {key: after["tracked"][key] for key in ("count", "sha256")}}
        _, final_cli, final_dependencies = dependencies()
        final_generated, _pins = generated(site)
        receipt.update(sourceUnchanged=before == after, dependenciesUnchanged=dependency_pins == final_dependencies and cli == final_cli,
                       generatedUnchanged=built == final_generated, binaryUnchanged=binary_pin == fingerprint(binary, 64 * 1024 * 1024))
        if not all(receipt[key] is True for key in ("sourceUnchanged", "dependenciesUnchanged", "generatedUnchanged", "binaryUnchanged")):
            raise ValueError("provenance_changed")
        receipt["ownedProcessesSettled"] = all(value["groupAbsent"] is True and value["leaderExited"] is True
                                                for value in processes.PROCESS_RECORDS if value.get("launched", True))
        if receipt["ownedProcessesSettled"] is not True:
            raise ValueError("owned_group_unsettled")
        receipt["result"] = results["classification"] = classes[0]
    except (OSError, ValueError, RuntimeError, KeyError, TypeError, StopIteration, UnicodeError, KeyboardInterrupt):
        receipt["blockedPhase"] = phase
        receipt["dependencyStage"] = receipt_stage(phase)
        receipt["result"] = results["classification"] = "incomplete"
    finally:
        receipt["processes"] = processes.PROCESS_RECORDS
        if receipt["ownedProcessesSettled"] is None:
            receipt["ownedProcessesSettled"] = all(value["groupAbsent"] is True and value["leaderExited"] is True
                                                    for value in processes.PROCESS_RECORDS if value.get("launched", True))
        for number, handler in previous.items():
            signal.signal(number, handler)
        if output_safe:
            artifacts(receipt, results, sources)
    return 0 if results["classification"] == "green" else 1 if results["classification"] == "deadline-contract-red" else 2


if __name__ == "__main__":
    try:
        exit_code = main()
    except (OSError, ValueError, RuntimeError, KeyError, TypeError, StopIteration, UnicodeError, KeyboardInterrupt):
        exit_code = 2
    raise SystemExit(exit_code)
