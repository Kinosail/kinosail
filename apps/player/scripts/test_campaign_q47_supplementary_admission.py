"""Fictional Q47 secondary ledgers; no file, process, browser or application IO."""
import copy
import unittest
from campaign_q47_admission import NAMES, IDS, BOOLS, COUNTS, CHECKS, scope
from campaign_q47_supplementary_admission import admit, selector_valid
from test_campaign_q47_admission import report as primary_report, PINS as PRIMARY_PINS

PINS = {"helper": {"bytes": 42, "sha256": "a" * 64},
        "templates": {app: {"bytes": 100 + index, "sha256": str(index + 1) * 64}
                      for index, app in enumerate(("player", "subtitles", "both"))},
        "supplementaryOutputs": {name: {"bytes": 200 + index, "sha256": "b" * 64}
                                for index, name in enumerate(sum([NAMES[s] for s in ("recovery", "supersession", "contracts")], []))}}


def state(output=None, failed=False):
    value = dict.fromkeys(BOOLS, False) | {
        "action": "retry" if failed else "make", "fallbackCanonical": True,
        "retainedApp": True, "retainedPath": True, "retainedPort": True, "retainedSecondPort": True,
        "errorVisible": failed, "fallbackVisible": failed, "elapsedMs": 10, "responses": 1,
        "blobMatches": None, "copyMatches": None, "blobSHA256": None, "blobBytes": None}
    if output:
        value.update(resultVisible=True, downloadReady=True, blobMatches=True, copyMatches=True,
                     blobSHA256=output["sha256"], blobBytes=output["bytes"])
    return value


def peer(app, mode, **counts):
    return {"app": app, "mode": mode, "templateSHA256": PINS["templates"][app]["sha256"],
            "counts": dict.fromkeys(COUNTS, 0) | {"templateBytes": PINS["templates"][app]["bytes"]} | counts,
            "checks": dict.fromkeys(CHECKS, False) | {"bodyPrefixSent": mode == "late_body"}}


def observe(stage, value, witness):
    return {"stage": stage, "observation": {"state": value, "peer": witness}}


def case(name, suite, index):
    selected = scope(name)
    entries = {key: {"attempted": key in selected, "completed": key in selected,
                     "passed": True if key in selected else None, "attempts": int(key in selected)} for key in IDS}
    entries["original-links-present"]["attempts"] = 3
    value = {"name": name, "status": "passed", "retry": 0, "durationMs": 100,
             "outcome": "passed", "failure": "none", "ledger": {"assertions": entries, "clock": None},
             "failedAssertions": [], "unattemptedAssertions": [], "incompleteAssertions": [],
             "totalErrorCount": 0, "knownAssertionErrorIDs": [], "unknownErrorCount": 0,
             "assertionErrorsExact": True, "deadlineDisposition": "not-applicable",
             "privateRunnerAttachmentCount": 0, "runnerAttachmentAdmission": "none",
             "observations": [{"stage": "q47-asset", "observation": {"asset": PINS["helper"] | {"sourceMatches": True}}}]}
    output = PINS["supplementaryOutputs"][name]
    if suite == "recovery":
        mode = ("loss", "http_error")[index]
        entries["recovery-input-retention"]["attempts"] = 4
        value["observations"] += [
            observe("q47-started", state(), peer("both", mode, requests=1, failures=1)),
            observe("q47-failed", state(failed=True), peer("both", mode, requests=1, failures=1)),
            observe("q47-recovered", state(output), peer("both", "valid", requests=2, failures=1, completed=1))]
    elif suite == "supersession":
        mode = ("late_headers", "late_body")[index]
        value["observations"] += [
            observe("q47-started", state(), peer("player", mode, requests=1, active=1, holds=1)),
            observe("q47-stale", state(output), peer("player", mode, requests=2, holds=1, canceled=1, completed=1))]
    elif index == 3:
        for key in ("invalid-path-error", "invalid-path-no-request"): entries[key]["attempts"] = 7
        for key in ("invalid-port-error", "invalid-port-no-request"): entries[key]["attempts"] = 4
        value["observations"].append(observe("q47-rejected", state(failed=True), peer("both", "valid")))
    else:
        app = ("player", "subtitles", "both")[index]
        value["observations"] += [
            observe("q47-started", state(), peer(app, "valid", requests=1, completed=1)),
            observe("q47-recovered", state(output), peer(app, "valid", requests=1, completed=1))]
    return value


def report(suite):
    return {"schemaVersion": 2, "campaign": "Q47", "phase": "journey", "suite": suite,
            "status": "passed", "collected": NAMES[suite], "errors": [], "runnerErrorCount": 0,
            "cases": [case(name, suite, index) for index, name in enumerate(NAMES[suite])]}


class SecondaryControls(unittest.TestCase):
    def test_original_primary_red_and_green_delegate_without_reinterpretation(self):
        for green in (False, True):
            expected = "green" if green else "deadline-contract-red"
            self.assertEqual(admit(primary_report(green), "journey", "primary", PRIMARY_PINS)["classification"], expected)

    def test_all_three_exact_secondary_successes(self):
        for suite in ("recovery", "supersession", "contracts"):
            self.assertEqual(admit(report(suite), "journey", suite, PINS)["classification"], "green")

    def test_fixed_selectors_reject_arbitrary_and_cross_selected_modes(self):
        for suite in NAMES:
            env = {"CAMPAIGN_PROOF": "Q47", "CAMPAIGN_Q47_SUITE": suite}
            self.assertTrue(selector_valid(["driver"], env))
            for changes in ({"CAMPAIGN_PROOF": "R06"}, {"KINOSAIL_Q47_SUITE": "wrong"}):
                self.assertFalse(selector_valid(["driver"], env | changes))
            self.assertFalse(selector_valid(["driver", "extra"], env))
        for suite in ("", "all", "../primary", "recovery;private"):
            self.assertFalse(selector_valid(["driver"], {"CAMPAIGN_PROOF": "Q47", "CAMPAIGN_Q47_SUITE": suite}))

    def test_collection_is_exact_and_never_journey_success(self):
        for suite in ("recovery", "supersession", "contracts"):
            value = report(suite); value.update(phase="collection", cases=[])
            self.assertEqual(admit(value, "collection", suite, PINS)["classification"], "collection")
            value["collected"] = value["collected"][:-1]
            self.assertNotEqual(admit(value, "collection", suite, PINS)["classification"], "collection")

    def test_missing_reordered_duplicated_or_extra_cases_never_green(self):
        for suite in ("recovery", "supersession", "contracts"):
            for mutate in (lambda v: v["cases"].pop(), lambda v: v["cases"].reverse(),
                           lambda v: v["cases"].append(copy.deepcopy(v["cases"][0])),
                           lambda v: v["cases"][0].update(body="fictional-private-canary")):
                value = report(suite); mutate(value)
                self.assertNotEqual(admit(value, "journey", suite, PINS)["classification"], "green")

    def test_every_selected_assertion_and_zero_error_contract_is_required(self):
        mutations = [
            lambda c: c.update(status="failed"), lambda c: c.update(retry=1),
            lambda c: c.update(unknownErrorCount=1), lambda c: c.update(totalErrorCount=1),
            lambda c: c.update(knownAssertionErrorIDs=["preview-equal"]),
            lambda c: c.update(assertionErrorsExact=False),
            lambda c: c.update(deadlineDisposition="within-product-deadline"),
            lambda c: c.update(privateRunnerAttachmentCount=1, runnerAttachmentAdmission="recognized"),
            lambda c: c.update(runnerAttachmentAdmission="rejected"),
            lambda c: c["ledger"]["assertions"]["original-links-present"].update(attempts=2),
            lambda c: c["ledger"]["assertions"]["builder-visible"].update(passed=False),
            lambda c: c["ledger"]["assertions"]["builder-visible"].update(attempted=False, completed=False, passed=None, attempts=0)]
        for suite in ("recovery", "supersession", "contracts"):
            for mutate in mutations:
                value = report(suite); mutate(value["cases"][0])
                result = admit(value, "journey", suite, PINS)
                self.assertNotIn(result["classification"], ("green", "deadline-contract-red"))

    def test_identity_privacy_bytes_and_completion_witnesses_are_required(self):
        for suite in ("recovery", "supersession", "contracts"):
            mutations = [
                lambda r: r["q47-asset"]["asset"].update(sha256="c" * 64),
                lambda r: r[next(k for k in r if k != "q47-asset")]["peer"].update(templateSHA256="d" * 64),
                lambda r: r[next(k for k in r if k != "q47-asset")]["peer"]["checks"].update(cookieSeen=True)]
            for mutate in mutations:
                value = report(suite)
                records = {x["stage"]: x["observation"] for x in value["cases"][0]["observations"]}
                mutate(records)
                self.assertNotEqual(admit(value, "journey", suite, PINS)["classification"], "green")
            for field, altered in (("blobSHA256", "f" * 64), ("blobBytes", 2010), ("copyMatches", False),
                                   ("resultVisible", False), ("retainedPath", False), ("createDisabled", True)):
                value = report(suite); value["cases"][0]["observations"][-1]["observation"]["state"][field] = altered
                self.assertNotEqual(admit(value, "journey", suite, PINS)["classification"], "green")

    def test_failed_recovery_has_exact_retained_inputs_and_new_request(self):
        for field in ("retainedApp", "retainedPath", "retainedPort", "retainedSecondPort", "fallbackCanonical"):
            value = report("recovery"); value["cases"][0]["observations"][2]["observation"]["state"][field] = False
            self.assertNotEqual(admit(value, "journey", "recovery", PINS)["classification"], "green")
        for counts in ({"requests": 1}, {"failures": 0}, {"active": 1}, {"expired": 1}, {"completed": 2}):
            value = report("recovery"); value["cases"][0]["observations"][-1]["observation"]["peer"]["counts"].update(counts)
            self.assertNotEqual(admit(value, "journey", "recovery", PINS)["classification"], "green")

    def test_supersession_requires_one_canceled_old_and_one_completed_current(self):
        for counts in ({"canceled": 0}, {"canceled": 2}, {"requests": 1}, {"active": 1}, {"holds": 0}, {"completed": 2}):
            value = report("supersession"); value["cases"][0]["observations"][-1]["observation"]["peer"]["counts"].update(counts)
            self.assertNotEqual(admit(value, "journey", "supersession", PINS)["classification"], "green")
        value = report("supersession"); value["cases"][1]["observations"][-1]["observation"]["peer"]["checks"]["bodyPrefixSent"] = False
        self.assertNotEqual(admit(value, "journey", "supersession", PINS)["classification"], "green")

    def test_validation_runs_all_original_path_port_cases_without_requests(self):
        for counts in ({"requests": 1}, {"active": 1}, {"completed": 1}, {"failures": 1}):
            value = report("contracts"); value["cases"][3]["observations"][-1]["observation"]["peer"]["counts"].update(counts)
            self.assertNotEqual(admit(value, "journey", "contracts", PINS)["classification"], "green")
        for key in ("invalid-path-error", "invalid-path-no-request", "invalid-port-error", "invalid-port-no-request"):
            value = report("contracts"); value["cases"][3]["ledger"]["assertions"][key]["attempts"] -= 1
            self.assertNotEqual(admit(value, "journey", "contracts", PINS)["classification"], "green")
