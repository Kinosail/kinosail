"""Failure-first isolated controls; synthetic evidence is never public Go proof."""
import copy
import unittest

from campaign_r18_document_admission import admit, settled
from campaign_r18_document_events import Projection
from campaign_r18_document_sources import valid_handoff

NAMES = (
    "TestHomeAssistantDocumentTargetsRemainIndependentThroughPublicApp",
    "TestHomeAssistantDocumentSiblingCannotOverwriteReleaseOrDrain",
    "TestHomeAssistantDocumentReleaseKeepsCandidateAndRejectsRetiredClaim",
    "TestHomeAssistantDocumentNativeStrictContractRemainsUnchanged",
)
PACKAGE = "github.com/MikeO7/kinosail-player/internal/server"
CALL_LINES = (11, 30, 53)


def evidence(baseline=True):
    projection = Projection()
    import json
    def send(action, test=None, output=None):
        event = {"Time": "2026-10-05T19:00:00Z", "Action": action, "Package": PACKAGE}
        if test is not None:
            event["Test"] = test
        if output is not None:
            event["Output"] = output
        projection.consume(json.dumps(event).encode())
    send("start")
    for _repeat in range(2):
        for index, name in enumerate(NAMES):
            send("run", name)
            if baseline and index < 3:
                send("output", name, f"    home_assistant_document_targets_test.go:{CALL_LINES[index]}: R18 claim-route prerequisite: HTTP 405, expected 201\n")
                send("fail", name)
            else:
                send("pass", name)
    send("fail" if baseline else "pass")
    return projection.result()


def phase(code=1):
    return {"exitCode": code, "stopReason": None, "ownedProcessExited": True,
            "ownedGroupSettled": True, "captureSettled": True}


def identity():
    return {"sourceUnchanged": True, "toolchainUnchanged": True,
            "binaryUnchanged": True, "exactInvocation": True,
            "formattingVerified": True, "handoffVerified": True}


class AdmissionControls(unittest.TestCase):
    def test_exact_six_absence_and_two_native_controls_are_prerequisites_only(self):
        result = admit(evidence(), phase(), identity(), "baseline")
        self.assertEqual(result["classification"], "route-prerequisite-absence")
        self.assertEqual(result["count"], 8)
        self.assertEqual(result["nativePassed"], 2)
        self.assertEqual(result["absentClaimRecords"], 6)
        self.assertFalse(result["incumbentDefectRED"])
        self.assertFalse(result["r18FeatureAccepted"])

    def test_candidate_requires_all_eight_complete_passes(self):
        result = admit(evidence(False), phase(0), identity(), "candidate")
        self.assertEqual(result["classification"], "public-document-contract-green")
        self.assertEqual(result["count"], 8)
        self.assertFalse(result["r18FeatureAccepted"])

    def test_baseline_cannot_credit_candidate_success_or_native_failure(self):
        self.assertEqual(admit(evidence(False), phase(0), identity(), "baseline")["classification"], "prerequisite-blocked")
        data = evidence()
        data["records"][-1]["status"] = "fail"
        self.assertEqual(admit(data, phase(), identity(), "baseline")["classification"], "prerequisite-blocked")

    def test_candidate_cannot_credit_missing_route_as_defect_red(self):
        result = admit(evidence(), phase(), identity(), "candidate")
        self.assertEqual(result["classification"], "prerequisite-blocked")
        self.assertFalse(result["incumbentDefectRED"])

    def test_missing_or_extra_record_cannot_be_admitted(self):
        for change in ("missing", "extra"):
            with self.subTest(change=change):
                data = evidence()
                if change == "missing":
                    data["records"].pop()
                else:
                    data["records"].append(copy.deepcopy(data["records"][0]))
                self.assertEqual(admit(data, phase(), identity(), "baseline")["classification"], "prerequisite-blocked")

    def test_wrong_test_occurrence_or_failure_frame_cannot_be_admitted(self):
        for field, value in (("test", "UnknownTest"), ("occurrence", 3),
                             ("absenceHTTP", 403), ("errorFrames", 2)):
            with self.subTest(field=field):
                data = evidence()
                data["records"][0][field] = value
                self.assertEqual(admit(data, phase(), identity(), "baseline")["classification"], "prerequisite-blocked")

    def test_unknown_export_fields_cannot_escape_closed_schema(self):
        data = evidence()
        data["records"][0]["rawBody"] = "PRIVATE_AUTH_SENTINEL"
        result = admit(data, phase(), identity(), "baseline")
        self.assertEqual(result["classification"], "prerequisite-blocked")
        self.assertNotIn("PRIVATE_AUTH_SENTINEL", repr(result))

    def test_each_integrity_prerequisite_is_mandatory(self):
        for name in identity():
            with self.subTest(name=name):
                checks = identity()
                checks[name] = False
                self.assertEqual(admit(evidence(), phase(), checks, "baseline")["classification"], "prerequisite-blocked")

    def test_incomplete_or_interrupted_owned_process_is_blocked(self):
        for name, value in (("ownedProcessExited", False), ("ownedGroupSettled", False),
                            ("captureSettled", False), ("stopReason", "external-timeout"),
                            ("stopReason", "interrupted"), ("exitCode", None),
                            ("exitCode", -9), ("exitCode", True)):
            with self.subTest(name=name, value=value):
                execution = phase()
                execution[name] = value
                self.assertFalse(settled(execution, (0, 1)))
                self.assertEqual(admit(evidence(), execution, identity(), "baseline")["classification"], "prerequisite-blocked")

    def test_exit_status_must_match_complete_package_result(self):
        self.assertEqual(admit(evidence(), phase(0), identity(), "baseline")["classification"], "prerequisite-blocked")
        self.assertEqual(admit(evidence(False), phase(1), identity(), "candidate")["classification"], "prerequisite-blocked")

    def test_unknown_mode_and_incomplete_package_are_blocked(self):
        self.assertEqual(admit(evidence(), phase(), identity(), "all")["classification"], "prerequisite-blocked")
        data = evidence()
        data["packageTerminal"] = None
        self.assertEqual(admit(data, phase(), identity(), "baseline")["classification"], "prerequisite-blocked")

    def test_boolean_occurrence_and_nonboolean_integrity_are_rejected(self):
        data = evidence()
        data["records"][0]["occurrence"] = True
        self.assertEqual(admit(data, phase(), identity(), "baseline")["classification"], "prerequisite-blocked")
        checks = identity()
        checks["sourceUnchanged"] = 1
        self.assertEqual(admit(evidence(), phase(), checks, "baseline")["classification"], "prerequisite-blocked")



class HandoffControls(unittest.TestCase):
    def handoff(self):
        return {"schemaVersion": 1,
                "baseRevision": "bd1ea2e148787ae8d4a0a46640bc2a965e10fe6a",
                "baseTree": "6070e86cc8a3522810218d311cc2d5d6246f02d0",
                "revision": "a" * 40, "tree": "b" * 40, "mode": "baseline",
                "sourceSHA256": "c" * 64, "toolchainSHA256": "d" * 64,
                "binary": {"sha256": "e" * 64, "bytes": 100, "mode": 0o700},
                "formattingVerified": True, "modulesVerified": True,
                "compileSettled": True, "inputCount": 7029,
                "compileInvocation": "go-test-c-player-r18-v1",
                "runInvocation": "go-test2json-player-r18-four-twice-v1"}

    def test_exact_handoff_is_required_before_runtime(self):
        self.assertTrue(valid_handoff(self.handoff()))
        for field, value in (("schemaVersion", True), ("compileSettled", 1),
                             ("formattingVerified", False), ("modulesVerified", False),
                             ("sourceSHA256", "bad"), ("inputCount", True),
                             ("baseRevision", "f" * 40), ("mode", "all"),
                             ("compileInvocation", "all-tests")):
            with self.subTest(field=field):
                data = self.handoff()
                data[field] = value
                self.assertFalse(valid_handoff(data))

    def test_unknown_handoff_fields_and_unsafe_binary_shape_are_rejected(self):
        data = self.handoff()
        data["rawCredentials"] = "PRIVATE_AUTH_SENTINEL"
        self.assertFalse(valid_handoff(data))
        for field, value in (("bytes", 0), ("bytes", 128 * 1024 * 1024 + 1),
                             ("bytes", True), ("mode", 0o600), ("mode", True),
                             ("sha256", "invalid")):
            with self.subTest(field=field):
                data = self.handoff()
                data["binary"][field] = value
                self.assertFalse(valid_handoff(data))

if __name__ == "__main__":
    unittest.main()
