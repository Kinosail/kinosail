"""Source-bound cardinality and explicit synthetic-check classification controls."""
from pathlib import Path
import unittest
from hls_aac_v2_public_http import source_invocations


class SourceInvocationControls(unittest.TestCase):
    def setUp(self):
        self.source = Path('/fixed/Fixture.mp4')
        self.actual = {'parent': 17, 'args': ['-i', str(self.source), '-hls_time', '0.1']}
        self.probe = {'parent': 17, 'args': ['-i', '/tmp/kinosail-transcoder-check-test/source.mp4',
            '-c:a', 'aac', '-frames:v', '24', '-shortest', '-hls_time', '1',
            '/tmp/kinosail-transcoder-check-test/index.m3u8']}

    def test_owned_source_and_known_synthetic_check(self):
        self.assertEqual(source_invocations([self.probe, self.actual], self.source, {17}), [self.actual])

    def test_previous_owned_process_remains_counted_after_reopen(self):
        reopened = dict(self.actual, parent=18)
        self.assertEqual(source_invocations([self.actual, reopened], self.source, {17, 18}),
                         [self.actual, reopened])

    def reject(self, row):
        with self.assertRaises(RuntimeError):
            source_invocations([row], self.source, {17})

    def test_foreign_parent_cannot_be_filtered_away(self):
        self.reject(dict(self.probe, parent=18))

    def test_actual_source_with_an_extra_input_is_rejected(self):
        self.reject(dict(self.actual, args=[*self.actual['args'], '-i', '/other/input.mp4']))

    def test_unknown_input_is_rejected(self):
        self.reject(dict(self.actual, args=['-i', '/other/input.mp4', '-hls_time', '0.1']))

    def test_wrong_synthetic_timing_is_rejected(self):
        self.reject(dict(self.probe, args=[value if value != '1' else '2' for value in self.probe['args']]))

    def test_synthetic_output_must_share_its_owned_fixture(self):
        self.reject(dict(self.probe, args=[*self.probe['args'][:-1], '/tmp/other/index.m3u8']))

    def test_source_count_bound_is_retained(self):
        with self.assertRaises(RuntimeError):
            source_invocations([self.actual] * 17, self.source, {17})
