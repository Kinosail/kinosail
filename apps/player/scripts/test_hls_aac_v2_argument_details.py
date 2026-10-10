"""Written-first bounded argv-shape diagnostics; no admission or lifetime proof."""
import io
import json
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import patch
import hls_aac_v2_live_observer as live
from hls_followon_public import sample_resources
from test_hls_aac_v2_fd_sampler import StopAfterOneSample


class ArgumentDetailControls(unittest.TestCase):
    def rejected(self, data, shape, count=None, maximum=None, failure=RuntimeError):
        with patch.object(live.Path, 'open', return_value=io.BytesIO(data)):
            with self.assertRaises(failure) as caught:
                live.actual_arguments(17)
        if failure is RuntimeError:
            self.assertEqual(str(caught.exception), live.FAILURE)
        self.assertEqual(getattr(caught.exception, 'observer_argument_bytes', None), len(data))
        self.assertEqual(getattr(caught.exception, 'observer_argument_shape', None), shape)
        self.assertEqual(getattr(caught.exception, 'observer_argument_count', None), count)
        self.assertEqual(getattr(caught.exception, 'observer_maximum_argument_bytes', None), maximum)

    def test_empty_argument_record_retains_safe_shape(self):
        self.rejected(b'', 'empty')

    def test_excessive_argument_bytes_retains_safe_shape(self):
        self.rejected(b'x' * 65537, 'byte_bound')

    def test_nonterminated_argument_record_retains_safe_shape(self):
        self.rejected(b'synthetic_private_target', 'nonterminated')

    def test_excessive_argument_count_retains_safe_shape(self):
        self.rejected(b'x\0' * 129, 'argument_count', 129, 1)

    def test_excessive_argument_length_retains_safe_shape(self):
        self.rejected(b'x' * 2049 + b'\0', 'argument_length', 1, 2049)

    def test_undecodable_argument_record_retains_safe_shape(self):
        self.rejected(b'\xff\0', 'argument_decode', failure=UnicodeDecodeError)

    def sample(self, attrs):
        error = RuntimeError('synthetic_private_target_must_not_escape')
        error.observer_stage = 'arguments_before'
        error.observer_exception_class = 'RuntimeError'
        for key, value in attrs.items():
            setattr(error, key, value)
        resources = {'samples': 0, 'peakOwnedFFmpeg': 0, 'samplingErrors': 0}
        with patch('hls_followon_public.encoder_count', side_effect=error):
            sample_resources(SimpleNamespace(pid=17), Path('/fixed/Fixture.mp4'),
                StopAfterOneSample(), resources)
        self.assertEqual(resources['samples'], 0)
        self.assertEqual(resources['samplingErrors'], 1)
        self.assertEqual(resources['samplingQualificationFailures'], ['owned_encoder_observation_failed'])
        self.assertEqual(resources['samplingObservationFailures'],
            [{'stage': 'arguments_before', 'exceptionClass': 'RuntimeError'}])
        self.assertNotIn('synthetic_private_target', json.dumps(resources))
        return resources

    def test_sampler_retains_safe_process_and_argument_details(self):
        resources = self.sample({'observer_before_state': 'S', 'observer_after_state': 'R',
            'observer_argument_bytes': 0, 'observer_argument_shape': 'empty'})
        self.assertEqual(resources.get('samplingObservationDetails'), [{
            'stage': 'arguments_before', 'exceptionClass': 'RuntimeError',
            'beforeState': 'S', 'afterState': 'R', 'argumentShape': 'empty',
            'argumentBytes': 0, 'argumentCount': None, 'maximumArgumentBytes': None,
        }])

    def test_sampler_rejects_untrusted_argument_detail_values(self):
        resources = self.sample({'observer_before_state': 'synthetic_private_target',
            'observer_after_state': ['synthetic_private_target'],
            'observer_argument_shape': 'synthetic_private_target',
            'observer_argument_bytes': True, 'observer_argument_count': -1,
            'observer_maximum_argument_bytes': 65538})
        self.assertEqual(resources.get('samplingObservationDetails'), [{
            'stage': 'arguments_before', 'exceptionClass': 'RuntimeError',
            'beforeState': 'other', 'afterState': 'other', 'argumentShape': 'other',
            'argumentBytes': None, 'argumentCount': None, 'maximumArgumentBytes': None,
        }])


if __name__ == '__main__':
    unittest.main()
