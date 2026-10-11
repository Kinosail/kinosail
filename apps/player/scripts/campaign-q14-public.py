#!/usr/bin/env python3
"""Bounded opt-in hosted Q14 diagnosis; upload only four safe JSON receipts."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import threading
import time

from campaign_q14_admission import SPECS, admit, complete, go_boundary, cache_diagnostic
from campaign_q14_suites import SUITES

ROOT = Path(__file__).resolve().parents[3]
OUTPUT = ROOT / ".verification/campaign-proof/Q14"
PRIVATE = ROOT / ".verification/campaign-proof-private/Q14"
QA = ROOT / "engineering/qa/2026-10-04-q14-browse-return"
CLIP = "engineering/qa/2026-09-30-android-playback-overlay/fixtures/Native portrait contrast 01a0f2ab.mp4"
CLIP_SHA = "507ff669647ce0eda1f8965341a52fbb9a7bc9fe3a4b0fea4efa3933777a50c2"
OUTPUT_CAP = 16 * 1024 * 1024
active = None


def sha(data):
    return hashlib.sha256(data).hexdigest()


def file_pin(path):
    hasher = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            hasher.update(block)
    return {"bytes": path.stat().st_size, "sha256": hasher.hexdigest()}


def write(name, value):
    (OUTPUT / name).write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def git(*arguments):
    return subprocess.run(["git", *arguments], cwd=ROOT, capture_output=True, check=True, timeout=5).stdout.decode().strip()


def clean_source():
    allowed = {"?? apps/player/e2e/node_modules", "?? scripts/quality/node_modules"}
    return all(line in allowed for line in git("status", "--porcelain", "--untracked-files=normal").splitlines())


def group_signal(process, signum):
    try:
        os.killpg(process.pid, signum)
    except ProcessLookupError:
        pass


def stop(_signum, _frame):
    if active:
        group_signal(active, signal.SIGTERM)
    raise KeyboardInterrupt


def settle_group(process, deadline):
    group_signal(process, signal.SIGTERM)
    kill_at = min(time.monotonic() + 3, deadline - 1)
    while time.monotonic() < deadline:
        try:
            os.killpg(process.pid, 0)
        except ProcessLookupError:
            return True
        if time.monotonic() >= kill_at:
            group_signal(process, signal.SIGKILL)
        process.poll()
        time.sleep(0.05)
    return False


def run(command, cwd, environment, seconds, label, command_seconds=None, suite="primary"):
    global active
    started = time.monotonic()
    command_seconds = command_seconds or seconds - 5
    receipt = {"phase": label, "boundSeconds": seconds, "commandDeadlineSeconds": command_seconds,
               "command": command, "cwd": str(cwd.relative_to(ROOT))}
    captured = {"data": bytearray(), "bytes": 0, "overflow": False, "hasher": hashlib.sha256()}
    reader = None
    def drain(process):
        while piece := process.stdout.read(65536):
            captured["bytes"] += len(piece)
            captured["hasher"].update(piece)
            retained = min(len(piece), max(0, OUTPUT_CAP - len(captured["data"])))
            captured["data"].extend(piece[:retained])
            if captured["bytes"] > OUTPUT_CAP:
                captured["overflow"] = True
                group_signal(process, signal.SIGTERM)
    try:
        active = subprocess.Popen(command, cwd=cwd, env=environment, stdout=subprocess.PIPE,
                                  stderr=subprocess.STDOUT, start_new_session=True)
        reader = threading.Thread(target=drain, args=(active,), daemon=True)
        reader.start()
        try:
            active.wait(timeout=command_seconds)
        except subprocess.TimeoutExpired:
            receipt["timeout"] = True
            group_signal(active, signal.SIGTERM)
    except (OSError, KeyboardInterrupt, subprocess.TimeoutExpired) as error:
        receipt["errorClass"] = type(error).__name__
    finally:
        if active:
            receipt["ownedGroupStopped"] = settle_group(active, started + seconds)
            receipt["exitCode"] = active.poll()
            active = None
        if reader:
            reader.join(timeout=max(0, started + seconds - time.monotonic()))
            receipt["captureSettled"] = not reader.is_alive()
        receipt.update(durationSeconds=round(time.monotonic() - started, 3),
                       outputBytes=captured["bytes"], outputSHA256=captured["hasher"].hexdigest(),
                       outputOverflow=captured["overflow"])
    raw = bytes(captured["data"])
    receipt["cacheDiagnostic"] = cache_diagnostic(raw) if suite == "bfcache" and label not in ("collection", "compile") else None
    receipt["goBoundary"] = go_boundary(raw) if label not in ("collection", "compile") else None
    reports = [line[len(b"Q14_PROOF_RESULT "):] for line in raw.splitlines() if line.startswith(b"Q14_PROOF_RESULT ")]
    try:
        report = admit(json.loads(reports[0]), label == "collection", suite) if len(reports) == 1 and len(reports[0]) <= 2 * 1024 * 1024 and not captured["overflow"] else None
    except (ValueError, TypeError):
        report = None
    receipt["reportAdmitted"] = report is not None
    # Raw stdout/stderr, Go panic text and Playwright errors never reach artifacts.
    return receipt, report


def compile_test_binary(go, environment):
    binary = PRIVATE / "browse-return.test"
    command = [go, "test", "-c", "-p", "1", "-o", str(binary), "./internal/server"]
    phase, _report = run(command, ROOT / "apps/player", environment, 95, "compile", 90)
    admitted = (phase.get("exitCode") == 0 and not phase.get("timeout") and not phase.get("outputOverflow")
                and phase.get("ownedGroupStopped") and phase.get("captureSettled")
                and not binary.is_symlink() and binary.is_file() and 0 < binary.stat().st_size <= 256 * 1024 * 1024)
    return binary, phase, file_pin(binary) if admitted else None


def manifest(revision):
    paths = set(json.loads((QA / "direct-cli-preparation-context.json").read_text())["sources"])
    paths.update({str(Path(__file__).relative_to(ROOT)), CLIP,
                  "apps/player/e2e/browse-return-proof-reporter.ts", "go.work", "go.work.sum",
                  "apps/player/scripts/campaign_q14_admission.py", "apps/player/scripts/test_campaign_q14_public.py",
                  "apps/player/scripts/test_campaign_q14_cache_diagnostic.py",
                  "apps/player/scripts/campaign_q14_suites.py", "apps/player/scripts/test_campaign_q14_suites.py",
                  "apps/player/internal/server/browse_return_browser_test.go",
                  "apps/player/go.mod", "apps/player/go.sum", "packages/go.mod", "packages/go.sum",
                  ".github/workflows/layout-stability.yml",
                  "packages/webassets/static/pwa-browse-return.js",
                  "packages/webassets/static/pwa-library.js", "packages/webassets/static/pwa-navigation.js",
                  "packages/webassets/webassets.go", "apps/player/internal/server/locale.go",
                  "apps/subtitles/internal/server/locale.go", "apps/player/internal/server/player.go",
                  "apps/player/internal/server/browse_return_contract_test.go",
                  "apps/player/e2e/browse-return-safety.spec.ts",
                  "apps/player/e2e/browse-return-home.spec.ts",
                  "engineering/qa/2026-10-04-q14-browse-return/direct-cli-preparation-context.json"})
    catalogs = list((ROOT / "apps/player/internal/server/locales").glob("active.*.json"))
    if len(catalogs) != 108 or any(path.is_symlink() or not path.is_file() for path in catalogs):
        raise ValueError("active catalog source prerequisite")
    paths.update(str(path.relative_to(ROOT)) for path in catalogs)
    sources = {name: {"bytes": (ROOT / name).stat().st_size, "sha256": sha((ROOT / name).read_bytes())} for name in sorted(paths)}
    canonical = "".join(f"{name}\t{entry['sha256']}\n" for name, entry in sources.items()).encode()
    return {"revision": revision, "trackedTree": git("rev-parse", "HEAD^{tree}"),
            "boundary": "Selected input hashes plus Git tree identity; no dependency-cache byte claim.",
            "recipe": "UTF-8 sorted path<TAB>lowercase SHA-256<LF>, trailing LF", "canonicalSHA256": sha(canonical), "sources": sources}


def main():
    os.umask(0o077)
    suite = os.environ.get("CAMPAIGN_Q14_SUITE", "primary")
    if suite not in SUITES or suite == "all":
        print("Q14 prerequisite: fixed suite required", file=sys.stderr)
        return 2
    receipt = {"id": "Q14", "schemaVersion": 1, "phases": [], "result": "prerequisite-blocked",
               "suite": suite, "scope": "Fixed selected actual Go public journeys; no media decoding or independent visual acceptance",
               "limits": {"collectionSeconds": 15, "collectionIncludingShutdownSeconds": 20,
                          "compileSeconds": 90, "compileIncludingShutdownSeconds": 95,
                          "primaryGoSeconds": 70, "primaryExternalSeconds": 75, "workers": 1, "retries": 0}}
    runtime_key = "primary" if suite == "primary" else "journeys"
    results = {"collection": None, runtime_key: []}
    OUTPUT.mkdir(parents=True, exist_ok=True)
    if any(OUTPUT.iterdir()) or PRIVATE.exists():
        print("Q14 prerequisite: fresh proof paths required; existing evidence preserved", file=sys.stderr)
        return 2
    PRIVATE.mkdir(parents=True)
    result = 2
    try:
        if (ROOT / ".gates-disabled").exists() or not clean_source():
            raise ValueError("gates/source prerequisite")
        revision = git("rev-parse", "HEAD")
        if os.environ.get("GITHUB_SHA") != revision:
            raise ValueError("hosted checkout revision prerequisite")
        inputs = manifest(revision)
        write("source-manifest.json", inputs)
        node, go = shutil.which("node"), shutil.which("go")
        cli = ROOT / "apps/player/e2e/node_modules/@playwright/test/cli.js"
        package = cli.parent / "package.json"
        if not node or not go or not cli.is_file() or json.loads(package.read_text()).get("version") != "1.63.0":
            raise ValueError("installed dependency prerequisite")
        clip = ROOT / CLIP
        if clip.stat().st_size != 53073 or sha(clip.read_bytes()) != CLIP_SHA:
            raise ValueError("fictional fixture prerequisite")
        playwright = cli.resolve().parents[2] / "playwright"
        core = playwright.resolve().parent / "playwright-core"
        if any(json.loads((path / "package.json").read_text()).get("version") != "1.63.0" for path in (playwright, core)):
            raise ValueError("pinned Playwright prerequisite")
        browser_description = json.loads((core / "browsers.json").read_text())
        browser_name = "chromium" if suite == "bfcache" else "chromium-headless-shell"
        shell = next(item for item in browser_description["browsers"] if item["name"] == browser_name)
        cache = Path(os.environ.get("PLAYWRIGHT_BROWSERS_PATH", str(Path.home() / ".cache/ms-playwright")))
        folder = "chromium" if suite == "bfcache" else "chromium_headless_shell"
        executable = "chrome" if suite == "bfcache" else "chrome-headless-shell"
        binaries = list((cache / f"{folder}-{shell['revision']}").glob(f"*/{executable}"))
        if len(binaries) != 1 or not binaries[0].is_file():
            raise ValueError("installed headless Chromium prerequisite")
        receipt.update(revision=revision, fixture={"path": CLIP, "bytes": 53073, "sha256": CLIP_SHA},
                       executables={name: file_pin(Path(path)) for name, path in (("node", node), ("go", go), ("playwrightCLI", str(cli)), ("selectedChromium", str(binaries[0])))},
                       dependencies={name: file_pin(path) for name, path in (("testPackage", package), ("playwrightPackage", playwright / "package.json"), ("corePackage", core / "package.json"), ("browserDescriptor", core / "browsers.json"))},
                       browser={"name": shell["name"], "revision": shell["revision"], "version": shell.get("browserVersion")})
        environment = dict(os.environ, CI="1", PLAYWRIGHT_CHANNEL="", KINOSAIL_BROWSER_PROJECT="chromium",
                           KINOSAIL_BROWSER_WORKERS="1", KINOSAIL_E2E_VIDEO="off", KINOSAIL_E2E_ARTIFACT_DIR="",
                           GOMAXPROCS="2", GOPROXY="off", GOTOOLCHAIN="local", KINOSAIL_BROWSE_RETURN_PROOF="1")
        environment.pop("PLAYWRIGHT_JSON_OUTPUT_NAME", None)
        environment.pop("PLAYWRIGHT_JSON_OUTPUT_FILE", None)
        environment.update(KINOSAIL_BROWSE_RETURN_CASES="all" if suite == "primary" else suite, KINOSAIL_BROWSE_RETURN_URL="http://127.0.0.1:9",
                           KINOSAIL_E2E_OUTPUT_DIR=str(PRIVATE / "collection"))
        specs = SPECS if suite == "primary" else sorted({file for file, _title in SUITES[suite]})
        command = [node, "node_modules/@playwright/test/cli.js", "test", *specs, "--list", "--workers=1", "--retries=0", "--project=chromium", "--reporter=./browse-return-proof-reporter.ts"]
        phase, report = run(command, ROOT / "apps/player/e2e", environment, 20, "collection", 15, suite)
        receipt["phases"].append(phase)
        results["collection"] = report
        if phase.get("exitCode") != 0 or phase.get("timeout") or not phase.get("ownedGroupStopped") or not phase.get("captureSettled") or not complete(report, True, suite):
            raise ValueError("collection prerequisite")
        binary, phase, binary_pin = compile_test_binary(go, environment)
        receipt["phases"].append(phase)
        if binary_pin is None:
            raise ValueError("compile prerequisite")
        receipt["compiledTestBinary"] = {"file": binary.name, **binary_pin}
        environment.update(KINOSAIL_BROWSE_RETURN_BROWSER="1", KINOSAIL_BROWSE_RETURN_CASES=suite, KINOSAIL_BROWSE_RETURN_MEDIA=str(clip))
        environment.pop("KINOSAIL_BROWSE_RETURN_URL", None)
        for repeat in ((1, 2) if suite == "primary" else (1,)):
            label = f"{suite}-{repeat}"
            environment["KINOSAIL_E2E_OUTPUT_DIR"] = str(PRIVATE / label)
            command = [str(binary), "-test.v", "-test.run=^TestBrowseReturnBrowserJourney$", "-test.count=1", "-test.timeout=70s", "-test.parallel=1"]
            phase, report = run(command, ROOT / "apps/player/internal/server", environment, 75, label, suite=suite)
            receipt["phases"].append(phase)
            results[runtime_key].append(report)
            if phase.get("timeout") or not phase.get("ownedGroupStopped") or not phase.get("captureSettled") or not complete(report, suite=suite):
                raise ValueError("primary completeness prerequisite")
            if any(failure.get("phase") == "prerequisite" for case in report["cases"] for failure in case.get("failures", [])):
                raise ValueError("journey prerequisite")
            passed = all(item["status"] == "passed" for item in report["cases"])
            expected_boundary = "completed-pass" if passed else "completed-fail"
            if phase.get("goBoundary") != expected_boundary or (passed and phase.get("exitCode") != 0) or (not passed and phase.get("exitCode") == 0):
                raise ValueError("runner/browser result mismatch")
        receipt["sourceUnchanged"] = git("rev-parse", "HEAD") == revision and clean_source() and manifest(revision) == inputs and file_pin(binary) == binary_pin
        if not receipt["sourceUnchanged"]:
            raise ValueError("source changed during proof")
        passed = all(item["status"] == "passed" for report in results[runtime_key] for item in report["cases"])
        receipt["result"] = "passed" if passed else "completed-failures-require-boundary-review"
        result = 0 if passed else 1
    except (OSError, ValueError, KeyError, subprocess.SubprocessError, KeyboardInterrupt) as error:
        receipt["errorClass"] = type(error).__name__
    finally:
        write("receipt.json", receipt)
        write("results.json", results)
        if not (OUTPUT / "source-manifest.json").exists():
            write("source-manifest.json", {"status": "prerequisite-blocked"})
        write("artifact-manifest.json", {name: {"bytes": (OUTPUT / name).stat().st_size, "sha256": sha((OUTPUT / name).read_bytes())} for name in ("receipt.json", "results.json", "source-manifest.json")})
    print(f"Q14 {receipt['result']}; safe receipts: .verification/campaign-proof/Q14")
    return result


if __name__ == "__main__":
    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)
    sys.exit(main())
