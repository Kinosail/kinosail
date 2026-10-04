"""Allowlisted JSON projection; raw assertion text never leaves memory."""
import json
import math
from pathlib import PurePosixPath
import re

PACKAGE = "github.com/MikeO7/kinosail-subtitles/internal/server"
PREREQUISITES = {
    "startup admission prerequisite remained busy": "startup-admission-prerequisite",
    "public operation preparation =": "preparation-prerequisite",
    "public operation status =": "private-status-prerequisite",
    "legacy public audio prerequisite =": "audio-prerequisite",
    "legacy public Save setup =": "save-prerequisite",
    "controlled descendant process prerequisite did not become ready": "child-prerequisite",
    "fictional analysis process did not start": "analysis-prerequisite",
    "durable-boundary fixture prerequisite:": "durable-fixture-prerequisite",
}

class Projection:
    def __init__(self, scope):
        self.top = scope["namedR06Tests"] + scope["incumbentControls"]
        self.sub = scope["expectedSubtests"]
        self.files = {PurePosixPath(s["path"]).name for s in scope["sourcePins"] if s["path"].endswith("_test.go")}
        self.rows, self.runs, self.phases, self.locations = {}, {}, {}, {}
        self.package, self.package_terminals = None, 0
        self.invalid = self.unknown = self.duplicates = 0
        self.prerequisite = None

    def elapsed(self, event):
        value = event.get("Elapsed")
        if isinstance(value, bool) or not isinstance(value, (int, float)) or not math.isfinite(value) or not 0 <= value <= 90:
            self.invalid += 1
            return None
        return value

    def consume(self, line):
        try:
            event = json.loads(line)
        except (ValueError, UnicodeError):
            self.invalid += 1
            return
        if not isinstance(event, dict):
            self.invalid += 1
            return
        name, action = event.get("Test"), event.get("Action")
        if not isinstance(action, str) or not isinstance(event.get("Time"), str) or not event["Time"]:
            self.invalid += 1
            return
        if action in ("build-start", "build-output", "build-fail") and "Test" not in event:
            if action == "build-fail":
                self.invalid += 1
            return
        if event.get("Package") != PACKAGE or "Test" in event and (not isinstance(name, str) or not name):
            self.invalid += 1
            return
        if not name:
            if action not in ("start", "output", "pass", "fail"):
                self.invalid += 1
            elif action in ("pass", "fail"):
                self.elapsed(event)
                self.package_terminals += 1
                self.package = action
            elif action == "output" and not isinstance(event.get("Output"), str):
                self.invalid += 1
            return
        if name not in self.top and name not in self.sub:
            self.unknown += 1
            return
        if action not in ("run", "pause", "cont", "output", "pass", "fail", "skip"):
            self.invalid += 1
            return
        if action == "run":
            self.runs[name] = self.runs.get(name, 0) + 1
        elif action == "output":
            if not isinstance(event.get("Output"), str):
                self.invalid += 1
                return
            for marker, phase in PREREQUISITES.items():
                if marker in event["Output"]:
                    self.phases[name] = phase
            match = re.match(r"\s*([a-zA-Z0-9_]+_test\.go):([0-9]{1,4}):", event["Output"])
            if match and match[1] in self.files and 1 <= int(match[2]) <= 9999:
                self.locations[name] = {"file": match[1], "line": int(match[2])}
        elif action in ("pass", "fail", "skip"):
            elapsed = self.elapsed(event)
            if name in self.rows:
                self.duplicates += 1
            self.rows[name] = {"name": name, "status": action, "seconds": elapsed}
            if action == "fail":
                self.rows[name]["failurePhaseHint"] = self.phases.get(name, "unclassified-inconclusive")
                if name in self.locations:
                    self.rows[name]["sourceLocation"] = self.locations[name]
                self.prerequisite = self.phases.get(name)

    def results(self):
        def counts(names):
            statuses = [self.rows.get(n, {}).get("status") for n in names]
            return {"expected": len(names), **{s: statuses.count(s) for s in ("pass", "fail", "skip")}, "notCompleted": statuses.count(None)}
        return {"topLevel": [self.rows.get(n, {"name": n, "status": "not-completed"}) for n in self.top],
                "subtests": [self.rows.get(n, {"name": n, "status": "not-completed"}) for n in self.sub],
                "counts": {"topLevel": counts(self.top), "subtests": counts(self.sub)}, "packageStatus": self.package,
                "invalidEventCount": self.invalid, "unallowlistedTestEventCount": self.unknown, "duplicateTerminalCount": self.duplicates,
                "duplicatePackageTerminalCount": max(0, self.package_terminals - 1),
                "packageTerminalComplete": self.package_terminals == 1,
                "runIdentityComplete": all(self.runs.get(n) == 1 for n in self.top + self.sub),
                "prerequisiteFailurePhaseHint": self.prerequisite,
                "failureClassificationBoundary": "Fixed phase hints and pinned source locations only. Unknown assertions are inconclusive because private raw output is not exported."}
