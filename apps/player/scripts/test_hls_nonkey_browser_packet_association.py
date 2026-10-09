"""Gap: strict AAC-tail failure has no public-safe interior source ordinal map.
These isolated controls protect the bounded diagnostic projection, not readiness."""
import unittest
from hls_nonkey_browser_packet_association import packet_association

def row(n):
    return {'data_hash': 'SHA256:' + format(n, '064x')}

def tail(source, public):
    return {'completeSourceRows': [row(n) for n in source],
            'completePublicRows': [row(n) for n in public]}

class PacketAssociationTests(unittest.TestCase):
    def test_whole_contiguous_suffix_keeps_every_ordinal(self):
        actual = packet_association(tail(range(8), range(3, 8)))
        self.assertTrue(actual['observed'])
        self.assertEqual(actual['sourceOrdinals'], [3, 4, 5, 6, 7])
        self.assertEqual(actual['transitionCount'], 0)
        self.assertEqual(actual['unknownPackets'], 0)
        self.assertEqual(actual['ambiguousPackets'], 0)
        self.assertFalse(actual['acceptance'])

    def test_interior_missing_and_repeated_packets_remain_visible(self):
        actual = packet_association(tail(range(8), [2, 3, 5, 5, 4, 7]))
        self.assertEqual(actual['sourceOrdinals'], [2, 3, 5, 5, 4, 7])
        self.assertEqual(actual['transitions'], [
            {'publicOrdinal': 2, 'previousSourceOrdinal': 3, 'sourceOrdinal': 5, 'delta': 2},
            {'publicOrdinal': 3, 'previousSourceOrdinal': 5, 'sourceOrdinal': 5, 'delta': 0},
            {'publicOrdinal': 4, 'previousSourceOrdinal': 5, 'sourceOrdinal': 4, 'delta': -1},
            {'publicOrdinal': 5, 'previousSourceOrdinal': 4, 'sourceOrdinal': 7, 'delta': 3}])

    def test_ambiguous_and_unmatched_payloads_are_not_guessed(self):
        actual = packet_association(tail([1, 2, 2, 3], [1, 2, 99, 3]))
        self.assertEqual(actual['sourceOrdinals'], [0, None, None, 3])
        self.assertEqual((actual['ambiguousPackets'], actual['unknownPackets']), (1, 1))

    def test_transition_overflow_is_explicit_without_trimming_ordinal_rows(self):
        actual = packet_association(tail(range(200), range(0, 200, 2)))
        self.assertEqual(len(actual['sourceOrdinals']), 100)
        self.assertEqual(actual['transitionCount'], 99)
        self.assertEqual(len(actual['transitions']), 64)
        self.assertTrue(actual['transitionOverflow'])

    def test_invalid_or_unbounded_inputs_disclose_no_untrusted_values(self):
        for value in [{}, tail([], []), tail(range(4097), [1]),
                      {'completeSourceRows': [{'data_hash': 'private-url'}],
                       'completePublicRows': [row(1)]}]:
            actual = packet_association(value)
            self.assertEqual(actual, {'observed': False, 'failureClass': 'packet_association_shape',
                                      'acceptance': False})

if __name__ == '__main__':
    unittest.main()
