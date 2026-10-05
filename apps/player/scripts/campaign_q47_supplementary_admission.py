"""Q47 secondary successes only; primary deadline admission remains byte-exact."""
from campaign_q47_admission import (
    admit as primary_admit, selector_valid as primary_selector,
    BOOLS, CHECKS, COUNTS, NAMES, scope,
)


def selector_valid(argv, env):
    suite = env.get("CAMPAIGN_Q47_SUITE")
    if suite == "primary":
        return primary_selector(argv, env)
    return (type(argv) is list and len(argv) == 1 and env.get("CAMPAIGN_PROOF") == "Q47"
            and suite in ("recovery", "supersession", "contracts")
            and env.get("KINOSAIL_Q47_SUITE", suite) == suite)


def consistent_secondary(case):
    ledger = case["ledger"]
    if (ledger is None or case["retry"] != 0 or case["status"] != "passed"
            or case["outcome"] != "passed" or case["failure"] != "none"
            or case["deadlineDisposition"] != "not-applicable" or not case["assertionErrorsExact"]
            or case["unknownErrorCount"] != 0 or case["totalErrorCount"] != 0
            or case["knownAssertionErrorIDs"] or case["privateRunnerAttachmentCount"] != 0
            or case["runnerAttachmentAdmission"] != "none"
            or any(case[key] for key in ("failedAssertions", "unattemptedAssertions", "incompleteAssertions"))):
        return False
    selected = scope(case["name"])
    return (ledger["assertions"]["original-links-present"]["attempts"] == 3
            and all(ledger["assertions"][key]["attempted"] and ledger["assertions"][key]["completed"]
                    and ledger["assertions"][key]["passed"] is True for key in selected))


def observations(case, pins, app, mode, stages):
    records = {item["stage"]: item["observation"] for item in case["observations"]}
    if list(records) != stages or records["q47-asset"]["asset"] != {**pins["helper"], "sourceMatches": True}:
        return None
    for label, item in records.items():
        if label == "q47-asset":
            continue
        peer = item["peer"]
        if (peer["app"] != app or peer["templateSHA256"] != pins["templates"][app]["sha256"]
                or peer["counts"]["templateBytes"] != pins["templates"][app]["bytes"]
                or peer["counts"]["expired"] != 0 or any(peer["checks"][key] for key in CHECKS[:4])):
            return None
        if peer["mode"] not in (mode, "valid"):
            return None
    return records


def counts(peer, pin, **values):
    expected = dict.fromkeys(COUNTS, 0) | {"templateBytes": pin["bytes"]} | values
    return peer["counts"] == expected


def ready(state, pin):
    return (state["action"] == "make" and not state["createDisabled"] and not state["errorVisible"]
            and all(state[key] for key in ("resultVisible", "downloadReady", *BOOLS[6:]))
            and state["blobMatches"] is True and state["copyMatches"] is True
            and state["blobSHA256"] == pin["sha256"] and state["blobBytes"] == pin["bytes"])


def recovery(case, pins, index):
    mode = ("loss", "http_error")[index]
    records = observations(case, pins, "both", mode, ["q47-asset", "q47-started", "q47-failed", "q47-recovered"])
    if records is None:
        return False
    failed, final = records["q47-failed"], records["q47-recovered"]
    state, peer = failed["state"], failed["peer"]
    number = peer["counts"]["requests"]
    if (peer["mode"] != mode or number < 1
            or not counts(peer, pins["templates"]["both"], requests=number, failures=number)
            or state["action"] != "retry" or state["createDisabled"]
            or any(state[key] for key in ("resultVisible", "downloadReady"))
            or any(not state[key] for key in ("errorVisible", "fallbackCanonical", *BOOLS[6:]))
            or case["ledger"]["assertions"]["recovery-input-retention"]["attempts"] != 4
            or final["peer"]["mode"] != "valid"
            or not counts(final["peer"], pins["templates"]["both"], requests=number + 1, failures=number, completed=1)):
        return False
    return ready(final["state"], pins["supplementaryOutputs"][case["name"]])


def supersession(case, pins, index):
    mode = ("late_headers", "late_body")[index]
    records = observations(case, pins, "player", mode, ["q47-asset", "q47-started", "q47-stale"])
    if records is None:
        return False
    started, final = records["q47-started"], records["q47-stale"]
    if (started["peer"]["mode"] != mode or final["peer"]["mode"] != mode
            or not counts(started["peer"], pins["templates"]["player"], requests=1, active=1, holds=1)
            or not counts(final["peer"], pins["templates"]["player"], requests=2, holds=1, canceled=1, completed=1)
            or final["peer"]["checks"]["bodyPrefixSent"] != (mode == "late_body")):
        return False
    return ready(final["state"], pins["supplementaryOutputs"][case["name"]])


def contracts(case, pins, index):
    if index == 3:
        records = observations(case, pins, "both", "valid", ["q47-asset", "q47-rejected"])
        if records is None:
            return False
        rejected = records["q47-rejected"]
        entries = case["ledger"]["assertions"]
        return (rejected["peer"]["mode"] == "valid" and counts(rejected["peer"], pins["templates"]["both"])
                and rejected["state"]["errorVisible"] and not rejected["state"]["createDisabled"]
                and not rejected["state"]["resultVisible"] and not rejected["state"]["downloadReady"]
                and all(entries[key]["attempts"] == 7 for key in ("invalid-path-error", "invalid-path-no-request"))
                and all(entries[key]["attempts"] == 4 for key in ("invalid-port-error", "invalid-port-no-request")))
    app = ("player", "subtitles", "both")[index]
    records = observations(case, pins, app, "valid", ["q47-asset", "q47-started", "q47-recovered"])
    if records is None:
        return False
    final = records["q47-recovered"]
    return (final["peer"]["mode"] == "valid" and counts(final["peer"], pins["templates"][app], requests=1, completed=1)
            and ready(final["state"], pins["supplementaryOutputs"][case["name"]]))


def admit(report, phase, suite, pins):
    original = primary_admit(report, phase, suite, pins)
    if suite == "primary" or phase == "collection" or original["classification"] == "invalid":
        return original
    if suite not in ("recovery", "supersession", "contracts") or original["report"] is None:
        return original
    value = original["report"]
    if (value["status"] != "passed" or value["errors"] or value["runnerErrorCount"]
            or value["collected"] != NAMES[suite] or [case["name"] for case in value["cases"]] != NAMES[suite]
            or not all(consistent_secondary(case) for case in value["cases"])):
        return original
    judge = {"recovery": recovery, "supersession": supersession, "contracts": contracts}[suite]
    try:
        accepted = all(judge(case, pins, index) for index, case in enumerate(value["cases"]))
    except (KeyError, TypeError, IndexError):
        accepted = False
    return {"classification": "green" if accepted else "incomplete", "report": value}
