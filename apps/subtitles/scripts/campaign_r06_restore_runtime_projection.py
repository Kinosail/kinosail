"""Independent closed admission of the fixed Restore product assertions."""
import math
import re

SCHEMA = "r06-restore-browser-v1"
FILE = "apps/subtitles/e2e/subtitle-restore-recovery.journey.ts"
CASES = {
    "r06-restore-headers-desktop": "R06 Restore held headers releases desktop editor",
    "r06-restore-headers-phone": "R06 Restore held headers releases phone editor",
    "r06-restore-inspect-body-desktop": "R06 Restore held inspection body releases desktop editor",
    "r06-restore-inspect-body-phone": "R06 Restore held inspection body releases phone editor",
}
SELECTED = {"restore-controls": (),
            "restore-headers": ("r06-restore-headers-desktop", "r06-restore-headers-phone"),
            "restore-inspect-body": ("r06-restore-inspect-body-desktop", "r06-restore-inspect-body-phone")}
ASSERTIONS = ("actual-restore", "history-once", "recovery-swapped", "hold-proven",
              "editor-released-by-45s", "finite-truthful-outcome", "original-correction-kept",
              "original-import-kept", "library-navigation", "back-current-subtitle", "no-restore-replay", "fixture-settled")
COUNTERS = ("setupSaveAttempts", "restoreAttempts", "prepareAttempts", "responseStatus",
            "inspectionResponseStatus", "activeHolds", "browserCompletedReceiptReads", "browserRestoredInspections")
FLAGS = ("actualRestored", "historyOnce", "recoverySwapped", "inspectionMatches", "receiptCompleted",
         "receiptSucceeded", "holdEligible", "headersReleased", "bodyReleased", "restoreResponseDelivered",
         "inspectionResponseDelivered", "restoreClientCancelled", "inspectionClientCancelled", "holdExpired", "boundaryFailed")
EXTRA_FLAGS = ("eligible", "fixtureStopped", "restoreRequestObserved", "restoreResponseObserved", "inspectRequestObserved",
               "inspectResponseObserved", "restoreBodyDelivered", "inspectionBodyDelivered", "releaseAttempted")
TIMINGS = ("clickToWitnessMs", "clickToUnlockMs", "holdDurationMs", "durationMs")
FIELDS = ("schema", "kind", "caseID", "stage", "failureStage", "protocol", *COUNTERS, *FLAGS, *EXTRA_FLAGS,
          "restoreTerminal", "inspectTerminal", "restoreFailureCode", "inspectFailureCode", *TIMINGS, "servedScriptSHA256", "assertions", "causalDiagnostic")
ROOT_FIELDS = ("schema", "kind", "collection", "cases", "malformedRecords", "duplicateTerminals", "globalErrorCount", "runnerStatus")
CASE_FIELDS = ("caseID", "outcome", "data", "failedAssertions", "unattemptedAssertions", "incompleteAssertions",
               "retry", "totalErrorCount", "knownAssertionErrorIDs", "unknownErrorCount", "assertionErrorsExact")
STAGES = ("setup", "witness", "deadline", "release", "terminal", "navigation", "cleanup")


def exact(value, fields):
    return type(value) is dict and set(value) == set(fields)


def valid_data(value, case):
    if (not exact(value, FIELDS) or value["schema"] != SCHEMA or value["kind"] != "runtime-case" or
            value["caseID"] != case or value["stage"] not in STAGES or
            value["failureStage"] is not None and value["failureStage"] not in STAGES or
            value["protocol"] not in ("unreached", "legacy", "prepared")): return False
    if any(type(value[key]) is not bool for key in (*FLAGS, *EXTRA_FLAGS)): return False
    if any(type(value[key]) is not int or not 0 <= value[key] <= 1000 for key in COUNTERS): return False
    if value["responseStatus"] > 599 or value["inspectionResponseStatus"] > 599: return False
    for key in ("restoreTerminal", "inspectTerminal"):
        if value[key] not in ("unreached", "pending", "finished", "request-failed"): return False
    for prefix in ("restore", "inspect"):
        code = value[prefix + "FailureCode"]
        if (type(code) is not str or code not in ("none", "aborted", "content-length", "decoding", "connection-reset",
                "connection-closed", "empty-response", "unclassified") or
                (value[prefix + "Terminal"] == "request-failed") != (code != "none")): return False
    for key in TIMINGS:
        number = value[key]
        if number is None and key != "durationMs": continue
        if type(number) not in (int, float) or not math.isfinite(number) or not 0 <= number <= 125000: return False
    scripts = value["servedScriptSHA256"]
    if not exact(scripts, ("inspector",)): return False
    digest = scripts["inspector"]
    if digest is not None and (type(digest) is not str or not re.fullmatch("[a-f0-9]{64}", digest)): return False
    if not valid_causal(value["causalDiagnostic"]): return False
    if not exact(value["assertions"], ASSERTIONS): return False
    for item in value["assertions"].values():
        if not exact(item, ("attempted", "completed", "passed")): return False
        if type(item["attempted"]) is not bool or type(item["completed"]) is not bool: return False
        if item["completed"]:
            if not item["attempted"] or type(item["passed"]) is not bool: return False
        elif item["passed"] is not None: return False
    return True



def valid_native(value):
    return (exact(value, ("outcome", "status", "signalPresent", "signalAborted", "pagehide")) and
            type(value["outcome"]) is str and value["outcome"] in ("unreached", "pending", "fulfilled", "rejected") and
            type(value["status"]) is int and 0 <= value["status"] <= 599 and
            all(type(value[key]) is bool for key in ("signalPresent", "signalAborted", "pagehide")))


def valid_network(value):
    return (exact(value, ("terminal", "failureCode", "resourceType", "cancelled", "navigation")) and
            type(value["terminal"]) is str and value["terminal"] in ("unreached", "pending", "finished", "request-failed") and
            type(value["failureCode"]) is str and value["failureCode"] in ("none", "aborted", "content-length", "decoding",
                "connection-reset", "connection-closed", "empty-response", "unclassified") and
            (value["terminal"] == "request-failed") == (value["failureCode"] != "none") and
            type(value["resourceType"]) is str and value["resourceType"] in ("unreached", "Fetch", "XHR", "Document", "Other") and
            type(value["cancelled"]) is bool and type(value["navigation"]) is bool)


def valid_server(value):
    return exact(value, ("seen", "held", "delivered", "cancelled", "timedOut", "settled")) and all(type(v) is bool for v in value.values())


def valid_causal(value):
    if value is None: return True
    fields = ("schema", "valid", "reason", "headersEqual", "framingEqual", "stopped", "probes", "restoreNative", "restoreNetwork")
    if type(value) is not dict: return False
    if value.get("schema") == "r06-causal-v2":
        if not exact(value, (*fields, "completion")) or not valid_completion(value["completion"]): return False
    elif value.get("schema") != "r06-causal-v1" or not exact(value, fields): return False
    if (type(value["reason"]) is not str or
            value["reason"] not in ("none", "timeout", "overflow", "mismatch", "control", "unreached") or
            any(type(value[key]) is not bool for key in ("valid", "headersEqual", "framingEqual", "stopped")) or
            type(value["probes"]) is not list or len(value["probes"]) != 3 or
            not valid_native(value["restoreNative"]) or not valid_network(value["restoreNetwork"])): return False
    for row, mode in zip(value["probes"], ("direct", "captured", "held"), strict=True):
        if (not exact(row, ("mode", "native", "network", "server")) or row["mode"] != mode or
                not valid_native(row["native"]) or not valid_network(row["network"]) or not valid_server(row["server"])): return False
    return True


def valid_completion(value):
    return (exact(value, ("scope", "caseID", "browserVersion", "pinnedBrowserMatched", "requestMatched", "responseMatched", "bodyless", "fixtureMatched")) and
            value["scope"] == "r06-native-bodyless-204-v1" and type(value["caseID"]) is str and value["caseID"] in CASES and value["browserVersion"] == "153.0.8010.12" and
            all(type(value[k]) is bool for k in ("pinnedBrowserMatched", "requestMatched", "responseMatched", "bodyless", "fixtureMatched")))


def causal_ready(value):
    # Diagnostic readiness alone grants no completion authority; v2 adds exact binding.
    if (value is None or not valid_causal(value) or not value["valid"] or value["reason"] != "none" or
            not all(value[key] for key in ("headersEqual", "framingEqual", "stopped"))): return False
    native, network = value["restoreNative"], value["restoreNetwork"]
    if (native["outcome"] not in ("fulfilled", "rejected") or network["terminal"] not in ("finished", "request-failed") or
            network["resourceType"] not in ("Fetch", "XHR") or native["signalAborted"] and not native["signalPresent"] or
            native["outcome"] == "fulfilled" and not 100 <= native["status"] <= 599 or
            native["outcome"] == "rejected" and native["status"] != 0): return False
    for row in value["probes"]:
        n, c, s = row["native"], row["network"], row["server"]
        if n["pagehide"] or c["navigation"] or c["resourceType"] not in ("Fetch", "XHR") or not s["seen"] or not s["settled"] or s["timedOut"]: return False
        if row["mode"] == "held":
            if (n["outcome"] != "rejected" or not n["signalPresent"] or not n["signalAborted"] or
                    c["terminal"] != "request-failed" or c["failureCode"] != "aborted" or not c["cancelled"] or
                    not s["held"] or not s["cancelled"] or s["delivered"]): return False
        elif (n["outcome"] != "fulfilled" or n["status"] != 204 or n["signalPresent"] or n["signalAborted"] or
                c["terminal"] not in ("finished", "request-failed") or not s["delivered"] or s["cancelled"]): return False
    return True


def errors_exact(row):
    data = row["data"]
    expected = []
    if not data["eligible"]: expected.append("eligible-real-restore-history-transport")
    for key in ASSERTIONS:
        item = data["assertions"][key]
        if not item["attempted"]: expected.append(key + "-attempted")
        if not item["completed"]: expected.append(key + "-completed")
        if item["passed"] is not True: expected.append(key)
    if not data["fixtureStopped"]: expected.append("owned-fixture-stopped")
    return (type(row["retry"]) is int and row["retry"] == 0 and row["unknownErrorCount"] == 0 and
            row["assertionErrorsExact"] is True and sorted(row["knownAssertionErrorIDs"]) == sorted(expected) and
            row["totalErrorCount"] == len(expected))


def admit(value, mode, suite):
    if suite not in SELECTED or mode not in ("collection", "runtime") or not exact(value, ROOT_FIELDS):
        raise ValueError("projection-shape")
    if value["schema"] != SCHEMA or value["kind"] != mode or value["runnerStatus"] not in ("passed", "failed"):
        raise ValueError("projection-identity")
    if any(type(value[key]) is not int or value[key] != 0 for key in ("malformedRecords", "duplicateTerminals", "globalErrorCount")):
        raise ValueError("projection-events")
    selected = sorted(CASES if mode == "collection" else SELECTED[suite])
    collection = [{"caseID": key, "file": FILE, "title": CASES[key]} for key in selected]
    if value["collection"] != collection: raise ValueError("collection-identity")
    rows = value["cases"]
    if type(rows) is not list or len(rows) != (0 if mode == "collection" else len(selected)):
        raise ValueError("projection-count")
    if mode == "collection":
        if value["runnerStatus"] != "passed": raise ValueError("collection-terminal")
        return value
    known = {"eligible-real-restore-history-transport", "owned-fixture-stopped", *ASSERTIONS,
             *(key + "-attempted" for key in ASSERTIONS), *(key + "-completed" for key in ASSERTIONS)}
    for row, case in zip(rows, selected, strict=True):
        if (not exact(row, CASE_FIELDS) or row["caseID"] != case or
                row["outcome"] not in ("passed", "failed", "incomplete") or not valid_data(row["data"], case)):
            raise ValueError("projection-case")
        if (any(type(row[key]) is not int or not 0 <= row[key] <= 64 for key in ("retry", "totalErrorCount", "unknownErrorCount")) or
                type(row["assertionErrorsExact"]) is not bool): raise ValueError("projection-error-shape")
        errors = row["knownAssertionErrorIDs"]
        if type(errors) is not list or len(errors) > 64 or any(type(key) is not str or key not in known for key in errors):
            raise ValueError("projection-errors")
        assertions = row["data"]["assertions"]
        failed = [key for key in ASSERTIONS if assertions[key]["completed"] and assertions[key]["passed"] is False]
        unattempted = [key for key in ASSERTIONS if not assertions[key]["attempted"]]
        incomplete = [key for key in ASSERTIONS if not assertions[key]["completed"]]
        if row["failedAssertions"] != failed or row["unattemptedAssertions"] != unattempted or row["incompleteAssertions"] != incomplete:
            raise ValueError("projection-summary")
    return value


def terminal(data, body):
    key = "inspect" if body else "restore"
    completed = (data[key + "Terminal"] == "finished" and data[key + "ResponseObserved"] and
                 data["inspectionBodyDelivered" if body else "restoreBodyDelivered"] and
                 data["inspectionResponseDelivered" if body else "restoreResponseDelivered"])
    cancelled = (data[key + "Terminal"] == "request-failed" and
                 data["inspectionClientCancelled" if body else "restoreClientCancelled"] and
                 not data["inspectionBodyDelivered" if body else "restoreBodyDelivered"] and
                 not data["inspectionResponseDelivered" if body else "restoreResponseDelivered"])
    native = not body and data["restoreBodyDelivered"] and native_bodyless_completion(data)
    return data[key + "RequestObserved"] and (completed or cancelled or native)


def native_bodyless_completion(data):
    d = data.get("causalDiagnostic")
    if not valid_causal(d) or d is None or d["schema"] != "r06-causal-v2" or not causal_ready(d): return False
    c = d["completion"]
    if (any(type(data[k]) is not int for k in ("responseStatus", "prepareAttempts", "setupSaveAttempts", "restoreAttempts")) or
            c["caseID"] != data["caseID"] or not all(c[k] for k in ("pinnedBrowserMatched", "requestMatched", "responseMatched", "bodyless", "fixtureMatched")) or
            data["protocol"] != "legacy" or data["responseStatus"] != 204 or data["prepareAttempts"] != 0 or data["setupSaveAttempts"] != data["restoreAttempts"] or data["restoreAttempts"] != 1 or
            not all(data[k] is True for k in ("restoreRequestObserved", "restoreResponseObserved", "restoreResponseDelivered", "eligible", "actualRestored", "historyOnce", "recoverySwapped", "inspectionMatches", "holdEligible")) or
            not all(data[k] is False for k in ("restoreClientCancelled", "holdExpired", "boundaryFailed")) or data["restoreTerminal"] != "request-failed" or data["restoreFailureCode"] != "aborted"): return False
    def fulfilled(n):
        return n == dict(outcome="fulfilled", status=204, signalPresent=False, signalAborted=False, pagehide=False)
    signature = dict(terminal="request-failed", failureCode="aborted", resourceType="Fetch", cancelled=True, navigation=False)
    if not fulfilled(d["restoreNative"]) or d["restoreNetwork"] != signature: return False
    held = d["probes"][2]
    return (held["native"] == dict(outcome="rejected", status=0, signalPresent=True, signalAborted=True, pagehide=False) and held["network"] == signature and
            all(fulfilled(row["native"]) and row["network"] == signature and row["server"]["held"] is False for row in d["probes"][:2]))


def classify(value, inspector_sha):
    outcomes = []
    try:
        for row in value["cases"]:
            data, assertions = row["data"], row["data"]["assertions"]
            body = row["caseID"].startswith("r06-restore-inspect-body-")
            protocol = (data["protocol"] == "legacy" and data["responseStatus"] == 204 and data["prepareAttempts"] == 0 or
                        data["protocol"] == "prepared" and data["responseStatus"] == 202 and data["prepareAttempts"] == 1
                        and data["receiptCompleted"] and data["receiptSucceeded"])
            mandatory = set(ASSERTIONS) - {"editor-released-by-45s", "finite-truthful-outcome"}
            ready = (errors_exact(row) and data["eligible"] and data["fixtureStopped"] and data["failureStage"] is None
                     and data["stage"] == "cleanup" and data["actualRestored"] and data["historyOnce"] and data["recoverySwapped"]
                     and data["inspectionMatches"] and data["holdEligible"] and not data["holdExpired"] and not data["boundaryFailed"]
                     and data["activeHolds"] == 0 and data["setupSaveAttempts"] == data["restoreAttempts"] == 1 and protocol
                     and data["releaseAttempted"] and data["servedScriptSHA256"]["inspector"] == inspector_sha
                     and data["clickToWitnessMs"] is not None and data["clickToWitnessMs"] <= 10000
                     and data["holdDurationMs"] is not None and terminal(data, body)
                     and (not body or (data["restoreTerminal"] == "finished" or native_bodyless_completion(data)) and data["restoreResponseObserved"]
                          and data["restoreResponseDelivered"] and data["inspectionResponseStatus"] == 200)
                     and all(assertions[key] == {"attempted": True, "completed": True, "passed": True} for key in mandatory))
            deadline = assertions["editor-released-by-45s"]
            consistent = (deadline["attempted"] and deadline["completed"] and deadline["passed"] ==
                          (data["clickToUnlockMs"] is not None and data["clickToUnlockMs"] <= 45000))
            completed = all(item["attempted"] and item["completed"] for item in assertions.values())
            green = (ready and consistent and completed and row["outcome"] == "passed" and
                     not row["failedAssertions"] and not row["unattemptedAssertions"] and not row["incompleteAssertions"])
            red = (ready and consistent and completed and row["outcome"] == "failed" and deadline["passed"] is False and
                   set(row["failedAssertions"]) <= {"editor-released-by-45s", "finite-truthful-outcome"} and
                   not row["unattemptedAssertions"] and not row["incompleteAssertions"])
            outcomes.append("green" if green else "red" if red else "blocked")
        if outcomes and all(item == "green" for item in outcomes) and value["runnerStatus"] == "passed":
            return "focused-restore-browser-green"
        if outcomes and all(item == "red" for item in outcomes) and value["runnerStatus"] == "failed":
            return "confirmed-restore-deadline-red"
    except (KeyError, TypeError, ValueError): pass
    return "prerequisite-blocked"
