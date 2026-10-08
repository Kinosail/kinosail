"""Boundary diagnosis fails closed while preserving signed raw clocks."""
import unittest
from hls_remaining_nonkey_boundary import (
    edit_binding, frame_clock_diagnosis, integer, native_clock_rows, packet_tail)


class BoundaryControls(unittest.TestCase):
    def binding(self, shift=10):
        init = {'tracks': [{'trackID': 1, 'handler': 'vide', 'mediaTimescale': 100,
            'edits': [{'duration': 0, 'mediaTime': shift, 'rateInteger': 1, 'rateFraction': 0}]}]}
        fragments = [{'tracks': [{'trackID': 1, 'samples': [
            {'pts': 5, 'dts': 0, 'duration': 10}, {'pts': 15, 'dts': 10, 'duration': 10}]}]}]
        streams = [{'id': '0x1', 'index': 0, 'time_base': '1/100'}]
        packets = [{'stream_index': 0, 'pts': 5-shift, 'dts': -shift,
                    'flags': 'KD', 'data_hash': 'SHA256:aaa'},
                   {'stream_index': 0, 'pts': 15-shift, 'dts': 10-shift,
                    'flags': '__', 'data_hash': 'SHA256:bbb'}]
        return init, fragments, streams, packets

    def test_signed_edit_clock_and_all_rows_retained(self):
        row = edit_binding(*self.binding())[0]
        self.assertTrue(row['allDemuxClocksMatchEdit'])
        self.assertEqual(row['negativeEditedPTSRows'], [0])
        self.assertEqual(row['discardFlagRows'], [0])
        self.assertEqual(len(row['completeRows']), 2)
        self.assertFalse(row['presentationOrDiscardRuleApplied'])

    def test_clock_mismatch_is_recorded(self):
        args = self.binding()
        args[3][0]['pts'] += 1
        row = edit_binding(*args)[0]
        self.assertFalse(row['allDemuxClocksMatchEdit'])
        self.assertEqual(len(row['completeRows']), 2)

    def test_duplicate_stream_rejected(self):
        args = self.binding()
        args[2].append(dict(args[2][0]))
        with self.assertRaises(RuntimeError):
            edit_binding(*args)

    def test_wrong_clock_rejected(self):
        args = self.binding()
        args[2][0]['time_base'] = '1/1000'
        with self.assertRaises(RuntimeError):
            edit_binding(*args)

    def test_missing_sample_rejected(self):
        args = self.binding()
        args[3].pop()
        with self.assertRaises(RuntimeError):
            edit_binding(*args)

    def test_raw_discontinuity_retained(self):
        args = self.binding()
        args[1][0]['tracks'][0]['samples'][1]['dts'] = 12
        row = edit_binding(*args)[0]
        self.assertEqual(row['rawDecodeDiscontinuities'], [1])
        self.assertFalse(row['allDemuxClocksMatchEdit'])

    def test_unsupported_edit_rejected(self):
        args = self.binding()
        args[0]['tracks'][0]['edits'][0]['duration'] = 20
        with self.assertRaises(RuntimeError):
            edit_binding(*args)

    def test_frame_clock_subset_does_not_relabel_raw(self):
        mapping = {'publicRows': [[1, .5, -.5, 'a'], [2, 1, 0, 'b']],
            'actualSourceIndices': [1, 2], 'expectedSourceIndices': [2],
            'exactRequestedSequence': False}
        row = frame_clock_diagnosis(mapping)
        self.assertEqual(row['negativeClockIndices'], [1])
        self.assertTrue(row['nonnegativeClockSubsetEqualsRequested'])
        self.assertFalse(row['rawRequestedSequenceExact'])
        self.assertEqual(row['allActualIndices'], [1, 2])
        self.assertEqual(row['framesTrimmed'], 0)
        self.assertFalse(row['rendererQualification'])

    def test_native_coarse_clock_and_ordinal_are_separate(self):
        row = native_clock_rows([{'pts': 0, 'nb_samples': 1024},
            {'pts': 21, 'nb_samples': 1024}], '1/1000', 1024, 1040)
        self.assertEqual(row['nativeSampleSum'], 2048)
        self.assertEqual(row['targetRows'][0]['clockMinusOrdinalSamples'], '-16')
        self.assertEqual(row['targetRows'][0]['containsTargets'], [1024, 1040])
        self.assertEqual(row['samplesTrimmed'], 0)

    def test_packet_tail_requires_unique_complete_match(self):
        make = lambda h: {'stream_index': 1, 'data_hash': h}
        row = packet_tail([make('a'), make('b'), make('c')], [make('b'), make('c')])
        self.assertEqual(row['uniqueSourceStart'], 1)
        self.assertTrue(row['wholePublicPacketTail'])
        row = packet_tail([make('a'), make('b'), make('b')], [make('b')])
        self.assertEqual(row['sequenceMatches'], 2)
        self.assertFalse(row['wholePublicPacketTail'])

    def test_integer_clock_rejects_inferred_values(self):
        for value in [None, True, 1.0, '1.5']:
            with self.assertRaises(RuntimeError):
                integer(value)


if __name__ == '__main__':
    unittest.main()
