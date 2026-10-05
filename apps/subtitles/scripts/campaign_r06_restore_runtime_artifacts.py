"""Fresh exclusive closed four-file artifact seal; no private output export."""
import json
import math
import re
import os
from pathlib import Path
import stat
from campaign_r06_restore_runtime_sources import strict_json, pin, checked_parents

NAMES = ("receipt.json", "results.json", "source-manifest.json", "artifact-manifest.json")
CAP = 4 * 1024 * 1024
LIMITS = {"ownedSettlementReserveSeconds": 5, "captureBytes": 16777216, "lineBytes": 65536,
          "privateResultBytes": 131072, "workers": 1, "retries": 0,
          "unlockMilliseconds": 45000, "observationGraceMilliseconds": 10000}
PENDING = "Published 2b816fa8 startup-corrected raw inputs; stock formatter qualification and hosted runtime isolation proof remain pending."
COVERAGE = "Selected sources/executables/package roots only; no universal network, host, system-library or whole-environment absence claim."
DEPENDENCY_LIMITS = {"restore-controls": "Selected Go executable only; no browser, Node, pnpm or installed browser dependency was used.",
                    "browser": "Selected executable and three transitive package roots only; system libraries and whole runner not fingerprinted."}
FAILURE_STAGES = {"checkout", "dependencies", "compile", "public-controls", "collection", "browser", "integrity", "artifact-admission"}
FAILURE_CLASSES = {"OSError", "FileNotFoundError", "FileExistsError", "NotADirectoryError", "IsADirectoryError",
                   "PermissionError", "ValueError", "RuntimeError", "KeyError", "TypeError", "StopIteration",
                   "UnicodeError", "UnicodeDecodeError", "JSONDecodeError", "OverflowError", "MemoryError", "UnexpectedError"}
PHASE_FIELDS = {"activeSeconds", "totalBoundSeconds", "processLaunched", "exitCode", "ownedProcessExited",
                "ownedGroupSettled", "captureSettled", "stopReason", "durationSeconds", "capturedBytes", "handlersRestored"}
STOP_REASONS = {None, "interrupted", "active-timeout", "output-bound", "process-boundary", "leader-poll",
                "group-stop", "leader-unsettled", "group-unsettled", "capture-close", "settlement-timeout", "handler-restore"}


def safe_failure_class(error):
    name = type(error).__name__
    return name if name in FAILURE_CLASSES else "UnexpectedError"


def hex_value(value, length):
    return type(value) is str and re.fullmatch("[a-f0-9]{" + str(length) + "}", value) is not None


def digest(value):
    return (type(value) is dict and set(value) == {"bytes", "sha256"} and type(value["bytes"]) is int and
            0 <= value["bytes"] <= CAP and hex_value(value["sha256"], 64))


def file_pin(value, executable=False):
    fields = {"bytes", "sha256", "gitBlob"} | ({"mode"} if executable else set())
    return (type(value) is dict and set(value) == fields and type(value["bytes"]) is int and
            (0 < value["bytes"] if executable else 0 <= value["bytes"]) and value["bytes"] <= (536870912 if executable else 33554432) and hex_value(value["sha256"], 64) and
            hex_value(value["gitBlob"], 40) and (not executable or
            (type(value["mode"]) is int and 0 <= value["mode"] <= 0o777 and bool(value["mode"] & 0o111))))


def phase_shape(phase, active, total):
    if phase == {}: return True
    return (type(phase) is dict and set(phase) == PHASE_FIELDS and
            type(phase["activeSeconds"]) is int and phase["activeSeconds"] == active and
            type(phase["totalBoundSeconds"]) is int and phase["totalBoundSeconds"] == total and
            all(type(phase[k]) is bool for k in ("processLaunched", "ownedProcessExited", "ownedGroupSettled", "captureSettled", "handlersRestored")) and
            (phase["exitCode"] is None or (type(phase["exitCode"]) is int and -255 <= phase["exitCode"] <= 255)) and
            phase["stopReason"] in STOP_REASONS and type(phase["capturedBytes"]) is int and
            0 <= phase["capturedBytes"] <= 16777216 + 65536 and type(phase["durationSeconds"]) in (int, float) and
            math.isfinite(phase["durationSeconds"]) and 0 <= phase["durationSeconds"] <= 3600)


def valid_phase(phase, active, total, exits=(0,)):
    from campaign_r06_restore_runtime_process import accepted
    return (phase_shape(phase, active, total) and bool(phase) and accepted(phase, exits) and
            phase["processLaunched"] is True and phase["capturedBytes"] <= 16777216 and phase["durationSeconds"] <= total)


def valid_dependencies(state, probes, suite):
    from campaign_r06_restore_runtime_tools import valid_tools, PACKAGES
    if type(state) is not dict or set(state) != {"executables", "installedClosure", "limits"}: return False
    limit = DEPENDENCY_LIMITS["restore-controls" if suite == "restore-controls" else "browser"]
    if not valid_tools(state["executables"], suite) or state["limits"] != limit: return False
    if suite == "restore-controls": return state["installedClosure"] is None and probes == {}
    if set(probes) != {"packageResolution", "browserResolution"} or any(not valid_phase(v, 5, 10) for v in probes.values()): return False
    closure = state["installedClosure"]
    if type(closure) is not dict or set(closure) != {"files", "canonical"}: return False
    rows = closure["files"]
    if type(rows) is not list or not 3 <= len(rows) <= 5000: return False
    labels, order, total = set(), [], 0
    for row in rows:
        if (type(row) is not dict or set(row) != {"package", "path", "bytes", "sha256", "gitBlob"} or
                row["package"] not in PACKAGES or not relative_path(row["path"]) or
                not file_pin({k: row[k] for k in ("bytes", "sha256", "gitBlob")}) or row["bytes"] > 4194304): return False
        labels.add(row["package"]); order.append((row["package"], row["path"])); total += row["bytes"]
    canonical = json.dumps(rows, sort_keys=True, separators=(",", ":")).encode()
    return (labels == set(PACKAGES) and order == sorted(set(order)) and total <= 67108864 and
            closure["canonical"] == {"bytes": len(canonical), "sha256": pin(canonical)["sha256"]} and digest(closure["canonical"]))


def relative_path(value):
    return (type(value) is str and bool(value) and "\0" not in value and not Path(value).is_absolute() and
            ".." not in Path(value).parts and all(ord(char) >= 32 for char in value))


def source_shape(source, receipt):
    from campaign_r06_restore_runtime_sources import bind_inputs, REQUIRED_INPUTS, QA
    if type(source.get("schemaVersion")) is not int or source["schemaVersion"] != 1: raise ValueError("artifact-source-version")
    if set(source) == {"schemaVersion", "revision", "tree", "status"}:
        if source["status"] != "unreached" or receipt["sourceBefore"] is not None: raise ValueError("artifact-unreached")
        return False
    if set(source) != {"schemaVersion", "revision", "tree", "ledger", "inputs", "runtimePackage"}: raise ValueError("artifact-source")
    if not hex_value(source["tree"], 40): raise ValueError("artifact-source-tree")
    ledger = source["ledger"]
    if type(ledger) is not dict or set(ledger) != {"schemaVersion", "scope", "files", "canonical"}: raise ValueError("artifact-ledger")
    rows = ledger["files"]
    if (type(ledger["schemaVersion"]) is not int or ledger["schemaVersion"] != 1 or ledger["scope"] != "entire-tracked-source" or
            type(rows) is not list or not 1 <= len(rows) <= 10000): raise ValueError("artifact-ledger")
    names = []
    for row in rows:
        if (type(row) is not dict or set(row) != {"path", "gitMode", "bytes", "sha256", "gitBlob"} or
                not relative_path(row["path"]) or row["gitMode"] not in ("100644", "100755", "120000") or
                type(row["bytes"]) is not int or not 0 <= row["bytes"] <= 33554432 or
                not hex_value(row["sha256"], 64) or not hex_value(row["gitBlob"], 40)): raise ValueError("artifact-ledger-entry")
        names.append(row["path"])
    if names != sorted(set(names)): raise ValueError("artifact-ledger-order")
    canonical = json.dumps(rows, sort_keys=True, separators=(",", ":")).encode()
    expected = {"bytes": len(canonical), "sha256": pin(canonical)["sha256"]}
    if ledger["canonical"] != expected or not digest(ledger["canonical"]) or receipt["sourceBefore"] != expected: raise ValueError("artifact-source-canonical")
    inputs = source["inputs"]
    if (type(inputs) is not dict or set(inputs) != REQUIRED_INPUTS or
            any(not file_pin(value) for value in inputs.values())): raise ValueError("artifact-inputs")
    package, package_path = source["runtimePackage"], QA + "runtime-source-manifest.json"
    if (type(package) is not dict or set(package) != {"path", "bytes", "sha256", "gitBlob"} or package["path"] != package_path or
            {k: package[k] for k in ("bytes", "sha256", "gitBlob")} != inputs[package_path]): raise ValueError("artifact-runtime-package")
    expected_rows = [{"path": path, "blob": value["gitBlob"], "bytes": value["bytes"], "sha256": value["sha256"]} for path, value in inputs.items()]
    if bind_inputs(ledger, expected_rows) != inputs: raise ValueError("artifact-inputs")
    return True


def control_shape(controls):
    from campaign_r06_restore_runtime_controls import NAMES as TESTS, LOCATIONS, STAGES
    fields = {"green", "names", "started", "terminals", "packageTerminal", "ownerComplete", "owner", "invalidEventCount", "duplicateTerminalCount"}
    if type(controls) is not dict or set(controls) != fields or controls["names"] != list(TESTS): raise ValueError("artifact-controls")
    started, terminals, owner = controls["started"], controls["terminals"], controls["owner"]
    if (type(started) is not list or any(name not in TESTS for name in started) or started != sorted(set(started)) or
            type(terminals) is not dict or set(terminals) - set(TESTS) or any(v not in ("pass", "fail", "skip") for v in terminals.values()) or
            controls["packageTerminal"] not in (None, "pass", "fail", "skip") or
            (controls["packageTerminal"] is not None and set(terminals) != set(TESTS)) or
            type(owner) is not dict or set(owner) != set(TESTS) or
            any(type(controls[k]) is not int or not 0 <= controls[k] <= 16777216 for k in ("invalidEventCount", "duplicateTerminalCount"))):
        raise ValueError("artifact-control-accounting")
    required = {"request-" + key for key in STAGES} | set(LOCATIONS) - {"R06_RESTORE_OWNER_REQUEST"}
    for marks in owner.values():
        if (type(marks) is not dict or set(marks) - required - {"setupPageCSRF"} or
                any(value is not True for key, value in marks.items() if key != "setupPageCSRF") or
                ("setupPageCSRF" in marks and type(marks["setupPageCSRF"]) is not bool)): raise ValueError("artifact-owner")
    complete = all(set(marks) == required | {"setupPageCSRF"} for marks in owner.values())
    green = (started == sorted(TESTS) and terminals == {name: "pass" for name in TESTS} and
             controls["packageTerminal"] == "pass" and complete and controls["invalidEventCount"] == controls["duplicateTerminalCount"] == 0)
    if type(controls["green"]) is not bool or type(controls["ownerComplete"]) is not bool or controls["green"] != green or controls["ownerComplete"] != complete:
        raise ValueError("artifact-control-derived")


def validate_acceptance(receipt, results, source):
    from campaign_r06_restore_runtime_sources import SUITES
    from campaign_r06_restore_runtime_tools import same_state
    from campaign_r06_restore_runtime_projection import admit, classify
    fields = {"schemaVersion", "id", "mode", "suite", "revision", "tree", "classification",
              "sourceUnchanged", "dependenciesUnchanged", "compiledBinaryUnchanged", "startedUTC", "endedUTC",
              "phases", "metadataPhases", "dependencyPhasesBefore", "dependencyPhasesAfter", "dependenciesBefore", "dependenciesAfter",
              "compiledBinaryBefore", "compiledBinaryAfter", "sourceBefore", "sourceAfter", "inspectorSHA256", "limits", "pendingIsolation", "coverageLimit"}
    if set(receipt) not in (fields, fields | {"failure"}) or type(receipt["schemaVersion"]) is not int or receipt["schemaVersion"] != 1: raise ValueError("artifact-receipt")
    suite, classification = receipt["suite"], receipt["classification"]
    if suite not in SUITES or classification not in ("prerequisite-blocked", "focused-restore-public-controls-green", "focused-restore-browser-green", "confirmed-restore-deadline-red"):
        raise ValueError("artifact-classification")
    if not hex_value(receipt["revision"], 40) or (receipt["tree"] is not None and not hex_value(receipt["tree"], 40)): raise ValueError("artifact-source-identity")
    for key in ("sourceUnchanged", "dependenciesUnchanged", "compiledBinaryUnchanged"):
        if type(receipt[key]) is not bool: raise ValueError("artifact-integrity-type")
    if (receipt["limits"] != LIMITS or any(type(v) is not int for v in receipt["limits"].values()) or
            receipt["pendingIsolation"] != PENDING or receipt["coverageLimit"] != COVERAGE): raise ValueError("artifact-fixed-limits")
    for key in ("startedUTC", "endedUTC"):
        if type(receipt[key]) is not str or not re.fullmatch(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,6})?\+00:00", receipt[key]): raise ValueError("artifact-time")
    if "failure" in receipt:
        failure = receipt["failure"]
        if type(failure) is not dict or set(failure) != {"stage", "class"} or failure["stage"] not in FAILURE_STAGES or failure["class"] not in FAILURE_CLASSES:
            raise ValueError("artifact-failure")
    for key in ("sourceBefore", "sourceAfter"):
        if receipt[key] is not None and not digest(receipt[key]): raise ValueError("artifact-source-digest")
    if receipt["inspectorSHA256"] is not None and not hex_value(receipt["inspectorSHA256"], 64): raise ValueError("artifact-inspector")
    expected = {"compile": (90, 95), "publicControls": (60, 65)}
    if suite != "restore-controls": expected.update(collection=(15, 20), browser=(230, 235))
    phases, metadata = receipt["phases"], receipt["metadataPhases"]
    if (type(phases) is not dict or set(phases) - set(expected) or any(not phase_shape(v, *expected[k]) for k, v in phases.items()) or
            type(metadata) is not list or len(metadata) > 10 or any(not phase_shape(v, 5, 10) for v in metadata)): raise ValueError("artifact-phases")
    for side in ("Before", "After"):
        probes, state, binary = (receipt[k + side] for k in ("dependencyPhases", "dependencies", "compiledBinary"))
        if (type(probes) is not dict or set(probes) - (set() if suite == "restore-controls" else {"packageResolution", "browserResolution"}) or
                any(not phase_shape(v, 5, 10) for v in probes.values()) or
                (state is not None and not valid_dependencies(state, probes, suite)) or
                (binary is not None and (not file_pin(binary, True) or binary["bytes"] > 134217728))): raise ValueError("artifact-dependencies")
    full_source = source_shape(source, receipt)
    if type(results) is not dict or set(results) != {"schemaVersion", "controls", "collection", "browser"} or type(results["schemaVersion"]) is not int or results["schemaVersion"] != 1:
        raise ValueError("artifact-results")
    if results["controls"] is not None: control_shape(results["controls"])
    if suite == "restore-controls":
        if results["collection"] is not None or results["browser"] is not None: raise ValueError("artifact-controls-mode")
    else:
        for key, mode in (("collection", "collection"), ("browser", "runtime")):
            if results[key] is not None: admit(results[key], mode, suite)
    if classification == "prerequisite-blocked": return
    if "failure" in receipt: raise ValueError("artifact-failed-positive")
    if not hex_value(receipt["inspectorSHA256"], 64): raise ValueError("artifact-accepted-inspector")
    if (not full_source or any(receipt[k] is not True for k in ("sourceUnchanged", "dependenciesUnchanged", "compiledBinaryUnchanged")) or
            receipt["sourceAfter"] != receipt["sourceBefore"] or receipt["compiledBinaryBefore"] is None or
            receipt["compiledBinaryBefore"] != receipt["compiledBinaryAfter"] or
            not same_state(receipt["dependenciesBefore"], receipt["dependenciesAfter"], suite)): raise ValueError("artifact-integrity")
    if results["controls"] is None or results["controls"]["green"] is not True: raise ValueError("artifact-controls")
    if len(metadata) != 10 or any(not valid_phase(v, 5, 10) for v in metadata): raise ValueError("artifact-metadata-settlement")
    if set(phases) != set(expected) or any(not valid_phase(v, *expected[k], (0, 1) if k == "browser" else (0,)) for k, v in phases.items()):
        raise ValueError("artifact-phase-settlement")
    if suite == "restore-controls":
        if classification != "focused-restore-public-controls-green": raise ValueError("artifact-controls-classification")
    else:
        admit(results["collection"], "collection", suite)
        browser = admit(results["browser"], "runtime", suite)
        if classify(browser, receipt["inspectorSHA256"]) != classification or phases["browser"]["exitCode"] != (0 if classification == "focused-restore-browser-green" else 1):
            raise ValueError("artifact-browser-classification")


def seal(values):
    if type(values) is not dict or set(values) != set(NAMES[:-1]): raise ValueError("artifact-names")
    try: bodies = {name: (json.dumps(value, sort_keys=True, indent=2, allow_nan=False) + "\n").encode() for name, value in values.items()}
    except (TypeError, ValueError, RecursionError) as error: raise ValueError("artifact-json") from error
    if any(not body or len(body) > CAP for body in bodies.values()): raise ValueError("artifact-bound")
    bindings = {name: {"bytes": len(body), "sha256": pin(body)["sha256"]} for name, body in bodies.items()}
    bodies[NAMES[-1]] = (json.dumps(bindings, sort_keys=True, indent=2) + "\n").encode()
    return bodies


def admit_bundle(bodies, revision, tree):
    if type(bodies) is not dict or set(bodies) != set(NAMES): raise ValueError("artifact-names")
    values = {name: strict_json(body, CAP) for name, body in bodies.items()}
    bindings = values[NAMES[-1]]
    if type(bindings) is not dict or set(bindings) != set(NAMES[:-1]): raise ValueError("artifact-bindings")
    for name in NAMES[:-1]:
        expected = {"bytes": len(bodies[name]), "sha256": pin(bodies[name])["sha256"]}
        if bindings[name] != expected or not digest(bindings[name]): raise ValueError("artifact-binding")
    receipt, source = values["receipt.json"], values["source-manifest.json"]
    if (type(receipt) is not dict or type(source) is not dict or
            any(value.get("revision") != revision or value.get("tree") != tree for value in (receipt, source)) or
            receipt.get("id") != "R06" or receipt.get("mode") != "restore-browser"): raise ValueError("artifact-identity")
    try: validate_acceptance(receipt, values["results.json"], source)
    except (KeyError, TypeError, AttributeError) as error: raise ValueError("artifact-evidence") from error
    return values

def fresh_output(root):
    base = Path(root)
    for part in (".verification", "campaign-proof"):
        base /= part
        if base.exists():
            checked_parents(base)
            info = os.lstat(base)
            if not stat.S_ISDIR(info.st_mode) or stat.S_ISLNK(info.st_mode): raise ValueError("output-parent")
        else: base.mkdir(mode=0o700)
    output = base / "R06"
    checked_parents(output)
    output.mkdir(mode=0o700)
    return output


def write_bundle(output, bodies):
    if set(bodies) != set(NAMES): raise ValueError("artifact-names")
    for name in NAMES:
        body = bodies[name]
        if type(body) is not bytes or not 0 < len(body) <= CAP: raise ValueError("artifact-bound")
    for name in NAMES:
        path = Path(output) / name
        checked_parents(path)
        descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW | os.O_CLOEXEC, 0o600)
        error = None
        try:
            offset = 0
            while offset < len(bodies[name]):
                written = os.write(descriptor, bodies[name][offset:])
                if written <= 0: raise ValueError("artifact-write")
                offset += written
            os.fsync(descriptor)
        except BaseException as failure: error = failure
        try: os.close(descriptor)
        except OSError: error = ValueError("artifact-close")
        if error is not None: raise error
