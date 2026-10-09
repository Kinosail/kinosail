import copy
import sys
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "apps/player/scripts"))
from hls_remaining_process import annotate_case, source_snapshot

class AudioLifecycleTests(unittest.TestCase):
    def case(self, duration=10):
        return {
            "result": "passed", "failures": [], "planDurationSeconds": duration,
            "preparationAttempt": {"posts": 1, "completionState": "ready", "ownedFFmpeg": 0},
            "physicalBeforeFirstGET": {"manifest": {"playlistType": "EVENT", "endlist": False,
                "durationSeconds": 7.999999, "segmentCount": 4}, "packetCount": 375,
                "segments": [f"segment-{n:05d}.m4s" for n in range(4)],
                "assets": [{"name": name, "sha256": "a"*64, "size": 1, "inode": 1, "device": 1, "mtimeNs": 1}
                    for name in ["index.m3u8", "init.mp4", *[f"segment-{n:05d}.m4s" for n in range(4)]]]},
            "publicVariant": {"playlistType": "VOD", "endlist": True,
                "durationSeconds": duration, "segmentCount": 6},
            "ownedFFmpegBeforeTeardown": 0, "cleanupFailures": [],
            "ownedProcessJoin": {"confirmedZeroSamples": 2, "qualificationFailures": []},
        }

    def annotate(self, case, starts=None, concurrent=False):
        starts = starts if starts is not None else [(0, 0, "background"), (8000, 4, "playback")]
        case["physicalAfterPublicDelivery"] = case.get("physicalAfterPublicDelivery",
            copy.deepcopy(case.get("physicalBeforeFirstGET")))
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / "fixture.flac"
            source.write_bytes(b"pure-control-source-not-media")
            log = Path(directory) / "private.log"
            rows = [f"HLS transcode started input_seek_ms={seek} segment_start={number} mode=audio-transcode work_class={role}"
                    for seek, number, role in starts]
            rows = rows + ["HLS transcode completed"]*len(rows) if concurrent else [
                line for row in rows for line in (row, "HLS transcode completed")]
            log.write_text("\n".join(rows))
            annotate_case(case, log, source, source_snapshot(source), SimpleNamespace(pid=42),
                {"samples": 1, "peakOwnedFFmpeg": 1, "samplingErrors": 0}, 0, None, None, True)
        return case

    def test_qualified_prepared_prefix_then_owned_refill_is_two_sequential_workers(self):
        case = self.annotate(self.case())
        self.assertEqual(case["result"], "passed")
        self.assertEqual(case["failures"], [])
        self.assertEqual(case["encoderLifecycle"]["peakActive"], 1)

    def test_complete_eight_second_source_keeps_one_worker(self):
        case = self.annotate(self.case(8), [(0, 0, "background")])
        self.assertEqual(case["result"], "passed")

    def test_prefix_refill_certificate_rejects_wrong_or_missing_facts(self):
        edits = {
            "missing prefix": lambda c: c.pop("physicalBeforeFirstGET"),
            "missing assets": lambda c: c["physicalBeforeFirstGET"].pop("assets"),
            "empty assets": lambda c: c["physicalBeforeFirstGET"].update(assets=[]),
            "duplicate asset": lambda c: c["physicalBeforeFirstGET"]["assets"][0].update(name="init.mp4"),
            "unknown asset": lambda c: c["physicalBeforeFirstGET"]["assets"][0].update(name="unknown"),
            "malformed hash": lambda c: c["physicalBeforeFirstGET"]["assets"][0].update(sha256="unknown"),
            "empty asset": lambda c: c["physicalBeforeFirstGET"]["assets"][0].update(size=0),
            "malformed preparation": lambda c: c.update(preparationAttempt=None),
            "malformed manifest": lambda c: c["physicalBeforeFirstGET"].update(manifest="unknown"),
            "malformed public": lambda c: c.update(publicVariant=None),
            "nonfinite prefix": lambda c: c["physicalBeforeFirstGET"]["manifest"].update(durationSeconds=float("nan")),
            "missing duration": lambda c: c.pop("planDurationSeconds"),
            "wrong duration": lambda c: c.update(planDurationSeconds=9),
            "unknown playlist": lambda c: c["physicalBeforeFirstGET"]["manifest"].update(playlistType="unknown"),
            "prefix EOF": lambda c: c["physicalBeforeFirstGET"]["manifest"].update(endlist=True),
            "cut count": lambda c: c["physicalBeforeFirstGET"]["manifest"].update(segmentCount=3),
            "prefix duration": lambda c: c["physicalBeforeFirstGET"]["manifest"].update(durationSeconds=8.1),
            "packet count": lambda c: c["physicalBeforeFirstGET"].update(packetCount=374),
            "wrong ordinals": lambda c: c["physicalBeforeFirstGET"].update(segments=["segment-00004.m4s"]),
            "not prepared": lambda c: c["preparationAttempt"].update(completionState="queued"),
            "producer still active": lambda c: c["preparationAttempt"].update(ownedFFmpeg=1),
            "no complete public output": lambda c: c["publicVariant"].update(endlist=False),
            "wrong full duration": lambda c: c["publicVariant"].update(durationSeconds=9.5),
            "wrong public cuts": lambda c: c["publicVariant"].update(segmentCount=5),
            "changed init": lambda c: c.update(physicalAfterPublicDelivery={
                **copy.deepcopy(c["physicalBeforeFirstGET"]), "assets": [{"name": "init.mp4", "sha256": "b"*64}]}),
        }
        for name, edit in edits.items():
            with self.subTest(name=name):
                c = self.case(); edit(c)
                self.assertIn("audio_required_new_encoder", self.annotate(c)["failures"])

    def test_extra_concurrent_wrong_role_or_wrong_seek_does_not_qualify(self):
        for starts, concurrent in [
            ([(0,0,"background"),(8000,4,"playback"),(10000,5,"playback")], False),
            ([(0,0,"background"),(8000,4,"playback")], True),
            ([(0,0,"background"),(8000,4,"background")], False),
            ([(0,0,"background"),(6000,3,"playback")], False),
        ]:
            with self.subTest(starts=starts, concurrent=concurrent):
                case = self.annotate(self.case(), starts, concurrent)
                self.assertEqual(case["result"], "failed")
                self.assertIn("audio_required_new_encoder", case["failures"])

if __name__ == "__main__":
    unittest.main()
