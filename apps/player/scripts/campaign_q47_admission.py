#!/usr/bin/env python3
"""Strict Q47 private reporter admission. Unknown evidence is never product RED."""
import math
import re

IDS = ["control-status", "witness-status", "prior-handler-idle", "served-helper-equal", "builder-visible", "original-links-present", "template-request-observed", "result-visible", "preview-equal", "download-name-equal", "native-blob-digest-equal", "copy-status-equal", "native-copy-equal", "retry-keyboard-focused", "primary-one-request", "primary-one-hold", "primary-body-prefix", "primary-template-digest", "primary-pending-label", "primary-pending-disabled", "primary-no-output", "deadline-retry-label", "deadline-enabled", "deadline-error-visible", "deadline-retry-visible", "deadline-error-still-visible", "fallback-visible", "deadline-elapsed-at-least20s", "fallback-canonical", "primary-input-retention", "primary-peer-canceled", "primary-new-completed", "primary-two-requests", "primary-settled", "primary-privacy", "recovery-error-visible", "recovery-retry-visible", "recovery-input-retention", "recovery-fallback-canonical", "recovery-new-completed", "recovery-new-request", "recovery-prior-failed", "recovery-settled", "recovery-privacy", "supersession-one-hold", "supersession-no-active", "supersession-peer-canceled", "supersession-preview-current", "contract-one-completed", "contract-one-request", "contract-settled", "contract-privacy", "invalid-path-error", "invalid-path-no-request", "invalid-port-error", "invalid-port-no-request", "invalid-no-result", "invalid-no-download", "pending-confirmed-before-deadline", "absolute-clock-window-eligible", "recovery-observed-by-product-deadline"]
NAMES = {
    "primary": ["Q47 headers deadline phone Player", "Q47 body deadline desktop Both"],
    "recovery": ["Q47 lost request recovers with retained inputs", "Q47 failed HTTP recovers with retained inputs"],
    "supersession": ["Q47 input change discards late headers", "Q47 app change discards late body"],
    "contracts": ["Q47 Player preserves Compose download and copy", "Q47 Subtitles preserves Compose download and copy",
                  "Q47 Both preserves Compose download and copy", "Q47 rejects invalid inputs before template requests"],
}
ERRORS = ["suite_invalid", "project_invalid", "collection_bound", "collection_invalid", "case_bound",
          "case_invalid", "attachment_or_error_bound", "attachment_invalid", "assertions_invalid",
          "observation_invalid", "assertions_missing_or_bound", "runner_error", "terminal_invalid",
          "selection_invalid", "collection_case_emitted", "case_incomplete", "empty_or_inconsistent_green"]
STAGES = ["q47-asset", "q47-started", "q47-pending", "q47-deadline", "q47-failed",
          "q47-recovered", "q47-stale", "q47-rejected"]
BOOLS = ["createDisabled", "resultVisible", "errorVisible", "fallbackVisible", "fallbackCanonical",
         "downloadReady", "retainedApp", "retainedPath", "retainedPort", "retainedSecondPort"]
COUNTS = ["requests", "active", "holds", "canceled", "completed", "failures", "expired", "templateBytes"]
CHECKS = ["cookieSeen", "authorizationSeen", "bodySeen", "querySeen", "bodyPrefixSent"]
ACTIONS = ["make", "preparing", "retry", "unknown"]
CASE_KEYS = ["name", "status", "retry", "durationMs", "outcome", "failure", "ledger", "failedAssertions",
             "unattemptedAssertions", "incompleteAssertions", "totalErrorCount", "knownAssertionErrorIDs",
             "unknownErrorCount", "assertionErrorsExact", "deadlineDisposition", "observations"]


def exact(value, keys):
    return type(value) is dict and set(value) == set(keys)


def integer(value, maximum, minimum=0):
    return type(value) is int and minimum <= value <= maximum


def ms(value):
    return type(value) in (int, float) and math.isfinite(value) and 0 <= value <= 70_000


def digest(value):
    return type(value) is str and re.fullmatch("[0-9a-f]{64}", value) is not None


def vector(value, allowed, maximum, unique=False):
    return (type(value) is list and len(value) <= maximum and all(type(item) is str and item in allowed for item in value)
            and (not unique or len(set(value)) == len(value)))


def scope(title):
    common, ready = IDS[:6], IDS[7:13]
    if title in NAMES["primary"]:
        return common + IDS[6:7] + ready + (IDS[13:14] if title == NAMES["primary"][1] else []) + IDS[14:35] + IDS[58:]
    if title in NAMES["recovery"]:
        return common + IDS[6:7] + ready + IDS[35:44]
    if title in NAMES["supersession"]:
        return common + IDS[6:7] + ready + IDS[44:48]
    if title in NAMES["contracts"][:3]:
        return common + IDS[6:7] + ready + IDS[48:52]
    return common + IDS[52:58] if title == NAMES["contracts"][3] else []


def clock_valid(value):
    if not exact(value, ["elapsedMs", "trustedClick", "clicks", "pendingObserved", "firstEnabledMs", "firstRecoveryMs", "sample"]):
        return False
    if (type(value["trustedClick"]) is not bool or type(value["pendingObserved"]) is not bool
            or not integer(value["clicks"], 32)
            or any(item is not None and not ms(item) for item in (value["elapsedMs"], value["firstEnabledMs"], value["firstRecoveryMs"]))):
        return False
    elapsed, enabled, recovered, sample = (value[key] for key in ("elapsedMs", "firstEnabledMs", "firstRecoveryMs", "sample"))
    if elapsed is None:
        return value["clicks"] == 0 and not value["pendingObserved"] and enabled is None and recovered is None and sample is None
    if value["clicks"] == 0 or any(item is not None and item > elapsed for item in (enabled, recovered)):
        return False
    if recovered is not None and (enabled is None or not value["pendingObserved"] or recovered < enabled):
        return False
    return sample is None or (exact(sample, ["elapsedMs", "action", "createDisabled", "errorVisible"])
                              and ms(sample["elapsedMs"]) and sample["elapsedMs"] <= elapsed
                              and sample["action"] in ACTIONS and type(sample["action"]) is str
                              and type(sample["createDisabled"]) is bool and type(sample["errorVisible"]) is bool)


def ledger_valid(value, name):
    if not exact(value, ["assertions", "clock"]) or not exact(value["assertions"], IDS):
        return False
    if value["clock"] is not None and not clock_valid(value["clock"]):
        return False
    selected = scope(name)
    for key, entry in value["assertions"].items():
        if (not exact(entry, ["attempted", "completed", "passed", "attempts"])
                or type(entry["attempted"]) is not bool or type(entry["completed"]) is not bool
                or not (entry["passed"] is None or type(entry["passed"]) is bool) or not integer(entry["attempts"], 1024)):
            return False
        if (not entry["attempted"] and (entry["completed"] or entry["passed"] is not None or entry["attempts"] != 0)
                or entry["attempted"] and entry["attempts"] == 0 or entry["completed"] != (entry["passed"] is not None)
                or key not in selected and entry["attempted"]):
            return False
    return True


def observation_valid(record):
    if not exact(record, ["stage", "observation"]) or type(record["stage"]) is not str or record["stage"] not in STAGES:
        return False
    value = record["observation"]
    if record["stage"] == "q47-asset":
        return (exact(value, ["asset"]) and exact(value["asset"], ["bytes", "sha256", "sourceMatches"])
                and integer(value["asset"]["bytes"], 65_536, 1) and digest(value["asset"]["sha256"])
                and type(value["asset"]["sourceMatches"]) is bool)
    if not exact(value, ["state", "peer"]):
        return False
    state, peer = value["state"], value["peer"]
    if (not exact(state, BOOLS + ["action", "blobMatches", "copyMatches", "blobSHA256", "blobBytes", "elapsedMs", "responses"])
            or not all(type(state[key]) is bool for key in BOOLS) or type(state["action"]) is not str or state["action"] not in ACTIONS
            or not integer(state["elapsedMs"], 70_000) or not integer(state["responses"], 32)
            or not all(item is None or type(item) is bool for item in (state["blobMatches"], state["copyMatches"]))
            or not (state["blobSHA256"] is None or digest(state["blobSHA256"]))
            or not (state["blobBytes"] is None or integer(state["blobBytes"], 65_536, 32))):
        return False
    return (exact(peer, ["mode", "app", "templateSHA256", "counts", "checks"])
            and type(peer["mode"]) is str and peer["mode"] in ["valid", "headers", "body", "loss", "late_headers", "late_body", "http_error"]
            and type(peer["app"]) is str and peer["app"] in ["player", "subtitles", "both"] and digest(peer["templateSHA256"])
            and exact(peer["counts"], COUNTS) and exact(peer["checks"], CHECKS)
            and all(integer(peer["counts"][key], 16_384 if key == "templateBytes" else 32, 32 if key == "templateBytes" else 0) for key in COUNTS)
            and all(type(peer["checks"][key]) is bool for key in CHECKS))


def case_valid(value, suite):
    if not exact(value, CASE_KEYS) or type(value["name"]) is not str or value["name"] not in NAMES[suite]:
        return False
    if (value["status"] not in ["passed", "failed", "timedOut", "skipped", "interrupted"] or type(value["status"]) is not str
            or not integer(value["retry"], 3) or not integer(value["durationMs"], 70_000)
            or value["outcome"] not in ["passed", "failed", "incomplete"] or type(value["outcome"]) is not str
            or value["failure"] not in ["none", "known-assertion", "unclassified"] or type(value["failure"]) is not str
            or value["deadlineDisposition"] not in ["within-product-deadline", "observation-grace", "no-recovery-observed", "ineligible", "not-applicable"]
            or type(value["deadlineDisposition"]) is not str or type(value["assertionErrorsExact"]) is not bool
            or not integer(value["totalErrorCount"], 1024) or not integer(value["unknownErrorCount"], 1024)):
        return False
    if not all(vector(value[key], IDS, 61, key != "knownAssertionErrorIDs") for key in
               ("failedAssertions", "unattemptedAssertions", "incompleteAssertions", "knownAssertionErrorIDs")):
        return False
    records = value["observations"]
    return ((value["ledger"] is None or ledger_valid(value["ledger"], value["name"]))
            and type(records) is list and len(records) <= 8 and all(observation_valid(item) for item in records)
            and len({item["stage"] for item in records}) == len(records))


def disposition(clock):
    if (not clock or not clock["trustedClick"] or clock["clicks"] != 1 or not clock["pendingObserved"]
            or not clock["sample"] or not 20_000 <= clock["sample"]["elapsedMs"] <= 20_200):
        return "ineligible"
    enabled, recovered = clock["firstEnabledMs"], clock["firstRecoveryMs"]
    if enabled is None or recovered is None:
        return "no-recovery-observed"
    if enabled <= 20_000 and recovered <= 20_000:
        return "within-product-deadline"
    return "observation-grace" if recovered <= 20_200 else "ineligible"


def consistent_case(value):
    ledger = value["ledger"]
    if ledger is None or value["retry"] != 0 or value["unknownErrorCount"] != 0:
        return False
    entries, selected = ledger["assertions"], scope(value["name"])
    failed = [key for key in selected if entries[key]["completed"] and entries[key]["passed"] is False]
    unattempted = [key for key in selected if not entries[key]["attempted"]]
    incomplete = [key for key in selected if entries[key]["attempted"] and not entries[key]["completed"]]
    expected_errors = [key for key in IDS if entries[key]["completed"] and entries[key]["passed"] is False]
    return (value["failedAssertions"] == failed and value["unattemptedAssertions"] == unattempted
            and value["incompleteAssertions"] == incomplete and value["assertionErrorsExact"]
            and value["totalErrorCount"] == len(expected_errors) and sorted(value["knownAssertionErrorIDs"]) == sorted(expected_errors)
            and value["deadlineDisposition"] == disposition(ledger["clock"]))


def primary_case(value, index, pins):
    if not consistent_case(value):
        return "incomplete"
    app, mode = ("player", "headers") if index == 0 else ("both", "body")
    records = {item["stage"]: item["observation"] for item in value["observations"]}
    asset = records.get("q47-asset", {}).get("asset")
    if asset != {**pins["helper"], "sourceMatches": True}:
        return "incomplete"
    for label, item in records.items():
        if label == "q47-asset":
            continue
        peer = item["peer"]
        if (peer["app"] != app or peer["templateSHA256"] != pins["templates"][app]["sha256"]
                or peer["counts"]["templateBytes"] != pins["templates"][app]["bytes"]
                or peer["counts"]["expired"] != 0 or any(peer["checks"][key] for key in CHECKS[:4])):
            return "incomplete"
    if list(records) not in [STAGES[:4], STAGES[:6]]:
        return "incomplete"
    entries, clock = value["ledger"]["assertions"], value["ledger"]["clock"]
    if entries["original-links-present"]["attempts"] != 3:
        return "incomplete"
    if any(entries[key]["passed"] is not True for key in IDS[:7] + IDS[14:21] + IDS[58:60]):
        return "incomplete"
    pending, boundary = records["q47-pending"], records["q47-deadline"]
    for item in (pending, boundary):
        if item["peer"]["mode"] != mode or item["peer"]["checks"]["bodyPrefixSent"] != (mode == "body"):
            return "incomplete"
    expected_counts = {"requests": 1, "active": 1, "holds": 1, "canceled": 0, "completed": 0,
                       "failures": 0, "expired": 0, "templateBytes": pins["templates"][app]["bytes"]}
    loading = pending["state"]
    if (pending["peer"]["counts"] != expected_counts or loading["action"] != "preparing" or not loading["createDisabled"]
            or any(loading[key] for key in ("resultVisible", "errorVisible", "downloadReady")) or loading["elapsedMs"] >= 20_000):
        return "incomplete"
    if (disposition(clock) == "ineligible" or boundary["state"]["elapsedMs"] != math.floor(clock["sample"]["elapsedMs"])
            or any(boundary["state"][key] != clock["sample"][key] for key in ("action", "createDisabled", "errorVisible"))):
        return "incomplete"
    if (value["failedAssertions"] == ["deadline-retry-label"] and value["status"] == "failed"
            and value["outcome"] == "failed" and value["failure"] == "known-assertion"
            and list(records) == STAGES[:4] and value["incompleteAssertions"] == []
            and boundary["peer"]["counts"] == expected_counts and boundary["state"]["action"] == "preparing"
            and boundary["state"]["createDisabled"] and not boundary["state"]["errorVisible"]
            and clock["firstEnabledMs"] is None and clock["firstRecoveryMs"] is None
            and all(not entries[key]["attempted"] for key in scope(value["name"])
                    if key not in IDS[:7] + IDS[14:22] + IDS[58:60])):
        return "deadline-contract-red"
    if (value["status"] != "passed" or value["outcome"] != "passed" or value["failure"] != "none"
            or value["failedAssertions"] or value["unattemptedAssertions"] or value["incompleteAssertions"]
            or entries["primary-input-retention"]["attempts"] != 4
            or disposition(clock) != "within-product-deadline" or list(records) != STAGES[:6]):
        return "incomplete"
    failed, final = records["q47-failed"], records["q47-recovered"]
    state, peer = final["state"], final["peer"]
    if (any(not failed["state"][key] for key in ("errorVisible", "fallbackVisible", "fallbackCanonical", *BOOLS[6:]))
            or failed["state"]["action"] != "retry" or failed["state"]["createDisabled"]
            or failed["peer"]["counts"] != {**expected_counts, "active": 0, "canceled": 1}
            or peer["mode"] != "valid" or peer["counts"] != {**expected_counts, "requests": 2, "active": 0, "canceled": 1, "completed": 1}
            or state["action"] != "make" or state["createDisabled"] or state["errorVisible"]
            or any(not state[key] for key in ("resultVisible", "downloadReady", *BOOLS[6:]))
            or state["blobMatches"] is not True or state["copyMatches"] is not True
            or state["blobSHA256"] != pins["outputs"][index]["sha256"] or state["blobBytes"] != pins["outputs"][index]["bytes"]):
        return "incomplete"
    return "green"


def admit(report, phase, suite, pins):
    invalid = {"classification": "invalid", "report": None}
    if suite not in NAMES or phase not in ["collection", "journey"]:
        return invalid
    if (not exact(report, ["schemaVersion", "campaign", "phase", "suite", "status", "collected", "cases", "errors", "runnerErrorCount"])
            or type(report["schemaVersion"]) is not int or report["schemaVersion"] != 2 or report["campaign"] != "Q47"
            or report["phase"] != phase or report["suite"] != suite or type(report["status"]) is not str
            or report["status"] not in ["passed", "failed", "timedout", "interrupted"]
            or not vector(report["collected"], sum(NAMES.values(), []), 10, True)
            or not vector(report["errors"], ERRORS, 16) or not integer(report["runnerErrorCount"], 1024)
            or type(report["cases"]) is not list or len(report["cases"]) > 10
            or not all(case_valid(value, suite) for value in report["cases"])):
        return invalid
    result = {"classification": "incomplete", "report": report}
    if report["errors"] or report["runnerErrorCount"] or report["collected"] != NAMES[suite]:
        return result
    if phase == "collection":
        if report["status"] == "passed" and not report["cases"]:
            result["classification"] = "collection"
        return result
    if suite != "primary" or [value["name"] for value in report["cases"]] != NAMES[suite]:
        return result
    outcomes = [primary_case(value, index, pins) for index, value in enumerate(report["cases"])]
    if outcomes == ["green", "green"] and report["status"] == "passed":
        result["classification"] = "green"
    elif outcomes == ["deadline-contract-red", "deadline-contract-red"] and report["status"] == "failed":
        result["classification"] = "deadline-contract-red"
    return result


def format_binding_valid(value, fixtures, inputs):
    if (not exact(value, ["schemaVersion", "mode", "revision", "tree", "runId", "artifactId",
                          "sourceManifestSHA256", "formatter", "inputs", "outputs"])
            or type(value["schemaVersion"]) is not int or value["schemaVersion"] != 1 or value["mode"] != "source-format"
            or not all(type(value[key]) is str and re.fullmatch("[0-9a-f]{40}", value[key]) for key in ("revision", "tree"))
            or not all(type(value[key]) is str and re.fullmatch("[1-9][0-9]{0,19}", value[key]) for key in ("runId", "artifactId"))
            or not digest(value["sourceManifestSHA256"]) or not exact(value["formatter"], ["bytes", "sha256"])
            or not integer(value["formatter"]["bytes"], 32 * 1024 * 1024, 1) or not digest(value["formatter"]["sha256"])):
        return False
    for field, names in (("inputs", inputs), ("outputs", fixtures)):
        records = value[field]
        keys = ["path", "bytes", "sha256"] + (["lines"] if field == "outputs" else [])
        if (type(records) is not list or len(records) != len(names)
                or any(not exact(item, keys) for item in records)
                or [item["path"] for item in records] != list(names)
                or any(not integer(item["bytes"], 512 * 1024, 1) or not digest(item["sha256"]) for item in records)
                or field == "outputs" and any(not integer(item["lines"], 300, 1) for item in records)):
            return False
    return True


def selector_valid(argv, env):
    return (type(argv) is list and len(argv) == 1
            and env.get("CAMPAIGN_PROOF") == "Q47" and env.get("CAMPAIGN_Q47_SUITE") == "primary"
            and env.get("KINOSAIL_Q47_SUITE", "primary") == "primary")
