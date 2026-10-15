#!/usr/bin/env python3
"""Plan and run one sharded media proof from the checked-in manifest; failures and evidence are never silent."""

import argparse
import glob
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import subprocess
import sys
import time


ROOT = Path(__file__).resolve().parents[2]
MANIFEST = ROOT / "scripts/ci/proofs.json"
PROOF_APPS = ("player",)
SHARD_NAME = re.compile(r"^[a-z0-9]+(-[a-z0-9]+)*$")
ENV_NAME = re.compile(r"^KINOSAIL_[A-Z0-9_]{1,60}$")


def _bool(value, message):
    if type(value) is not bool:
        raise ValueError(message)
    return value


def _evidence_pattern(pattern):
    if (not isinstance(pattern, str) or not 0 < len(pattern) <= 200 or not pattern.startswith(".verification/")
            or any(part in ("", ".", "..") for part in pattern.split("/")) or "\0" in pattern):
        raise ValueError("evidence pattern must stay inside .verification")
    return pattern


def _command(entry):
    if not isinstance(entry, dict) or not {"argv"} <= set(entry) <= {"argv", "env", "always"}:
        raise ValueError("proof command must declare argv with optional env and always")
    argv = entry["argv"]
    if (not isinstance(argv, list) or not 0 < len(argv) <= 64
            or any(not isinstance(item, str) or not 0 < len(item) <= 200 or "\0" in item for item in argv)
            or argv[0] != "python3" or any(".." in PurePosixPath(item).parts for item in argv)
            or not any(item == "apps/player/scripts" or item.startswith("apps/player/scripts/") for item in argv[1:])):
        raise ValueError("proof command must run a repository Python script")
    env = entry.get("env", {})
    if (not isinstance(env, dict) or len(env) > 8
            or any(not isinstance(key, str) or not ENV_NAME.fullmatch(key)
                   or not isinstance(value, str) or not 0 < len(value) <= 200 for key, value in env.items())):
        raise ValueError("proof command environment must be bounded KINOSAIL_ names")
    return {"argv": argv, "env": env, "always": _bool(entry.get("always", False), "always must be a boolean")}


def _shard(name, raw):
    if not isinstance(name, str) or not SHARD_NAME.fullmatch(name) or len(name) > 32:
        raise ValueError("invalid proof shard name")
    if not isinstance(raw, dict) or set(raw) != {"browser", "timeout", "gomaxprocs", "commands", "evidence"}:
        raise ValueError(f"shard {name} must declare browser, timeout, gomaxprocs, commands, and evidence")
    timeout, gomaxprocs = raw["timeout"], raw["gomaxprocs"]
    if type(timeout) is not int or not 1 <= timeout <= 45 or type(gomaxprocs) is not int or not 1 <= gomaxprocs <= 8:
        raise ValueError(f"shard {name} timeout or gomaxprocs is out of range")
    commands, evidence = raw["commands"], raw["evidence"]
    if not isinstance(commands, list) or not 0 < len(commands) <= 64 or not isinstance(evidence, list) or not 0 < len(evidence) <= 32:
        raise ValueError(f"shard {name} must list bounded commands and evidence")
    return {"browser": _bool(raw["browser"], f"shard {name} browser must be a boolean"), "timeout": timeout,
            "gomaxprocs": gomaxprocs, "commands": [_command(entry) for entry in commands],
            "evidence": [_evidence_pattern(pattern) for pattern in evidence]}


def load(path=MANIFEST):
    path = Path(path)
    if path.stat().st_size > 256 * 1024:
        raise ValueError("proof manifest too large")
    manifest = json.loads(path.read_text())
    if not isinstance(manifest, dict) or set(manifest) != set(PROOF_APPS):
        raise ValueError("proof manifest must cover exactly the media-proof apps")
    loaded = {}
    for app, shards in manifest.items():
        if not isinstance(shards, dict) or not 0 < len(shards) <= 16:
            raise ValueError(f"{app} must declare between one and sixteen shards")
        loaded[app] = {name: _shard(name, raw) for name, raw in shards.items()}
    return loaded


def plan(app, manifest):
    if app not in PROOF_APPS:
        raise ValueError("invalid app")
    return [{"shard": name, "browser": shard["browser"], "timeout": shard["timeout"]}
            for name, shard in manifest[app].items()]


def _collect(patterns, destination):
    copied = 0
    for pattern in patterns:
        matches = sorted(glob.glob(pattern, root_dir=ROOT, recursive=True))
        if not matches:
            print(f"no evidence matched {pattern}", file=sys.stderr)
        for relative in matches:
            source = ROOT / relative
            if source.is_file() and not source.is_symlink():
                target = destination / relative
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(source, target)
                copied += 1
    return copied


def run(app, name, manifest, evidence_dir):
    if app not in PROOF_APPS or name not in manifest[app]:
        raise ValueError("unknown proof shard")
    shard = manifest[app][name]
    evidence_dir = Path(evidence_dir)
    evidence_dir.mkdir(parents=True, exist_ok=True)
    base = {**os.environ, "GOMAXPROCS": str(shard["gomaxprocs"])}
    results, failed = [], False
    for index, command in enumerate(shard["commands"]):
        record = {"index": index, "argv": command["argv"], "env": sorted(command["env"])}
        if failed and not command["always"]:
            record["status"] = "skipped"
        else:
            print(f"::group::{name} [{index}] {' '.join(command['argv'])}", flush=True)
            started = time.monotonic()
            completed = subprocess.run(command["argv"], cwd=ROOT, env={**base, **command["env"]},
                                       timeout=shard["timeout"] * 60, check=False)
            record.update(status="passed" if completed.returncode == 0 else "failed",
                          exitCode=completed.returncode, seconds=round(time.monotonic() - started, 1))
            print("::endgroup::", flush=True)
            failed = failed or completed.returncode != 0
        results.append(record)
    copied = _collect(shard["evidence"], evidence_dir)
    revision = subprocess.run(["git", "rev-parse", "HEAD"], cwd=Path(__file__).resolve().parents[2], capture_output=True, text=True, check=True).stdout.strip()
    receipt = {"app": app, "shard": name, "revision": revision, "gomaxprocs": shard["gomaxprocs"],
               "commands": results, "evidenceFiles": copied, "result": "failed" if failed else "passed"}
    (evidence_dir / "shard-receipt.json").write_text(json.dumps(receipt, indent=2) + "\n")
    print(json.dumps({key: receipt[key] for key in ("app", "shard", "result", "evidenceFiles")}))
    if not failed and copied == 0:
        raise ValueError("proof shard passed without producing evidence")
    return not failed


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest", default=str(MANIFEST))
    commands = parser.add_subparsers(dest="command", required=True)
    planner = commands.add_parser("plan")
    planner.add_argument("app")
    runner = commands.add_parser("run")
    runner.add_argument("app")
    runner.add_argument("shard")
    runner.add_argument("--evidence-dir", required=True)
    args = parser.parse_args()
    manifest = load(args.manifest)
    if args.command == "plan":
        shards = plan(args.app, manifest)
        encoded = json.dumps(shards, separators=(",", ":"))
        with open(os.environ["GITHUB_OUTPUT"], "a") as output:
            output.write("shards=" + encoded + "\n")
        print("Proof shards: " + ", ".join(shard["shard"] for shard in shards))
        return
    if not run(args.app, args.shard, manifest, args.evidence_dir):
        sys.exit(1)


if __name__ == "__main__":
    main()
