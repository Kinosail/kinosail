"""Pure admission controls; never run an app, browser, compiler or formatter."""
import copy
import unittest
from campaign_r06_browser_projection import (ASSERTIONS, BOOLEANS, CASES, COUNTERS, FILE, SCHEMA,
                                            admit_projection, classify)

DIGEST = "a"*64
SELECTED = ["save-headers-desktop", "save-headers-phone"]


def sample(green=True):
    rows = []
    for case in SELECTED:
        data = {"schema":SCHEMA,"kind":"runtime-case","caseID":case,"stage":"settlement",
                "failureStage":None,"protocol":"legacy","saveTerminal":"finished","servedScriptSHA256":{"inspector":DIGEST},
                "assertions":{key:{"attempted":True,"passed":True} for key in ASSERTIONS},
                **{key:True for key in BOOLEANS}, **{key:0 for key in COUNTERS},
                "saveClickToWitnessMs":100,"saveClickToUnlockMs":30000 if green else None,
                "holdDurationMs":55000,"durationMs":60000}
        data.update(receiptCompleted=False,receiptSucceeded=False,holdExpired=False,fixtureClientCancelled=False,responseStatus=200,saveAttempts=1)
        failed, unreached = [], []
        if not green:
            failed = ["editing-unlocked-by-45s","finite-truthful-status"]
            unreached = list(ASSERTIONS[7:10])
            for key in failed: data["assertions"][key]["passed"] = False
            for key in unreached: data["assertions"][key] = {"attempted":False,"passed":None}
        rows.append({"caseID":case,"outcome":"passed" if green else "failed","data":data,
                     "failedAssertions":failed,"unreachableAssertions":unreached,"retry":0,"totalErrorCount":len(failed)+2*len(unreached),
                     "knownAssertionErrorIDs":failed+unreached+[key+"-attempted" for key in unreached],
                     "unknownErrorCount":0,"assertionErrorsExact":True})
    return {"schema":SCHEMA,"kind":"runtime","collection":[{"caseID":case,"file":FILE,"title":CASES[case]} for case in SELECTED],
            "cases":rows,"malformedRecords":0,"duplicateTerminals":0,"globalErrorCount":0,
            "runnerStatus":"passed" if green else "failed"}


class ProjectionControls(unittest.TestCase):
    def test_green_requires_all_real_assertions(self):
        result = admit_projection(sample(),"runtime",SELECTED)
        self.assertEqual(classify(result,DIGEST),"focused-save-browser-green")

    def test_red_requires_completed_fault_and_settlement(self):
        result = admit_projection(sample(False),"runtime",SELECTED)
        self.assertEqual(classify(result,DIGEST),"confirmed-save-deadline-red")
        for key in ("eligible","fixtureStopped","actualSaved","historyOnce","recoveryMatches","inspectionMatches","holdEligible"):
            changed = copy.deepcopy(result); changed["cases"][0]["data"][key] = False
            self.assertEqual(classify(changed,DIGEST),"prerequisite-blocked")

    def test_not_reached_and_other_failures_never_confirm_bug(self):
        for key, value in (("failureStage","witness"),("activeHolds",1),("holdExpired",True),("saveAttempts",2)):
            changed = sample(False); changed["cases"][0]["data"][key] = value
            self.assertEqual(classify(admit_projection(changed,"runtime",SELECTED),DIGEST),"prerequisite-blocked")
        self.assertEqual(classify(sample(False),"b"*64),"prerequisite-blocked")

    def test_unknown_fields_duplicate_missing_and_raw_errors_rejected(self):
        for change in ("raw","duplicate","missing","error","counter","timing","boolean","assertion"):
            value = sample()
            if change == "raw": value["url"] = "excluded"
            if change == "duplicate": value["cases"][1] = value["cases"][0]
            if change == "missing": value["cases"].pop()
            if change == "error": value["globalErrorCount"] = 1
            if change == "counter": value["cases"][0]["data"]["saveAttempts"] = True
            if change == "timing": value["cases"][0]["data"]["durationMs"] = float("nan")
            if change == "boolean": value["cases"][0]["data"]["actualSaved"] = 1
            if change == "assertion": value["cases"][0]["data"]["assertions"][ASSERTIONS[0]]["error"] = "excluded"
            with self.assertRaises(ValueError): admit_projection(value,"runtime",SELECTED)

    def test_unknown_error_retry_or_error_multiset_never_confirm_bug(self):
        for key,value in (("retry",1),("totalErrorCount",9),("unknownErrorCount",1),("assertionErrorsExact",False),("knownAssertionErrorIDs",[])):
            changed = sample(False); changed["cases"][0][key] = value
            self.assertEqual(classify(admit_projection(changed,"runtime",SELECTED),DIGEST),"prerequisite-blocked")

    def test_cancelled_request_is_distinct_from_delivered_response(self):
        value = sample(); data = value["cases"][0]["data"]
        data.update(saveTerminal="request-failed",fixtureClientCancelled=True,responseBodyDelivered=False)
        self.assertEqual(classify(admit_projection(value,"runtime",SELECTED),DIGEST),"focused-save-browser-green")
        data["fixtureClientCancelled"] = False
        self.assertEqual(classify(value,DIGEST),"prerequisite-blocked")
        data.update(saveTerminal="pending",fixtureClientCancelled=True)
        self.assertEqual(classify(value,DIGEST),"prerequisite-blocked")

    def test_collection_is_identity_only_not_fabricated_runtime(self):
        value = sample(); value.update(kind="collection",cases=[])
        value["collection"] = [{"caseID":case,"file":FILE,"title":CASES[case]} for case in sorted(CASES)]
        self.assertEqual(admit_projection(value,"collection",SELECTED)["cases"],[])
        value["cases"] = sample()["cases"]
        with self.assertRaises(ValueError): admit_projection(value,"collection",SELECTED)

    def test_deadline_receipt_and_assertion_summary_cannot_disagree(self):
        value = sample(); value["cases"][0]["data"]["saveClickToUnlockMs"] = 45001
        self.assertEqual(classify(value,DIGEST),"prerequisite-blocked")
        value = sample(False); value["cases"][0]["failedAssertions"] = []
        with self.assertRaises(ValueError): admit_projection(value,"runtime",SELECTED)


if __name__ == "__main__":
    unittest.main()
