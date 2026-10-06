"""Failure-first closed Restore browser evidence, including intended RED."""
import copy
import unittest
import campaign_r06_restore_runtime_projection as projection

class RestoreBrowserProjectionControls(unittest.TestCase):
    def case(self, case_id):
        data = {key: False for key in projection.FLAGS}
        data.update({key: 0 for key in projection.COUNTERS})
        data.update({key: True for key in projection.EXTRA_FLAGS})
        data.update(schema="r06-restore-browser-v1", kind="runtime-case", caseID=case_id,
                    stage="cleanup", failureStage=None, protocol="legacy",
                    setupSaveAttempts=1, restoreAttempts=1, prepareAttempts=0,
                    responseStatus=204, inspectionResponseStatus=200, activeHolds=0,
                    browserRestoredInspections=1, actualRestored=True, historyOnce=True,
                    recoverySwapped=True, inspectionMatches=True, holdEligible=True,
                    headersReleased=True, bodyReleased=True, restoreResponseDelivered=True,
                    inspectionResponseDelivered=True, restoreTerminal="finished", inspectTerminal="finished",
                    clickToWitnessMs=100, clickToUnlockMs=30000, holdDurationMs=30000, durationMs=35000,
                    restoreFailureCode="none",inspectFailureCode="none",causalDiagnostic=None,
                    servedScriptSHA256={"inspector":"a"*64},
                    assertions={key:{"attempted":True,"completed":True,"passed":True} for key in projection.ASSERTIONS})
        return {"caseID":case_id,"outcome":"passed","data":data,"failedAssertions":[],
                "unattemptedAssertions":[],"incompleteAssertions":[],"retry":0,"totalErrorCount":0,
                "knownAssertionErrorIDs":[],"unknownErrorCount":0,"assertionErrorsExact":True}

    def value(self, suite="restore-headers", mode="runtime"):
        ids = sorted(projection.CASES if mode=="collection" else projection.SELECTED[suite])
        return {"schema":"r06-restore-browser-v1","kind":mode,
                "collection":[{"caseID":key,"file":projection.FILE,"title":projection.CASES[key]} for key in ids],
                "cases":[] if mode=="collection" else [self.case(key) for key in ids],
                "malformedRecords":0,"duplicateTerminals":0,"globalErrorCount":0,"runnerStatus":"passed"}

    def test_all_twelve_assertions_and_four_cases_are_fixed(self):
        self.assertEqual(len(projection.CASES), 4)
        self.assertEqual(projection.ASSERTIONS, ("actual-restore","history-once","recovery-swapped","hold-proven",
            "editor-released-by-45s","finite-truthful-outcome","original-correction-kept","original-import-kept",
            "library-navigation","back-current-subtitle","no-restore-replay","fixture-settled"))
        self.assertEqual(projection.admit(self.value(mode="collection"), "collection", "restore-headers")["cases"], [])

    def test_complete_exact_pair_is_green(self):
        value = projection.admit(self.value(), "runtime", "restore-headers")
        self.assertEqual(projection.classify(value, "a"*64), "focused-restore-browser-green")

    def test_partial_extra_foreign_collection_and_case_are_rejected(self):
        for field in ("collection", "cases"):
            value=self.value(); value[field].pop()
            with self.assertRaises(ValueError): projection.admit(value,"runtime","restore-headers")
        value=self.value();value["collection"][0]["file"]="foreign.ts"
        with self.assertRaises(ValueError): projection.admit(value,"runtime","restore-headers")
        value=self.value();value["cases"].append(copy.deepcopy(value["cases"][0]))
        with self.assertRaises(ValueError): projection.admit(value,"runtime","restore-headers")

    def test_unknown_nested_keys_boolean_counts_and_nonfinite_times_reject(self):
        for changes in ({"privateURL":"secret"}, {"restoreAttempts":True}, {"clickToUnlockMs":float("nan")},
                        {"servedScriptSHA256":{"inspector":"a"*64,"foreign":"private"}}):
            value=self.value();value["cases"][0]["data"].update(changes)
            with self.assertRaises(ValueError): projection.admit(value,"runtime","restore-headers")

    def test_unattempted_incomplete_and_false_prerequisite_never_green(self):
        for changes in ({"eligible":False}, {"fixtureStopped":False}, {"boundaryFailed":True},
                        {"activeHolds":1}, {"restoreAttempts":2}, {"holdExpired":True}):
            value=self.value();value["cases"][0]["data"].update(changes)
            self.assertEqual(projection.classify(value,"a"*64),"prerequisite-blocked")
        value=self.value()
        value["cases"][0]["data"]["assertions"]["actual-restore"]={"attempted":True,"completed":False,"passed":None}
        self.assertEqual(projection.classify(value,"a"*64),"prerequisite-blocked")

    def test_retry_unknown_errors_and_missing_soft_error_multiset_block(self):
        for changes in ({"retry":1},{"unknownErrorCount":1},{"assertionErrorsExact":False},
                        {"knownAssertionErrorIDs":["foreign"]},{"totalErrorCount":1}):
            value=self.value();value["cases"][0].update(changes)
            self.assertEqual(projection.classify(value,"a"*64),"prerequisite-blocked")

    def red(self):
        value=self.value();value["runnerStatus"]="failed"
        for row in value["cases"]:
            row["outcome"]="failed";row["data"]["clickToUnlockMs"]=None
            for key in ("editor-released-by-45s","finite-truthful-outcome"):
                row["data"]["assertions"][key]["passed"]=False
            row["failedAssertions"]=["editor-released-by-45s","finite-truthful-outcome"]
            row["knownAssertionErrorIDs"]=list(row["failedAssertions"]);row["totalErrorCount"]=2
        return value

    def test_only_settled_eligible_deadline_outcome_failure_is_intended_red(self):
        value=projection.admit(self.red(),"runtime","restore-headers")
        self.assertEqual(projection.classify(value,"a"*64),"confirmed-restore-deadline-red")
        value["cases"][0]["data"]["historyOnce"]=False
        self.assertEqual(projection.classify(value,"a"*64),"prerequisite-blocked")

    def test_claimed_timely_unlock_conflicting_with_deadline_failure_blocks(self):
        value=self.red();value["cases"][0]["data"]["clickToUnlockMs"]=30000
        self.assertEqual(projection.classify(value,"a"*64),"prerequisite-blocked")

    def test_inspection_body_has_its_own_terminal_and_restore204(self):
        value=self.value("restore-inspect-body")
        for row in value["cases"]: row["data"]["restoreBodyDelivered"]=False
        self.assertEqual(projection.classify(value,"a"*64),"focused-restore-browser-green")
        value["cases"][0]["data"]["inspectTerminal"]="pending"
        self.assertEqual(projection.classify(value,"a"*64),"prerequisite-blocked")
        value=self.value("restore-inspect-body");value["cases"][0]["data"]["restoreTerminal"]="request-failed"
        self.assertEqual(projection.classify(value,"a"*64),"prerequisite-blocked")

    def test_cancellation_is_distinct_from_delivery(self):
        value=self.value()
        for row in value["cases"]:
            row["data"].update(restoreTerminal="request-failed",restoreBodyDelivered=False,
                               restoreResponseDelivered=False,restoreClientCancelled=True,restoreFailureCode="aborted")
        self.assertEqual(projection.classify(value,"a"*64),"focused-restore-browser-green")
        value["cases"][0]["data"]["restoreBodyDelivered"]=True
        self.assertEqual(projection.classify(value,"a"*64),"prerequisite-blocked")

    def test_source_drift_mixed_red_green_and_wrong_runner_result_block(self):
        self.assertEqual(projection.classify(self.value(),"b"*64),"prerequisite-blocked")
        value=self.red();value["cases"][0]=self.case(value["cases"][0]["caseID"])
        self.assertEqual(projection.classify(value,"a"*64),"prerequisite-blocked")
        value=self.value();value["runnerStatus"]="failed"
        self.assertEqual(projection.classify(value,"a"*64),"prerequisite-blocked")

    def blocked_variants(self):
        from test_campaign_r06_restore_runtime_sources import RestoreSourceControls
        helper=RestoreSourceControls()
        unreached=__import__("campaign_r06_restore_runtime_artifacts").admit_bundle(helper.bundle(),"a"*40,"b"*40)
        unreached={key:value for key,value in unreached.items() if key!="artifact-manifest.json"}
        full=helper.green_bundle();full["receipt.json"]["classification"]="prerequisite-blocked"
        partial=copy.deepcopy(full)
        partial["receipt.json"].update(sourceUnchanged=False,compiledBinaryUnchanged=False,compiledBinaryAfter=None,
                                       failure={"stage":"public-controls","class":"ValueError"})
        partial["receipt.json"]["phases"]["publicControls"].update(exitCode=1,stopReason="process-boundary",captureSettled=False)
        partial["results.json"]["controls"]["terminals"].pop(next(iter(partial["results.json"]["controls"]["terminals"])))
        partial["results.json"]["controls"].update(green=False,ownerComplete=True,packageTerminal=None)
        return (unreached,partial,full)

    def test_actual_unreached_partial_and_full_blocked_artifacts_are_closed(self):
        import campaign_r06_restore_runtime_artifacts as artifacts
        for values in self.blocked_variants():
            artifacts.admit_bundle(artifacts.seal(values),"a"*40,"b"*40)
            for key in ("receipt.json","results.json","source-manifest.json"):
                bad=copy.deepcopy(values);bad[key]["privateCookie"]="fictional private"
                with self.assertRaises(ValueError):artifacts.admit_bundle(artifacts.seal(bad),"a"*40,"b"*40)

    def test_unknown_nested_blocked_values_and_private_diagnostics_are_rejected(self):
        import campaign_r06_restore_runtime_artifacts as artifacts
        full=self.blocked_variants()[-1]
        mutations=[("receipt.json",("limits",),"privateURL"),("receipt.json",("phases","compile"),"privateCookie"),
                   ("receipt.json",("dependenciesBefore","executables","go"),"privateToken"),
                   ("results.json",("controls","owner",next(iter(full["results.json"]["controls"]["owner"]))),"privateCookie"),
                   ("source-manifest.json",("ledger","canonical"),"privateURL"),
                   ("source-manifest.json",("runtimePackage",),"privateToken")]
        for file,path,key in mutations:
            bad=copy.deepcopy(full);target=bad[file]
            for part in path:target=target[part]
            target[key]="fictional private"
            with self.assertRaises(ValueError):artifacts.admit_bundle(artifacts.seal(bad),"a"*40,"b"*40)
        for key in ("pendingIsolation","coverageLimit"):
            bad=copy.deepcopy(full);bad["receipt.json"][key]="fictional private URL"
            with self.assertRaises(ValueError):artifacts.admit_bundle(artifacts.seal(bad),"a"*40,"b"*40)
        for key in ("stage","class"):
            bad=copy.deepcopy(full);bad["receipt.json"]["failure"]={"stage":"compile","class":"ValueError"}
            bad["receipt.json"]["failure"][key]="fictional private"
            with self.assertRaises(ValueError):artifacts.admit_bundle(artifacts.seal(bad),"a"*40,"b"*40)

    def test_green_receipt_top_level_is_closed_and_known_texts_are_fixed(self):
        from test_campaign_r06_restore_runtime_sources import RestoreSourceControls
        import campaign_r06_restore_runtime_artifacts as artifacts
        full=RestoreSourceControls().green_bundle()
        mutations=({"privateCookie":"fictional private"},{"schemaVersion":True},
                   {"limits":{**full["receipt.json"]["limits"],"workers":True}},
                   {"startedUTC":"fictional private"},{"failure":{"stage":"compile","class":"fictional private"}},{"failure":{"stage":"compile","class":"ValueError"}})
        for changes in mutations:
            bad=copy.deepcopy(full);bad["receipt.json"].update(changes)
            with self.assertRaises(ValueError):artifacts.admit_bundle(artifacts.seal(bad),"a"*40,"b"*40)

    def test_blocked_known_phase_and_control_values_are_bounded(self):
        import campaign_r06_restore_runtime_artifacts as artifacts
        full=self.blocked_variants()[-1]
        for changes in ({"activeSeconds":1},{"stopReason":"fictional private"},{"durationSeconds":float("inf")},
                        {"processLaunched":1},{"capturedBytes":-1},{"exitCode":True}):
            bad=copy.deepcopy(full);bad["receipt.json"]["phases"]["compile"].update(changes)
            with self.assertRaises(ValueError):artifacts.admit_bundle(artifacts.seal(bad),"a"*40,"b"*40)
        for changes in ({"packageTerminal":"fictional private"},{"invalidEventCount":True},{"started":["foreign"]},
                        {"ownerComplete":False},{"green":False}):
            bad=copy.deepcopy(full);bad["results.json"]["controls"].update(changes)
            with self.assertRaises(ValueError):artifacts.admit_bundle(artifacts.seal(bad),"a"*40,"b"*40)

    def test_blocked_source_and_dependency_partial_shapes_are_closed(self):
        import campaign_r06_restore_runtime_artifacts as artifacts
        full=self.blocked_variants()[-1]
        for key in ("inputs","ledger","runtimePackage"):
            bad=copy.deepcopy(full);bad["source-manifest.json"].pop(key)
            with self.assertRaises(ValueError):artifacts.admit_bundle(artifacts.seal(bad),"a"*40,"b"*40)
        for changes in ({"installedClosure":{}},{"limits":"fictional private"},{"executables":None}):
            bad=copy.deepcopy(full);bad["receipt.json"]["dependenciesBefore"].update(changes)
            with self.assertRaises(ValueError):artifacts.admit_bundle(artifacts.seal(bad),"a"*40,"b"*40)
        bad=copy.deepcopy(full);bad["receipt.json"]["compiledBinaryAfter"]={"private":"fictional private"}
        with self.assertRaises(ValueError):artifacts.admit_bundle(artifacts.seal(bad),"a"*40,"b"*40)

    def test_accepted_artifacts_require_nonnull_inspector_source_digest(self):
        from test_campaign_r06_restore_runtime_sources import RestoreSourceControls
        import campaign_r06_restore_runtime_artifacts as artifacts
        helper=RestoreSourceControls()
        for values in (helper.green_bundle(),helper.browser_bundle()):
            omitted=copy.deepcopy(values);omitted["receipt.json"].pop("inspectorSHA256")
            with self.assertRaises(ValueError):artifacts.admit_bundle(artifacts.seal(omitted),"a"*40,"b"*40)
            absent=copy.deepcopy(values);absent["receipt.json"]["inspectorSHA256"]=None
            if absent["results.json"]["browser"] is not None:
                for case in absent["results.json"]["browser"]["cases"]:
                    case["data"]["servedScriptSHA256"]["inspector"]=None
            with self.assertRaises(ValueError):artifacts.admit_bundle(artifacts.seal(absent),"a"*40,"b"*40)

    def test_browser_served_null_cannot_bind_an_accepted_null_inspector(self):
        from test_campaign_r06_restore_runtime_sources import RestoreSourceControls
        import campaign_r06_restore_runtime_artifacts as artifacts
        values=RestoreSourceControls().browser_bundle()
        values["receipt.json"]["inspectorSHA256"]=None
        for case in values["results.json"]["browser"]["cases"]:
            case["data"]["servedScriptSHA256"]["inspector"]=None
        with self.assertRaises(ValueError):artifacts.admit_bundle(artifacts.seal(values),"a"*40,"b"*40)


class RestoreNetworkDiagnosticControls(unittest.TestCase):
    def test_closed_failure_codes_do_not_admit_delivery_mismatch(self):
        value=RestoreBrowserProjectionControls().value()
        for row in value['cases']:
            row['data'].update(restoreFailureCode='none',inspectFailureCode='none')
        data=value['cases'][0]['data']
        data.update(restoreTerminal='request-failed',restoreFailureCode='aborted',restoreClientCancelled=False)
        projection.admit(value,'runtime','restore-headers')
        self.assertEqual(projection.classify(value,'a'*64),'prerequisite-blocked')
        for code in ('private https://fictional.invalid/path?token=fixture',True,'x'*1000):
            bad=copy.deepcopy(value);bad['cases'][0]['data']['restoreFailureCode']=code
            with self.assertRaises(ValueError):projection.admit(bad,'runtime','restore-headers')

    def test_actual_request_observer_exports_only_closed_failure_identity(self):
        import json
        import subprocess
        from pathlib import Path
        network=Path(__file__).resolve().parents[1]/'e2e/subtitle-restore-recovery-network.ts'
        harness="""import {EventEmitter} from 'node:events';
const {observeRestore}=await import(process.argv[1]);
const values=[];
for(const [error,expected] of [['net::ERR_ABORTED','aborted'],['net::ERR_CONTENT_LENGTH_MISMATCH','content-length'],['net::ERR_CONTENT_DECODING_FAILED','decoding'],['net::ERR_CONNECTION_RESET','connection-reset'],['net::ERR_CONNECTION_CLOSED','connection-closed'],['net::ERR_EMPTY_RESPONSE','empty-response'],['private https://fictional.invalid/?secret=fixture','unclassified']]){
 const page=new EventEmitter(),result={};
 const observed=observeRestore(page,'https://127.0.0.1:1234','aaaaaaaaaaaaaaaa',result);
 const request={method:()=> 'POST',url:()=> 'https://127.0.0.1:1234/api/v1/subtitle-library/aaaaaaaaaaaaaaaa/restore',failure:()=>({errorText:error})};
 page.emit('request',request);page.emit('requestfailed',request);
 values.push([result.restoreFailureCode,expected,result.inspectFailureCode,result.restoreTerminal]);
 observed.close();if(page.eventNames().length)throw new Error('observers not removed');
}
process.stdout.write(JSON.stringify(values));"""
        result=subprocess.run(['node','--experimental-strip-types','--input-type=module','-e',harness,network.as_uri()],
                              capture_output=True,timeout=5,check=False)
        self.assertEqual(result.returncode,0,'owned observer diagnostic harness failed')
        values=json.loads(result.stdout)
        self.assertEqual(len(values),7)
        for actual,expected,untouched,terminal in values:
            self.assertEqual(actual,expected)
            self.assertEqual(untouched,'none')
            self.assertEqual(terminal,'request-failed')
