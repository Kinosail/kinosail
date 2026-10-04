"""Fictional admission controls only; never invoke Node, Go or a browser."""
import copy
import importlib.util
import io
import json
from pathlib import Path
import unittest
from unittest import mock

PATH = Path(__file__).with_name("campaign-q14-public.py")
SPEC = importlib.util.spec_from_file_location("q14_driver", PATH)
DRIVER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(DRIVER)
FILES = ["browse-return.spec.ts", "browse-return-cold.spec.ts", "browse-return-bfcache.spec.ts"]
TITLES = [f"visible Player Back preserves Movies query, offset, extent, focus and scroll at {width}px" for width in (390, 1440)]
COLLECTED = [(FILES[0], title) for title in TITLES] + [
    (FILES[0], "HTMX title-letter Back fetches current browse data and restores extent without a native pageshow"),
    *[(FILES[0], f"visible Player Back restores original Shows action via {action}") for action in ("direct Play", "details and episode")],
    *[(FILES[1], f"cold native Back restores later Movie cards at {width}px") for width in (390, 1440)],
    (FILES[1], "live query uses current URL rather than the document's initial browse key"),
    (FILES[2], "native BFCache preserves loaded Movie DOM without repeated continuation"),
]


def admit(value, collection=False):
    if hasattr(DRIVER, "admit"):
        return DRIVER.admit(value, collection)
    return value if DRIVER.safe_report(value, 9 if collection else 2, collection) else None


def report(collection=False):
    pairs = COLLECTED if collection else [(FILES[0], title) for title in TITLES]
    result = {"schemaVersion": 1, "status": "passed", "errors": [], "collected": [{"file": file, "title": title} for file, title in pairs], "cases": []}
    href = "/watch/0123456789abcdef"
    state = {"path": "/", "values": {"q": "Return Movie", "offset": "4", "limit": "4", "sort": "title", "view": "movies"}, "profile": "local-owner", "titles": ["Return Movie 25"], "hrefs": [href], "focused": href, "scroll": {"x": 0, "y": 500}, "selected": {"href": href, "top": 100, "bottom": 200}}
    for file, title in ([] if collection else pairs):
        attachments = [{"name": name, "bytes": 128, "sha256": "0" * 64, "observation": {"state": copy.deepcopy(state), "peer": []}} for name in ("before-state", "player-state", "returned-state")]
        attachments.append({"name": "served-browse-asset", "bytes": 128, "sha256": "0" * 64, "observation": {"src": "/static/main.kinosail.bundle.js?v=35", "bytes": 1000, "sha256": "1" * 64}})
        result["cases"].append({"file": file, "title": title, "status": "passed", "retry": 0, "durationMs": 1, "expectedStatus": "passed", "failures": [], "attachments": attachments})
    return result


class AdmissionTests(unittest.TestCase):
    def test_fixed_collection_and_primary_reports_are_admitted(self):
        for collection in (False, True):
            value = report(collection)
            self.assertEqual(admit(value, collection), value)

    def test_missing_or_false_global_error_list_is_rejected(self):
        for value in (None, False, {}, "secret"):
            candidate = report(True)
            candidate["errors"] = value
            self.assertIsNone(admit(candidate, True))
        candidate.pop("errors")
        self.assertIsNone(admit(candidate, True))

    def test_non_list_and_non_dictionary_containers_are_rejected(self):
        for key in ("collected", "cases"):
            for value in (False, {}, None, ["secret"]):
                candidate = report(True)
                candidate[key] = value
                self.assertIsNone(admit(candidate, True))

    def test_arbitrary_extra_fields_never_survive_admission(self):
        for target in ("report", "case", "attachment", "state"):
            candidate = report()
            item = {"report": candidate, "case": candidate["cases"][0], "attachment": candidate["cases"][0]["attachments"][0], "state": candidate["cases"][0]["attachments"][0]["observation"]["state"]}[target]
            item["rawURLTokenError"] = "https://fictional.invalid/secret"
            self.assertIsNone(admit(candidate))

    def test_renamed_duplicate_or_additional_journeys_are_rejected(self):
        for mutate in (lambda value: value["collected"][0].update(title="different"), lambda value: value["collected"].append(value["collected"][0]), lambda value: value["collected"].__setitem__(1, value["collected"][0])):
            candidate = report(True)
            mutate(candidate)
            self.assertIsNone(admit(candidate, True))

    def test_false_numeric_fields_and_unbounded_data_are_rejected(self):
        candidate = report()
        candidate["schemaVersion"] = True
        self.assertIsNone(admit(candidate))
        for key, value in (("retry", False), ("durationMs", -1), ("durationMs", 1000000)):
            candidate = report()
            candidate["cases"][0][key] = value
            self.assertIsNone(admit(candidate))
        candidate = report()
        candidate["cases"][0]["attachments"][0]["observation"]["state"]["titles"] = ["Return Movie 25"] * 65
        self.assertIsNone(admit(candidate))

    def test_bad_routes_queries_and_attachment_names_are_rejected(self):
        for target in ("route", "query", "attachment"):
            candidate = report()
            item = candidate["cases"][0]["attachments"][0]
            if target == "route":
                item["observation"]["state"]["focused"] = "https://fictional.invalid/secret"
            elif target == "query":
                item["observation"]["state"]["values"]["q"] = "private content"
            else:
                item["name"] = ["unhashable"]
            self.assertIsNone(admit(candidate))

    def test_go_package_panic_or_missing_terminal_cannot_certify_completion(self):
        boundary = getattr(DRIVER, "go_boundary", lambda _raw: "incomplete")
        passing = b"--- PASS: TestBrowseReturnBrowserJourney (1.00s)\nPASS\n"
        failing = b"--- FAIL: TestBrowseReturnBrowserJourney (1.00s)\nFAIL\n"
        self.assertEqual(boundary(passing), "completed-pass")
        self.assertEqual(boundary(failing), "completed-fail")
        for raw in (b"PASS\n", passing + b"panic: fictional fixture fault\n", b"FAIL package [build failed]\n", failing + b"fatal error: fictional fixture fault\n"):
            self.assertEqual(boundary(raw), "incomplete")

    def test_no_empty_or_skipped_green_is_complete(self):
        complete = getattr(DRIVER, "complete", lambda value: DRIVER.safe_report(value, 2))
        for mutate in (lambda value: value.update(cases=[]), lambda value: value["cases"][0].update(status="skipped"), lambda value: value["cases"][0].update(attachments=[])):
            candidate = report()
            mutate(candidate)
            self.assertFalse(complete(admit(candidate)))


class FakeProcess:
    def __init__(self, raw):
        self.stdout, self.pid, self.returncode = io.BytesIO(raw), 123456789, 0
    def wait(self, timeout=None):
        return self.returncode
    def poll(self):
        return self.returncode


class CaptureTests(unittest.TestCase):
    def run_fake(self, raw):
        with mock.patch.object(DRIVER.subprocess, "Popen", return_value=FakeProcess(raw)), mock.patch.object(DRIVER, "settle_group", return_value=True), mock.patch.object(DRIVER, "group_signal"):
            return DRIVER.run(["fictional-process"], DRIVER.ROOT, {}, 15, "collection", 10)

    def test_invalid_marker_payload_is_not_returned_for_artifacts(self):
        candidate = report(True)
        candidate["token"] = "fictional-secret"
        receipt, accepted = self.run_fake(b"Q14_PROOF_RESULT " + json.dumps(candidate).encode() + b"\n")
        self.assertIsNone(accepted)
        self.assertNotIn("fictional-secret", json.dumps(receipt))

    def test_output_overflow_is_bounded_and_not_admitted(self):
        cap = DRIVER.OUTPUT_CAP
        receipt, accepted = self.run_fake(b"x" * (cap + 1))
        self.assertTrue(receipt["outputOverflow"])
        self.assertEqual(receipt["outputBytes"], cap + 1)
        self.assertIsNone(accepted)


if __name__ == "__main__":
    unittest.main()
