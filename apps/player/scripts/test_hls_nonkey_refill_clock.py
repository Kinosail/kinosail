"""Fixed packet-origin arithmetic; public E2E cannot inspect raw mux/input rescaling.
Failure matrix: ambiguous first payload, absent/inexact clocks, foreign timebase,
unbounded packet rows, integer rescale ties and distinct canonical track origins.
"""
import unittest
from hls_nonkey_refill_clock import audio_origin, rescale_us, refill_shift, matrix_failure, fixed_streams

HASH = 'SHA256:' + 'a' * 64
OTHER = 'SHA256:' + 'b' * 64

def row(pts=571400, digest=HASH):
    return {'stream_index': 1, 'pts': pts, 'data_hash': digest}

class RefillClockTests(unittest.TestCase):
    def test_baseline_video_clock_is_two_ticks_larger(self):
        origin = audio_origin([row()], [row(0)], 0, 48000)
        self.assertEqual(origin, -571400)
        self.assertEqual(refill_shift(origin, 20000000, 8095875, 48000), -2)

    def test_normal_initial_edit_needs_separate_audio_origin(self):
        origin = audio_origin([row()], [row(-4600)], 0, 48000)
        self.assertEqual(refill_shift(origin, 20000000, 8000000, 48000), 4600)

    def test_demux_rescale_matches_actual_fractional_seek(self):
        self.assertEqual(rescale_us(20000020, 48000), 960001)
        self.assertEqual(rescale_us(1, 500000), 1)
        self.assertEqual(rescale_us(-1, 500000), -1)
        self.assertEqual(rescale_us(83312, 48000), 3999)

    def test_first_payload_requires_unique_source_identity(self):
        for source in [[row(), row(572424)], [row(digest=OTHER)]]:
            with self.assertRaises(RuntimeError):
                audio_origin(source, [row(0)], 0, 48000)

    def test_packet_integer_hash_and_timebase_are_required(self):
        for source, public, raw, scale in [
            ([row(None)], [row(0)], 0, 48000),
            ([row(571400.0)], [row(0)], 0, 48000),
            ([row(digest='SHA256:bad')], [row(0)], 0, 48000),
            ([row()], [row(0)], -1, 48000),
            ([row()], [row(0)], 0, 44100),
            ([row()], [row(0)], 0.0, 48000),
            ([row()], [], 0, 48000),
            ([row()] * 4097, [row(0)], 0, 48000)]:
            with self.assertRaises(RuntimeError):
                audio_origin(source, public, raw, scale)

    def test_rescale_rejects_inexact_or_unbounded_inputs(self):
        for value, scale in [(True, 48000), (0.1, 48000), (1 << 53, 48000),
                             (0, 0), (0, 1000001)]:
            with self.assertRaises(RuntimeError):
                rescale_us(value, scale)

    def test_refill_shift_does_not_choose_against_pcm(self):
        self.assertEqual(refill_shift(-571400, 20000000, 8095833, 48000), 0)
        self.assertEqual(refill_shift(-571400, 20000020, 8095833, 48000), 1)

def streams():
    return {'streams':[
        {'index':0,'codec_type':'video','codec_name':'h264','time_base':'1/16000'},
        {'index':1,'codec_type':'audio','codec_name':'aac','time_base':'1/48000',
         'sample_rate':'48000','channels':2}]}

class FixedStreamTests(unittest.TestCase):
    def test_fixed_stream_grid_is_explicit(self):
        self.assertIsNone(fixed_streams(streams()))

    def test_foreign_or_missing_audio_and_video_grid_is_rejected(self):
        for index, key, value in [(0,'time_base','1/1000'),(0,'codec_name','hevc'),
                (1,'time_base','1/1000'),(1,'sample_rate','44100'),(1,'channels',1),
                (1,'index',2),(1,'codec_type','video'),(0,'index',True)]:
            facts=streams()
            facts['streams'][index][key]=value
            with self.assertRaises(RuntimeError):
                fixed_streams(facts)
        for facts in [{}, {'streams':[]}, {'streams':[{}]}, {'streams':[{},{}]}]:
            with self.assertRaises(RuntimeError):
                fixed_streams(facts)

class MatrixFailureTests(unittest.TestCase):
    def test_failed_cell_keeps_only_a_safe_class(self):
        self.assertEqual(matrix_failure(RuntimeError('fixed_failure')), 'fixed_failure')
        self.assertEqual(matrix_failure(RuntimeError('/private/fixture')), 'RuntimeError')
        self.assertEqual(matrix_failure(ValueError('private details')), 'ValueError')

    def test_shared_deadline_remains_terminal(self):
        with self.assertRaises(RuntimeError):
            matrix_failure(RuntimeError('bounded_diagnostic_deadline'))

if __name__ == '__main__':
    unittest.main()
