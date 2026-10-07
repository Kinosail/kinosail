"""Integrity gaps that valid public media cannot deliberately inject."""
import os
import hashlib
import json
from pathlib import Path
import struct
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock
from hls_followon_frames import audio_sequence
from hls_timeline_http import PublicServer


class FloatingAudioCenters(unittest.TestCase):
    def test_negative_origin_control_extends_only_the_output_time_bound(self):
        result = SimpleNamespace(returncode=0, stdout=struct.pack('<f', 0.1) * 16000)
        with mock.patch('hls_followon_frames.subprocess.run', return_value=result) as run:
            audio_sequence(Path('synthetic'), 1, centers=[0.25], output_budget_extra=0.5)
        command = run.call_args.args[0]
        self.assertEqual(command[command.index('-t') + 1], '1.7')
        self.assertNotIn('-copyts', command)
        self.assertNotIn('-ss', command)
        for bad in [True, -0.1, 1.1, float('nan')]:
            with mock.patch('hls_followon_frames.subprocess.run', side_effect=AssertionError('process_effect')):
                with self.assertRaises(RuntimeError): audio_sequence(Path('synthetic'), 1, output_budget_extra=bad)

    def test_fractional_centers_select_actual_sample_windows(self):
        result = SimpleNamespace(returncode=0, stdout=struct.pack('<f', 0.1) * 16000)
        with mock.patch('hls_followon_frames.subprocess.run', return_value=result):
            facts = audio_sequence(Path('synthetic'), 1, centers=[0.25, 0.5, 0.75])
        self.assertEqual(len(facts['windows']), 3)
        self.assertTrue(all(v['available'] for v in facts['windows']))


class AACClockIntegrity(unittest.TestCase):
    def probe(self):
        return {'streams': [{'sample_rate': '48000'}], 'format': {'start_time': '-0.5', 'duration': '20.5'},
            'packets_and_frames': [{'type': 'packet', 'pts_time': '0.0', 'duration_time': '0.021333',
                'data_hash': 'SHA256:' + 'a' * 64, 'side_data_list': [{'side_data_type': 'Skip Samples',
                    'skip_samples': 10, 'discard_padding': 0}]},
                {'type': 'frame', 'pts_time': '0.000208', 'nb_samples': 1014}]}

    def test_complete_clock_samples_payload_and_skip_facts_are_retained(self):
        from hls_followon_frames import parse_aac_clock_probe
        facts = parse_aac_clock_probe(json.dumps(self.probe()).encode())
        self.assertEqual(facts['packetRows'][0][-1], [[10, 0]])
        self.assertEqual(facts['frameRows'], [[0.000208, 1014]])
        self.assertEqual(facts['packetRows'][0][2], 'a' * 64)
        self.assertEqual(facts['decodedSamplesAtSourceRate'], 1014)

    def test_missing_foreign_nonfinite_and_oversized_facts_fail(self):
        from hls_followon_frames import parse_aac_clock_probe
        for field, value in [('pts_time', 'NaN'), ('data_hash', 'private'), ('duration_time', '-1')]:
            probe = self.probe(); probe['packets_and_frames'][0][field] = value
            with self.assertRaises(RuntimeError): parse_aac_clock_probe(json.dumps(probe).encode())
        for value in [True, -1, 999999]:
            probe = self.probe(); probe['packets_and_frames'][0]['side_data_list'][0]['skip_samples'] = value
            with self.assertRaises(RuntimeError): parse_aac_clock_probe(json.dumps(probe).encode())
        with self.assertRaises(RuntimeError): parse_aac_clock_probe(b'x' * (2 * 1024 * 1024 + 1))

    def test_multi_packet_edit_skip_is_retained_but_original_scope_stays_false(self):
        from hls_followon_frames import parse_aac_clock_probe
        probe = self.probe()
        probe['packets_and_frames'][0]['side_data_list'][0]['skip_samples'] = 27600
        facts = parse_aac_clock_probe(json.dumps(probe).encode())
        self.assertEqual(facts['packetRows'][0][-1], [[27600, 0]])
        self.assertFalse(facts['skipDiscardCountsInOriginalScope'])
        self.assertTrue(parse_aac_clock_probe(json.dumps(self.probe()).encode())['skipDiscardCountsInOriginalScope'])
        for field, value in [('skip_samples', 48001), ('discard_padding', 48001), ('skip_samples', None)]:
            probe = self.probe(); probe['packets_and_frames'][0]['side_data_list'][0][field] = value
            with self.assertRaises(RuntimeError): parse_aac_clock_probe(json.dumps(probe).encode())
        probe = self.probe(); probe['packets_and_frames'][0]['side_data_list'][0]['side_data_type'] = 'foreign'
        with self.assertRaises(RuntimeError): parse_aac_clock_probe(json.dumps(probe).encode())

    def test_failed_aac_diagnostic_is_retained_without_aborting_other_evidence(self):
        import hls_nonkey_timing as module
        reference = {'decodedSamples': 1, 'windows': []}
        case = {'expectedTimelineSeconds': 19.5, 'failures': ['historical_failure']}
        with mock.patch.object(module, 'audio_sequence', return_value=reference), \
                mock.patch.object(module, 'aac_clock_evidence', side_effect=RuntimeError('aac_skip_shape')):
            module.marked_audio_proof(Path('synthetic-source'), Path('synthetic-public'), {'audioTimeMarked': True}, 12.5, case)
        self.assertEqual(case['markedAAC']['decoderBudgetControl']['failureClass'], 'aac_skip_shape')
        self.assertIn('historical_failure', case['failures'])
        self.assertIn('aac_clock_unqualified', case['failures'])
        self.assertFalse(case['markedAAC']['contentMatches'])

    def test_partial_clock_and_timeout_are_bounded_failed_diagnostics(self):
        import hls_nonkey_timing as module
        import subprocess
        reference = {'decodedSamples': 1, 'windows': []}
        for error, safe in [(RuntimeError('foreign\nprivate'), 'aac_clock_error'),
                            (subprocess.TimeoutExpired('private target', 40), 'aac_clock_timeout')]:
            case = {'expectedTimelineSeconds': 19.5, 'failures': []}
            with mock.patch.object(module, 'audio_sequence', return_value=reference), \
                    mock.patch.object(module, 'aac_clock_evidence', side_effect=[{'sampleRate': 48000}, error]):
                module.marked_audio_proof(Path('synthetic-source'), Path('synthetic-public'), {'audioTimeMarked': True}, 12.5, case)
            facts = case['markedAAC']['decoderBudgetControl']
            self.assertEqual(facts['sourceClock'], {'sampleRate': 48000})
            self.assertEqual(facts['phase'], 'public_probe')
            self.assertEqual(facts['failureClass'], safe)
            self.assertFalse(facts['complete'])
            self.assertIn('aac_clock_unqualified', case['failures'])

    def test_unique_complete_source_tail_is_required(self):
        from hls_nonkey_timing import copied_audio_tail
        source = {'packetRows': [[n / 10, 0.1, format(n, '064x'), []] for n in range(4)]}
        self.assertTrue(copied_audio_tail(source, {'packetRows': source['packetRows'][2:]})['sourceTailComplete'])
        self.assertFalse(copied_audio_tail(source, {'packetRows': source['packetRows'][2:3]})['sourceTailComplete'])
        repeated = {'packetRows': [source['packetRows'][0]] * 4}
        self.assertFalse(copied_audio_tail(repeated, {'packetRows': repeated['packetRows'][2:]})['sourceTailComplete'])


class TimingIntegrity(unittest.TestCase):
    def setUp(self):
        import hls_nonkey_timing
        self.module = hls_nonkey_timing
        self.playlist = (b'#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:2\n'
            b'#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:EVENT\n'
            b'#EXT-X-MAP:URI="init.mp4"\n#EXTINF:2.0,\nsegment-00000.m4s\n#EXT-X-ENDLIST\n')

    def test_raw_bytes_cuts_and_target_validity_are_retained(self):
        value = self.module.playlist_details(self.playlist)
        self.assertEqual(value['text'].encode(), self.playlist)
        self.assertEqual(value['cuts'], [['segment-00000.m4s', 2.0]])
        self.assertTrue(value['targetDurationValid'])
        changed = self.module.playlist_details(self.playlist.replace(b'2.0,', b'2.6,'))
        self.assertFalse(changed['targetDurationValid'])

    def test_private_foreign_malformed_and_oversized_playlists_are_rejected(self):
        for value in [self.playlist.replace(b'init.mp4', b'https://private/init.mp4'),
                      self.playlist.replace(b'segment-00000.m4s', b'../private'),
                      self.playlist.replace(b'#EXT-X-TARGETDURATION:2', b'#EXT-X-TARGETDURATION:NaN'),
                      self.playlist + b'#unknown-secret\n', b'x' * 65537]:
            with self.assertRaises(RuntimeError): self.module.playlist_details(value)

    def test_snapshot_symlink_fifo_and_oversize_are_unqualified_without_wait(self):
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary); real = base / 'real'; real.write_bytes(self.playlist)
            link = base / 'link'; link.symlink_to(real)
            fifo = base / 'fifo'; os.mkfifo(fifo)
            huge = base / 'huge'; huge.write_bytes(b'x' * 65537)
            for path in [link, fifo, huge]:
                self.assertFalse(self.module.physical_snapshot(path)['stable'])
            self.assertTrue(self.module.physical_snapshot(real)['stable'])

    def test_changed_physical_bracket_is_never_called_stable(self):
        same = {'stable': True, 'generation': [1], 'identity': {'sha256': 'a'}}
        self.assertTrue(self.module.stable_bracket(same, same, 0, 0))
        for other in [same | {'stable': False}, same | {'generation': [2]},
                      same | {'identity': {'sha256': 'b'}}]:
            self.assertFalse(self.module.stable_bracket(same, other, 0, 0))
        self.assertFalse(self.module.stable_bracket(same, same, 0, 1))

    def test_missing_silent_and_shifted_audio_content_cannot_pass(self):
        reference = {'windows': [{'available': True, 'frequencyHz': 880, 'rms': 0.1}]}
        self.assertTrue(self.module.audio_content_matches(reference, reference))
        for public in [{'windows': []}, {'windows': [{'available': False}]},
                       {'windows': [{'available': True, 'frequencyHz': 880, 'rms': 0}]},
                       {'windows': [{'available': True, 'frequencyHz': 990, 'rms': 0.1}]}]:
            self.assertFalse(self.module.audio_content_matches(reference, public))

    def test_duplicate_headers_and_unsupported_generated_eof_layout_fail(self):
        for value in [self.playlist.replace(b'#EXT-X-PLAYLIST-TYPE:EVENT',
                b'#EXT-X-PLAYLIST-TYPE:EVENT\n#EXT-X-PLAYLIST-TYPE:VOD'),
                self.playlist.replace(b'#EXT-X-MEDIA-SEQUENCE:0',
                b'#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-MEDIA-SEQUENCE:1'),
                b'#EXT-X-ENDLIST\n' + self.playlist.removesuffix(b'#EXT-X-ENDLIST\n')]:
            with self.assertRaises(RuntimeError): self.module.playlist_details(value)

    def test_available_windows_require_actual_finite_frequency(self):
        for frequency in [None, True, '880', float('nan'), float('inf'), -1]:
            value = {'windows': [{'available': True, 'rms': 0.1, 'frequencyHz': frequency}]}
            self.assertFalse(self.module.audio_content_matches(value, value))
        value = {'windows': [{'available': True, 'rms': 0.1}]}
        self.assertFalse(self.module.audio_content_matches(value, value))
        for rms in [None, True, '0.1', float('nan'), float('inf')]:
            value = {'windows': [{'available': True, 'rms': rms, 'frequencyHz': 880}]}
            self.assertFalse(self.module.audio_content_matches(value, value))

    def test_marked_source_requires_the_independent_frequency_pattern(self):
        centers = [0.25, 3.5, 3.75, 4]
        source = {'windows': [{'available': True, 'rms': 0.1, 'frequencyHz': frequency,
            'sourceTimeSeconds': center + 12.5} for center, frequency in zip(centers, [770, 825, 880, 880])]}
        self.assertTrue(self.module.source_pattern_matches(source, centers, 12.5))
        for field, value in [('frequencyHz', 990), ('sourceTimeSeconds', 0), ('rms', 0)]:
            broken = {'windows': [v | {field: value} for v in source['windows']]}
            self.assertFalse(self.module.source_pattern_matches(broken, centers, 12.5))

    def final_case(self):
        init, fragment = b'init', b'fragment'
        case = {'playlistObservations': {'variantURI': '/hls/' + 'a' * 16 +
            '/p/r-h264-aac-o12500/360p/index.m3u8', 'initial': {'public': self.module.playlist_details(self.playlist)}},
            'initializationSHA256': hashlib.sha256(init).hexdigest(),
            'publicFragments': [{'segment': 'segment-00000.m4s',
                'fragmentSHA256': hashlib.sha256(init + fragment).hexdigest()}], 'failures': []}
        snapshot = {'stable': True, 'generation': [1], 'identity': {'sha256': 'a'},
                    'playlist': self.module.playlist_details(self.playlist)}
        return case, snapshot, init, fragment

    def test_final_qualification_binds_joined_generation_eof_and_targets(self):
        for fault in ['generation', 'eof', 'target']:
            case, snapshot, init, fragment = self.final_case()
            changed = dict(snapshot)
            if fault == 'generation': changed['generation'] = [2]
            else: changed['playlist'] = snapshot['playlist'] | ({'endlist': False} if fault == 'eof' else {'targetDurationValid': False})
            api = SimpleNamespace(http=lambda uri, **kw: (200, self.playlist if uri.endswith('m3u8')
                else init if uri.endswith('init.mp4') else fragment, {}))
            with mock.patch.object(self.module, 'physical_snapshot', side_effect=[snapshot] * 4 + [changed] * 2), \
                    mock.patch.object(self.module.time, 'sleep'):
                self.module.finish_playlist_proof(api, Path('/owned'), None, None, case, lambda *a: 0)
            self.assertFalse(case['playlistObservations']['finalObservationQualified'])

    def test_a_last_response_after_deadline_cannot_qualify(self):
        case, snapshot, init, fragment = self.final_case(); clock = [0]
        def response(uri, **kwargs):
            if uri.endswith('.m4s'): clock[0] = 91
            return 200, self.playlist if uri.endswith('m3u8') else init if uri.endswith('init.mp4') else fragment, {}
        with mock.patch.object(self.module, 'physical_snapshot', return_value=snapshot), \
                mock.patch.object(self.module.time, 'sleep'), \
                mock.patch.object(self.module.time, 'monotonic', side_effect=lambda: clock[0]):
            with self.assertRaises(RuntimeError):
                self.module.finish_playlist_proof(SimpleNamespace(http=response), Path('/owned'), None, None, case, lambda *a: 0)

    def test_invalid_http_budget_has_no_network_effects(self):
        api = PublicServer('http://localhost:12345')
        with mock.patch.object(api.opener, 'open', side_effect=AssertionError('network_effect')):
            for value in [True, 0, -1, 41, float('nan'), float('inf')]:
                with self.assertRaises(RuntimeError): api.http('/healthz', timeout=value)


if __name__ == '__main__':
    unittest.main()
