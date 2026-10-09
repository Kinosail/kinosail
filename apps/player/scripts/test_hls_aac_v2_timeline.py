"""Pinned source-grid and every-cut controls missing from the endpoint-only timeline check."""
import copy
import unittest
from hls_aac_v2_public_evidence import assert_fixed_source_grid, assert_fixed_timeline


class FixedTimelineControls(unittest.TestCase):
    def setUp(self):
        self.data = {'streams': [{'time_base': '1/16000'}], 'packets': [
            {'pts': round(n * 1000 / 24) * 16, 'duration': 656, 'flags': 'K_' if n % 48 == 0 else '__'}
            for n in range(768)]}
        self.grid = assert_fixed_source_grid(self.data)
        self.facts = {'endlist': True, 'playlistType': 'VOD', 'durationSeconds': 19.999}
        self.fragments = [('segment-' + format(n, '05d') + '.m4s', 2 if n < 9 else 1.999)
                          for n in range(10)]

    def test_complete_source_derived_timeline(self):
        assert_fixed_timeline(self.facts, self.fragments, self.grid)

    def test_wrong_source_end_cannot_change_the_expected_timeline(self):
        self.data['packets'][-1]['duration'] += 1
        with self.assertRaises(RuntimeError):
            assert_fixed_source_grid(self.data)

    def test_missing_source_key_is_rejected(self):
        self.data['packets'][48]['flags'] = '__'
        with self.assertRaises(RuntimeError):
            assert_fixed_source_grid(self.data)

    def test_interior_duration_is_not_hidden_by_the_total(self):
        self.fragments[3] = (self.fragments[3][0], 1.999)
        self.fragments[4] = (self.fragments[4][0], 2.001)
        with self.assertRaises(RuntimeError):
            assert_fixed_timeline(self.facts, self.fragments, self.grid)

    def test_wrong_last_duration_is_rejected(self):
        self.fragments[-1] = (self.fragments[-1][0], 2)
        with self.assertRaises(RuntimeError):
            assert_fixed_timeline(self.facts, self.fragments, self.grid)

    def test_duplicate_segment_name_is_rejected(self):
        self.fragments[-1] = (self.fragments[0][0], self.fragments[-1][1])
        with self.assertRaises(RuntimeError):
            assert_fixed_timeline(self.facts, self.fragments, self.grid)

    def test_inert_event_prefix_is_rejected(self):
        self.facts.update(endlist=False, playlistType='EVENT')
        with self.assertRaises(RuntimeError):
            assert_fixed_timeline(self.facts, self.fragments, self.grid)
