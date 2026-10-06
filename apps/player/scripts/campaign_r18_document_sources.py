"""Fixed R18 provenance and guarded import; no overlay or product writes."""
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import stat
import sys
import time
import types

from campaign_r18_document_inputs import Budget, iter_tool_paths, read_small, stream_regular

ROOT = Path(__file__).resolve().parents[3]
APP = ROOT / "apps/player"
BASE = "bd1ea2e148787ae8d4a0a46640bc2a965e10fe6a"
BASE_TREE = "6070e86cc8a3522810218d311cc2d5d6246f02d0"
GO_FILES = ("apps/player/internal/server/home_assistant_document_targets_test.go",
            "apps/player/internal/server/home_assistant_document_targets_helpers_test.go")
PINS = {
    GO_FILES[0]: "69f15a59a3291dbedcef0df575f851e4c95c4d50",
    GO_FILES[1]: "01f262d1b308736172f33d751e7c17a3e3efda6b",
}
BASELINE = {
    "packages/homeassistant/http.go": "245a18cc5ddd2ad0ad921dd0dd6ca03f1407633e",
    "packages/homeassistant/player.go": "ceb2130f568d0c5300f4dd2c2c5dc8868efbf076",
    "packages/homeassistant/integration.go": "11598c3bd1360b71d19a7d662353fdffbe500f20",
}
REUSE = (
    ("campaign_r06_sources", "apps/subtitles/scripts/campaign_r06_sources.py",
     "7bb611975c0e42f65780f73d982ca5a35c70f019", "6c20ed559d58d13b8bbe9a0cd8e1306b60b05dcd79897cd94b8a92af617172d8"),
    ("campaign_r06_browser_process", "apps/subtitles/scripts/campaign_r06_browser_process.py",
     "c88075fb514f93209796e79d6dd0bb37b079e7e7", "45aa8fe859bde5c0c978589d220e938d6e341e05da5fd6e004c583ada5b4462e"),
)
HANDOFF_FIELDS = {"schemaVersion", "baseRevision", "baseTree", "revision", "tree", "mode",
                  "sourceSHA256", "toolchainSHA256", "binary", "formattingVerified",
                  "modulesVerified", "compileSettled", "inputCount", "compileInvocation", "runInvocation"}


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(",", ":")).encode()).hexdigest()


def fingerprint(path, limit=128 * 1024 * 1024, deadline=None, budget=None):
    sha, oid = hashlib.sha256(), hashlib.sha1()
    def begin(info):
        if budget is not None:
            budget.reserve(info.st_size)
        oid.update(("blob " + str(info.st_size) + "\0").encode())
    def consume(chunk):
        sha.update(chunk)
        oid.update(chunk)
    info = stream_regular(path, limit, deadline, begin, consume)
    return {"sha256": sha.hexdigest(), "gitBlob": oid.hexdigest(), "bytes": info.st_size,
            "mode": stat.S_IMODE(info.st_mode)}


def import_execute(deadline):
    # Capture and validate both byte strings before executing either import.
    verified = []
    for name, relative, oid, sha in REUSE:
        data = read_small(ROOT / relative, 16384, deadline)
        actual_oid = hashlib.sha1(("blob " + str(len(data)) + "\0").encode() + data).hexdigest()
        if actual_oid != oid or hashlib.sha256(data).hexdigest() != sha:
            raise ValueError("process-helper-pin")
        verified.append((name, ROOT / relative, data))
    sys.dont_write_bytecode = True
    for name, path, data in verified:
        if name in sys.modules:
            raise ValueError("unexpected-process-module")
        module = types.ModuleType(name)
        module.__file__ = str(path)
        sys.modules[name] = module
        exec(compile(data, str(path), "exec"), module.__dict__)
    return sys.modules["campaign_r06_browser_process"].execute


class Capture:
    def __init__(self, kind):
        self.kind, self.lines, self.prerequisite = kind, [], False
        self.hash, self.count = hashlib.sha256(), 0

    def consume(self, raw):
        self.hash.update(raw + b"\n")
        self.count += 1
        if self.kind == "silent":
            self.prerequisite = True
            return
        try:
            line = raw.decode("utf-8")
            if len(raw) > 65536 or self.count > 20000:
                raise ValueError("capture-bound")
            if self.kind == "hash":
                valid = re.fullmatch(r"[a-f0-9]{40}", line)
            elif self.kind == "files":
                valid = re.fullmatch(r"(100644|100755) [a-f0-9]{40} 0\t[\x20-\x7e]{1,1024}", line)
            elif self.kind == "modules":
                valid = line == "all modules verified" and self.count == 1
            else:
                valid = False
            if not valid:
                raise ValueError("unexpected-preflight-output")
            self.lines.append(line)
        except (ValueError, UnicodeError):
            self.prerequisite = True


def snapshot(lines, mode, deadline):
    rows, seen, total = [], set(), 0
    for line in lines:
        prefix, name = line.split("\t", 1)
        permissions, oid, _stage = prefix.split(" ")
        path = Path(name)
        if path.is_absolute() or ".." in path.parts or name in seen or name.startswith('"'):
            raise ValueError("tracked-path")
        seen.add(name)
        item = fingerprint(ROOT / path, 16 * 1024 * 1024, deadline)
        if item["gitBlob"] != oid or bool(item["mode"] & 0o111) != (permissions == "100755"):
            raise ValueError("tracked-content-mismatch")
        total += item["bytes"]
        if total > 512 * 1024 * 1024:
            raise ValueError("tracked-input-budget")
        rows.append({"path": name, **item})
    if not 1 <= len(rows) <= 20000:
        raise ValueError("tracked-count")
    selected = {row["path"]: row["gitBlob"] for row in rows}
    required = dict(PINS)
    if mode == "baseline":
        required.update(BASELINE)
    if any(selected.get(name) != oid for name, oid in required.items()):
        raise ValueError("named-source-pin")
    return rows


def tool_state(deadline):
    found = shutil.which("go")
    if found is None:
        raise ValueError("go-tool-missing")
    go = Path(found).resolve(strict=True)
    goroot = go.parents[1]
    budget, rows = Budget(deadline), []
    def retain(name, path):
        item = fingerprint(path, deadline=deadline, budget=budget)
        rows.append({"name": name, **item})
    for path in (go, goroot / "bin/gofmt", goroot / "VERSION"):
        retain(str(path.relative_to(goroot)), path)
    # Stream directory entries; reserve count/bytes before reading or retaining.
    iterator = iter_tool_paths(goroot / "pkg/tool", deadline)
    try:
        for path in iterator:
            retain(str(path.relative_to(goroot)), path)
    finally:
        iterator.close()
    if not any(row["name"].endswith("/compile") for row in rows):
        raise ValueError("compiler-identity-missing")
    for name in ("git", "gcc", "g++", "ld"):
        found = shutil.which(name)
        if found is not None:
            retain(name, Path(found).resolve(strict=True))
    retain("python", Path(sys.executable).resolve(strict=True))
    # Sorting is safe only after the complete bounded inventory has settled.
    rows.sort(key=lambda row: row["name"])
    return go, goroot / "bin/gofmt", rows


def valid_handoff(value):
    if not isinstance(value, dict) or set(value) != HANDOFF_FIELDS:
        return False
    if type(value["schemaVersion"]) is not int or value["schemaVersion"] != 1:
        return False
    if value["baseRevision"] != BASE or value["baseTree"] != BASE_TREE or value["mode"] not in ("baseline", "candidate"):
        return False
    for field, length in (("revision", 40), ("tree", 40), ("sourceSHA256", 64), ("toolchainSHA256", 64)):
        if not isinstance(value[field], str) or not re.fullmatch(r"[a-f0-9]{" + str(length) + "}", value[field]):
            return False
    if any(value[field] is not True for field in ("formattingVerified", "modulesVerified", "compileSettled")):
        return False
    if type(value["inputCount"]) is not int or not 1 <= value["inputCount"] <= 20000:
        return False
    if value["compileInvocation"] != "go-test-c-player-r18-v1" or value["runInvocation"] != "go-test2json-player-r18-four-twice-v1":
        return False
    item = value["binary"]
    if not isinstance(item, dict) or set(item) != {"sha256", "bytes", "mode"}:
        return False
    return (isinstance(item["sha256"], str) and bool(re.fullmatch(r"[a-f0-9]{64}", item["sha256"]))
            and type(item["bytes"]) is int and 0 < item["bytes"] <= 128 * 1024 * 1024
            and type(item["mode"]) is int and 0 <= item["mode"] <= 0o777 and bool(item["mode"] & 0o100))


def binary_state(path, deadline):
    item = fingerprint(path, deadline=deadline)
    result = {field: item[field] for field in ("sha256", "bytes", "mode")}
    if not 0 < result["bytes"] <= 128 * 1024 * 1024 or not result["mode"] & 0o100:
        raise ValueError("compiled-binary-shape")
    return result


def save_new(path, value):
    with path.open("x", encoding="utf-8") as target:
        json.dump(value, target, indent=2, sort_keys=True, allow_nan=False)
        target.write("\n")


def safe_process(value):
    allowed = {"exitCode", "ownedProcessExited", "ownedGroupSettled", "captureSettled",
               "processLaunched", "seconds", "stopReason"}
    return {key: item for key, item in value.items() if key in allowed}
