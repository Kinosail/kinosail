"""Failure-first JSON/process projection controls, not real application evidence."""
import json
import unittest

from campaign_r18_document_events import Projection

PACKAGE = "github.com/MikeO7/kinosail-player/internal/server"
NAME = "TestHomeAssistantDocumentTargetsRemainIndependentThroughPublicApp"


def event(action, test=None, **fields):
    value = {"Time": "2026-10-05T19:00:00Z", "Action": action, "Package": PACKAGE}
    if test is not None:
        value["Test"] = test
    return json.dumps(value | fields).encode()


class EventControls(unittest.TestCase):
    def test_non_json_invalid_utf8_duplicate_keys_and_nan_block_capture(self):
        for raw in (b"not json", b"\xff", b'{"Action":"start","Action":"fail"}',
                    b'{"Action":"pass","Elapsed":NaN}', b"[]" ):
            with self.subTest(raw=raw):
                projection = Projection()
                projection.consume(raw)
                self.assertTrue(projection.prerequisite)

    def test_wrong_package_unknown_test_and_unknown_action_block_capture(self):
        rows = (event("start", Package="other-package"),
                event("run", "UnknownTest"), event("bench", NAME))
        for raw in rows:
            with self.subTest(raw=raw):
                projection = Projection()
                projection.consume(event("start"))
                projection.consume(raw)
                self.assertTrue(projection.prerequisite)

    def test_terminal_without_run_and_concurrent_runs_are_rejected(self):
        projection = Projection()
        projection.consume(event("start"))
        projection.consume(event("pass", NAME))
        self.assertTrue(projection.prerequisite)
        projection = Projection()
        projection.consume(event("start"))
        projection.consume(event("run", NAME))
        projection.consume(event("run", NAME))
        self.assertTrue(projection.prerequisite)

    def test_third_run_skip_and_pause_cannot_fake_repetitions(self):
        for action in ("skip", "pause"):
            with self.subTest(action=action):
                projection = Projection()
                projection.consume(event("start"))
                projection.consume(event("run", NAME))
                projection.consume(event(action, NAME))
                self.assertTrue(projection.prerequisite)
        projection = Projection()
        projection.consume(event("start"))
        for _repeat in range(2):
            projection.consume(event("run", NAME))
            projection.consume(event("pass", NAME))
        projection.consume(event("run", NAME))
        self.assertTrue(projection.prerequisite)

    def test_only_exact_source_line_and_absent_http_status_count(self):
        cases = (
            ("home_assistant_document_targets_test.go", 11, 405, 405),
            ("home_assistant_document_targets_test.go", 11, 404, 404),
            ("home_assistant_document_targets_helpers_test.go", 152, 405, None),
            ("home_assistant_document_targets_test.go", 30, 405, None),
            ("home_assistant_document_targets_test.go", 11, 403, None),
        )
        for file, line, status, expected in cases:
            with self.subTest(file=file, line=line, status=status):
                projection = Projection()
                projection.consume(event("start"))
                projection.consume(event("run", NAME))
                output = f"    {file}:{line}: R18 claim-route prerequisite: HTTP {status}, expected 201\n"
                projection.consume(event("output", NAME, Output=output))
                projection.consume(event("fail", NAME))
                self.assertEqual(projection.result()["records"][0]["absenceHTTP"], expected)

    def test_each_test_requires_its_own_helper_skipped_caller_location(self):
        names = (
            (NAME, 11),
            ("TestHomeAssistantDocumentSiblingCannotOverwriteReleaseOrDrain", 30),
            ("TestHomeAssistantDocumentReleaseKeepsCandidateAndRejectsRetiredClaim", 53),
        )
        for name, line in names:
            with self.subTest(name=name):
                projection = Projection()
                projection.consume(event("start"))
                projection.consume(event("run", name))
                output = f"    home_assistant_document_targets_test.go:{line}: R18 claim-route prerequisite: HTTP 405, expected 201\n"
                projection.consume(event("output", name, Output=output))
                projection.consume(event("fail", name))
                self.assertEqual(projection.result()["records"][0]["absenceHTTP"], 405)
                self.assertEqual(projection.result()["records"][0]["sourceLocation"], f"home_assistant_document_targets_test.go:{line}")

    def test_unexpected_output_cannot_be_admitted_as_complete(self):
        projection = Projection()
        projection.consume(event("start"))
        projection.consume(event("output", Output="unexpected output\n"))
        self.assertTrue(projection.prerequisite)

    def test_output_status_cannot_disagree_with_terminal_event(self):
        projection = Projection()
        projection.consume(event("start"))
        projection.consume(event("run", NAME))
        projection.consume(event("output", NAME, Output=f"--- FAIL: {NAME} (0.01s)\n"))
        projection.consume(event("pass", NAME))
        self.assertTrue(projection.prerequisite)
        projection = Projection()
        projection.consume(event("start"))
        projection.consume(event("output", Output="FAIL\n"))
        projection.consume(event("pass"))
        self.assertTrue(projection.prerequisite)

    def test_raw_credentials_urls_and_bodies_are_never_exported(self):
        projection = Projection()
        projection.consume(event("start"))
        projection.consume(event("run", NAME))
        projection.consume(event("output", NAME, Output="PRIVATE_TOKEN_SENTINEL https://private.invalid/?secret=PRIVATE_BODY_SENTINEL\n"))
        projection.consume(event("pass", NAME))
        exported = repr(projection.result())
        self.assertNotIn("PRIVATE_TOKEN_SENTINEL", exported)
        self.assertNotIn("PRIVATE_BODY_SENTINEL", exported)
        self.assertNotIn("private.invalid", exported)

    def test_unknown_event_fields_bad_time_and_elapsed_types_are_rejected(self):
        for fields in ({"Secret": "private"}, {"Time": 1}, {"Time": "invalid"},
                       {"Elapsed": True}, {"Elapsed": -1}, {"Elapsed": float("inf")}):
            with self.subTest(fields=fields):
                projection = Projection()
                projection.consume(event("start", **fields))
                self.assertTrue(projection.prerequisite)

    def test_duplicate_package_start_terminal_and_event_after_close_are_rejected(self):
        projection = Projection()
        projection.consume(event("start"))
        projection.consume(event("start"))
        self.assertTrue(projection.prerequisite)
        projection = Projection()
        projection.consume(event("start"))
        projection.consume(event("fail"))
        projection.consume(event("output", Output="after terminal\n"))
        self.assertTrue(projection.prerequisite)

    def test_incomplete_selected_records_and_fatal_output_never_close(self):
        projection = Projection()
        projection.consume(event("start"))
        projection.consume(event("run", NAME))
        projection.consume(event("pass", NAME))
        self.assertFalse(projection.result()["closed"])
        projection.consume(event("output", Output="panic: fixture failed\n"))
        self.assertTrue(projection.prerequisite)

    def test_line_event_and_output_bounds_are_enforced(self):
        projection = Projection()
        projection.consume(b"x" * 65537)
        self.assertTrue(projection.prerequisite)
        projection = Projection()
        projection.consume(event("start"))
        projection.consume(event("output", Output="x" * 65536))
        self.assertTrue(projection.prerequisite)
        projection = Projection()
        projection.consume(event("start"))
        for _line in range(10000):
            projection.consume(event("output", Output=""))
        self.assertTrue(projection.prerequisite)


if __name__ == "__main__":
    unittest.main()
