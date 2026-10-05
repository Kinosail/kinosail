"""Strict typed projections; no raw error, response, URL, cookie or runtime state."""
import math

SCHEMA = "r06-save-browser-v1"
FILE = "apps/subtitles/e2e/subtitle-save-recovery.journey.ts"
CASES = {
    "save-headers-phone": "R06 Save headers held after completed write - phone",
    "save-headers-desktop": "R06 Save headers held after completed write - desktop",
    "save-body-phone": "R06 Save body held after completed write - phone",
    "save-body-desktop": "R06 Save body held after completed write - desktop",
}
STAGES = ("auth", "preview", "witness", "deadline", "new-edit", "late-response", "navigation", "settlement", "release-budget-exhausted")
ASSERTIONS = (
    "actual-save-completed", "actual-history-once", "actual-recovery-retained", "response-hold-eligible",
    "editing-unlocked-by-45s", "finite-truthful-status", "original-retained-if-uncertain", "newer-edit-accepted",
    "newer-edit-survives-old-response", "newer-status-survives-old-response", "library-navigation-exercised",
    "browser-back-exercised", "no-save-replay", "current-server-state", "fixture-settled",
)
BOOLEANS = ("eligible", "fixtureStopped", "actualSaved", "historyOnce", "recoveryMatches",
            "inspectionMatches", "receiptCompleted", "receiptSucceeded", "holdEligible", "holdExpired",
            "saveRequestObserved", "saveResponseObserved", "releaseAttempted", "responseBodyDelivered", "fixtureClientCancelled")
COUNTERS = ("responseStatus", "saveAttempts", "prepareAttempts", "browserCompletedReads",
            "browserSavedInspections", "activeHolds")
TIMINGS = ("saveClickToWitnessMs", "saveClickToUnlockMs", "holdDurationMs", "durationMs")
FIELDS = ("schema", "kind", "caseID", "stage", "failureStage", "protocol", "servedScriptSHA256",
          "assertions", "saveTerminal", *BOOLEANS, *COUNTERS, *TIMINGS)
ROOT_FIELDS = ("schema", "kind", "collection", "cases", "malformedRecords", "duplicateTerminals",
               "globalErrorCount", "runnerStatus")


def exact(value, keys):
    return type(value) is dict and set(value) == set(keys)


def number(value, limit, nullable=False):
    return nullable and value is None or type(value) in (int, float) and math.isfinite(value) and 0 <= value <= limit


def valid_case(value, case):
    if not exact(value, FIELDS) or value["schema"] != SCHEMA or value["kind"] != "runtime-case":
        return False
    if value["caseID"] != case or value["stage"] not in STAGES or (
            value["failureStage"] is not None and value["failureStage"] not in STAGES):
        return False
    if value["saveTerminal"] not in ("unreached", "pending", "finished", "request-failed"):
        return False
    if value["protocol"] not in ("unreached", "legacy", "prepared"):
        return False
    if any(type(value[key]) is not bool for key in BOOLEANS):
        return False
    if any(type(value[key]) is not int or not 0 <= value[key] <= 1000 for key in COUNTERS):
        return False
    if value["responseStatus"] > 599 or any(not number(value[key], 90000, key != "durationMs") for key in TIMINGS):
        return False
    scripts = value["servedScriptSHA256"]
    if not exact(scripts, ("inspector",)):
        return False
    digest = scripts["inspector"]
    if digest is not None and (type(digest) is not str or len(digest) != 64 or any(c not in "0123456789abcdef" for c in digest)):
        return False
    if not exact(value["assertions"], ASSERTIONS):
        return False
    return all(exact(a, ("attempted", "passed")) and type(a["attempted"]) is bool and
               (type(a["passed"]) is bool if a["attempted"] else a["passed"] is None)
               for a in value["assertions"].values())


def admit_projection(value, mode, selected):
    if mode not in ("collection", "runtime") or not exact(value, ROOT_FIELDS):
        raise ValueError("projection-shape")
    if value["schema"] != SCHEMA or value["kind"] != mode or value["runnerStatus"] not in ("passed", "failed"):
        raise ValueError("projection-identity")
    if any(type(value[key]) is not int or value[key] != 0
           for key in ("malformedRecords", "duplicateTerminals", "globalErrorCount")):
        raise ValueError("projection-events")
    expected = sorted(CASES if mode == "collection" else selected)
    records = value["collection"]
    if type(records) is not list or len(records) != len(expected):
        raise ValueError("collection-shape")
    for row, case in zip(records, expected, strict=True):
        if not exact(row, ("caseID", "file", "title")) or row != {"caseID":case,"file":FILE,"title":CASES[case]}:
            raise ValueError("collection-identity")
    if type(value["cases"]) is not list or len(value["cases"]) != (0 if mode == "collection" else len(expected)):
        raise ValueError("case-count")
    if mode == "collection":
        if value["runnerStatus"] != "passed":
            raise ValueError("collection-terminal")
        return value
    for row, case in zip(value["cases"], expected, strict=True):
        if not exact(row, ("caseID", "outcome", "data", "failedAssertions", "unreachableAssertions",
                           "retry", "totalErrorCount", "knownAssertionErrorIDs", "unknownErrorCount", "assertionErrorsExact")):
            raise ValueError("case-shape")
        if row["caseID"] != case or row["outcome"] not in ("passed", "failed", "incomplete") or not valid_case(row["data"], case):
            raise ValueError("case-identity")
        if any(type(row[key]) is not int or not 0 <= row[key] <= 32
               for key in ("retry", "totalErrorCount", "unknownErrorCount")) or type(row["assertionErrorsExact"]) is not bool:
            raise ValueError("case-error-shape")
        ids = row["knownAssertionErrorIDs"]
        known = ("eligible-real-save-history-transport", "owned-fixture-stopped", *ASSERTIONS,
                 *(key+"-attempted" for key in ASSERTIONS))
        if type(ids) is not list or len(ids) > 32 or any(type(key) is not str or key not in known for key in ids):
            raise ValueError("case-error-identity")
        assertions = row["data"]["assertions"]
        failed = [key for key in ASSERTIONS if assertions[key]["attempted"] and assertions[key]["passed"] is False]
        unreached = [key for key in ASSERTIONS if not assertions[key]["attempted"]]
        if row["failedAssertions"] != failed or row["unreachableAssertions"] != unreached:
            raise ValueError("case-summary")
    return value


def errors_exact(row):
    data = row["data"]
    expected = []
    if not data["eligible"]: expected.append("eligible-real-save-history-transport")
    if not data["fixtureStopped"]: expected.append("owned-fixture-stopped")
    for key, assertion in data["assertions"].items():
        if not assertion["attempted"]: expected.append(key+"-attempted")
        if assertion["passed"] is not True: expected.append(key)
    return (row["retry"] == 0 and row["unknownErrorCount"] == 0 and row["assertionErrorsExact"]
            and sorted(row["knownAssertionErrorIDs"]) == sorted(expected)
            and row["totalErrorCount"] == len(expected))


def classify(value, inspector_sha):
    outcomes = []
    for row in value["cases"]:
        data, assertions = row["data"], row["data"]["assertions"]
        mandatory = ASSERTIONS[:4] + ("original-retained-if-uncertain",) + ASSERTIONS[10:]
        network_settled = (data["saveRequestObserved"] and data["releaseAttempted"] and
                           (data["saveTerminal"] == "finished" and data["saveResponseObserved"] and data["responseBodyDelivered"]
                            or data["saveTerminal"] == "request-failed" and data["fixtureClientCancelled"] and not data["responseBodyDelivered"]))
        ready = (errors_exact(row) and network_settled and data["eligible"] and data["fixtureStopped"] and data["failureStage"] is None
                 and data["stage"] == "settlement" and data["actualSaved"] and data["historyOnce"]
                 and data["recoveryMatches"] and data["inspectionMatches"] and data["holdEligible"]
                 and not data["holdExpired"] and data["activeHolds"] == 0 and data["saveAttempts"] == 1
                 and data["servedScriptSHA256"]["inspector"] == inspector_sha
                 and data["saveClickToWitnessMs"] is not None and data["holdDurationMs"] is not None
                 and (data["protocol"] == "legacy" and data["responseStatus"] == 200 and data["prepareAttempts"] == 0
                      or data["protocol"] == "prepared" and data["responseStatus"] == 202
                      and data["prepareAttempts"] == 1 and data["receiptCompleted"] and data["receiptSucceeded"])
                 and all(assertions[key] == {"attempted":True,"passed":True} for key in mandatory))
        deadline = assertions["editing-unlocked-by-45s"]
        consistent = (deadline["attempted"] and deadline["passed"] ==
                      (data["saveClickToUnlockMs"] is not None and data["saveClickToUnlockMs"] <= 45000))
        green = ready and consistent and row["outcome"] == "passed" and not row["failedAssertions"] and not row["unreachableAssertions"]
        red = (ready and consistent and row["outcome"] == "failed" and deadline["passed"] is False
               and set(row["failedAssertions"]) <= {"editing-unlocked-by-45s", "finite-truthful-status"}
               and set(row["unreachableAssertions"]) <= set(ASSERTIONS[7:10]))
        outcomes.append("green" if green else "intended-deadline-red" if red else "prerequisite-blocked")
    if outcomes and all(outcome == "green" for outcome in outcomes) and value["runnerStatus"] == "passed":
        return "focused-save-browser-green"
    if outcomes and all(outcome == "intended-deadline-red" for outcome in outcomes) and value["runnerStatus"] == "failed":
        return "confirmed-save-deadline-red"
    return "prerequisite-blocked"
