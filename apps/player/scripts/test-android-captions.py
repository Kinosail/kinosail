#!/usr/bin/env python3
"""Real Android compositor caption journey on one owned disposable hosted emulator."""
import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import struct
import time
from android_caption_fixture import fixture
from android_caption_bridge_checks import verify_bridge

APP = "com.kinosail.player.dev"
TEST_CLASS = "com.kinosail.player.mobile.LivePlaybackOverlayTest"
TEST = TEST_CLASS + "#captionsStayAboveTheSystemNavigationArea"
RUNNER = APP + ".test/androidx.test.runner.AndroidJUnitRunner"
SERIAL = "emulator-5554"
IMAGE = "system-images;android-36;google_apis;x86_64"


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def command(args, *, timeout=60, input=None, check=True, env=None):
    return subprocess.run(args, input=input, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                          timeout=timeout, check=check, env=env).stdout


def check(condition, message):
    if not condition:
        raise RuntimeError(message)


def adb(*args, **options):
    return command([str(SDK / "platform-tools/adb"), "-s", SERIAL, *args], **options)


def await_boot(process):
    deadline = time.monotonic() + 180
    while time.monotonic() < deadline and process.poll() is None:
        if adb("shell", "getprop", "sys.boot_completed", check=False, timeout=10).strip() == b"1":
            return
        time.sleep(2)
    raise RuntimeError("emulator_boot_failed")


def capture_case(run, safe, token, label, landscape, tablet, negative=False):
    case = {"name": label, "negativeControl": negative, "expectedTests": 1 if negative else 4,
            "result": "failed", "stage": "configure"}
    try:
        adb("shell", "am", "force-stop", APP)
        adb("shell", "wm", "size", "1600x2560" if tablet else "1080x2400")
        adb("shell", "wm", "density", "320" if tablet else "420")
        adb("shell", "wm", "user-rotation", "lock", "1" if landscape else "0")
        expected_size = (2560, 1600) if tablet else ((2400, 1080) if landscape else (1080, 2400))
        case["expectedScreenshotSize"] = list(expected_size)
        case["displaySizes"] = {kind.decode().lower(): [int(width), int(height)] for kind,width,height in
            re.findall(rb"(Physical|Override) size: (\d+)x(\d+)", adb("shell", "wm", "size"))}
        case["displayDensities"] = {kind.decode().lower(): int(value) for kind,value in
            re.findall(rb"(Physical|Override) density: (\d+)", adb("shell", "wm", "density"))}
        case["stage"] = "seed"
        # The original tablet overlap used portrait video on a landscape display.
        case["videoOrientation"] = "landscape" if landscape and not tablet else "portrait"
        case["fixtureTitle"] = "Native " + case["videoOrientation"] + " contrast 01a0f2ab"
        case["fixtureSHA256"] = {suffix: sha(ROOT / "engineering/qa/2026-09-30-android-playback-overlay/fixtures" /
            (case["fixtureTitle"] + suffix)) for suffix in (".mp4", ".en.srt", ".nfo")}
        seed = {"version": 1, "server": "http://10.0.2.2:39231", "token": token,
                "title": case["fixtureTitle"],
                "capture": label, "landscape": landscape}
        adb("shell", "run-as", APP, "mkdir", "-p", "files")
        adb("shell", "run-as", APP, "tee", "files/video-overlay-seed.json",
            input=json.dumps(seed).encode())
        adb("shell", "run-as", APP, "chmod", "600", "files/video-overlay-seed.json")
        args = ["shell", "am", "instrument", "-w", "-r", "-e", "class", TEST if negative else TEST_CLASS,
                "-e", "captionOracleControl", str(negative).lower(), RUNNER]
        case["command"] = ["adb", "-s", SERIAL, *args]
        case["stage"] = "instrument"
        case["timedOut"] = False
        try:
            log = adb(*args, timeout=180, check=False)
        except subprocess.TimeoutExpired as error:
            log = error.output or b""
            case["timedOut"] = True
            try:
                adb("shell", "am", "force-stop", APP, timeout=10, check=False)
            except (OSError, subprocess.TimeoutExpired):
                case["timeoutStopFailed"] = True
        case["logTruncated"] = len(log) > 4 << 20
        log = log[:4 << 20]
        (run / (label + ".instrumentation.private.log")).write_bytes(log)
        # am instrument may return zero even when JUnit failed or skipped.
        case["selectedExpectedTests"] = set(re.findall(rb"(?m)^INSTRUMENTATION_STATUS: numtests=(\d+)\r?$", log)) == {str(case["expectedTests"]).encode()}
        case["junitPassed"] = ("OK (" + str(case["expectedTests"]) + (" test)" if negative else " tests)")).encode() in log and b"FAILURES!!!" not in log
        case["ignored"] = b"INSTRUMENTATION_STATUS_CODE: -3" in log or b"INSTRUMENTATION_STATUS_CODE: -4" in log
        case["failureAssertions"] = [message for message in (
            "Captions must visibly render in the paired screenshots",
            "Caption pixels must not overlap the system navigation area",
            "The system navigation area must be fully observed and stable",
            "Operator-owned media fixture accepted") if message.encode() in log]
        case["fixtureFailureStatuses"] = [int(status) for status in re.findall(
            rb"Operator-owned media fixture accepted expected:<204> but was:<([1-5][0-9]{2})>", log)[:8]]
        case["testSourceLines"] = sorted({int(line) for line in
            re.findall(rb"LivePlaybackOverlayTest\.kt:(\d{1,5})\)", log)})[:32]
        case["stage"] = "collect"
        listing = adb("shell", "run-as", APP, "ls", "files/video-overlay").decode().splitlines()
        names = [name for name in listing if name.startswith(label + "-") and
            re.fullmatch(r"[a-z0-9-]+(?:\.png|-caption-safe-area\.json)", name)]
        case["availableArtifactCount"] = len(names)
        check(0 < len(names) <= 19, "caption_artifact_count")
        destination = safe / label
        destination.mkdir()
        case["screenshotSizes"] = {}
        for name in names:
            data = adb("exec-out", "run-as", APP, "cat", "files/video-overlay/" + name)
            check(0 < len(data) <= 16 << 20, "caption_artifact_size")
            check(token.encode() not in data, "caption_artifact_secret")
            (destination / name).write_bytes(data)
            if name.endswith(".png"):
                check(data[:8] == b"\x89PNG\r\n\x1a\n" and data[12:16] == b"IHDR", "caption_png_invalid")
                case["screenshotSizes"][name] = list(struct.unpack(">II", data[16:24]))
        metric = destination / (label + "-caption-safe-area.json")
        check(metric.exists() and metric.stat().st_size <= 4096, "caption_metric_missing")
        measure = json.loads(metric.read_text())
        keys = {"bottomInset", "safeBottom", "width", "height", "captionPixels", "protectedCaptionPixels",
                "unstableProtectedPixels", "lowestCaptionPixel", "forcedCaptionsOffControl", "playbackObservations"}
        check(set(measure) == keys, "caption_metric_shape")
        case["stage"] = "verify"
        check(all(type(measure[k]) is int for k in keys - {"forcedCaptionsOffControl", "playbackObservations"}) and
              type(measure["forcedCaptionsOffControl"]) is bool, "caption_metric_types")
        observations = measure["playbackObservations"]
        check(type(observations) is list and len(observations) <= 4, "caption_observation_count")
        stages = set()
        values = {"positionMs", "state", "playing", "playWhenReady", "cueCount", "selectedTextTracks"}
        for row in observations:
            check(type(row) is dict and set(row) == values | {"stage", "available"}, "caption_observation_shape")
            check(type(row["stage"]) is str and row["stage"] in {"initial-playing", "paused", "enabled", "after-resume"}
                  and row["stage"] not in stages and type(row["available"]) is bool, "caption_observation_stage")
            stages.add(row["stage"])
            if not row["available"]:
                check(all(row[key] is None for key in values), "caption_observation_unavailable")
                continue
            check(type(row["state"]) is int and 1 <= row["state"] <= 4 and
                  all(type(row[key]) is bool for key in ("playing", "playWhenReady")), "caption_observation_state")
            for key, maximum in (("positionMs", 3_600_000), ("cueCount", 256), ("selectedTextTracks", 256)):
                check(row[key] is None or type(row[key]) is int and 0 <= row[key] <= maximum,
                      "caption_observation_bounds")
        case["measurement"] = measure
        check(all(size == list(expected_size) for size in case["screenshotSizes"].values()), "caption_screenshot_orientation_mismatch")
        check(0 < measure["width"] <= 4096 and 0 < measure["height"] <= 4096 and
              0 < measure["bottomInset"] < measure["height"] and
              measure["safeBottom"] == measure["height"] - measure["bottomInset"], "caption_metric_bounds")
        check((measure["width"], measure["height"]) == expected_size, "caption_metric_orientation_mismatch")
        check(not case["logTruncated"], "instrumentation_log_bound")
        check(case["selectedExpectedTests"] and not case["ignored"], "caption_test_not_executed")
        check(measure["forcedCaptionsOffControl"] == negative, "caption_control_binding")
        check(measure["bottomInset"] > 0 and measure["unstableProtectedPixels"] == 0,
              "caption_protected_area_not_observed")
        if negative:
            check(not case["junitPassed"] and b"Captions must visibly render in the paired screenshots" in log
                  and measure["captionPixels"] <= 100, "caption_negative_control_not_rejected")
        else:
            check(measure["captionPixels"] > 100, "caption_not_visible")
            check(measure["protectedCaptionPixels"] == 0, "caption_system_navigation_overlap")
            check(case["junitPassed"], "caption_other_assertion_failed")
        case["result"] = "passed"
    except Exception as error:
        case["failureClass"] = str(error) if type(error) is RuntimeError else type(error).__name__
    if case.get("timedOut"):
        case.update(result="failed", failureClass="instrumentation_timeout")
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
           "revision": command(["git", "rev-parse", "HEAD"]).decode().strip(), "result": "failed", "cases": [],
           "boundary": "Disposable API36 x86_64 Android emulator, actual Media3 decoder and Go Server, synthetic media/session, loopback HTTP fault bridge; no physical device, native TLS, Cast or deployment proof"}
emulator = None
emulator_log = None
try:
    check(not command(["git", "status", "--porcelain"]).strip(), "checkout_not_clean")
    receipt["sourceSHA256"] = {str(path.relative_to(ROOT)): sha(path) for path in (
        ROOT / "apps/player/scripts/test-android-captions.py", ROOT / "apps/player/scripts/android_caption_fixture.py",
        ROOT / "apps/player/scripts/android_caption_bridge_checks.py",
        ROOT / "apps/player/apps/android/app/src/androidTest/java/com/kinosail/player/mobile/LivePlaybackOverlayTest.kt",
        ROOT / "apps/player/apps/android/app/src/main/java/com/kinosail/player/core/PlaybackMediaSourceFactory.kt",
        ROOT / "apps/player/apps/android/app/src/main/java/com/kinosail/player/core/PlaybackModel.kt",
        ROOT / "apps/player/apps/android/app/src/main/java/androidx/media3/exoplayer/source/KinosailSubtitleLoading.kt",
        ROOT / "apps/player/apps/android/app/src/main/java/com/kinosail/player/core/PlaybackScreen.kt")}
    receipt["bridgeBoundary"] = {}
    verify_bridge(receipt["bridgeBoundary"])
    # All application work runs on the hosted runner, never on the shared Mac.
    command(["go", "-C", "apps/player", "build", "-trimpath", "-o", str(run / "player"), "./cmd/kinosail"], timeout=180)
    receipt["serverBinarySHA256"] = sha(run / "player")
    receipt["codecVersions"] = {name: command([name, "-version"]).decode().splitlines()[0] for name in ("ffmpeg", "ffprobe")}
    apks = [ROOT / "apps/player/apps/android/app/build/outputs/apk/debug/app-debug.apk",
            ROOT / "apps/player/apps/android/app/build/outputs/apk/androidTest/debug/app-debug-androidTest.apk"]
    receipt["apkSHA256"] = {str(path.relative_to(ROOT)): sha(path) for path in apks}
    avd = run / "avd"
    avd.mkdir()
    env = dict(os.environ, ANDROID_AVD_HOME=str(avd))
    devices = command([str(SDK / "platform-tools/adb"), "devices"]).decode().splitlines()
    check(not any(re.match(r"\S+\s+(device|offline|unauthorized)$", line) for line in devices), "runner_device_already_present")
    manager = SDK / "cmdline-tools/latest/bin/avdmanager"
    command([str(manager), "create", "avd", "-n", "kinosail-caption", "-k", IMAGE], input=b"no\n", env=env)
    configuration = avd / "kinosail-caption.avd/config.ini"
    settings = dict(line.split("=", 1) for line in configuration.read_text().splitlines() if "=" in line)
    # Set the hardware display before boot. wm overrides alone did not produce the requested captures.
    settings.update({"hw.lcd.width": "1600", "hw.lcd.height": "2560", "hw.lcd.density": "320"})
    configuration.write_text("".join(key + "=" + value + "\n" for key,value in sorted(settings.items())))
    receipt["hardwareDisplay"] = {"width": 1600, "height": 2560, "density": 320, "configurationSHA256": sha(configuration)}
    emulator_tool = SDK / "emulator/emulator"
    receipt["emulatorVersion"] = command([str(emulator_tool), "-version"]).decode().splitlines()[0]
    receipt["image"] = IMAGE
    receipt["imageMetadataSHA256"] = sha(SDK / "system-images/android-36/google_apis/x86_64/source.properties")
    emulator_log = (run / "emulator.private.log").open("wb")
    emulator = subprocess.Popen([str(emulator_tool), "-avd", "kinosail-caption", "-port", "5554",
        "-no-window", "-no-audio", "-no-boot-anim", "-no-snapshot", "-gpu", "swiftshader", "-accel", "on",
        "-cores", "2", "-memory", "3072", "-camera-back", "none", "-camera-front", "none"],
        env=env, stdout=emulator_log, stderr=emulator_log)
    await_boot(emulator)
    check(b"kinosail-caption" in adb("emu", "avd", "name"), "emulator_owner_mismatch")
    receipt["deviceFingerprint"] = adb("shell", "getprop", "ro.build.fingerprint").decode().strip()
    for name in ("window_animation_scale", "transition_animation_scale", "animator_duration_scale"):
        adb("shell", "settings", "put", "global", name, "0")
    adb("shell", "settings", "put", "system", "font_scale", "1.0")
    adb("shell", "cmd", "overlay", "enable-exclusive", "--category", "com.android.internal.systemui.navbar.gestural")
    check(b"[x] com.android.internal.systemui.navbar.gestural" in adb("shell", "cmd", "overlay", "list"), "gestural_navigation_not_active")
    adb("shell", "input", "keyevent", "82")
    for apk in apks:
        check(b"Success" in adb("install", "-r", str(apk), timeout=90), "apk_install_failed")
    with fixture(ROOT, run, receipt) as token:
        try:
            for label, landscape, tablet, negative in (
            ("phone-negative", False, False, True), ("phone-portrait", False, False, False),
            ("tablet-landscape-first", True, True, False), ("tablet-landscape-repeat", True, True, False),
            ("phone-landscape", True, False, False)):
                case = capture_case(run, safe, token, label, landscape, tablet, negative)
                receipt["cases"].append(case)
                print(json.dumps({k:case[k] for k in ("name", "result", "stage", "failureClass") if k in case}), flush=True)
        finally:
            adb("shell", "am", "force-stop", APP, check=False)
    receipt["finalRevision"] = command(["git", "rev-parse", "HEAD"]).decode().strip()
    receipt["sourceUnchanged"] = not command(["git", "status", "--porcelain"]).strip() and receipt["finalRevision"] == receipt["revision"]
    check(receipt["sourceUnchanged"], "checkout_changed")
    check(receipt.get("serverSettled") and receipt.get("bridgeSettled") and len(receipt["cases"]) == 5 and all(c["result"] == "passed" for c in receipt["cases"]),
          "android_caption_cases_failed")
    receipt["result"] = "passed"
except Exception as error:
    receipt["failureClass"] = str(error) if type(error) is RuntimeError else type(error).__name__
finally:
    if emulator is not None:
        if emulator.poll() is None:
            try:
                adb("emu", "kill", timeout=10, check=False)
            except (OSError, subprocess.TimeoutExpired):
                pass
        try:
            emulator.wait(timeout=20)
        except subprocess.TimeoutExpired:
            emulator.terminate()
            try:
                emulator.wait(timeout=5)
            except subprocess.TimeoutExpired:
                emulator.kill()
                emulator.wait(timeout=5)
    if emulator_log is not None:
        emulator_log.close()
    receipt["emulatorSettled"] = emulator is None or emulator.poll() is not None
    receipt["finishedAt"] = datetime.datetime.now(datetime.timezone.utc).isoformat()
    receipt["artifactsSHA256"] = {str(path.relative_to(safe)): sha(path) for path in sorted(safe.rglob("*")) if path.is_file()}
    (safe / "receipt.json").write_text(json.dumps(receipt, indent=2) + "\n")
print(json.dumps({"result": receipt["result"], "cases": [{k: case[k] for k in ("name", "result", "failureClass") if k in case} for case in receipt["cases"]]}))
raise SystemExit(0 if receipt["result"] == "passed" else 1)
