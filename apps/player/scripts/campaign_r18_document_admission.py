"""Admit only exact completed R18 public records; no browser feature acceptance."""
import re

from campaign_r18_document_events import CALL_LINES, NAMES, SOURCE

DATA_FIELDS = {"schemaValid", "started", "closed", "packageTerminal", "records",
               "events", "captureBytes", "captureSHA256"}
RECORD_FIELDS = {"test", "occurrence", "status", "absenceHTTP", "errorFrames", "sourceLocation"}
IDENTITY_FIELDS = {"sourceUnchanged", "toolchainUnchanged", "binaryUnchanged",
                   "exactInvocation", "formattingVerified", "handoffVerified"}


def settled(execution, codes):
    return (isinstance(execution, dict) and type(execution.get("exitCode")) is int
            and execution["exitCode"] in codes and execution.get("stopReason") is None
            and all(execution.get(key) is True for key in
                    ("ownedProcessExited", "ownedGroupSettled", "captureSettled")))


def blocked():
    return {"classification": "prerequisite-blocked", "count": 0, "nativePassed": 0,
            "absentClaimRecords": 0, "incumbentDefectRED": False, "r18FeatureAccepted": False}


def valid_record(row):
    if not isinstance(row, dict) or set(row) != RECORD_FIELDS or row["test"] not in NAMES:
        return False
    if type(row["occurrence"]) is not int or row["occurrence"] not in (1, 2):
        return False
    if row["status"] not in ("pass", "fail") or type(row["errorFrames"]) is not int:
        return False
    if not 0 <= row["errorFrames"] <= 10000:
        return False
    if row["absenceHTTP"] is None:
        return row["sourceLocation"] is None
    return (type(row["absenceHTTP"]) is int and row["absenceHTTP"] in (404, 405)
            and row["test"] in CALL_LINES and row["sourceLocation"] == SOURCE + ":" + str(CALL_LINES[row["test"]]))


def valid_data(data):
    if not isinstance(data, dict) or set(data) != DATA_FIELDS:
        return False
    if any(data[key] is not True for key in ("schemaValid", "started", "closed")):
        return False
    if data["packageTerminal"] not in ("pass", "fail") or not isinstance(data["records"], list):
        return False
    if type(data["events"]) is not int or not 18 <= data["events"] <= 10000:
        return False
    if type(data["captureBytes"]) is not int or not 0 < data["captureBytes"] <= 16 * 1024 * 1024:
        return False
    if not isinstance(data["captureSHA256"], str) or not re.fullmatch(r"[a-f0-9]{64}", data["captureSHA256"]):
        return False
    rows = data["records"]
    if len(rows) != 8 or not all(valid_record(row) for row in rows):
        return False
    expected = {(name, repeat) for name in NAMES for repeat in (1, 2)}
    return {(row["test"], row["occurrence"]) for row in rows} == expected


def admit(data, execution, identity, mode):
    result = blocked()
    if mode not in ("baseline", "candidate") or not valid_data(data):
        return result
    if not isinstance(identity, dict) or set(identity) != IDENTITY_FIELDS:
        return result
    if any(identity[field] is not True for field in IDENTITY_FIELDS) or not settled(execution, (0, 1)):
        return result
    rows = data["records"]
    passed = lambda row: row["status"] == "pass" and row["errorFrames"] == 0 and row["absenceHTTP"] is None
    native = [row for row in rows if row["test"] == NAMES[-1]]
    if not all(passed(row) for row in native):
        return result
    if mode == "baseline":
        claims = [row for row in rows if row["test"] != NAMES[-1]]
        if data["packageTerminal"] != "fail" or execution["exitCode"] != 1:
            return result
        if not all(row["status"] == "fail" and row["errorFrames"] == 1 and row["absenceHTTP"] in (404, 405) for row in claims):
            return result
        result.update(classification="route-prerequisite-absence", absentClaimRecords=6)
    else:
        if data["packageTerminal"] != "pass" or execution["exitCode"] != 0 or not all(passed(row) for row in rows):
            return result
        result["classification"] = "public-document-contract-green"
    result.update(count=8, nativePassed=2)
    return result
