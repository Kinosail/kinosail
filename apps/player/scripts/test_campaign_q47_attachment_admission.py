#!/usr/bin/env python3
"""Fictional attachment-field admission; no IO, runner, process or real artifacts."""
import unittest
from campaign_q47_admission import admit
from campaign_q47_attachment_admission import attachment_fields_valid
from test_campaign_q47_admission import PINS, report


class AttachmentAdmissionControls(unittest.TestCase):
    def test_no_context_and_one_recognized_context_keep_narrow_red(self):
        for recognized in (False, True):
            value = report()
            for entry in value["cases"]:
                entry.update(privateRunnerAttachmentCount=int(recognized),
                             runnerAttachmentAdmission="recognized" if recognized else "none")
            self.assertEqual(admit(value, "journey", "primary", PINS)["classification"], "deadline-contract-red")

    def test_typed_count_enum_and_closed_schema_reject_private_metadata(self):
        mutations = [
            lambda x: x.pop("privateRunnerAttachmentCount"),
            lambda x: x.pop("runnerAttachmentAdmission"),
            lambda x: x.update(privateRunnerAttachmentCount=True),
            lambda x: x.update(privateRunnerAttachmentCount=-1),
            lambda x: x.update(privateRunnerAttachmentCount=2),
            lambda x: x.update(runnerAttachmentAdmission="arbitrary-private-canary"),
            lambda x: x.update(runnerAttachmentAdmission=None),
            lambda x: x.update(privateRunnerAttachmentCount=1, runnerAttachmentAdmission="none"),
            lambda x: x.update(privateRunnerAttachmentCount=0, runnerAttachmentAdmission="recognized"),
            lambda x: x.update(path="/fictional/private/error-context.md"),
            lambda x: x.update(body="fictional-private-canary"),
        ]
        for mutate in mutations:
            with self.subTest(mutation=mutations.index(mutate)):
                value = report()
                mutate(value["cases"][0])
                self.assertEqual(admit(value, "journey", "primary", PINS),
                                 {"classification": "invalid", "report": None})

    def test_rejected_context_is_never_red_even_with_exact_known_error(self):
        for count in (0, 1):
            value = report()
            value["cases"][0].update(privateRunnerAttachmentCount=count, runnerAttachmentAdmission="rejected",
                                     outcome="incomplete", failure="unclassified")
            self.assertEqual(admit(value, "journey", "primary", PINS)["classification"], "incomplete")

    def test_rejected_context_cannot_claim_completed_failure(self):
        value = report()
        value["cases"][0].update(runnerAttachmentAdmission="rejected")
        self.assertEqual(admit(value, "journey", "primary", PINS),
                         {"classification": "invalid", "report": None})

    def test_recognized_context_requires_exact_failed_case_error_multiset(self):
        mutations = [
            lambda x: x.update(status="passed"),
            lambda x: x.update(status="interrupted"),
            lambda x: x.update(retry=1),
            lambda x: x.update(unknownErrorCount=1, totalErrorCount=2, assertionErrorsExact=False),
            lambda x: x.update(totalErrorCount=0),
            lambda x: x.update(knownAssertionErrorIDs=[]),
            lambda x: x.update(knownAssertionErrorIDs=["deadline-retry-label", "deadline-retry-label"]),
        ]
        for mutate in mutations:
            with self.subTest(mutation=mutations.index(mutate)):
                value = report()
                value["cases"][0].update(privateRunnerAttachmentCount=1, runnerAttachmentAdmission="recognized")
                mutate(value["cases"][0])
                self.assertNotIn(admit(value, "journey", "primary", PINS)["classification"],
                                 ("green", "deadline-contract-red"))

    def test_passed_case_never_accepts_ancillary_failure_artifact(self):
        value = report(True)
        value["cases"][0].update(privateRunnerAttachmentCount=1, runnerAttachmentAdmission="recognized")
        self.assertEqual(admit(value, "journey", "primary", PINS),
                         {"classification": "invalid", "report": None})

    def test_context_metadata_does_not_mask_clock_or_witness_failure(self):
        for mutate in (
                lambda x: x["ledger"]["clock"].update(trustedClick=False),
                lambda x: x["ledger"]["clock"].update(elapsedMs=20_300, sample={**x["ledger"]["clock"]["sample"], "elapsedMs": 20_201}),
                lambda x: x["ledger"]["assertions"]["primary-one-hold"].update(passed=False)):
            value = report()
            for entry in value["cases"]:
                entry.update(privateRunnerAttachmentCount=1, runnerAttachmentAdmission="recognized")
            mutate(value["cases"][0])
            self.assertEqual(admit(value, "journey", "primary", PINS)["classification"], "incomplete")

    def test_absent_or_unknown_records_fail_closed(self):
        for value in (None, [], {}, {"privateRunnerAttachmentCount": 0}, {"runnerAttachmentAdmission": "none"}):
            self.assertFalse(attachment_fields_valid(value))

    def test_unattempted_recovery_stays_null_after_recognized_metadata(self):
        value = report()
        for entry in value["cases"]:
            entry.update(privateRunnerAttachmentCount=1, runnerAttachmentAdmission="recognized")
        result = admit(value, "journey", "primary", PINS)
        for entry in result["report"]["cases"]:
            for key in ("primary-peer-canceled", "primary-new-completed", "native-blob-digest-equal", "native-copy-equal"):
                self.assertEqual(entry["ledger"]["assertions"][key],
                                 {"attempted": False, "completed": False, "passed": None, "attempts": 0})


if __name__ == "__main__":
    unittest.main()
