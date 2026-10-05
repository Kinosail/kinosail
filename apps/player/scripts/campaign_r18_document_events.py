"""Closed Go test2json projection: retain fixed identities, never raw Output."""
from datetime import datetime
import hashlib
import json
import math
import re

PACKAGE = "github.com/MikeO7/kinosail-player/internal/server"
NAMES = (
    "TestHomeAssistantDocumentTargetsRemainIndependentThroughPublicApp",
    "TestHomeAssistantDocumentSiblingCannotOverwriteReleaseOrDrain",
    "TestHomeAssistantDocumentReleaseKeepsCandidateAndRejectsRetiredClaim",
    "TestHomeAssistantDocumentNativeStrictContractRemainsUnchanged",
)
CALL_LINES = dict(zip(NAMES[:3], (11, 30, 53), strict=True))
SOURCE = "home_assistant_document_targets_test.go"
SAFE_SOURCE_LINES = {SOURCE: 91, "home_assistant_document_targets_helpers_test.go": 232}
TIME = re.compile(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z")
FRAME = re.compile(r"^\s+([a-z_]+\.go):([1-9]\d*): (.*)$")
ABSENCE = re.compile(r"R18 claim-route prerequisite: HTTP (404|405), expected 201")
FIELDS = {"Time", "Action", "Package", "Test", "Elapsed", "Output"}


def pairs(items):
    value = {}
    for key, item in items:
        if key in value:
            raise ValueError("duplicate-json-key")
        value[key] = item
    return value


def constant(_value):
    raise ValueError("nonfinite-json")


class Projection:
    def __init__(self):
        self.prerequisite = False
        self.started = False
        self.package_terminal = None
        self.active = None
        self.test_marker = None
        self.package_marker = None
        self.records = []
        self.occurrences = dict.fromkeys(NAMES, 0)
        self.event_count = 0
        self.byte_count = 0
        self.digest = hashlib.sha256()

    def consume(self, raw):
        self.event_count += 1
        self.byte_count += len(raw) + 1
        self.digest.update(raw + b"\n")
        if self.prerequisite:
            return
        try:
            if len(raw) > 65536 or self.byte_count > 16 * 1024 * 1024 or self.event_count > 10000:
                raise ValueError("capture-bound")
            value = json.loads(raw.decode("utf-8"), object_pairs_hook=pairs, parse_constant=constant)
            self.validate(value)
            self.advance(value)
        except (ValueError, TypeError, UnicodeError, KeyError):
            self.prerequisite = True

    def validate(self, value):
        if not isinstance(value, dict) or set(value) - FIELDS:
            raise ValueError("event-shape")
        if value.get("Package") != PACKAGE or not isinstance(value.get("Time"), str) or not TIME.fullmatch(value["Time"]):
            raise ValueError("event-identity")
        datetime.fromisoformat(value["Time"])
        if value.get("Action") not in ("start", "run", "output", "pass", "fail"):
            raise ValueError("event-action")
        if "Test" in value and value["Test"] not in NAMES:
            raise ValueError("test-identity")
        if "Elapsed" in value:
            elapsed = value["Elapsed"]
            if type(elapsed) not in (int, float) or not math.isfinite(elapsed) or not 0 <= elapsed <= 45:
                raise ValueError("event-elapsed")
        if "Output" in value and (value["Action"] != "output" or not isinstance(value["Output"], str)):
            raise ValueError("event-output")
        if value["Action"] == "output" and "Output" not in value:
            raise ValueError("missing-output")

    def advance(self, value):
        action, name = value["Action"], value.get("Test")
        if self.package_terminal is not None:
            raise ValueError("after-package-terminal")
        if action == "start":
            if self.started or name is not None:
                raise ValueError("package-start")
            self.started = True
            return
        if not self.started:
            raise ValueError("missing-package-start")
        if action == "run":
            if name is None or self.active is not None or self.occurrences[name] >= 2:
                raise ValueError("test-run")
            self.occurrences[name] += 1
            self.test_marker = None
            self.active = {"test": name, "occurrence": self.occurrences[name], "status": None,
                           "absenceHTTP": None, "errorFrames": 0, "sourceLocation": None}
            return
        if action == "output":
            if name is not None and (self.active is None or self.active["test"] != name):
                raise ValueError("output-owner")
            for line in value["Output"].splitlines():
                self.output(line, name)
            return
        if name is None:
            if self.active is not None:
                raise ValueError("package-with-active-test")
            if self.package_marker is not None and self.package_marker != action:
                raise ValueError("package-marker-status")
            self.package_terminal = action
            return
        if self.active is None or self.active["test"] != name:
            raise ValueError("terminal-owner")
        if self.test_marker is not None and self.test_marker != action:
            raise ValueError("test-marker-status")
        self.active["status"] = action
        self.records.append(self.active)
        self.active = None

    def output(self, line, name):
        if not line:
            return
        if name is None:
            if line not in ("PASS", "FAIL") or self.package_marker is not None or self.active is not None or len(self.records) != 8:
                raise ValueError("unexpected-package-output")
            self.package_marker = line.lower()
            return
        if line == "=== RUN   " + name:
            return
        marker = re.fullmatch(r"--- (PASS|FAIL): " + re.escape(name) + r" \(\d+(?:\.\d+)?s\)", line)
        if marker:
            if self.test_marker is not None:
                raise ValueError("duplicate-test-marker")
            self.test_marker = marker[1].lower()
            return
        frame = FRAME.fullmatch(line)
        if not frame:
            raise ValueError("unexpected-test-output")
        self.active["errorFrames"] += 1
        file, line_number, message = frame.groups()
        if file in SAFE_SOURCE_LINES and 1 <= int(line_number) <= SAFE_SOURCE_LINES[file] and self.active["sourceLocation"] is None:
            self.active["sourceLocation"] = file + ":" + line_number
        status = ABSENCE.fullmatch(message)
        if file == SOURCE and int(line_number) == CALL_LINES.get(name) and status:
            self.active["absenceHTTP"] = int(status[1])
            self.active["sourceLocation"] = SOURCE + ":" + line_number

    def result(self):
        return {"schemaValid": not self.prerequisite, "started": self.started,
                "closed": self.package_terminal is not None and self.active is None and not self.prerequisite,
                "packageTerminal": self.package_terminal, "records": [dict(row) for row in self.records],
                "events": self.event_count, "captureBytes": self.byte_count,
                "captureSHA256": self.digest.hexdigest()}
