#!/usr/bin/env python3
"""Independently verify a retained Wear artifact before accepting its runtime claim."""
import hashlib
import io
import json
from pathlib import Path
import re
import subprocess
import sys
import zipfile

from PIL import Image


def check(condition, failure):
    if not condition:
        raise RuntimeError(failure)


def sha(data):
    return hashlib.sha256(data).hexdigest()


archive, revision = Path(sys.argv[1]), sys.argv[2]
check(re.fullmatch(r"[0-9a-f]{40}", revision), "revision_invalid")
stages = ("heart-permission", "unpaired", "launch-unpaired", "recreated-unpaired")
expected_tests = {"changingRemoteStatusCanBeAnnounced", "disconnectedStateCanBeAnnounced",
                  "unpairedWatchLaunchesAndSurvivesRecreation"}
with zipfile.ZipFile(archive) as bundle:
    entries = [item for item in bundle.infolist() if not item.is_dir()]
    check(len(entries) <= 9 and all(0 < item.file_size <= 2 << 20 for item in entries), "artifact_size")
    receipt = json.loads(bundle.read("receipt.json"))
    profile = receipt["profile"]
    check(profile in ("small-round", "large-round"), "profile_invalid")
    files = {f"{profile}/wear-{stage}{suffix}" for stage in stages for suffix in (".png", "-semantics.txt")}
    check(len(entries) == len(files) + 1 and {item.filename for item in entries} == files | {"receipt.json"},
          "artifact_inventory")
    check(receipt["revision"] == revision == receipt["finalRevision"], "revision_mismatch")
    check(receipt["result"] == "passed" and receipt["sourceUnchanged"] is True, "run_failed_or_source_changed")
    check(receipt["emulatorSettled"] is True and receipt["serialGone"] is True and
          not receipt.get("cleanupFailures"), "cleanup_not_verified")
    check(receipt["image"] == "system-images;android-36;android-wear-signed;x86_64", "image_mismatch")
    check(receipt["runtime"]["ro.build.version.sdk"] == "36" and
          "watch" in receipt["runtime"]["ro.build.characteristics"].split(","), "platform_mismatch")
    check(set(receipt["setup"]) == {"device_provisioned", "user_setup_complete"} and
          all(v["before"] in ("0", "1", "null") and v["after"] == "1" for v in receipt["setup"].values()),
          "device_setup_not_verified")
    check(receipt["homeLaunch"]["statusOK"] is True and receipt["homeLaunch"]["complete"] is True,
          "home_launch_not_verified")
    pins = receipt["sourceSHA256"]
    source_files = subprocess.check_output(["git", "ls-tree", "-r", "--name-only", revision,
                                           "apps/player/apps/android/wear/src"], text=True).splitlines()
    expected_pins = {p for p in source_files if p.endswith(".kt")} | {
        "apps/player/scripts/test-wear-journeys.py", "apps/player/scripts/inspect-wear-evidence.py", ".github/workflows/app.yml",
        "apps/player/apps/android/wear/build.gradle.kts", "apps/player/scripts/wear_event_projection.py"}
    check(set(pins) == expected_pins, "source_pin_inventory")
    for path, digest in pins.items():
        check(path.startswith(("apps/player/", ".github/workflows/")) and ".." not in Path(path).parts,
              "source_pin_path")
        source = subprocess.check_output(["git", "show", revision + ":" + path])
        check(sha(source) == digest, "source_pin_mismatch")
    check({"apps/player/scripts/test-wear-journeys.py", "apps/player/scripts/inspect-wear-evidence.py", ".github/workflows/app.yml",
           "apps/player/apps/android/wear/build.gradle.kts"} <= set(pins), "source_pins_missing")
    check(len(receipt["apkSHA256"]) == 2 and all(re.fullmatch(r"[0-9a-f]{64}", value)
          for value in receipt["apkSHA256"].values()), "apk_hashes_missing")
    check(set(receipt["artifactsSHA256"]) == files, "artifact_hash_inventory")
    dimensions = {}
    for path in sorted(files):
        data = bundle.read(path)
        check(sha(data) == receipt["artifactsSHA256"][path], "artifact_hash_mismatch")
        if path.endswith(".png"):
            size = 384 if path.startswith("small-round/") else 454
            with Image.open(io.BytesIO(data)) as picture:
                check(picture.format == "PNG" and all(0 < n <= size for n in picture.size), "png_dimensions")
                dimensions[path] = list(picture.size)
                picture.verify()
            with Image.open(io.BytesIO(data)) as picture:
                picture.load()
                if path.endswith(("launch-unpaired.png", "recreated-unpaired.png")):
                    check(picture.size == (size, size), "compositor_dimensions")
        else:
            check(len(data) <= 65536 and b"\0" not in data, "semantics_bound")
            text = data.decode("utf-8")
            check("Connect phone" in text or "Allow heart rate access" in text, "semantics_missing")
            if path.endswith(("launch-unpaired-semantics.txt", "recreated-unpaired-semantics.txt")):
                check(text.startswith("activeWindowIsApp=true\n"), "foreground_not_verified")
    check(len(receipt["cases"]) == 1 and receipt["cases"][0]["name"] == profile,
          "case_inventory")
    for case in receipt["cases"]:
        check(case["physicalDisplayVerified"] is True, "physical_display_not_verified")
        expected_size = 384 if profile == "small-round" else 454
        check(case["displayPixels"] == [expected_size, expected_size] and case["density"] == 320, "display_mismatch")
        check(case["result"] == "passed" and case["instrumentationExitCode"] == 0 and case["junitPassed"] is True,
              "instrumentation_failed")
        check(case["selectedExpectedCount"] is True and set(case["selectedTests"]) == expected_tests,
              "test_inventory")
        check(not any(case.get(field) for field in ("timedOut", "logTruncated", "skipped", "artifactFailures",
                                                    "timeoutCleanupFailure")), "incomplete_case")
print(json.dumps({"revision": revision, "archiveSHA256": sha(archive.read_bytes()), "sourcePinsVerified": len(pins),
                  "artifactHashesVerified": len(files), "pngDecoded": len(dimensions), "pngDimensions": dimensions,
                  "profile": profile, "casesPassed": 1, "testExecutions": 3,
                  "boundary": "Artifact verification; inspect images visually before accepting UI. APK digests are recorded by the runner; APK bytes are not uploaded or independently rehashed. No physical watch, pairing, sensor or battery proof."}, indent=2))
