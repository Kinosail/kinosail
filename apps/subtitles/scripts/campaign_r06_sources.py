"""Source and overlay provenance for the bounded public R06 driver."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import stat
import subprocess

ROOT = Path(__file__).resolve().parents[3]
APP = ROOT / "apps/subtitles"
RECIPE = APP / "engineering/qa/2026-10-04-action-recovery/hosted"
SCOPE_FILE = "protocol-scope-3682759b.json"
SCOPE_SHA = "be16e9dd161aa1ac0a4a8364220e37b6bb34086474f07cf601d77d493b0f47e9"

def fingerprint(path):
    if not path.exists():
        return {"present": False}
    hasher, count = hashlib.sha256(), 0
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(65536), b""):
            count += len(chunk)
            hasher.update(chunk)
    return {"present": True, "bytes": count, "sha256": hasher.hexdigest()}

def compiled_binary(path):
    info = path.lstat()
    if not stat.S_ISREG(info.st_mode) or not 0 < info.st_size <= 128 * 1024 * 1024 or not info.st_mode & 0o100:
        raise ValueError("compiled-binary-shape")
    return {**fingerprint(path), "mode": info.st_mode & 0o777}

def save(path, value):
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")

def source_state(scope):
    return [{"path": item["path"], **fingerprint(ROOT / item["path"])} for item in scope["sourcePins"]]

def source_matches(scope, actual):
    return all(observed.get("present") and observed["bytes"] == expected["bytes"] and observed["sha256"] == expected["sha256"]
               for expected, observed in zip(scope["sourcePins"], actual, strict=True))

def r16_state(scope):
    return {name: fingerprint(ROOT / name) for name in scope["overlay"]["preservedLocalR16"]}

def checked_scope():
    data = (RECIPE / SCOPE_FILE).read_bytes()
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
    save(overlay, {"Replace": {str(ROOT / config["replace"]): str(ROOT / config["baselineFixture"]), str(ROOT / config["exclude"]): ""}})
    return overlay, before

def current_revision():
    return subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, timeout=5, stderr=subprocess.DEVNULL).decode().strip()

def tracked_clean(scope):
    command = ["git", "diff", "HEAD", "--quiet", "--", ".", ":(exclude)" + scope["overlay"]["replace"]]
    return subprocess.run(command, cwd=ROOT, timeout=10, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0

def tracked_tree(scope):
    names = subprocess.check_output(["git", "ls-files", "-z"], cwd=ROOT, timeout=10, stderr=subprocess.DEVNULL).split(b"\0")
    hasher, count = hashlib.sha256(), 0
    excluded = scope["overlay"]["replace"]
    for raw in sorted(name for name in names if name):
        name = os.fsdecode(raw)
        if name == excluded:
            continue
        path = ROOT / name
        if path.is_symlink():
            data = os.fsencode(os.readlink(path))
            state = {"kind": "symlink", "sha256": hashlib.sha256(data).hexdigest(), "bytes": len(data)}
        else:
            state = {"kind": "file", **fingerprint(path)}
        state["mode"] = path.lstat().st_mode & 0o777
        hasher.update(raw + b"\0" + json.dumps(state, sort_keys=True).encode() + b"\0")
        count += 1
    return {"sha256": hasher.hexdigest(), "trackedFileCount": count, "excludedPhysicalOverlay": [excluded]}

def tool_identity():
    found = shutil.which("go")
    if found is None:
        raise RuntimeError("go-tool-missing")
    return {"name": "go", **fingerprint(Path(found).resolve(strict=True))}
