"""Negative controls for the complete public packet assertions."""
import copy
import unittest
from hls_aac_v2_public_evidence import assert_fixed_packets


class FixedPacketOracleControls(unittest.TestCase):
    def setUp(self):
        self.source = []
        for stream, count in [(0, 768), (1, 1501)]:
            for number in range(count):
                self.source.append({'stream_index': stream, 'pts': number * 1024,
                    'dts': number * 1024, 'duration': 1024,
                    'data_hash': 'SHA256:' + format(stream * 10000 + number, '064x')})
        audio = [row for row in self.source if row['stream_index'] == 1][559:]
        self.public = copy.deepcopy([row for row in self.source if row['stream_index'] == 0][288:] + audio)
        for row in self.public:
            if row['stream_index'] == 1:
                row['pts'] -= 576000
                row['dts'] -= 576000
        self.facts = {'sourcePacketRows': self.source, 'publicPacketRows': self.public}

    def reject(self, expected):
        with self.assertRaisesRegex(RuntimeError, expected):
            assert_fixed_packets(self.facts)

    def test_interior_gap_with_unchanged_endpoints(self):
        self.public[600]['pts'] += 1
        self.reject('v2_every_aac_clock')

    def test_interior_dts_only_overlap(self):
        self.public[600]['dts'] -= 1
        self.reject('v2_every_aac_clock')

    def test_missing935_replaced_by_duplicate(self):
        self.public[480 + 935 - 559] = copy.deepcopy(self.public[480 + 934 - 559])
        self.reject('v2_complete_aac_payload_suffix')

    def test_interior_duration_corruption(self):
        self.public[600]['duration'] += 1
        self.reject('v2_adjacent_aac_gap_or_overlap')

    def test_video_payload_gap(self):
        self.public[100]['data_hash'] = self.public[101]['data_hash']
        self.reject('v2_every_video_payload')


if __name__ == '__main__':
    unittest.main()
