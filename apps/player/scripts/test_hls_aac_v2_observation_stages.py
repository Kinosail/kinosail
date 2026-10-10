"""Written-first safe failure-stage controls; this is not live lifetime proof."""
from contextlib import ExitStack
import json
import os
from pathlib import Path
from types import SimpleNamespace
import tempfile
import unittest
from unittest.mock import patch
import hls_aac_v2_live_observer as live
from hls_aac_v2_source_identity import regular_identity
from hls_followon_public import sample_resources
from test_hls_aac_v2_fd_sampler import StopAfterOneSample


class ObservationStageControls(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix='kinosail-stage-control-')
        self.addCleanup(self.directory.cleanup)
        self.source = Path(self.directory.name) / 'Fixture.mp4'
        self.source.write_bytes(b'fixed-stage-source')
        self.server = SimpleNamespace(pid=os.getpid(),
            _copiedSourceWitness=regular_identity(self.source))

    def failure_at(self, function, stage):
        with ExitStack() as stack:
            stack.enter_context(patch.object(live, 'owned_children', return_value=[self.server.pid + 1]))
            stack.enter_context(patch.object(live, 'process_identity', return_value=('S', 7)))
            stack.enter_context(patch.object(live, 'actual_arguments',
                return_value=['-i', str(self.source), '-hls_time', '0.1']))
            stack.enter_context(patch.object(live, function,
                side_effect=RuntimeError('synthetic_private_target_must_not_escape')))
            with self.assertRaisesRegex(RuntimeError, '^' + live.FAILURE + '$') as caught:
                live.owned_hls_count(self.server, self.source)
        self.assertEqual(getattr(caught.exception, 'observer_stage', None), stage)
        self.assertEqual(getattr(caught.exception, 'observer_exception_class', None), 'RuntimeError')
        self.assertNotIn('synthetic_private_target', str(caught.exception))

    def test_actual_argument_failure_stage_survives_normalization(self):
        self.failure_at('actual_arguments', 'arguments_before')

    def test_actual_input_failure_stage_survives_normalization(self):
        self.failure_at('input_classification', 'input_before')

    def sample_error(self, stage, exception_class):
        error = RuntimeError('synthetic_private_target_must_not_escape')
        error.observer_stage = stage
        error.observer_exception_class = exception_class
        resources = {'samples': 0, 'peakOwnedFFmpeg': 0, 'samplingErrors': 0}
        stop = StopAfterOneSample()
        with patch('hls_followon_public.encoder_count', side_effect=error):
            sample_resources(self.server, self.source, stop, resources)
        self.assertEqual(resources['samples'], 0)
        self.assertEqual(resources['samplingErrors'], 1)
        self.assertEqual(resources['peakOwnedFFmpeg'], 0)
        self.assertEqual(resources['samplingQualificationFailures'], ['owned_encoder_observation_failed'])
        self.assertTrue(stop.finished)
        self.assertNotIn('synthetic_private_target', json.dumps(resources))
        return resources

    def test_sampler_retains_whitelisted_stage_without_relaxing_error(self):
        resources = self.sample_error('arguments_before', 'RuntimeError')
        self.assertEqual(resources.get('samplingObservationFailures'),
            [{'stage': 'arguments_before', 'exceptionClass': 'RuntimeError'}])

    def test_sampler_rejects_untrusted_stage_and_exception_names(self):
        resources = self.sample_error('synthetic_private_target', 'synthetic_private_target')
        self.assertEqual(resources.get('samplingObservationFailures'),
            [{'stage': 'other', 'exceptionClass': 'other'}])


if __name__ == '__main__':
    unittest.main()
