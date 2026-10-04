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
import time

ROOT = Path(__file__).resolve().parents[3]
OUTPUT = ROOT / ".verification/campaign-proof/Q14"
PRIVATE = ROOT / ".verification/campaign-proof-private/Q14"
QA = ROOT / "engineering/qa/2026-10-04-q14-browse-return"
SPECS = ["browse-return.spec.ts", "browse-return-cold.spec.ts", "browse-return-bfcache.spec.ts"]
CLIP = "engineering/qa/2026-09-30-android-playback-overlay/fixtures/Native portrait contrast 01a0f2ab.mp4"
CLIP_SHA = "507ff669647ce0eda1f8965341a52fbb9a7bc9fe3a4b0fea4efa3933777a50c2"
PRIMARY = [f"visible Player Back preserves Movies query, offset, extent, focus and scroll at {width}px" for width in (390, 1440)]
COLLECTION = [(SPECS[0], title) for title in PRIMARY] + [
    (SPECS[0], "HTMX title-letter Back fetches current browse data and restores extent without a native pageshow"),
    *[(SPECS[0], f"visible Player Back restores original Shows action via {action}") for action in ("direct Play", "details and episode")],
    *[(SPECS[1], f"cold native Back restores later Movie cards at {width}px") for width in (390, 1440)],
    (SPECS[1], "live query uses current URL rather than the document's initial browse key"),
    (SPECS[2], "native BFCache preserves loaded Movie DOM without repeated continuation"),
]
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


def run(command, cwd, environment, seconds, label, command_seconds=None):
    global active
    started = time.monotonic()
    command_seconds = command_seconds or seconds - 5
    receipt = {"phase": label, "boundSeconds": seconds, "commandDeadlineSeconds": command_seconds,
               "command": command, "cwd": str(cwd.relative_to(ROOT))}
    raw = b""
    try:
        active = subprocess.Popen(command, cwd=cwd, env=environment, stdout=subprocess.PIPE,
                                  stderr=subprocess.STDOUT, start_new_session=True)
        try:
            raw = active.communicate(timeout=command_seconds)[0]
        except subprocess.TimeoutExpired:
            receipt["timeout"] = True
            group_signal(active, signal.SIGTERM)
            try:
                raw = active.communicate(timeout=3)[0]
            except subprocess.TimeoutExpired:
                group_signal(active, signal.SIGKILL)
                raw = active.communicate(timeout=2)[0]
        receipt["exitCode"] = active.returncode
    except (OSError, KeyboardInterrupt, subprocess.TimeoutExpired) as error:
        receipt["errorClass"] = type(error).__name__
    finally:
        if active:
            group_signal(active, signal.SIGKILL)  # Only this driver's owned group.
            try:
                active.wait(timeout=1)
                os.killpg(active.pid, 0)
                receipt["ownedGroupStopped"] = False
            except ProcessLookupError:
                receipt["ownedGroupStopped"] = True
            except subprocess.TimeoutExpired:
                receipt["ownedGroupStopped"] = False
            active = None
        receipt.update(durationSeconds=round(time.monotonic() - started, 3),
                       outputBytes=len(raw), outputSHA256=sha(raw))
    reports = [line[len(b"Q14_PROOF_RESULT "):] for line in raw.splitlines() if line.startswith(b"Q14_PROOF_RESULT ")]
    try:
        report = json.loads(reports[0]) if len(reports) == 1 and len(reports[0]) <= 2 * 1024 * 1024 else None
    except (ValueError, TypeError):
        report = None
    # Raw stdout/stderr, Go panic text and Playwright errors never reach artifacts.
    return receipt, report


def manifest(revision):
    paths = set(json.loads((QA / "direct-cli-preparation-context.json").read_text())["sources"])
    paths.update({str(Path(__file__).relative_to(ROOT)), CLIP,
                  "apps/player/e2e/browse-return-proof-reporter.ts", "go.work", "go.work.sum",
                  "apps/player/go.mod", "apps/player/go.sum", "packages/go.mod", "packages/go.sum",
                  ".github/workflows/layout-stability.yml",
                  "engineering/qa/2026-10-04-q14-browse-return/direct-cli-preparation-context.json"})
    sources = {name: {"bytes": (ROOT / name).stat().st_size, "sha256": sha((ROOT / name).read_bytes())} for name in sorted(paths)}
    canonical = "".join(f"{name}\t{entry['sha256']}\n" for name, entry in sources.items()).encode()
    return {"revision": revision, "recipe": "UTF-8 sorted path<TAB>lowercase SHA-256<LF>, trailing LF", "canonicalSHA256": sha(canonical), "sources": sources}


def safe_report(report, expected_count, collection=False):
    if not isinstance(report, dict) or report.get("schemaVersion") != 1:
        return False
    collected, cases = report.get("collected", []), report.get("cases", [])
    if len(collected) != expected_count or report.get("errors") or len({(item.get("file"), item.get("title")) for item in collected}) != expected_count:
        return False
    if any(item.get("file") not in SPECS or not isinstance(item.get("title"), str) or len(item["title"]) > 256 for item in collected):
        return False
    if collection:
        return report.get("status") == "passed" and not cases and sorted((item["file"], item["title"]) for item in collected) == sorted(COLLECTION)
    return (len(cases) == 2 and sorted((item.get("file"), item.get("title")) for item in cases) == sorted((SPECS[0], title) for title in PRIMARY)
            and all(item.get("retry") == 0 and item.get("status") in ("passed", "failed", "timedOut") and item.get("expectedStatus") == "passed" for item in cases))


def main():
    receipt = {"id": "Q14", "schemaVersion": 1, "phases": [], "result": "prerequisite-blocked",
               "scope": "all-nine collection; repeated actual Go public primary only; no media decoding/BFCache admission",
               "limits": {"collectionSeconds": 15, "collectionIncludingShutdownSeconds": 20,
                          "primaryGoSeconds": 70, "primaryExternalSeconds": 75, "workers": 1, "retries": 0}}
    results = {"collection": None, "primary": []}
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
        shell = next(item for item in browser_description["browsers"] if item["name"] == "chromium-headless-shell")
        cache = Path(os.environ.get("PLAYWRIGHT_BROWSERS_PATH", str(Path.home() / ".cache/ms-playwright")))
        binaries = list((cache / f"chromium_headless_shell-{shell['revision']}").glob("*/chrome-headless-shell"))
        if len(binaries) != 1 or not binaries[0].is_file():
            raise ValueError("installed headless Chromium prerequisite")
        receipt.update(revision=revision, fixture={"path": CLIP, "bytes": 53073, "sha256": CLIP_SHA},
                       executables={name: file_pin(Path(path)) for name, path in (("node", node), ("go", go), ("playwrightCLI", str(cli)), ("chromiumHeadlessShell", str(binaries[0])))},
                       dependencies={name: file_pin(path) for name, path in (("testPackage", package), ("playwrightPackage", playwright / "package.json"), ("corePackage", core / "package.json"), ("browserDescriptor", core / "browsers.json"))},
                       browser={"name": shell["name"], "revision": shell["revision"], "version": shell.get("browserVersion")})
        environment = dict(os.environ, CI="1", PLAYWRIGHT_CHANNEL="", KINOSAIL_BROWSER_PROJECT="chromium",
                           KINOSAIL_BROWSER_WORKERS="1", KINOSAIL_E2E_VIDEO="off", KINOSAIL_E2E_ARTIFACT_DIR="",
                           GOMAXPROCS="2", KINOSAIL_BROWSE_RETURN_PROOF="1")
        environment.pop("PLAYWRIGHT_JSON_OUTPUT_NAME", None)
        environment.pop("PLAYWRIGHT_JSON_OUTPUT_FILE", None)
        environment.update(KINOSAIL_BROWSE_RETURN_CASES="all", KINOSAIL_BROWSE_RETURN_URL="http://127.0.0.1:9",
                           KINOSAIL_E2E_OUTPUT_DIR=str(PRIVATE / "collection"))
        command = [node, "node_modules/@playwright/test/cli.js", "test", *SPECS, "--list", "--workers=1", "--retries=0", "--project=chromium", "--reporter=./browse-return-proof-reporter.ts"]
        phase, report = run(command, ROOT / "apps/player/e2e", environment, 20, "collection", 15)
        receipt["phases"].append(phase)
        results["collection"] = report
        if phase.get("exitCode") != 0 or phase.get("timeout") or not phase.get("ownedGroupStopped") or not safe_report(report, 9, True):
            raise ValueError("collection prerequisite")
        environment.update(KINOSAIL_BROWSE_RETURN_BROWSER="1", KINOSAIL_BROWSE_RETURN_CASES="primary", KINOSAIL_BROWSE_RETURN_MEDIA=str(clip))
        environment.pop("KINOSAIL_BROWSE_RETURN_URL", None)
        for repeat in (1, 2):
            environment["KINOSAIL_E2E_OUTPUT_DIR"] = str(PRIVATE / f"primary-{repeat}")
            command = [go, "test", "-v", "-p", "1", "./internal/server", "-run", "^TestBrowseReturnBrowserJourney$", "-count=1", "-timeout=70s"]
            phase, report = run(command, ROOT / "apps/player", environment, 75, f"primary-{repeat}")
            receipt["phases"].append(phase)
            results["primary"].append(report)
            if phase.get("timeout") or not phase.get("ownedGroupStopped") or not safe_report(report, 2):
                raise ValueError("primary completeness prerequisite")
            if any(failure.get("phase") == "prerequisite" for case in report["cases"] for failure in case.get("failures", [])):
                raise ValueError("journey prerequisite")
            passed = all(item["status"] == "passed" for item in report["cases"])
            if (passed and phase.get("exitCode") != 0) or (not passed and phase.get("exitCode") == 0):
                raise ValueError("runner/browser result mismatch")
        receipt["sourceUnchanged"] = git("rev-parse", "HEAD") == revision and clean_source() and manifest(revision) == inputs
        if not receipt["sourceUnchanged"]:
            raise ValueError("source changed during proof")
        passed = all(item["status"] == "passed" for report in results["primary"] for item in report["cases"])
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
