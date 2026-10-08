#!/usr/bin/env python3
"""One hosted, disposable TV lane. Never selects existing or physical devices."""
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import time
import tempfile
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "e2e"))
from native_tool import probe_native_tool
from simulator import prepare_creation, cleanup_creation, strict_json
from preflight import apple_runtime, android_image, TV_TYPE, TV_IMAGE


def main():
    if len(sys.argv) != 2 or sys.argv[1] not in ("tvos", "androidtv"):
        raise RuntimeError("Choose exactly tvos or androidtv")
    profile = sys.argv[1]
    platform = 'ios' if profile == 'tvos' else 'android'
    run_id = os.environ.get("GITHUB_RUN_ID", "") + "-" + os.environ.get("GITHUB_RUN_ATTEMPT", "")
    if os.environ.get("GITHUB_ACTIONS") != "true" or not re.fullmatch(r"\d{1,20}-\d{1,5}", run_id):
        raise RuntimeError("This lane requires an isolated GitHub Actions runner")
    if os.environ.get("RUNNER_OS") != ("macOS" if platform == "ios" else "Linux"):
        raise RuntimeError("Wrong hosted runner platform")
    if platform == 'ios' and os.environ.get('DEVELOPER_DIR') != '/Applications/Xcode_27.1_beta.app/Contents/Developer':
        raise RuntimeError('Wrong hosted Apple developer directory')
    project = Path(__file__).resolve().parent
    if Path.cwd().resolve() != project:
        raise RuntimeError("Run from scripts/e2e-tv")
    repo = project.parent.parent
    revision = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=repo, text=True).strip()
    if not re.fullmatch(r"[a-f0-9]{40}", revision):
        raise RuntimeError("Invalid source revision")
    if subprocess.run(["git", "diff", "--quiet", "HEAD", "--"], cwd=repo).returncode:
        raise RuntimeError("Source must match the committed revision")
    # Refuse an existing output root; never clean another run's evidence or claim its device.
    os.umask(0o077)
    root = project / ".e2e"
    root.mkdir(mode=0o700)
    identity = root.stat()
    sdk = root / "sdk"
    sdk.mkdir(mode=0o700)
    (root / "bin").mkdir(mode=0o700)
    (root / "secrets.json").write_text(json.dumps({"secrets": [], "stage": "started"}))
    os.chmod(root / "secrets.json", 0o600)
    env = {key: value for key, value in os.environ.items() if key in (
        "PATH", "TMPDIR", "TMP", "TEMP", "LANG", "LC_ALL", "TZ", "HOME", "DEVELOPER_DIR",
        "ANDROID_HOME", "ANDROID_SDK_ROOT", "JAVA_HOME", "GITHUB_ACTIONS", "RUNNER_OS")}
    env.update(E2E_TELEMETRY_DISABLED="1", AGENT_DEVICE_STATE_DIR=str(root / "agent-device"),
               ANDROID_AVD_HOME=str(root / "avd"), GOMAXPROCS="4")
    device = None
    emulator = None
    active = None
    result = 1
    cleanup = []
    commands = []
    witness = {}
    started = time.time()
    ownership = {"run": run_id, "rootDev": identity.st_dev, "rootIno": identity.st_ino, "platform": platform, "profile": profile, "target": "tv", "device": None, "emulatorPID": None, "emulatorStart": None, "complete": False}

    def save_ownership():
        path = root / "owned-device.json"
        path.write_text(json.dumps(ownership))
        os.chmod(path, 0o600)

    save_ownership()
    canceled = False

    def stop(sig, _frame):
        nonlocal canceled
        canceled = True
        if active and active.poll() is None:
            os.killpg(active.pid, signal.SIGTERM)

    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)

    def command(args, cwd=project, timeout=120, allow_failure=False, input_data=None):
        nonlocal active
        if canceled:
            raise RuntimeError("Hosted run canceled")
        commands.append(args)
        with (sdk / "host.log").open("ab") as log:
            active = subprocess.Popen(args, cwd=cwd, env=env, stdin=subprocess.PIPE if input_data else subprocess.DEVNULL,
                                      stdout=log, stderr=log, start_new_session=True)
            try:
                active.communicate(input=input_data, timeout=timeout)
            except subprocess.TimeoutExpired:
                os.killpg(active.pid, signal.SIGTERM)
                try:
                    active.wait(timeout=15)
                except subprocess.TimeoutExpired:
                    os.killpg(active.pid, signal.SIGKILL)
                    active.wait(timeout=10)
                raise RuntimeError("Hosted command timed out") from None
            status = active.returncode
        active = None
        if status and not allow_failure:
            raise RuntimeError("Hosted command failed; private log withheld")
        return status

    def inspect(args):
        # Device IDs and tool versions only. No Owner credentials, pairing values or app logs.
        with tempfile.TemporaryFile() as output:
            subprocess.run(args, cwd=project, env=env, stdout=output, stderr=subprocess.DEVNULL, timeout=30, check=True)
            output.seek(0)
            raw = output.read(1024 * 1024 + 1)
        if len(raw) > 1024 * 1024: raise RuntimeError("Oversized TV inspection")
        return raw.decode("utf-8", errors="strict").strip()

    def native_tool(args):
        return probe_native_tool(platform, args, project, env, witness)

    try:
        # All TV metadata is admitted before Go/native builds or device creation.
        if platform == "ios":
            runtime = apple_runtime(strict_json(inspect(["xcrun", "simctl", "list", "runtimes", "--json"])), strict_json(inspect(["xcrun", "simctl", "list", "devicetypes", "--json"])))
        else:
            sdk_root = Path(env.get("ANDROID_HOME", env.get("ANDROID_SDK_ROOT", "")))
            if not sdk_root.is_absolute(): raise RuntimeError("Hosted Android SDK root unavailable")
            metadata = sdk_root / "system-images/android-36/android-tv/x86_64/package.xml"
            if metadata.is_symlink() or metadata.stat().st_size > 65536: raise RuntimeError("Unsafe TV image metadata")
            metadata_raw = metadata.read_bytes()
            android_image(metadata_raw)
            witness["imageMetadataSHA256"] = hashlib.sha256(metadata_raw).hexdigest()
        source_paths = subprocess.check_output(["git", "ls-files", "-z", "--", "apps/player", "packages", "scripts/e2e", "scripts/e2e-mobile", "scripts/e2e-tv"], cwd=repo).split(b"\0")
        witness["sourceManifestSHA256"] = hashlib.sha256(b"\n".join(
            path + b" " + hashlib.sha256((repo / os.fsdecode(path)).read_bytes()).hexdigest().encode()
            for path in source_paths if path and (repo / os.fsdecode(path)).is_file())).hexdigest()
        command(["go", "build", "-o", str(root / "bin/player"), "./apps/player/cmd/kinosail"], repo, 900)
        if platform == "ios":
            command(["./scripts/build-apple.sh", "tvos"], repo / "apps/player/apps/native", 1200)
            app_path = repo / "apps/player/apps/native/.build/tvos-simulator/Build/Products/Debug-appletvsimulator/KinosailPlayer.app"
            app_binary = app_path / "KinosailPlayer"
            prepare_creation(ownership, runtime)
            save_ownership()
            created_device = inspect(["xcrun", "simctl", "create", "Kinosail-TV-E2E-" + run_id,
                              TV_TYPE, runtime])
            if not re.fullmatch(r"[A-Fa-f0-9]{8}(?:-[A-Fa-f0-9]{4}){3}-[A-Fa-f0-9]{12}", created_device):
                raise RuntimeError("Created simulator identity invalid")
            device = created_device
            ownership["device"] = device
            save_ownership()
            command(["xcrun", "simctl", "boot", device])
            command(["xcrun", "simctl", "bootstatus", device, "-b"], timeout=180)
            witness["runtime"] = runtime
            witness["nativeTool"] = native_tool(["xcodebuild", "-version"])
        else:
            sdk_root = Path(env.get("ANDROID_HOME", env.get("ANDROID_SDK_ROOT", "")))
            if not sdk_root.is_absolute():
                raise RuntimeError("Hosted Android SDK root unavailable")
            command(["./gradlew", ":app:assembleDebug"], repo / "apps/player/apps/android", 1200)
            app_path = repo / "apps/player/apps/android/app/build/outputs/apk/debug/app-debug.apk"
            app_binary = app_path
            device = "emulator-5554"
            if device in inspect(["adb", "devices"]).split():
                raise RuntimeError("Owned emulator serial collision")
            avd = "Kinosail-TV-E2E-" + run_id
            (root / "avd").mkdir(mode=0o700)
            command(["avdmanager", "create", "avd", "--name", avd, "--package", TV_IMAGE, "--device", "tv_1080p"], input_data=b"no\n")
            log = (sdk / "emulator.log").open("ab")
            emulator = subprocess.Popen([str(sdk_root / "emulator/emulator"), "-avd", avd, "-port", "5554", "-no-window", "-no-audio", "-no-snapshot", "-gpu", "swiftshader_indirect"], env=env, stdout=log, stderr=log, start_new_session=True)
            log.close()
            ownership.update(device=device, emulatorPID=emulator.pid, emulatorStart=Path(f"/proc/{emulator.pid}/stat").read_text().split(") ", 1)[1].split()[19])
            save_ownership()
            deadline = time.monotonic() + 240
            while time.monotonic() < deadline:
                if emulator.poll() is not None or canceled:
                    raise RuntimeError("Owned emulator exited before readiness")
                if subprocess.run(["adb", "-s", device, "shell", "getprop", "sys.boot_completed"], env=env, capture_output=True, timeout=10).stdout.strip() == b"1":
                    break
                time.sleep(2)
            else:
                raise RuntimeError("Owned emulator boot timed out")
            witness["runtime"] = TV_IMAGE
            witness["nativeTool"] = native_tool(["java", "-version"])
        witness.update(serverSHA256=hashlib.sha256((root / "bin/player").read_bytes()).hexdigest(),
                       appSHA256=hashlib.sha256(app_binary.read_bytes()).hexdigest())
        (root / "control.json").write_text(json.dumps({"identity": {"platform": platform, "profile": profile, "target": "tv", "device": device, "port": "18769", "revision": revision, "run": run_id}, "appPath": str(app_path)}))
        os.chmod(root / "control.json", 0o600)
        # app.command.log and every SDK output file live physically in this project's .e2e.
        result = command(["node", "node_modules/e2e/dist/cli/bin.js", "run"], timeout=300, allow_failure=True)
    except Exception as error:
        # Error text can originate from external commands. Keep stdout generic and private logs local.
        print("Native lane failed; only sanitized evidence may be published.", file=sys.stderr)
        result = 1
    finally:
        def clean_command(args):
            try:
                return command(args, allow_failure=True)
            except Exception:
                return 1

        # Public daemon lifecycle command targets this run's private state directory only.
        canceled = False
        if (root / "agent-device").exists():
            status = clean_command(["node", "node_modules/agent-device/bin/agent-device.mjs", "daemon", "stop", "--state-dir", str(root / "agent-device")])
            cleanup.append({"resource": "owned agent-device daemon", "result": status})
            if status:
                result = result or 1
        creation_failed = False
        if platform == "ios" and ownership.get("creationPending") is not None:
            try:
                cleanup.extend(cleanup_creation(ownership, save_ownership, clean_command))
                device = ownership["device"]
            except Exception:
                creation_failed = True
                result = result or 1
        elif platform == "ios" and device:
            for action in ("shutdown", "delete"):
                status = clean_command(["xcrun", "simctl", action, device])
                cleanup.append({"resource": "owned simulator " + device, "action": action, "result": status})
                if status and action == "delete":
                    result = result or 1
        if emulator:
            if emulator.poll() is None:
                os.killpg(emulator.pid, signal.SIGTERM)
                try:
                    emulator.wait(timeout=15)
                except subprocess.TimeoutExpired:
                    os.killpg(emulator.pid, signal.SIGKILL)
                    emulator.wait(timeout=10)
            cleanup.append({"resource": "owned emulator process", "result": emulator.returncode})
        ownership["complete"] = not creation_failed and ownership.get("creationPending") is None and not any(row.get("result") for row in cleanup if row.get("action") != "shutdown" and row.get("resource") != "owned emulator process")
        save_ownership()
        if root.stat().st_ino != identity.st_ino or root.stat().st_dev != identity.st_dev:
            raise RuntimeError("Output ownership lost; publication withheld")
        receipt = {"revision": revision, "command": "python3 hosted.py " + profile, "commands": commands,
                   "platform": platform, "profile": profile, "target": "tv", "device": device, "environment": "disposable hosted simulator/emulator",
                   "data": "synthetic Example Movie (testsrc2/AAC); synthetic MFA Owner; actual dynamic pairing code",
                   "witness": witness, "result": result, "cleanup": cleanup, "elapsedSeconds": round(time.time() - started, 2),
                   "packages": {"e2e": "0.17.0", "agent-device": "0.21.22"},
                   "boundaries": ["no physical device", "no phone/watchOS/Wear pairing", "no casting/provider calls", "no deployed server"]}
        if (root / "journey.json").exists():
            receipt["journey"] = json.loads((root / "journey.json").read_text())
        (root / "receipt.private.json").write_text(json.dumps(receipt))
        os.chmod(root / "receipt.private.json", 0o600)
    return result


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception:
        print("Native run rejected or cleanup incomplete; inspect private hosted logs.", file=sys.stderr)
        sys.exit(1)
