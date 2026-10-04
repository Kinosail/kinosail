#!/usr/bin/env python3
"""Run a coordinated build and public native suite with immutable evidence."""
import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import sys
import time
import uuid
from native_result_evidence import METHODS, selected_execution

parser = argparse.ArgumentParser()
parser.add_argument("suite", choices=METHODS)
parser.add_argument("phase")
parser.add_argument("device")
parser.add_argument("--without-building", action="store_true")
parser.add_argument("--build-manifest", type=Path)
parser.add_argument("--iterations", type=int, choices=[1, 2], default=1)
parser.add_argument("--platform", choices=["ios", "tvos"], default="ios")
parser.add_argument("--timeout-seconds", type=int, choices=range(30, 361), default=360, metavar="30..360")
args = parser.parse_args()
uuid.UUID(args.device)
if not re.fullmatch(r"[A-Za-z0-9-]{1,60}", args.phase):
    parser.error("phase must be a bounded filename")
if args.platform == "tvos" and args.suite == "DownloadManagerRecoveryJourneys":
    parser.error("DownloadManagerRecoveryJourneys is iOS-only")
if args.without_building != (args.build_manifest is not None):
    parser.error("test-without-building requires an explicit retained build manifest")
root = Path(__file__).resolve().parents[3]
output = root / ".verification/apple-library-recovery"
output.mkdir(parents=True, exist_ok=True)
name = f"{args.platform}-{args.suite}-{args.phase}"
log = output / (name + ".log")
bundle = output / (name + ".xcresult")
build_bundle = output / (name + "-build.xcresult")
record = output / (name + ".json")
inputs_file = output / (name + "-inputs.json")
manifest_file = output / (name + "-build-manifest.json")
if any(p.exists() for p in [log, bundle, build_bundle, record, inputs_file, manifest_file]):
    parser.error("refusing to overwrite earlier evidence")

def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def write_new(path, value):
    with path.open("x") as handle:
        json.dump(value, handle, indent=2, sort_keys=True)
        handle.write("\n")

def source_inputs():
    names = subprocess.check_output(["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", "apps/player/apps/native"], cwd=root).decode().split("\0")
    return {p: digest(root / p) for p in sorted(set(names)) if p and (root / p).is_file()}

platform = "iOS" if args.platform == "ios" else "tvOS"
app = root / ".verification/apple-library-recovery-derived/Build/Products" / (
    "Debug-iphonesimulator" if args.platform == "ios" else "Debug-appletvsimulator") / "KinosailPlayer.app"

def products():
    return {str(p.relative_to(app)): digest(p) for p in sorted(app.rglob("*")) if p.is_file()}

def signed_products():
    for p in ["KinosailPlayer", "PlugIns/KinosailPlayerTests.xctest/KinosailPlayerTests"]:
        if not (app / p).is_file():
            return False
    try:
        return subprocess.run(["codesign", "--verify", "--deep", "--strict", str(app)], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=10).returncode == 0
    except subprocess.TimeoutExpired:
        return False

revision = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip()
source_hashes = source_inputs()
xcode = subprocess.check_output(["xcodebuild", "-version"], text=True, timeout=10).strip()
inputs = {"workspace_revision": revision, "source_sha256": source_hashes, "platform": platform,
          "xcode": xcode, "runner_sha256": digest(Path(__file__)),
          "result_parser_sha256": digest(Path(__file__).with_name("native_result_evidence.py")),
          "captured": datetime.now(timezone.utc).isoformat()}
build_manifest = None
if args.without_building:
    build_manifest = json.loads(args.build_manifest.read_text())
    original_inputs = Path(build_manifest.get("input_manifest", ""))
    if (build_manifest.get("source_sha256") != source_hashes or build_manifest.get("platform") != platform or
        build_manifest.get("xcode") != xcode or build_manifest.get("product_sha256") != products() or
        build_manifest.get("build_exit_code") != 0 or build_manifest.get("signed_product_verified") is not True or
        not re.fullmatch(r"[0-9a-f]{40}", build_manifest.get("compiled_revision", "")) or
        not original_inputs.is_file() or digest(original_inputs) != build_manifest.get("input_manifest_sha256") or
        json.loads(original_inputs.read_text()).get("source_sha256") != source_hashes or not signed_products()):
        parser.error("current sources/toolchain/signed products do not match the preserved compiled manifest")
write_new(inputs_file, inputs)
start = datetime.now(timezone.utc)
deadline = time.monotonic() + args.timeout_seconds
commands, results = [], []
timed_out = False
common = ["-quiet", "-project", "apps/player/apps/native/Kinosail.xcodeproj", "-scheme", f"Kinosail-{platform}",
          "-destination", f"platform={platform} Simulator,id={args.device}", "-derivedDataPath",
          str(root / ".verification/apple-library-recovery-derived"), "-parallel-testing-enabled", "NO", "-jobs", "2",
          "CODE_SIGNING_ALLOWED=YES", "CODE_SIGN_IDENTITY=-", f"KINOSAIL_SOURCE_REVISION={(build_manifest or {}).get('compiled_revision', revision)}"]

def execute(command, handle):
    global timed_out
    commands.append(command)
    process = subprocess.Popen(command, cwd=root, stdout=handle, stderr=subprocess.STDOUT, start_new_session=True)
    try:
        result = process.wait(timeout=max(0.01, deadline - time.monotonic()))
    except subprocess.TimeoutExpired:
        timed_out = True
        os.killpg(process.pid, signal.SIGTERM)
        try:
            result = process.wait(timeout=15)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGKILL)
            result = process.wait()
    results.append(result)
    return result

def report_json(category, path, suffix):
    try:
        result = subprocess.run(["xcrun", "xcresulttool", "get", *category, "--path", str(path), "--compact"], capture_output=True, text=True, timeout=15)
        if result.returncode:
            return None
        value = json.loads(result.stdout)
    except (subprocess.TimeoutExpired, json.JSONDecodeError):
        return None
    write_new(output / (name + suffix + ".json"), value)
    return value

test_started = False
with log.open("x") as handle:
    if not args.without_building:
        build_status = execute(["xcodebuild", "build-for-testing", *common, "-resultBundlePath", str(build_bundle)], handle)
        if build_status == 0 and source_inputs() == source_hashes and signed_products():
            build_manifest = {**inputs, "compiled_revision": revision, "product_sha256": products(),
                              "input_manifest": str(inputs_file), "input_manifest_sha256": digest(inputs_file),
                              "build_command": commands[-1], "build_result_bundle": str(build_bundle),
                              "build_exit_code": build_status, "signed_product_verified": True}
            write_new(manifest_file, build_manifest)
    if build_manifest and not timed_out and time.monotonic() < deadline:
        test_started = True
        execute(["xcodebuild", "test-without-building", *common, "-resultBundlePath", str(bundle),
                 f"-only-testing:Kinosail-{platform}Tests/{args.suite}", "-test-iterations", str(args.iterations)], handle)

summary = report_json(["test-results", "summary"], bundle, "-test-summary") if test_started and not timed_out else None
tree = report_json(["test-results", "tests"], bundle, "-test-tree") if summary else None
executed, valid_execution = selected_execution(tree, summary, args.suite, args.iterations)
sources_unchanged = source_inputs() == source_hashes
products_unchanged = build_manifest is not None and products() == build_manifest["product_sha256"]
screenshots = []
device_data = Path.home() / "Library/Developer/CoreSimulator/Devices" / args.device / "data/Containers/Data/Application"
for path in device_data.glob("*/tmp/apple-library-recovery-evidence/*.png"):
    if path.is_symlink() or not re.fullmatch(r"[a-z0-9-]{1,80}\.png", path.name) or path.stat().st_mtime < start.timestamp():
        continue
    destination = output / name / path.name
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(path, destination)
    screenshots.append({"file": str(destination), "sha256": digest(destination)})
expected_images = {
    "PhotoAuthorizationJourneys": {f"photo-{status}-{state}.png" for status in [401, 403, 404, 503] for state in ["saved", "settled"]},
    "LibraryRefreshRecoveryJourneys": {"library-complete-failed.png"},
    "DownloadManagerRecoveryJourneys": {"downloads-storage-failed.png", "downloads-network-failed.png"},
}[args.suite]
images_complete = expected_images.issubset({Path(p["file"]).name for p in screenshots})
result_consistent = summary is not None and bool(results) and ((results[-1] == 0) == (summary.get("result") == "Passed"))
accepted = valid_execution and sources_unchanged and products_unchanged and images_complete and result_consistent
write_new(record, {"workspace_revision": revision, "compiled_revision": (build_manifest or {}).get("compiled_revision"),
    "suite": args.suite, "phase": args.phase, "platform": platform, "simulator_id": args.device, "commands": commands,
    "started": start.isoformat(), "finished": datetime.now(timezone.utc).isoformat(), "timeout_seconds": args.timeout_seconds,
    "process_exit_codes": results, "timed_out": timed_out, "matching_executed_tests": executed,
    "execution_complete": valid_execution, "evidence_accepted": accepted, "sources_unchanged": sources_unchanged,
    "products_unchanged": products_unchanged, "images_complete": images_complete,
    "input_manifest": str(inputs_file), "input_manifest_sha256": digest(inputs_file),
    "build_manifest": str(args.build_manifest if args.without_building else manifest_file) if build_manifest else None,
    "screenshots": screenshots, "log": str(log), "log_sha256": digest(log), "result_bundle": str(bundle),
    "fixture": "fictional loopback media and task-scoped storage; no device delivery or user-data removal"})
print(record)
print(json.dumps({"timed_out": timed_out, "execution_complete": valid_execution, "evidence_accepted": accepted,
                  "matching_executed_tests": executed, "process_exit_codes": results}))
sys.exit(124 if timed_out else (results[-1] if accepted else 2))
