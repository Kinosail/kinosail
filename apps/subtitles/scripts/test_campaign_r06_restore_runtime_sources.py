"""Failure-first stable source and four-artifact admission boundaries."""
import hashlib
import json
import stat
from types import SimpleNamespace
import unittest
from unittest.mock import patch
import campaign_r06_restore_runtime_sources as sources
import campaign_r06_restore_runtime_artifacts as artifacts

class RestoreSourceControls(unittest.TestCase):
    def info(self, **changes):
        value = dict(st_dev=1, st_ino=2, st_mode=stat.S_IFREG | 0o600, st_size=3,
                     st_mtime_ns=4, st_ctime_ns=5)
        value.update(changes)
        return SimpleNamespace(**value)

    def read(self, chunks=(b"abc", b""), after=None, close=None):
        before = self.info()
        with patch.object(sources.os, "open", return_value=17), \
             patch.object(sources.os, "fstat", side_effect=[before, after or before]), \
             patch.object(sources.os, "read", side_effect=list(chunks)), \
             patch.object(sources.os, "close", side_effect=close), \
             patch.object(sources, "checked_parents", return_value=None):
            return sources.read_bytes("/owned/input", 16)

    def test_stable_descriptor_returns_exact_pin(self):
        self.assertEqual(self.read(), b"abc")
        expected = {"bytes": 3, "sha256": hashlib.sha256(b"abc").hexdigest(),
                    "gitBlob": hashlib.sha1(b"blob 3\0abc").hexdigest()}
        self.assertEqual(sources.pin(b"abc"), expected)

    def test_metadata_drift_checked_close_and_short_read_block(self):
        for after in (self.info(st_ino=3), self.info(st_mtime_ns=6), self.info(st_size=4)):
            with self.assertRaises(ValueError):
                self.read(after=after)
        with self.assertRaises(ValueError):
            self.read(close=OSError("fictional close failure"))
        with self.assertRaises(ValueError):
            self.read(chunks=(b"ab", b""))

    def test_nonregular_symlink_and_oversize_descriptor_reject(self):
        for info in (self.info(st_mode=stat.S_IFLNK | 0o777), self.info(st_mode=stat.S_IFIFO),
                     self.info(st_size=17), self.info(st_size=-1)):
            with patch.object(sources.os, "open", return_value=17), \
                 patch.object(sources.os, "fstat", return_value=info), \
                 patch.object(sources.os, "close"), \
                 patch.object(sources, "checked_parents"), self.assertRaises(ValueError):
                sources.read_bytes("/owned/input", 16)

    def test_parent_failure_occurs_before_descriptor_open(self):
        with patch.object(sources, "checked_parents", side_effect=ValueError("parent")), \
             patch.object(sources.os, "open") as opened, self.assertRaises(ValueError):
            sources.read_bytes("/owned/input", 16)
        opened.assert_not_called()

    def test_read_overflow_is_not_truncated_into_acceptance(self):
        with self.assertRaises(ValueError):
            self.read(chunks=(b"x"*17, b""))

    def test_duplicate_nonfinite_wrong_encoding_and_depth_reject(self):
        for raw in (b'{"a":1,"a":2}', b'{"a":NaN}', b'{"a":Infinity}', b"\xff",
                    b"["*33+b"]"*33, b"x"*1025):
            with self.subTest(raw=raw[:16]), self.assertRaises(ValueError):
                sources.strict_json(raw, 1024)
        self.assertEqual(sources.strict_json(b'{"quoted":"[[[","a":[1]}', 1024),
                         {"quoted": "[[[", "a": [1]})

    def bundle(self):
        receipt = self.base_receipt()
        source = {"schemaVersion":1,"revision":"a"*40,"tree":"b"*40,"status":"unreached"}
        return artifacts.seal({"receipt.json":receipt,
                               "results.json":{"schemaVersion":1,"controls":None,"collection":None,"browser":None},
                               "source-manifest.json":source})

    def test_four_names_and_three_bindings_are_complete(self):
        value = self.bundle()
        self.assertEqual(set(value), {"receipt.json", "results.json", "source-manifest.json", "artifact-manifest.json"})
        self.assertEqual(set(json.loads(value["artifact-manifest.json"])),
                         {"receipt.json", "results.json", "source-manifest.json"})
        admitted = artifacts.admit_bundle(value, "a"*40, "b"*40)
        self.assertEqual(admitted["receipt.json"]["classification"], "prerequisite-blocked")

    def test_artifact_mutation_omission_extra_and_wrong_head_block(self):
        value = self.bundle()
        for altered in ({**value, "receipt.json": value["receipt.json"]+b" "},
                        {k:v for k,v in value.items() if k != "results.json"},
                        {**value, "private.log": b"private"}):
            with self.assertRaises(ValueError):
                artifacts.admit_bundle(altered, "a"*40, "b"*40)
        with self.assertRaises(ValueError):
            artifacts.admit_bundle(value, "c"*40, "b"*40)

    def test_duplicate_manifest_binding_and_oversized_body_block(self):
        value = self.bundle()
        for body in (b'{"receipt.json":{},"receipt.json":{}}', b"x"*(4194304+1)):
            with self.assertRaises(ValueError):
                artifacts.admit_bundle({**value, "artifact-manifest.json": body}, "a"*40, "b"*40)

    def test_seal_rejects_arbitrary_names_and_nonfinite_values(self):
        with self.assertRaises(ValueError):
            artifacts.seal({"private.log": {}})
        with self.assertRaises(ValueError):
            artifacts.seal({"receipt.json": {"x": float("nan")}, "results.json": {},
                            "source-manifest.json": {}})

    def test_metadata_git_uses_owned_exact_settlement_before_returning_bytes(self):
        good={"exitCode":0,"stopReason":None,"ownedProcessExited":True,"ownedGroupSettled":True,"captureSettled":True,"handlersRestored":True}
        def capture(command,active,total,projection,receipt,cwd):
            self.assertEqual(active,5);self.assertEqual(total,10)
            self.assertEqual(command[:2],["git","rev-parse"])
            projection.consume(b"a"*40+b"\n");receipt.update(good)
        with patch.object(sources,"execute_metadata",side_effect=capture):
            self.assertEqual(sources.git("rev-parse","HEAD"),b"a"*40+b"\n")
        for change in ({"stopReason":"active-timeout"},{"ownedGroupSettled":False},{"captureSettled":False}):
            def bad(command,active,total,projection,receipt,cwd):
                projection.consume(b"partial");receipt.update({**good,**change})
            with patch.object(sources,"execute_metadata",side_effect=bad),self.assertRaises(ValueError):
                sources.git("rev-parse","HEAD")

    def test_claimed_green_artifact_without_fixed_runtime_evidence_is_rejected(self):
        values=artifacts.admit_bundle(self.bundle(),"a"*40,"b"*40)
        for classification in ("focused-restore-public-controls-green","focused-restore-browser-green","confirmed-restore-deadline-red","unknown"):
            receipt=dict(values["receipt.json"],classification=classification)
            bodies=artifacts.seal({**{k:v for k,v in values.items() if k!="artifact-manifest.json"},"receipt.json":receipt})
            with self.assertRaises(ValueError):artifacts.admit_bundle(bodies,"a"*40,"b"*40)

    def test_metadata_collector_rejects_overflow_before_partial_return(self):
        collector=sources.MetadataCapture()
        with self.assertRaises(ValueError):collector.consume(b"x"*(8*1024*1024+1))

    def green_bundle(self):
        from test_campaign_r06_restore_runtime_controls import RestoreControlProjectionTests
        content=b"abc";p=sources.pin(content)
        paths=sorted(sources.REQUIRED_INPUTS)
        rows=[{"path":name,"gitMode":"100644",**p} for name in paths]
        canonical=json.dumps(rows,sort_keys=True,separators=(",",":")).encode()
        cp={"bytes":len(canonical),"sha256":sources.pin(canonical)["sha256"]}
        ledger={"schemaVersion":1,"scope":"entire-tracked-source","files":rows,"canonical":cp}
        source={"schemaVersion":1,"revision":"a"*40,"tree":"b"*40,"ledger":ledger,
                "inputs":{name:dict(p) for name in paths},
                "runtimePackage":{"path":sources.QA+"runtime-source-manifest.json",**p}}
        good={"exitCode":0,"stopReason":None,"ownedProcessExited":True,"ownedGroupSettled":True,"captureSettled":True,"handlersRestored":True}
        phase=lambda active,total:{**good,"activeSeconds":active,"totalBoundSeconds":total,
                                  "processLaunched":True,"capturedBytes":0,"durationSeconds":0.5}
        dependency={"executables":{"go":{**p,"mode":0o700}},"installedClosure":None,"limits":"Selected Go executable only; no browser, Node, pnpm or installed browser dependency was used."}
        receipt={**self.base_receipt(),"schemaVersion":1,"revision":"a"*40,"tree":"b"*40,"mode":"restore-browser","id":"R06","suite":"restore-controls",
                 "classification":"focused-restore-public-controls-green","sourceUnchanged":True,"dependenciesUnchanged":True,
                 "compiledBinaryUnchanged":True,"inspectorSHA256":"a"*64,"dependenciesBefore":dependency,"dependenciesAfter":dependency,
                 "compiledBinaryBefore":{**p,"mode":0o700},"compiledBinaryAfter":{**p,"mode":0o700},
                 "sourceBefore":cp,"sourceAfter":cp,"metadataPhases":[phase(5,10) for _ in range(10)],
                 "phases":{"compile":phase(90,95),"publicControls":phase(60,65)}}
        controls=RestoreControlProjectionTests().complete().result()
        return {"receipt.json":receipt,"source-manifest.json":source,
                "results.json":{"schemaVersion":1,"controls":controls,"collection":None,"browser":None}}

    def test_complete_controls_artifact_and_each_omitted_owned_input(self):
        import copy
        values=self.green_bundle()
        artifacts.admit_bundle(artifacts.seal(values),"a"*40,"b"*40)
        for path in sources.REQUIRED_INPUTS:
            bad=copy.deepcopy(values);bad["source-manifest.json"]["inputs"].pop(path)
            with self.assertRaises(ValueError):artifacts.admit_bundle(artifacts.seal(bad),"a"*40,"b"*40)

    def test_green_artifact_security_phase_source_and_dependency_mutations_block(self):
        import copy
        values=self.green_bundle()
        def rejected(value):
            with self.assertRaises(ValueError):artifacts.admit_bundle(artifacts.seal(value),"a"*40,"b"*40)
        for flag in ("sourceUnchanged","dependenciesUnchanged","compiledBinaryUnchanged"):
            bad=copy.deepcopy(values);bad["receipt.json"][flag]=False;rejected(bad)
        for phase in ("compile","publicControls"):
            for flag in ("ownedProcessExited","ownedGroupSettled","captureSettled"):
                bad=copy.deepcopy(values);bad["receipt.json"]["phases"][phase][flag]=False;rejected(bad)
        bad=copy.deepcopy(values);bad["receipt.json"]["metadataPhases"].pop();rejected(bad)
        bad=copy.deepcopy(values);bad["source-manifest.json"]["runtimePackage"]["sha256"]="c"*64;rejected(bad)
        bad=copy.deepcopy(values);bad["receipt.json"]["sourceAfter"]["sha256"]="c"*64;rejected(bad)
        bad=copy.deepcopy(values);bad["results.json"]["controls"]["owner"][sources.NAMES[0]]["R06_RESTORE_OWNER_CURRENT"]=False;rejected(bad)
        bad=copy.deepcopy(values);bad["receipt.json"]["dependenciesBefore"]["executables"]["node"]={"private":"stale"};rejected(bad)

    def browser_bundle(self):
        from test_campaign_r06_restore_runtime_projection import RestoreBrowserProjectionControls
        values=self.green_bundle();receipt=values["receipt.json"];p=sources.pin(b"abc")
        receipt.update(suite="restore-headers",classification="focused-restore-browser-green",inspectorSHA256="a"*64)
        phase=lambda active,total:{**receipt["phases"]["compile"],"activeSeconds":active,"totalBoundSeconds":total}
        receipt["phases"].update(collection=phase(15,20),browser=phase(230,235))
        receipt["dependencyPhasesBefore"]={name:phase(5,10) for name in ("packageResolution","browserResolution")}
        receipt["dependencyPhasesAfter"]={name:phase(5,10) for name in ("packageResolution","browserResolution")}
        rows=[{"package":name,"path":"package.json",**p} for name in ("@playwright/test","playwright","playwright-core")]
        canonical=json.dumps(rows,sort_keys=True,separators=(",",":")).encode()
        closure={"files":rows,"canonical":{"bytes":len(canonical),"sha256":sources.pin(canonical)["sha256"]}}
        dependencies={"executables":{name:{**p,"mode":0o700} for name in ("go","node","pnpm","chromium","headless")},
                      "installedClosure":closure,"limits":"Selected executable and three transitive package roots only; system libraries and whole runner not fingerprinted."}
        receipt["dependenciesBefore"]=dependencies;receipt["dependenciesAfter"]=dependencies
        projection=RestoreBrowserProjectionControls()
        values["results.json"].update(collection=projection.value(mode="collection"),browser=projection.value())
        return values

    def test_complete_browser_artifact_requires_exact_dependency_closure_and_probes(self):
        import copy
        values=self.browser_bundle()
        artifacts.admit_bundle(artifacts.seal(values),"a"*40,"b"*40)
        for key in ("dependencyPhasesBefore","dependencyPhasesAfter"):
            bad=copy.deepcopy(values);bad["receipt.json"][key].pop("browserResolution")
            with self.assertRaises(ValueError):artifacts.admit_bundle(artifacts.seal(bad),"a"*40,"b"*40)
        for changes in ({"files":[]},{"canonical":{"bytes":1,"sha256":"c"*64}},{"unknown":"private"}):
            bad=copy.deepcopy(values)
            for key in ("dependenciesBefore","dependenciesAfter"):bad["receipt.json"][key]["installedClosure"].update(changes)
            with self.assertRaises(ValueError):artifacts.admit_bundle(artifacts.seal(bad),"a"*40,"b"*40)

    def test_controls_artifact_rejects_browser_probe_even_with_go_only_rows(self):
        values=self.green_bundle()
        values["receipt.json"]["dependencyPhasesBefore"]={"packageResolution":values["receipt.json"]["phases"]["compile"]}
        with self.assertRaises(ValueError):artifacts.admit_bundle(artifacts.seal(values),"a"*40,"b"*40)

    def base_receipt(self):
        return {"schemaVersion":1,"id":"R06","mode":"restore-browser","suite":"restore-controls",
                "revision":"a"*40,"tree":"b"*40,"classification":"prerequisite-blocked",
                "sourceUnchanged":False,"dependenciesUnchanged":False,"compiledBinaryUnchanged":False,
                "startedUTC":"2026-10-05T00:00:00+00:00","endedUTC":"2026-10-05T00:00:00+00:00",
                "phases":{},"metadataPhases":[],"dependencyPhasesBefore":{},"dependencyPhasesAfter":{},
                "dependenciesBefore":None,"dependenciesAfter":None,"compiledBinaryBefore":None,"compiledBinaryAfter":None,
                "sourceBefore":None,"sourceAfter":None,"inspectorSHA256":None,
                "limits":{"ownedSettlementReserveSeconds":5,"captureBytes":16777216,"lineBytes":65536,"privateResultBytes":131072,
                          "workers":1,"retries":0,"unlockMilliseconds":45000,"observationGraceMilliseconds":10000},
                "pendingIsolation":"Published 2b816fa8 startup-corrected raw inputs; stock formatter qualification and hosted runtime isolation proof remain pending.",
                "coverageLimit":"Selected sources/executables/package roots only; no universal network, host, system-library or whole-environment absence claim."}

    def test_reader_opens_nonblocking_nofollow_before_descriptor_shape(self):
        def opened(path,flags):
            self.assertTrue(flags & sources.os.O_NONBLOCK)
            self.assertTrue(flags & sources.os.O_NOFOLLOW)
            return 17
        with patch.object(sources,"checked_parents"),patch.object(sources.os,"open",side_effect=opened), \
             patch.object(sources.os,"fstat",return_value=self.info(st_mode=stat.S_IFIFO)), \
             patch.object(sources.os,"close") as closed,self.assertRaises(ValueError):
            sources.read_bytes("/owned/fifo",16)
        closed.assert_called_once_with(17)

    def test_numeric_overflow_at_any_json_depth_is_rejected(self):
        for raw in (b"1e999",b'{"value":1e999}',b'{"nested":[{"value":-1e999}]}'):
            with self.assertRaises(ValueError):sources.strict_json(raw)
        self.assertEqual(sources.strict_json(b'{"finite":[1e2,0.5]}'),{"finite":[100.0,0.5]})
