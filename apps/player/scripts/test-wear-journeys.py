#!/usr/bin/env python3
"""Retain fixture and real-Activity acceptance on an owned hosted round Wear emulator."""
import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import struct
import subprocess
import time
import uuid

APP = "com.kinosail.player.dev"
SERIAL = "emulator-5554"
IMAGE = "system-images;android-36;android-wear-signed;x86_64"
CLASSES = "com.kinosail.player.wear.WearAccessibilityTest,com.kinosail.player.wear.WearLaunchJourneyTest"
EXPECTED_TESTS = {"changingRemoteStatusCanBeAnnounced", "disconnectedStateCanBeAnnounced",
                  "unpairedWatchLaunchesAndSurvivesRecreation"}
STAGES = ("heart-permission", "unpaired", "launch-unpaired", "recreated-unpaired")


def check(condition, code):
    if not condition:
        raise RuntimeError(code)


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def command(args, *, timeout=60, input=None, env=None, check_exit=True):
    return subprocess.run(args, input=input, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                          timeout=timeout, check=check_exit, env=env).stdout


def adb(*args, **options):
    return command([str(SDK / "platform-tools/adb"), "-s", SERIAL, *args], **options)


def collect(safe, label, size):
    destination = safe / label
    destination.mkdir()
    sizes, errors = {}, []
    for stage in STAGES:
        for suffix in (".png", "-semantics.txt"):
            name = "wear-" + stage + suffix
            try:
                data = adb("exec-out", "run-as", APP, "cat", "files/parity-evidence/" + name)
                check(0 < len(data) <= (2 << 20), "wear_evidence_size")
                if suffix == ".png":
                    check(data[:8] == b"\x89PNG\r\n\x1a\n" and data[12:16] == b"IHDR", "wear_evidence_png")
                    dimensions = list(struct.unpack(">II", data[16:24]))
                    check(all(0 < n <= size for n in dimensions), "wear_fixture_dimensions")
                    if stage in ("launch-unpaired", "recreated-unpaired"):
                        check(dimensions == [size, size], "wear_compositor_dimensions")
                    sizes[name] = dimensions
                else:
                    text = data.decode("utf-8")
                    check(len(data) <= 65536 and "\x00" not in text, "wear_semantics_bound")
                (destination / name).write_bytes(data)
                if suffix != ".png":
                    check("Connect phone" in text or "Allow heart rate access" in text, "wear_semantics_missing")
            except Exception as error:
                errors.append({"file": name, "failureClass": str(error) if type(error) is RuntimeError else type(error).__name__})
    return sizes, errors


def run_case(safe, run, label, size, density):
    case = {"name": label, "result": "failed", "stage": "configure", "expectedTests": 3}
    try:
        check(adb("shell", "pm", "clear", APP).strip() == b"Success", "wear_clear_failed")
        adb("shell", "wm", "size", f"{size}x{size}")
        adb("shell", "wm", "density", str(density))
        adb("shell", "input", "keyevent", "KEYCODE_WAKEUP")
        case["displayPixels"] = [size, size]
        case["density"] = density
        case["stage"] = "instrument"
        args = ["shell", "am", "instrument", "-w", "-r", "-e", "class", CLASSES,
                APP + ".test/androidx.test.runner.AndroidJUnitRunner"]
        case["command"] = ["adb", "-s", SERIAL, *args]
        try:
            log = adb(*args, timeout=180)
            case["instrumentationExitCode"] = 0
        except subprocess.CalledProcessError as error:
            log = error.stdout or b""
            case["instrumentationExitCode"] = error.returncode
        except subprocess.TimeoutExpired as error:
            log = error.output or b""
            case["timedOut"] = True
            try:
                adb("shell", "am", "force-stop", APP, timeout=10)
            except Exception as cleanup_error:
                case["timeoutCleanupFailure"] = type(cleanup_error).__name__
        (run / (label + ".instrumentation.private.log")).write_bytes(log[:4 << 20])
        case["logTruncated"] = len(log) > 4 << 20
        case["selectedTests"] = sorted({m.decode() for m in re.findall(
            rb"(?m)^INSTRUMENTATION_STATUS: test=([A-Za-z0-9_]+)\r?$", log)} & EXPECTED_TESTS)
        case["selectedExpectedCount"] = set(re.findall(
            rb"(?m)^INSTRUMENTATION_STATUS: numtests=(\d+)\r?$", log)) == {b"3"}
        case["junitPassed"] = b"OK (3 tests)" in log and b"FAILURES!!!" not in log
        case["skipped"] = b"INSTRUMENTATION_STATUS_CODE: -3" in log or b"INSTRUMENTATION_STATUS_CODE: -4" in log
        case["stage"] = "collect"
        case["screenshots"], case["artifactFailures"] = collect(safe, label, size)
        case["stage"] = "verify"
        check(not case.get("timedOut") and not case["logTruncated"], "wear_instrumentation_bound")
        check(case["selectedExpectedCount"] and set(case["selectedTests"]) == EXPECTED_TESTS,
              "wear_test_selection")
        check(case.get("instrumentationExitCode") == 0 and case["junitPassed"] and not case["skipped"], "wear_junit_failure")
        check(not case["artifactFailures"], "wear_artifact_failure")
        case["result"] = "passed"
    except Exception as error:
        case["failureClass"] = str(error) if type(error) is RuntimeError else type(error).__name__
    return case


parser = argparse.ArgumentParser()
parser.add_argument("--output", type=Path, required=True)
args = parser.parse_args()
check(os.environ.get("GITHUB_ACTIONS") == "true", "requires_disposable_hosted_runner")
ROOT = Path.cwd()
SDK = Path(os.environ["ANDROID_HOME"])
run = args.output.resolve()
run.mkdir(parents=True, exist_ok=False)
safe = run / "safe"
safe.mkdir()
receipt = {"startedAt": datetime.datetime.now(datetime.timezone.utc).isoformat(),
           "revision": None, "result": "failed", "cases": [],
           "boundary": "Owned API36 signed Wear emulator; rendered accessibility fixtures and actual unpaired WearActivity launch/recreation; no phone pairing, sensor, battery, or physical-device proof"}
emulator = None
emulator_log = None
avd_name = "kinosail-wear-" + uuid.uuid4().hex[:12]


def owns_emulator():
    return emulator is not None and emulator.poll() is None and adb(
        "emu", "avd", "name", timeout=5).decode().splitlines()[:1] == [avd_name]
try:
    receipt["revision"] = command(["git", "rev-parse", "HEAD"]).decode().strip()
    check(re.fullmatch(r"[0-9a-f]{40}", receipt["revision"]), "revision_invalid")
    check(not command(["git", "status", "--porcelain"]).strip(), "checkout_not_clean")
    paths = ["apps/player/scripts/test-wear-journeys.py", "apps/player/scripts/inspect-wear-evidence.py", ".github/workflows/app.yml",
             "apps/player/apps/android/wear/build.gradle.kts"]
    source = ROOT / "apps/player/apps/android/wear/src"
    paths += [str(p.relative_to(ROOT)) for p in sorted(source.rglob("*.kt"))]
    receipt["sourceSHA256"] = {p: sha(ROOT / p) for p in paths}
    apks = [ROOT / "apps/player/apps/android/wear/build/outputs/apk/debug/wear-debug.apk",
            ROOT / "apps/player/apps/android/wear/build/outputs/apk/androidTest/debug/wear-debug-androidTest.apk"]
    receipt["apkSHA256"] = {str(p.relative_to(ROOT)): sha(p) for p in apks}
    devices = command([str(SDK / "platform-tools/adb"), "devices"]).decode().splitlines()
    check(devices and devices[0].startswith("List of devices attached"), "runner_device_inventory_invalid")
    check(not any(line.strip() for line in devices[1:]), "runner_device_already_present")
    avd = run / "avd"
    avd.mkdir()
    env = dict(os.environ, ANDROID_AVD_HOME=str(avd))
    command([str(SDK / "cmdline-tools/latest/bin/avdmanager"), "create", "avd", "-n", avd_name,
             "-k", IMAGE], input=b"no\n", env=env)
    configuration = avd / (avd_name + ".avd/config.ini")
    settings = dict(line.split("=", 1) for line in configuration.read_text().splitlines() if "=" in line)
    settings.update({"hw.lcd.width": "384", "hw.lcd.height": "384", "hw.lcd.density": "320",
                     "hw.lcd.circular": "yes", "hw.ramSize": "2048", "hw.cpu.ncore": "2"})
    configuration.write_text("".join(k + "=" + v + "\n" for k, v in settings.items()))
    receipt["avdName"] = avd_name
    receipt["avdConfigurationSHA256"] = sha(configuration)
    receipt["image"] = IMAGE
    properties = SDK / "system-images/android-36/android-wear-signed/x86_64/source.properties"
    receipt["imagePropertiesSHA256"] = sha(properties)
    values = {k.strip(): v.strip() for k, v in (line.split("=", 1) for line in properties.read_text().splitlines() if "=" in line)}
    receipt["imageRevision"] = values.get("Pkg.Revision", "").strip()
    check(re.fullmatch(r"[0-9.]{1,32}", receipt["imageRevision"]), "wear_image_revision")
    emulator_log = (run / "emulator.private.log").open("wb")
    emulator = subprocess.Popen([str(SDK / "emulator/emulator"), "-avd", avd_name, "-port", "5554",
                                 "-no-window", "-no-audio", "-no-snapshot", "-no-boot-anim", "-gpu", "swiftshader_indirect"],
                                env=env, stdout=emulator_log, stderr=subprocess.STDOUT)
    deadline = time.monotonic() + 180
    while time.monotonic() < deadline and emulator.poll() is None:
        try:
            owned = owns_emulator()
        except (OSError, subprocess.SubprocessError):
            owned = False
        if owned and adb("shell", "getprop", "sys.boot_completed", timeout=10, check_exit=False).strip() == b"1":
            break
        time.sleep(2)
    else:
        raise RuntimeError("wear_boot_failed")
    check(owns_emulator(), "wear_emulator_ownership_unconfirmed")
    for prop in ("ro.build.version.sdk", "ro.build.version.release", "ro.product.cpu.abi", "ro.build.characteristics"):
        value = adb("shell", "getprop", prop).decode().strip()
        check(re.fullmatch(r"[A-Za-z0-9_.,-]{1,80}", value), "wear_runtime_property")
        receipt.setdefault("runtime", {})[prop] = value
    check(receipt["runtime"]["ro.build.version.sdk"] == "36", "wear_api_mismatch")
    check("watch" in receipt["runtime"]["ro.build.characteristics"].split(","), "wear_device_mismatch")
    adb("shell", "settings", "put", "global", "stay_on_while_plugged_in", "3")
    for apk in apks:
        check(b"Success" in adb("install", "-r", "-t", str(apk), timeout=120), "wear_install_failed")
    for label, size, density in (("small-round", 384, 320), ("large-round", 454, 320)):
        receipt["cases"].append(run_case(safe, run, label, size, density))
    check(all(case["result"] == "passed" for case in receipt["cases"]), "wear_case_failed")
    receipt["result"] = "passed"
except Exception as error:
    receipt["failureClass"] = str(error) if type(error) is RuntimeError else type(error).__name__
finally:
    cleanup_errors = []

    def observe(name, action):
        try:
            return action()
        except Exception as error:
            cleanup_errors.append({"operation": name, "failureClass": type(error).__name__})
            return None

    if emulator is not None:
        if observe("confirm-owner", owns_emulator):
            observe("console-stop", lambda: adb("emu", "kill", timeout=10))
        observe("join", lambda: emulator.wait(timeout=30))
        if emulator.poll() is None:
            observe("terminate-owned-process", emulator.terminate)
            observe("join-after-terminate", lambda: emulator.wait(timeout=10))
        if emulator.poll() is None:
            observe("kill-owned-process", emulator.kill)
            observe("join-after-kill", lambda: emulator.wait(timeout=10))
        receipt["emulatorSettled"] = emulator.poll() is not None
        devices = observe("final-device-inventory", lambda: command([str(SDK / "platform-tools/adb"), "devices"]))
        receipt["serialGone"] = devices is not None and not any(line.startswith(SERIAL + "\t") for line in devices.decode(errors="replace").splitlines())
    else:
        receipt["emulatorSettled"] = receipt["serialGone"] = True
    if emulator_log is not None:
        observe("close-private-log", emulator_log.close)
    status = observe("final-source-status", lambda: command(["git", "status", "--porcelain"]))
    revision = observe("final-revision", lambda: command(["git", "rev-parse", "HEAD"]))
    receipt["sourceUnchanged"] = status is not None and not status.strip()
    receipt["finalRevision"] = revision.decode().strip() if revision is not None else None
    if cleanup_errors or not receipt["sourceUnchanged"] or receipt["finalRevision"] != receipt["revision"] or not (receipt["emulatorSettled"] and receipt["serialGone"]):
        receipt["result"] = "failed"
        receipt["cleanupFailures"] = cleanup_errors
    receipt["artifactsSHA256"] = observe("hash-artifacts", lambda: {str(p.relative_to(safe)): sha(p) for p in sorted(safe.rglob("*")) if p.is_file()})
    if receipt["artifactsSHA256"] is None:
        receipt["result"] = "failed"
        receipt["cleanupFailures"] = cleanup_errors
    receipt["finishedAt"] = datetime.datetime.now(datetime.timezone.utc).isoformat()
    (safe / "receipt.json").write_text(json.dumps(receipt, indent=2) + "\n")
print(json.dumps({"result": receipt["result"], "cases": [{"name": c["name"], "result": c["result"]} for c in receipt["cases"]]}))
raise SystemExit(0 if receipt["result"] == "passed" else 1)
