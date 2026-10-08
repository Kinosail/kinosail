"""Synthetic parser and failure-retention controls; no media eligibility proof."""
import json
from types import SimpleNamespace
import unittest
from unittest.mock import patch
from hls_remaining_nonkey_aac_prefix import measure_prefix, normalized_output_rows


HEADERS = ['#tb 0: 1/48000', '#media_type 0: audio', '#sample_rate 0: 48000']


def document(points, samples=1024, headers=None):
    rows = [f'0, {point}, {point}, {samples}, {samples * 4}, ' + '0' * 32 for point in points]
    return '\n'.join((HEADERS if headers is None else headers) + rows) + '\n'


class PrefixOutputParser(unittest.TestCase):
    def test_full_and_short_leading_frames(self):
        self.assertEqual(normalized_output_rows(document([0, 1024])), [(0, 1024), (1024, 1024)])
        self.assertEqual(normalized_output_rows(document([-8], 16)), [(-8, 16)])

    def test_conflicting_timebase_and_rate_headers(self):
        for header in ['#tb 0: 1/1000', '#sample_rate 0: 44100', '#media_type 0: video']:
            with self.subTest(header=header):
                with self.assertRaisesRegex(RuntimeError, 'aac_prefix_normalized_output_format'):
                    normalized_output_rows(document([0], headers=HEADERS + [header]))

    def test_wrong_stream_clock_and_extent(self):
        mutations = [
            ('0, 0, 0, 1024, 4096', '1, 0, 0, 1024, 4096'),
            ('0, 0, 0, 1024, 4096', '0, 1, 0, 1024, 4096'),
            ('0, 0, 0, 1024, 4096', '0, 0, 0, 1024, 4095'),
            ('0, 0, 0, 1024, 4096', '0, 0, 0, 0, 0'),
            ('0, 0, 0, 1024, 4096', '0, 0, 0, 1025, 4100')]
        for before, after in mutations:
            with self.subTest(after=after):
                with self.assertRaisesRegex(RuntimeError, 'aac_prefix_normalized_output_extent'):
                    normalized_output_rows(document([0]).replace(before, after))

    def test_shifted_clock_retains_both_compared_sequences(self):
        points = [number * 1024 for number in range(1024)]
        normalized = document([point + 1 for point in points])
        filter_log = '\n'.join(
            f'[Parsed_ashowinfo_0] n:{number} pts:{number * 1024} pts_time:0 '
            'fmt:fltp channels:2 rate:48000 nb_samples:1024 checksum:00000000'
            for number in range(1025))
        facts = {'streams': [{'codec_name': 'aac', 'profile': 'LC', 'sample_rate': '48000',
            'channels': 2, 'time_base': '1/1000'}], 'packets_and_frames': [
            {'type': 'frame', 'pts': 0, 'nb_samples': 1024},
            {'type': 'packet', 'pts': 0, 'data_hash': 'SHA256:' + '0' * 64}]}
        receipt = {}
        outputs = [SimpleNamespace(stdout=json.dumps(facts).encode(), stderr=b''),
            SimpleNamespace(stdout=normalized.encode(), stderr=filter_log.encode())]
        with patch('hls_remaining_nonkey_aac_prefix.prefix_command', side_effect=outputs):
            with self.assertRaisesRegex(RuntimeError, 'aac_prefix_output_filter_clock_equivalence'):
                measure_prefix('owned-synthetic-source', receipt)
        self.assertIs(receipt['outputFilterClockEquivalent'], False)
        self.assertEqual(len(receipt['normalizedFilter']['completeRows']), 1025)
        self.assertEqual(receipt['normalizedOutputRows'], [(p + 1, 1024) for p in points])


if __name__ == '__main__':
    unittest.main()
