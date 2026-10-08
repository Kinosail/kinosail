"""Preserve missing clocks and every raw frame without admitting a failed cut."""
import json
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import patch
from hls_remaining_nonkey_evidence import frame_mapping, packet_rows, pcm_tail_correspondence, observed_media, native_pcm


class NonkeyEvidenceTest(unittest.TestCase):
    def source(self):
        return [(n / 24, f'{n:032x}') for n in range(768)]

    def test_preceding_negative_frames_remain_visible(self):
        source = self.source()
        public = [(n / 24 - 12.5, digest) for n, (_, digest) in enumerate(source[288:], 288)]
        value = frame_mapping(source, public, 12.5)
        self.assertEqual(value['actualSourceIndices'], list(range(288, 768)))
        self.assertEqual(value['precedingSourceFrames'], list(range(288, 300)))
        self.assertEqual(value['expectedSourceIndices'], list(range(300, 768)))
        self.assertEqual(value['publicRows'][0][2], -0.5)
        self.assertEqual(value['negativeFramesDiscarded'], 0)
        self.assertFalse(value['exactRequestedSequence'])

    def test_key_and_exact_requested_controls(self):
        source = self.source()
        for start in [0, 288, 300]:
            with self.subTest(start=start):
                rows = [(n / 24 - start / 24, digest)
                        for n, (_, digest) in enumerate(source[start:], start)]
                self.assertTrue(frame_mapping(source, rows, start / 24)['exactRequestedSequence'])

    def test_unknown_or_ambiguous_frames_rejected(self):
        source = self.source()
        with self.assertRaisesRegex(RuntimeError, 'unknown_delivered_frame'):
            frame_mapping(source, [(0, 'f' * 32)], 0)
        source[1] = source[0]
        with self.assertRaisesRegex(RuntimeError, 'unique_source_frames'):
            frame_mapping(source, source, 0)

    def probe(self, packets):
        return SimpleNamespace(returncode=0, stdout=json.dumps({'packets': packets}).encode())

    def test_missing_demux_dts_is_preserved_as_unqualified(self):
        row = {'data_hash': 'SHA256:' + 'a' * 64, 'pts_time': '0.0', 'duration_time': '0.04'}
        with patch('hls_remaining_nonkey_evidence.subprocess.run', return_value=self.probe([row])):
            rows, missing = packet_rows('unused')
        self.assertEqual(rows, [row])
        self.assertEqual(missing, [{'packet': 0, 'field': 'dts_time'}])
        self.assertNotIn('dts_time', rows[0])

    def test_nonfinite_clock_and_bound_remain_rejected(self):
        row = {'data_hash': 'SHA256:' + 'a' * 64, 'pts_time': 'nan',
               'dts_time': '-0.04', 'duration_time': '0.04'}
        with patch('hls_remaining_nonkey_evidence.subprocess.run', return_value=self.probe([row])):
            with self.assertRaisesRegex(RuntimeError, 'nonkey_packet_clock'):
                packet_rows('unused')
        with patch('hls_remaining_nonkey_evidence.subprocess.run', return_value=self.probe([row] * 4097)):
            with self.assertRaisesRegex(RuntimeError, 'complete_packet_bound'):
                packet_rows('unused')

    def test_whole_pcm_correspondence_never_trims(self):
        source = b''.join(n.to_bytes(4, 'little') for n in range(100))
        value = pcm_tail_correspondence(source, source[20 * 4:], 20)
        self.assertEqual(value['uniqueStartSample'], 20)
        self.assertTrue(value['uniqueSourceTailComplete'])
        changed = pcm_tail_correspondence(source, source[20 * 4:] + bytes(4), 20)
        self.assertEqual(changed['sequenceMatches'], 0)

    def test_ambiguous_pcm_does_not_select_a_match(self):
        value = pcm_tail_correspondence(bytes(100 * 4), bytes(10 * 4), 20)
        self.assertGreater(value['sequenceMatches'], 1)
        self.assertIsNone(value['uniqueStartSample'])

    def test_late_probe_failure_retains_all_earlier_stage_facts(self):
        rows = self.source()
        metadata = {'sourceFramePTS': [p for p, _ in rows], 'sourceTimeOriginSeconds': 0}
        result = {}
        with patch('hls_remaining_nonkey_evidence.decode_frames', return_value=({'rawFrames': 768}, rows)), \
             patch('hls_remaining_nonkey_evidence.initialization_metadata', return_value={'tracks': []}), \
             patch('hls_remaining_nonkey_evidence.bounded_bytes', return_value=b'fragment'), \
             patch('hls_remaining_nonkey_evidence.fragment_metadata', return_value=[]), \
             patch('hls_remaining_nonkey_evidence.packet_rows', side_effect=RuntimeError('probe_failed')):
            with self.assertRaisesRegex(RuntimeError, 'probe_failed'):
                observed_media('source', 'public', b'init', [Path('fragment')], metadata, 0, result)
        self.assertEqual(result['mapping']['actualSourceIndices'], list(range(768)))
        self.assertEqual(result['completedStages'], ['source-decode', 'public-decode',
            'reference-decode', 'initialization', 'physical-fragments'])
        self.assertEqual(result['currentStage'], 'source-packets')
        self.assertEqual(result['sourceFrameRows'], rows)
        self.assertEqual(result['publicFrameRows'], rows)

    def test_source_pts_disagreement_remains_failed_and_visible(self):
        rows = self.source()
        metadata = {'sourceFramePTS': [p + 0.01 for p, _ in rows], 'sourceTimeOriginSeconds': 0}
        result = {}
        with patch('hls_remaining_nonkey_evidence.decode_frames', return_value=({'rawFrames': 768}, rows)):
            with self.assertRaisesRegex(RuntimeError, 'independent_source_pts'):
                observed_media('source', 'public', b'init', [Path('fragment')], metadata, 0, result)
        self.assertEqual(result['sourceFrameRows'], rows)
        self.assertNotIn('publicFrameRows', result)

    def test_pcm_sample_accounting_mismatch_is_explicit(self):
        streams = self.probe([])
        streams.stdout = json.dumps({'streams': [{'sample_rate': '48000', 'channels': 2}]}).encode()
        pcm = SimpleNamespace(returncode=0, stdout=bytes(8))
        frames = SimpleNamespace(returncode=0, stdout=json.dumps({'frames': [{'nb_samples': 3}]}).encode())
        with patch('hls_remaining_nonkey_evidence.subprocess.run', side_effect=[streams, pcm, frames]):
            facts, data = native_pcm('unused')
        self.assertEqual(facts['samples'], 2)
        self.assertEqual(facts['decodedFrameSampleSum'], 3)
        self.assertFalse(facts['completeEOFAccounted'])
        self.assertEqual(data, bytes(8))
