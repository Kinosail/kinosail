"""Independent clock, association and packet-dump controls for capability readers."""
from fractions import Fraction
import unittest
from hls_nonkey_supported_readers import expected_sequence, hex_dump, read_options


class SupportedReaderControls(unittest.TestCase):
    def source(self):
        return [(n / 24, 'frame-' + str(n)) for n in range(768)]

    def test_request_rebased_preroll_uses_delivered_clock(self):
        source = self.source()
        public = [(-0.5, source[288][1])]
        facts, absolute, relative = read_options(public, source, Fraction(25, 2),
                                                {'format': {'start_time': '-0.58'}})
        self.assertEqual(absolute, 0)
        self.assertEqual(relative, Fraction(29, 50))
        self.assertEqual(facts['matchedFirstSourceVideoPTS'], '12.0')

    def test_retained_source_origin_uses_relative_delta(self):
        source = self.source()
        facts, absolute, relative = read_options([(0, source[288][1])], source, Fraction(25, 2),
                                                {'format': {'start_time': '0'}})
        self.assertEqual(absolute, Fraction(1, 2))
        self.assertEqual(relative, Fraction(1, 2))
        self.assertEqual(facts['requestedSourcePTS'], '25/2')

    def test_container_start_and_first_video_are_independent(self):
        source = self.source()
        _, absolute, relative = read_options([(0.0625, source[288][1])], source, Fraction(25, 2),
                                             {'format': {'start_time': '-0.0625'}})
        self.assertEqual(absolute, Fraction(9, 16))
        self.assertEqual(relative, Fraction(5, 8))

    def test_unknown_hash_rejected(self):
        with self.assertRaisesRegex(RuntimeError, 'public_association'):
            read_options([(0, 'unknown')], self.source(), Fraction(25, 2),
                         {'format': {'start_time': '0'}})

    def test_duplicate_source_hash_rejected(self):
        source = self.source()
        source[1] = (source[1][0], source[0][1])
        with self.assertRaisesRegex(RuntimeError, 'unique_source_association'):
            read_options([(0, source[288][1])], source, Fraction(25, 2),
                         {'format': {'start_time': '0'}})

    def test_empty_public_rejected(self):
        with self.assertRaisesRegex(RuntimeError, 'public_association'):
            read_options([], self.source(), Fraction(25, 2), {'format': {'start_time': '0'}})

    def test_complete_requested_tail_contains_468_frames(self):
        actual = expected_sequence(self.source(), Fraction(25, 2))
        self.assertEqual(actual, ['frame-' + str(n) for n in range(300, 768)])
        self.assertEqual(len(actual), 468)

    def test_packet_hex_preserves_bytes_not_ascii_column(self):
        data = '\n00000000: 0000 0003 6588 8400 0000 0241 9a            ....e......A.\n'
        self.assertEqual(hex_dump(data), bytes.fromhex('0000000365888400000002419a'))

    def test_malformed_packet_hex_rejected(self):
        for bad in ['00000000: xyz  text', 'offset: 0000  text', '00000000: 0  text',
                    '00000000: 0000  text\n00000010: 0011  text']:
            with self.subTest(value=bad), self.assertRaises(RuntimeError):
                hex_dump(bad)


if __name__ == '__main__':
    unittest.main()
