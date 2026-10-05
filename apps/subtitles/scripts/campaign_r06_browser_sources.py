"""Complete tracked-source identity and strict private typed-result boundary."""
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess

ROOT = Path(__file__).resolve().parents[3]
APP = ROOT / "apps/subtitles"
OUTPUT = ROOT / ".verification/campaign-proof/R06"
FIXTURE = APP / "engineering/qa/2026-10-05-save-browser/fixture"
PACKAGE = "github.com/MikeO7/kinosail-subtitles/engineering/qa/2026-10-05-save-browser/fixture"
GO_CASES = ("TestSaveHeadersLegacyActualCompletion", "TestSaveBodyLegacyActualCompletion",
            "TestSaveHeadersReceiptActualCompletion", "TestSaveBodyReceiptActualCompletion")


def pin(data):
    return {"bytes":len(data),"sha256":hashlib.sha256(data).hexdigest()}


def fingerprint(path, limit=128*1024*1024, executable=False):
    info = path.lstat()
    if (not stat.S_ISREG(info.st_mode) or not 0 <= info.st_size <= limit
            or executable and (info.st_size == 0 or not info.st_mode & 0o111)):
        raise ValueError("file-shape")
    digest = hashlib.sha256()
    git_digest = hashlib.sha1(("blob "+str(info.st_size)+"\0").encode())
    with path.open("rb") as stream:
        for block in iter(lambda:stream.read(65536), b""):
            digest.update(block)
            git_digest.update(block)
    return {"bytes":info.st_size,"sha256":digest.hexdigest(),"mode":info.st_mode & 0o777,"gitBlob":git_digest.hexdigest()}


def git(*args):
    return subprocess.check_output(["git", *args], cwd=ROOT, timeout=10, stderr=subprocess.DEVNULL)


def identity():
    revision = git("rev-parse","HEAD").decode().strip()
    tree = git("rev-parse","HEAD^{tree}").decode().strip()
    if not re.fullmatch("[a-f0-9]{40}",revision) or not re.fullmatch("[a-f0-9]{40}",tree):
        raise ValueError("git-identity")
    if git("diff","HEAD","--name-only","-z") or (ROOT/".gates-disabled").exists():
        raise ValueError("tracked-drift")
    return {"revision":revision,"tree":tree}


def tracked_sources():
    records = []
    for row in git("ls-files","--stage","-z").split(b"\0"):
        if not row: continue
        metadata, raw = row.split(b"\t",1)
        mode, blob, stage = metadata.decode().split()
        name = raw.decode("utf-8")
        if stage != "0" or not re.fullmatch("[a-f0-9]{40}",blob) or mode not in ("100644","100755","120000"):
            raise ValueError("tracked-entry")
        path = ROOT/name
        if mode == "120000":
            info = path.lstat()
            if not stat.S_ISLNK(info.st_mode): raise ValueError("symlink-drift")
            data = os.fsencode(os.readlink(path))
            state = pin(data)
            if hashlib.sha1(("blob "+str(len(data))+"\0").encode()+data).hexdigest() != blob:
                raise ValueError("symlink-blob-drift")
        else:
            state = fingerprint(path,limit=32*1024*1024)
            if bool(state["mode"] & 0o111) != (mode == "100755") or state["gitBlob"] != blob:
                raise ValueError("source-mode-blob-drift")
            state = {"bytes":state["bytes"],"sha256":state["sha256"]}
        records.append({"path":name,"gitBlob":blob,"gitMode":mode,**state})
    if not 1 <= len(records) <= 10000: raise ValueError("tracked-count")
    records.sort(key=lambda row:row["path"])
    canonical = json.dumps(records,sort_keys=True,separators=(",",":")).encode()
    return {"schemaVersion":1,"scope":"entire-tracked-source","files":records,"canonical":pin(canonical)}


def inspector_identity():
    # Exact existing Go append order; no inserted delimiter or token change.
    paths = tuple(APP/"internal/server/static"/name for name in
                  ("subtitle-source-cues.js","subtitle-inspector.js"))
    for path in paths: fingerprint(path,256*1024)
    return pin(b"".join(path.read_bytes() for path in paths))


def private_result(path):
    info = path.lstat()
    if not stat.S_ISREG(info.st_mode) or stat.S_ISLNK(info.st_mode) or not 0 < info.st_size <= 131072 or info.st_mode & 0o077:
        raise ValueError("private-result-shape")
    with path.open("rb") as stream:
        data = stream.read(131073)
    if len(data) > 131072 or b"\0" in data: raise ValueError("private-result-bound")
    return json.loads(data.decode("utf-8"))


class GoProjection:
    def __init__(self):
        self.started, self.terminals = set(), {}
        self.package_terminal, self.prerequisite = None, False
        self.invalid = self.duplicate = 0

    def consume(self, line):
        try:
            value = json.loads(line)
            if type(value) is not dict or value.get("Package") != PACKAGE:
                raise ValueError("event")
            action, test = value.get("Action"), value.get("Test")
            if action == "output": return
            if test is not None:
                if test not in GO_CASES: raise ValueError("case")
                if action == "run":
                    if test in self.started: self.duplicate += 1
                    self.started.add(test)
                elif action in ("pass","fail","skip"):
                    if test not in self.started or test in self.terminals: self.duplicate += 1
                    self.terminals[test] = action
                else: raise ValueError("action")
            elif action in ("pass","fail"):
                if self.package_terminal is not None: self.duplicate += 1
                self.package_terminal = action
            elif action != "start":
                raise ValueError("package-action")
        except (ValueError,TypeError,UnicodeDecodeError):
            self.invalid += 1
            self.prerequisite = True

    def result(self):
        green = (self.started == set(GO_CASES) and set(self.terminals) == set(GO_CASES)
                 and all(value == "pass" for value in self.terminals.values())
                 and self.package_terminal == "pass" and not self.invalid and not self.duplicate)
        return {"green":green,"cases":[{"name":name,"status":self.terminals.get(name,"unreached")} for name in GO_CASES],
                "packageStatus":self.package_terminal,"invalidEvents":self.invalid,"duplicates":self.duplicate}
