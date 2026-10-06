"""Protect the presentation oracle from silently accepting opening frames.

Public cases cannot inject malformed framemd5 evidence. These bounded parser
checks cover missing clocks and content-based trimming that could hide a seek bug.
"""
import unittest
from hls_followon_frames import frame_facts, parse_frames


class PresentationOracleTests(unittest.TestCase):
    def test_negative_clock_is_unqualified_and_never_trimmed_without_origin_proof(self):
        rows = [(-0.5, 'a' * 32), (0, 'b' * 32), (1 / 24, 'c' * 32)]
        actual = frame_facts(rows)
        self.assertEqual(actual['rawFrames'], 3)
        self.assertEqual(actual['negativeTimestampFrames'], 1)
        self.assertFalse(actual['presentationQualified'])
        self.assertEqual(actual['presentedFrames'], 3)
        self.assertNotEqual(actual['identity'], frame_facts(rows[1:])['identity'])

    def test_opening_frame_at_zero_cannot_be_trimmed_by_matching_reference(self):
        actual = frame_facts([(0, 'a' * 32), (1 / 24, 'b' * 32)])
        reference = frame_facts([(0, 'b' * 32)])
        self.assertNotEqual(actual['identity'], reference['identity'])

    def test_missing_clock_and_oversized_evidence_are_rejected(self):
        for data in [b'0, 0, 0, 1, 12, ' + b'a' * 32,
                     b'#tb 0: 0/24\n', b'x' * (2 * 1024 * 1024 + 1)]:
            with self.assertRaises(RuntimeError):
                parse_frames(data)

    def test_clock_and_hash_are_parsed_without_rebasing(self):
        rows = parse_frames(b'#tb 0: 1/24\n0, -1, -1, 1, 12, ' + b'a' * 32 +
                            b'\n0, 2, 2, 1, 12, ' + b'b' * 32 + b'\n')
        self.assertEqual(rows, [(-1 / 24, 'a' * 32), (2 / 24, 'b' * 32)])


if __name__ == '__main__':
    unittest.main()
