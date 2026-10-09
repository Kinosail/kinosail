"""Written-first sampler fault accounting protects qualification and owned join."""
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import patch
from hls_followon_public import sample_resources


class StopAfterOneSample:
    def __init__(self):
        self.finished = False

    def is_set(self):
        return self.finished

    def wait(self, unused):
        self.finished = True


class SourceSamplerQualificationControls(unittest.TestCase):
    def test_identity_fault_is_recorded_without_escaping_the_owned_sampler(self):
        stop = StopAfterOneSample()
        resources = {'samples': 0, 'peakOwnedFFmpeg': 0, 'samplingErrors': 0}
        with patch('hls_followon_public.encoder_count',
                   side_effect=RuntimeError('owned_input_identity_unresolved')):
            sample_resources(SimpleNamespace(pid=17), Path('/fixed/Fixture.mp4'), stop, resources)
        self.assertEqual(resources['samples'], 0)
        self.assertEqual(resources['samplingErrors'], 1)
        self.assertTrue(stop.finished)
