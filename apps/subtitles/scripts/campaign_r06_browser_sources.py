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
                  ("subtitle-source-cues.js","subtitle-save-operation.js","subtitle-inspector.js"))
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


CONTROL_FAILURE_CODES = {
    "private fixture directory unavailable": "private-root",
    "real Server fixture construction failed": "server-fixture",
    "actual TLS Owner/MFA/CSRF prerequisite failed": "owner-auth",
    "public fictional item prerequisite failed": "catalog",
    "public inspection prerequisite failed": "inspection",
    "real public preview prerequisite failed": "preview",
    "real public preparation prerequisite failed": "prepare",
    "owned request did not settle": "request-settlement",
    "owned body read did not settle": "body-settlement",
    "Save transport prerequisite failed": "save-transport",
    "actual Save/History/receipt witness missing": "save-witness",
    "actual persisted Save/History witness is not eligible": "saved-witness-ineligible",
    "held headers escaped before release": "headers-premature",
    "held request failed": "held-request-error",
    "actual response headers unavailable": "headers-unavailable",
    "body fault also withheld headers": "body-headers-withheld",
    "held body escaped before release": "body-premature",
    "released actual body changed": "released-body-changed",
    "released body did not settle": "released-body-unsettled",
    "released response failed": "released-response-error",
    "released response did not settle": "released-response-unsettled",
    "released actual response status or headers changed": "released-headers-changed",
    "release replayed or changed the actual Save": "save-replayed",
    "actual released response did not settle as a complete body write": "released-response-incomplete"
}

def control_diagnostics():
    return {"failures":[], "ownerRequests":{}, "ownerSetup":None, "ownerEnrollment":None, "ownerCurrent":None, "routes":{}}


def control_diagnostics_complete(value, prepared):
    expected = {"setup":200, "enrollment":200, "mfa":303, "current":200}
    if value["failures"] or value["ownerRequests"] != {key:{"status":status,"ok":True} for key,status in expected.items()}:
        return False
    setup = value["ownerSetup"]
    if (setup is None or {key:item for key,item in setup.items() if key != "csrf"}
            != {"status":200,"read":True,"secure":True,"totp":True} or type(setup.get("csrf")) is not bool):
        return False
    if value["ownerEnrollment"] != {"status":200,"read":True,"bounded":True,"csrf":True}:
        return False
    if value["ownerCurrent"] != {"status":200,"read":True,"csrf":True}:
        return False
    statuses = {"catalog":200,"inspection":200,"preview":200}
    if prepared: statuses["prepare"] = 201
    return value["routes"] == {key:{"status":status,"returned":status,"transport":True,
                                     "read":True,"bounded":True,"decoded":True} for key,status in statuses.items()}


def control_output(value, name, output):
    if type(output) is not str or len(output) > 65536: raise ValueError("control-output")
    match = re.fullmatch(r"[ \t]*(fixture_test\.go|owner_test\.go):([0-9]{1,3}): ([^\r\n]+)\n",output)
    if match is None:
        if any(marker in output for marker in ("R06_OWNER_","R06_ROUTE ")):
            raise ValueError("control-marker-location")
        return
    file, line, text = match.groups()
    if not 1 <= int(line) <= 300: raise ValueError("control-source-line")
    if text in CONTROL_FAILURE_CODES:
        caller = 25 + GO_CASES.index(name)
        allowed = {83,89} if text in ("owned request did not settle","owned body read did not settle") else {caller}
        if file != "fixture_test.go" or int(line) not in allowed: raise ValueError("control-failure-location")
        code = CONTROL_FAILURE_CODES[text]
        if code in value["failures"] or len(value["failures"]) >= 4: raise ValueError("control-failure-duplicate")
        value["failures"].append(code)
        return
    parts = text.split(" ")
    if not parts[0].startswith(("R06_OWNER_","R06_ROUTE")): return
    if file != "owner_test.go": raise ValueError("control-marker-location")
    def status(token):
        if not re.fullmatch(r"0|[1-5][0-9]{2}",token): raise ValueError("control-status")
        return int(token)
    def boolean(token):
        if token not in ("true","false"): raise ValueError("control-boolean")
        return token == "true"
    marker = parts[0]
    if marker == "R06_OWNER_REQUEST" and len(parts) == 4 and parts[1] in ("setup","enrollment","mfa","current"):
        key = parts[1]
        if key in value["ownerRequests"]: raise ValueError("control-marker-duplicate")
        value["ownerRequests"][key] = {"status":status(parts[2]),"ok":boolean(parts[3])}
    elif marker == "R06_OWNER_SETUP" and len(parts) == 6 and value["ownerSetup"] is None:
        value["ownerSetup"] = {"status":status(parts[1]), **dict(zip(("read","secure","totp","csrf"),map(boolean,parts[2:]),strict=True))}
    elif marker == "R06_OWNER_ENROLLMENT" and len(parts) == 5 and value["ownerEnrollment"] is None:
        value["ownerEnrollment"] = {"status":status(parts[1]), **dict(zip(("read","bounded","csrf"),map(boolean,parts[2:]),strict=True))}
    elif marker == "R06_OWNER_CURRENT" and len(parts) == 4 and value["ownerCurrent"] is None:
        value["ownerCurrent"] = {"status":status(parts[1]),"read":boolean(parts[2]),"csrf":boolean(parts[3])}
    elif marker == "R06_ROUTE" and len(parts) == 8 and parts[1] in ("catalog","inspection","preview","prepare"):
        key = parts[1]
        if key in value["routes"]: raise ValueError("control-marker-duplicate")
        value["routes"][key] = {"status":status(parts[2]),"returned":status(parts[3]),
            **dict(zip(("transport","read","bounded","decoded"),map(boolean,parts[4:]),strict=True))}
    else:
        raise ValueError("control-marker-shape")


class GoProjection:
    def __init__(self):
        self.started, self.terminals = set(), {}
        self.package_terminal, self.prerequisite = None, False
        self.invalid = self.duplicate = 0
        self.diagnostics = {name:control_diagnostics() for name in GO_CASES}

    def consume(self, line):
        try:
            value = json.loads(line)
            if type(value) is not dict or value.get("Package") != PACKAGE:
                raise ValueError("event")
            action, test = value.get("Action"), value.get("Test")
            if action == "output":
                if test is not None:
                    if test not in GO_CASES or test not in self.started: raise ValueError("control-output-case")
                    control_output(self.diagnostics[test],test,value.get("Output"))
                elif any(marker in str(value.get("Output")) for marker in ("R06_OWNER_","R06_ROUTE ")):
                    raise ValueError("control-output-case")
                return
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
                 and self.package_terminal == "pass" and not self.invalid and not self.duplicate
                 and all(control_diagnostics_complete(self.diagnostics[name], "Receipt" in name) for name in GO_CASES))
        return {"green":green,"cases":[{"name":name,"status":self.terminals.get(name,"unreached")} for name in GO_CASES],
                "packageStatus":self.package_terminal,"invalidEvents":self.invalid,"duplicates":self.duplicate,
                "diagnostics":[{"name":name,**self.diagnostics[name]} for name in GO_CASES]}
