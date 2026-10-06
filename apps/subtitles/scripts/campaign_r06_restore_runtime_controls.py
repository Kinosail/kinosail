"""Four exact Restore controls and source-bound safe Owner diagnostics."""
import re
from campaign_r06_restore_runtime_sources import strict_json

PACKAGE = "github.com/MikeO7/kinosail-subtitles/engineering/qa/2026-10-05-restore-recovery/fixture"
NAMES = ("TestRestoreLegacySwapPublicControl", "TestRestorePreparedReceiptPublicControl",
         "TestRestoreHeldHeadersPublicControl", "TestRestoreHeldInspectionBodyPublicControl")
LOCATIONS = {"R06_RESTORE_OWNER_REQUEST": 35, "R06_RESTORE_OWNER_SETUP": 52,
             "R06_RESTORE_OWNER_ENROLLMENT": 85, "R06_RESTORE_OWNER_CURRENT": 121}
STAGES = {"setup": 200, "enrollment": 200, "mfa": 303, "current": 200}

INVALID_LIMIT = 16777216
INVALID_REASONS = ("control-json", "control-invalid-type", "control-output", "control-marker-location",
                   "control-owner-shape", "control-marker-duplicate", "control-owner-prerequisite",
                   "control-package", "control-event", "control-after-package-terminal", "control-name",
                   "control-output-order", "control-action", "control-early-package-terminal",
                   "control-package-action", "control-package-marker")


def control_json(line):
    try: return strict_json(line, 65536)
    except (ValueError, TypeError, KeyError): raise ValueError("control-json") from None


class ControlProjection:
    def __init__(self):
        self.started, self.terminals, self.owner = set(), {}, {name: {} for name in NAMES}
        self.package_terminal = None
        self.invalid, self.duplicates = 0, 0
        self.invalid_reasons = {key: 0 for key in INVALID_REASONS}

    def diagnostic(self, name, output):
        if type(output) is not str or len(output.encode()) > 65536: raise ValueError("control-output")
        if "R06_RESTORE_OWNER_" not in output: return
        match = re.fullmatch(r"[ \t]*restore_owner_enrollment_test\.go:([0-9]{1,3}): ([^\r\n]+)\n", output)
        if match is None: raise ValueError("control-marker-location")
        line, text = match.groups()
        fields = text.split(" ")
        if fields[0] not in LOCATIONS or LOCATIONS[fields[0]] != int(line):
            raise ValueError("control-marker-location")
        marker, tokens = fields[0], fields[1:]
        if marker == "R06_RESTORE_OWNER_REQUEST":
            if len(tokens) != 3 or tokens[0] not in STAGES: raise ValueError("control-owner-shape")
            key = "request-" + tokens[0]
            expected = [str(STAGES[tokens[0]]), "true"]
            actual = tokens[1:]
        else:
            key = marker
            expected = {"R06_RESTORE_OWNER_SETUP": ["200", "true", "true", "true", "true"],
                        "R06_RESTORE_OWNER_ENROLLMENT": ["200", "true", "true", "true"],
                        "R06_RESTORE_OWNER_CURRENT": ["200", "true", "true"]}[marker]
            actual = tokens
        if key in self.owner[name]: raise ValueError("control-marker-duplicate")
        if marker == "R06_RESTORE_OWNER_SETUP":
            if len(actual) != 5 or actual[-1] not in ("true", "false") or actual[:-1] != expected[:-1]:
                raise ValueError("control-owner-prerequisite")
            self.owner[name]["setupPageCSRF"] = actual[-1] == "true"
        elif actual != expected: raise ValueError("control-owner-prerequisite")
        self.owner[name][key] = True

    def consume(self, line):
        try:
            value = control_json(line)
            if type(value) is not dict or value.get("Package") != PACKAGE: raise ValueError("control-package")
            if set(value) - {"Time", "Action", "Package", "Test", "Elapsed", "Output", "OutputType", "FailedBuild"}:
                raise ValueError("control-event")
            if "OutputType" in value and (value.get("Action") != "output" or value["OutputType"] != "frame"):
                raise ValueError("control-event")
            action, name = value.get("Action"), value.get("Test")
            if self.package_terminal is not None: raise ValueError("control-after-package-terminal")
            if name is not None:
                if name not in NAMES: raise ValueError("control-name")
                if action == "run":
                    if name in self.started: self.duplicates += 1
                    self.started.add(name)
                elif action == "output":
                    if name not in self.started or name in self.terminals: raise ValueError("control-output-order")
                    self.diagnostic(name, value.get("Output"))
                elif action in ("pass", "fail", "skip"):
                    if name not in self.started or name in self.terminals: self.duplicates += 1
                    self.terminals[name] = action
                else: raise ValueError("control-action")
            elif action in ("pass", "fail", "skip"):
                if set(self.terminals) != set(NAMES): raise ValueError("control-early-package-terminal")
                self.package_terminal = action
            elif action not in ("start", "output"): raise ValueError("control-package-action")
            elif action == "output" and "R06_RESTORE_OWNER_" in str(value.get("Output")):
                raise ValueError("control-package-marker")
        except (ValueError, TypeError, KeyError) as error:
            self.invalid += 1
            reason = error.args[0] if type(error) is ValueError and len(error.args) == 1 else None
            self.invalid_reasons[reason if type(reason) is str and reason in INVALID_REASONS else "control-invalid-type"] += 1

    def result(self):
        required = {"request-" + key for key in STAGES} | set(LOCATIONS) - {"R06_RESTORE_OWNER_REQUEST"}
        required.add("setupPageCSRF")
        owner_complete = all(set(self.owner[name]) == required for name in NAMES)
        green = (self.started == set(NAMES) and self.terminals == {name: "pass" for name in NAMES} and
                 self.package_terminal == "pass" and owner_complete and self.invalid == self.duplicates == 0)
        return {"green": green, "names": list(NAMES), "started": sorted(self.started),
                "terminals": dict(self.terminals), "packageTerminal": self.package_terminal,
                "ownerComplete": owner_complete, "owner": self.owner,
                "invalidEventCount": self.invalid, "duplicateTerminalCount": self.duplicates,
                "invalidReasons": dict(self.invalid_reasons)}
