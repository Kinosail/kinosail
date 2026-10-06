"""Protect the numeric AAC oracle gap that valid public fixtures cannot expose.

Deliberately damaged interior timing can retain the same min/max endpoints;
real Server journeys protect media delivery, while these inputs protect the oracle.
"""
import unittest
from hls_timeline_packets import audio_packet_facts


class AudioPacketOracle(unittest.TestCase):
    def packets(self, points):
        return [{"pts_time": str(point), "duration_time": "0.02"} for point in points]

    def test_contiguous_packets(self):
        facts = audio_packet_facts(self.packets([0, 0.02, 0.04, 0.06]))
        self.assertAlmostEqual(facts["maximumAudioGapSeconds"], 0)
        self.assertAlmostEqual(facts["maximumAudioOverlapSeconds"], 0)
        self.assertTrue(facts["audioPacketOrderValid"])

    def test_interior_gap_with_unchanged_endpoints(self):
        facts = audio_packet_facts(self.packets([0, 0.02, 0.10, 0.12]))
        control = audio_packet_facts(self.packets([0, 0.02, 0.04, 0.06, 0.08, 0.10, 0.12]))
        self.assertEqual((facts["firstAudioTime"], facts["lastAudioEnd"]), (control["firstAudioTime"], control["lastAudioEnd"]))
        self.assertAlmostEqual(facts["maximumAudioGapSeconds"], 0.06)

    def test_interior_overlap_with_unchanged_endpoints(self):
        packets = self.packets([0, 0.02, 0.04, 0.06, 0.08, 0.10, 0.12, 0.14, 0.16])
        control = audio_packet_facts(packets)
        packets[1]["duration_time"] = "0.10"
        facts = audio_packet_facts(packets)
        self.assertEqual((facts["firstAudioTime"], facts["lastAudioEnd"]), (control["firstAudioTime"], control["lastAudioEnd"]))
        self.assertAlmostEqual(facts["maximumAudioOverlapSeconds"], 0.08)

    def test_out_of_order_packets(self):
        self.assertFalse(audio_packet_facts(self.packets([0, 0.04, 0.02]))["audioPacketOrderValid"])

    def test_invalid_packets(self):
        for packets in [[], [{}], self.packets([float("nan")]),
                [{"pts_time": "0", "duration_time": "0"}], self.packets([0]) * 4096]:
            with self.subTest(packets=packets[:1]), self.assertRaises(RuntimeError):
                audio_packet_facts(packets)


if __name__ == "__main__":
    unittest.main()
