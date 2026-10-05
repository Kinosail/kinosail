#!/usr/bin/env python3
"""Fictional pure admission controls; no app, browser, subprocess or network."""
import copy
import unittest
from campaign_q47_admission import IDS, NAMES, scope, admit, format_binding_valid
from campaign_q47_execution import complete, CAPTURE_LIMIT

H = "a" * 64
T = "b" * 64
PINS = {"helper": {"bytes": 1000, "sha256": H},
        "templates": {"player": {"bytes": 1000, "sha256": T},
                      "both": {"bytes": 1200, "sha256": T},
                      "subtitles": {"bytes": 900, "sha256": T}},
        "outputs": [{"bytes": 1010, "sha256": "c" * 64}, {"bytes": 1210, "sha256": "d" * 64}]}


def assertion(passed=None):
    return {"attempted": passed is not None, "completed": passed is not None,
            "passed": passed, "attempts": int(passed is not None)}


def observation(index, stage):
    app, mode = ("player", "headers") if index == 0 else ("both", "body")
    state = {key: True for key in ("retainedApp", "retainedPath", "retainedPort", "retainedSecondPort")}
    state.update({key: False for key in ("resultVisible", "errorVisible", "fallbackVisible",
                                       "fallbackCanonical", "downloadReady")})
    state.update(action="preparing", createDisabled=True, blobMatches=None, copyMatches=None,
                 blobSHA256=None, blobBytes=None, elapsedMs=100 if stage != "q47-deadline" else 20_050,
                 responses=int(mode == "body"))
    counts = dict(requests=1, active=1, holds=1, canceled=0, completed=0, failures=0,
                  expired=0, templateBytes=PINS["templates"][app]["bytes"])
    checks = {key: False for key in ("cookieSeen", "authorizationSeen", "bodySeen", "querySeen")}
    checks["bodyPrefixSent"] = mode == "body"
    return {"stage": stage, "observation": {"state": state, "peer": {
        "app": app, "mode": mode, "templateSHA256": T, "counts": counts, "checks": checks}}}


def case(index, green=False):
    name = NAMES["primary"][index]
    entries = {key: assertion() for key in IDS}
    prefix = IDS[:7] + IDS[14:21] + IDS[58:60]
    for key in (scope(name) if green else prefix):
        entries[key] = assertion(True)
    entries["original-links-present"]["attempts"] = 3
    if green:
        entries["primary-input-retention"]["attempts"] = 4
    if not green:
        entries["deadline-retry-label"] = assertion(False)
    clock = dict(elapsedMs=20_100, trustedClick=True, clicks=1, pendingObserved=True,
                 firstEnabledMs=19_999 if green else None, firstRecoveryMs=19_999 if green else None,
                 sample=dict(elapsedMs=20_050, action="retry" if green else "preparing",
                             createDisabled=not green, errorVisible=green))
    observations = [{"stage": "q47-asset", "observation": {"asset": {
        "bytes": 1000, "sha256": H, "sourceMatches": True}}}]
    observations += [observation(index, label) for label in ("q47-started", "q47-pending", "q47-deadline")]
    if green:
        for label in ("q47-deadline", "q47-failed", "q47-recovered"):
            item = observations[-1] if label == "q47-deadline" else observation(index, label)
            state, peer = item["observation"]["state"], item["observation"]["peer"]
            state.update(action="retry", createDisabled=False, errorVisible=True,
                         fallbackVisible=True, fallbackCanonical=True)
            peer["counts"].update(active=0, canceled=1)
            if label == "q47-recovered":
                state.update(action="make", resultVisible=True, errorVisible=False, downloadReady=True,
                             blobMatches=True, copyMatches=True, blobSHA256=PINS["outputs"][index]["sha256"],
                             blobBytes=PINS["outputs"][index]["bytes"], elapsedMs=21_000)
                peer.update(mode="valid")
                peer["counts"].update(requests=2, completed=1)
            if label != "q47-deadline":
                observations.append(item)
    selected = scope(name)
    failed = [key for key in selected if entries[key]["passed"] is False]
    unattempted = [key for key in selected if not entries[key]["attempted"]]
    return dict(name=name, status="passed" if green else "failed", retry=0, durationMs=22_000,
                outcome="passed" if green else "failed", failure="none" if green else "known-assertion",
                ledger={"assertions": entries, "clock": clock}, failedAssertions=failed,
                unattemptedAssertions=unattempted, incompleteAssertions=[],
                totalErrorCount=len(failed), knownAssertionErrorIDs=failed.copy(), unknownErrorCount=0,
                assertionErrorsExact=True, deadlineDisposition="within-product-deadline" if green else "no-recovery-observed",
                observations=observations, privateRunnerAttachmentCount=0,
                runnerAttachmentAdmission="none")


def report(green=False, collection=False):
    return dict(schemaVersion=2, campaign="Q47", phase="collection" if collection else "journey",
                suite="primary", status="passed" if green or collection else "failed",
                collected=NAMES["primary"].copy(), cases=[] if collection else [case(0, green), case(1, green)],
                errors=[], runnerErrorCount=0)


class AdmissionControls(unittest.TestCase):
    def check(self, value, expected, phase="journey"):
        result = admit(value, phase, "primary", PINS)
        self.assertEqual(result["classification"], expected)

    def test_exact_fictional_red_and_green_and_collection(self):
        self.check(report(), "deadline-contract-red")
        self.check(report(True), "green")
        self.check(report(collection=True), "collection", "collection")

    def test_malformed_private_report_is_never_exported(self):
        mutations = [
            lambda x: x.update(secret="fictional-private-bytes"),
            lambda x: x.pop("errors"),
            lambda x: x.update(errors=None),
            lambda x: x.update(cases="wrong"),
            lambda x: x["cases"][0].update(extra="fictional-private-bytes"),
            lambda x: x["cases"][0]["observations"][1]["observation"]["state"].update(rawPath="/fictional"),
            lambda x: x["cases"][0]["ledger"]["assertions"].pop(IDS[0]),
            lambda x: x["cases"][0].update(totalErrorCount=True),
            lambda x: x["cases"][0]["observations"][2]["observation"]["state"].update(blobSHA256="secret"),
            lambda x: x["cases"][0]["ledger"]["clock"].update(elapsedMs=float("nan")),
        ]
        for mutate in mutations:
            with self.subTest(mutation=mutations.index(mutate)):
                value = report(); mutate(value)
                result = admit(value, "journey", "primary", PINS)
                self.assertEqual(result, {"classification": "invalid", "report": None})

    def test_incomplete_cannot_be_deadline_red(self):
        mutations = [
            lambda x: x.update(status="interrupted"),
            lambda x: x.update(errors=["runner_error"], runnerErrorCount=1),
            lambda x: x["cases"][0].update(retry=1),
            lambda x: x["cases"][0].update(totalErrorCount=2, unknownErrorCount=1, assertionErrorsExact=False),
            lambda x: x["cases"][0].update(knownAssertionErrorIDs=["deadline-retry-label", "deadline-retry-label"]),
            lambda x: x["cases"][0].update(failedAssertions=[]),
            lambda x: x["cases"][0]["ledger"]["assertions"]["primary-one-hold"].update(passed=False),
            lambda x: x["cases"][0]["ledger"]["assertions"]["original-links-present"].update(attempts=2),
            lambda x: x["cases"][0]["ledger"]["clock"].update(trustedClick=False),
            lambda x: x["cases"][0]["ledger"]["clock"]["sample"].update(elapsedMs=20_201),
            lambda x: x["cases"][0]["ledger"]["clock"].update(pendingObserved=False),
            lambda x: x["cases"][0]["observations"].pop(2),
            lambda x: x["cases"][0]["observations"][2]["observation"]["peer"]["counts"].update(expired=1),
            lambda x: x["cases"][0]["observations"][2]["observation"]["peer"]["checks"].update(cookieSeen=True),
            lambda x: x["cases"][0]["observations"][0]["observation"]["asset"].update(sourceMatches=False),
            lambda x: x["cases"][0]["observations"][1]["observation"]["peer"].update(templateSHA256="e" * 64),
            lambda x: x["cases"][0]["observations"][2]["observation"]["state"].update(elapsedMs=20_000),
            lambda x: x.update(cases=[]),
            lambda x: x["cases"].reverse(),
        ]
        for mutate in mutations:
            with self.subTest(mutation=mutations.index(mutate)):
                value = report(); mutate(value)
                self.assertNotIn(admit(value, "journey", "primary", PINS)["classification"],
                                 ("deadline-contract-red", "green"))

    def test_observation_grace_is_never_product_green(self):
        for late in (20_001, 20_199):
            value = report(True)
            for entry in value["cases"]:
                entry["ledger"]["clock"].update(elapsedMs=20_200, firstEnabledMs=late, firstRecoveryMs=late)
                entry["deadlineDisposition"] = "observation-grace"
            self.assertNotEqual(admit(value, "journey", "primary", PINS)["classification"], "green")

    def test_unattempted_recovery_is_null_in_narrow_red(self):
        result = admit(report(), "journey", "primary", PINS)
        for entry in result["report"]["cases"]:
            for key in ("primary-peer-canceled", "primary-new-completed", "native-blob-digest-equal", "native-copy-equal"):
                self.assertEqual(entry["ledger"]["assertions"][key], assertion())

    def test_late_added_error_multiset_and_green_inconsistency(self):
        for mutate in (
                lambda x: x["cases"][0].update(knownAssertionErrorIDs=[]),
                lambda x: x["cases"][0].update(status="passed"),
                lambda x: x.update(status="passed"),
                lambda x: x["cases"][0]["ledger"]["assertions"]["deadline-enabled"].update(
                    attempted=True, completed=True, passed=False, attempts=1)):
            value = report(); mutate(value)
            self.assertNotIn(admit(value, "journey", "primary", PINS)["classification"],
                             ("deadline-contract-red", "green"))


    def test_process_terminal_record_cannot_mask_unsettled_or_unlaunched_work(self):
        terminal = dict(launched=True, exitCode=0, durationMs=10, stopReason=None, leaderExited=True,
                        groupAbsent=True, captureComplete=True, captureBytes=0, captureLimit=CAPTURE_LIMIT)
        self.assertTrue(complete(terminal, 0))
        self.assertFalse(complete(terminal, 1))
        for key, value in (("launched", False), ("exitCode", False), ("exitCode", None),
                           ("leaderExited", False), ("groupAbsent", False), ("captureComplete", False),
                           ("captureBytes", CAPTURE_LIMIT + 1), ("captureLimit", CAPTURE_LIMIT + 1),
                           ("durationMs", float("nan")), ("stopReason", "timeout"), ("stopReason", "interrupted")):
            with self.subTest(field=key, value=value):
                changed = copy.deepcopy(terminal); changed[key] = value
                self.assertFalse(complete(changed, 0))

    def test_format_binding_is_typed_exact_two_outputs_and_seven_known_inputs(self):
        fixtures = ["apps/player/e2e/compose-template-fixture.go", "apps/player/e2e/compose-template-peer.go"]
        paths = fixtures + ["apps/player/scripts/campaign_q47_format.py", "apps/player/scripts/test_campaign_q47_format.py",
                            ".github/workflows/layout-stability.yml", "scripts/ci/run-campaign-proof.sh", "go.work"]
        binding = dict(schemaVersion=1, mode="source-format", revision="e" * 40, tree="f" * 40,
                       runId="100", artifactId="200", sourceManifestSHA256=H,
                       formatter={"bytes": 1000, "sha256": H},
                       inputs=[{"path": path, "bytes": 1000, "sha256": H} for path in paths],
                       outputs=[{"path": path, "bytes": 1000, "lines": 100, "sha256": H} for path in fixtures])
        self.assertTrue(format_binding_valid(binding, fixtures, paths))
        for mutate in (lambda x: x["outputs"][0].update(lines=301),
                       lambda x: x["outputs"].reverse(), lambda x: x["inputs"].pop(),
                       lambda x: x["outputs"][0].update(sha256="secret"),
                       lambda x: x["formatter"].update(bytes=True),
                       lambda x: x.update(extra="fictional-private"),
                       lambda x: x.update(runId="https://fictional.invalid/"),
                       lambda x: x["inputs"][0].update(path="/fictional/private"),
                       lambda x: x["outputs"][0].update(lines=0)):
            changed = copy.deepcopy(binding); mutate(changed)
            self.assertFalse(format_binding_valid(changed, fixtures, paths))
        self.assertFalse(format_binding_valid(None, fixtures, paths))


if __name__ == "__main__":
    unittest.main()
