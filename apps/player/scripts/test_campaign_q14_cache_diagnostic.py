#!/usr/bin/env python3
"""Bounded diagnostic privacy controls; original journey assertions stay intact."""
import json
import unittest
from unittest import mock
import campaign_q14_admission as ADMISSION
from test_campaign_q14_public import DRIVER, FakeProcess

class CacheDiagnosticTests(unittest.TestCase):
    def value(self):
        return {"schemaVersion": 1, "supported": True, "present": True, "frameCount": 1,
                "reasons": ["response-cache-control-no-store"], "truncated": False, "navigationType": "back_forward"}

    def encoded(self, value):
        return b"Q14_CACHE_DIAGNOSTIC " + json.dumps(value).encode() + b"\n"

    def test_valid_bounded_diagnostic_is_admitted(self):
        value = self.value()
        self.assertEqual(ADMISSION.cache_diagnostic(self.encoded(value)), value)

    def test_unavailable_and_absent_reasons_are_truthful(self):
        for supported in (False, True):
            value = self.value()
            value.update(supported=supported, present=False, frameCount=0, reasons=[])
            self.assertEqual(ADMISSION.cache_diagnostic(self.encoded(value)), value)

    def test_private_fields_and_reason_strings_are_rejected(self):
        for field in ("url", "src", "name", "id", "token", "cookie", "error"):
            value = self.value()
            value[field] = "fictional-private-value"
            self.assertIsNone(ADMISSION.cache_diagnostic(self.encoded(value)))
        for reason in ("https://fictional.invalid/?token=secret", "fictional-private-value", ["unhashable"]):
            value = self.value()
            value["reasons"] = [reason]
            self.assertIsNone(ADMISSION.cache_diagnostic(self.encoded(value)))

    def test_wrong_types_bounds_and_duplicate_codes_are_rejected(self):
        changes = [("schemaVersion", True), ("frameCount", True), ("frameCount", 65), ("frameCount", -1),
                   ("supported", 1), ("present", None), ("truncated", "false"), ("navigationType", "private"),
                   ("reasons", None), ("reasons", ["other"] * 17), ("reasons", ["other", "other"])]
        for key, replacement in changes:
            value = self.value()
            value[key] = replacement
            self.assertIsNone(ADMISSION.cache_diagnostic(self.encoded(value)), key)

    def test_contradictory_unavailable_and_empty_frames_are_rejected(self):
        for changes in ({"supported": False}, {"present": False}, {"frameCount": 0}):
            value = self.value()
            value.update(changes)
            self.assertIsNone(ADMISSION.cache_diagnostic(self.encoded(value)))

    def test_missing_multiple_malformed_and_oversized_markers_are_rejected(self):
        line = self.encoded(self.value())
        for raw in (b"", line + line, b"Q14_CACHE_DIAGNOSTIC not-json\n", b"Q14_CACHE_DIAGNOSTIC " + b"x" * 4097 + b"\n"):
            self.assertIsNone(ADMISSION.cache_diagnostic(raw))

class CacheDiagnosticCaptureTests(unittest.TestCase):
    def captured(self, raw):
        with mock.patch.object(DRIVER.subprocess, "Popen", return_value=FakeProcess(raw)), mock.patch.object(DRIVER, "settle_group", return_value=True), mock.patch.object(DRIVER, "group_signal"):
            return DRIVER.run(["fictional-process"], DRIVER.ROOT, {}, 15, "bfcache-1", 10, "bfcache")

    def test_valid_cache_diagnostic_is_separate_from_product_admission(self):
        value = CacheDiagnosticTests().value()
        receipt, report = self.captured(CacheDiagnosticTests().encoded(value))
        self.assertEqual(receipt["cacheDiagnostic"], value)
        self.assertIsNone(report)

    def test_private_or_duplicate_diagnostic_never_reaches_receipt(self):
        value = CacheDiagnosticTests().value()
        value["url"] = "fictional-private-value"
        bad = CacheDiagnosticTests().encoded(value)
        for raw in (bad, bad + bad):
            receipt, report = self.captured(raw)
            self.assertIsNone(receipt["cacheDiagnostic"])
            self.assertIsNone(report)
            self.assertNotIn("fictional-private-value", json.dumps(receipt))


if __name__ == "__main__":
    unittest.main()
