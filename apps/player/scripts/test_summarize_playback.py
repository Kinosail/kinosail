import copy
import io
import tempfile
from contextlib import redirect_stdout, redirect_stderr
from pathlib import Path
import unittest

from summarize_playback import main, summarize


def fixture():
    sample = {"engine": "platform", "outcome": "ended", "firstFrameMs": 200, "firstProgressMs": 500,
              "seekProgressMs": [300], "seeks": 1, "incompleteSeeks": 0, "stalls": 0, "stallMs": 0, "elapsedMs": 5000}
    return {"cohort": {"device": "test-device", "os": "26", "network": "lan", "outputRoute": "speaker", "cache": "warm",
                       "fixtureSha256": "a" * 64, "appRevision": "abc123", "serverRevision": "abc123"}, "samples": [sample]}


class PlaybackReportTests(unittest.TestCase):
    def test_failures_and_missing_measurements_stay_visible(self):
        document = fixture()
        failed = copy.deepcopy(document["samples"][0])
        failed.update(outcome="error", firstFrameMs=None, firstProgressMs=None, seeks=1, incompleteSeeks=1, seekProgressMs=[])
        document["samples"].append(failed)
        report = summarize(document)["engines"]["platform"]
        self.assertEqual(report["attempts"], 2)
        self.assertEqual(report["errors"], 1)
        self.assertEqual(report["withoutFirstFrame"], 1)
        self.assertEqual(report["firstFrame"]["medianMs"], 200)
        self.assertTrue(report["firstFrame"]["p95ScreeningOnly"])
        self.assertEqual(report["incompleteSeeks"], 1)

    def test_absent_evidence_is_not_zero_latency(self):
        document = fixture()
        document["samples"][0].update(firstFrameMs=None, firstProgressMs=None)
        self.assertIsNone(summarize(document)["engines"]["platform"]["firstFrame"]["p95Ms"])

    def test_malformed_and_conflicting_samples_are_rejected(self):
        for key, value in [("firstFrameMs", float("nan")), ("firstFrameMs", 5001), ("elapsedMs", True), ("stalls", -1),
                           ("stallMs", 5001), ("seeks", 1.5), ("incompleteSeeks", 2), ("seekProgressMs", [100] * 129),
                           ("engine", "unknown"), ("sourceURL", "https://private.invalid")]:
            with self.subTest(key=key, value=value):
                document = fixture()
                document["samples"][0][key] = value
                with self.assertRaises(ValueError):
                    summarize(document)
        for samples in [[], [{}], [fixture()["samples"][0]] * 10001]:
            document = fixture()
            document["samples"] = samples
            with self.assertRaises(ValueError):
                summarize(document)

    def test_invalid_file_emits_no_report(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "bad.json"
            for body in ['{"samples":[],"samples":[]}', '{', 'x' * (4 * 1024 * 1024 + 1)]:
                path.write_text(body)
                output = io.StringIO()
                with redirect_stdout(output), redirect_stderr(io.StringIO()):
                    self.assertEqual(main([str(path)]), 2)
                self.assertEqual(output.getvalue(), "")


if __name__ == "__main__":
    unittest.main()
