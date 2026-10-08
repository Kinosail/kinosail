"""Protect the presentation oracle from silently accepting opening frames.

Public cases cannot inject malformed framemd5 evidence. These bounded parser
checks cover missing clocks and content-based trimming that could hide a seek bug.
"""
import json
from types import SimpleNamespace
import unittest
from unittest.mock import patch
from hls_followon_frames import frame_facts, parse_frames
from hls_followon_public import prepare_once
from hls_timeline_http import PublicServer


class AudioPreparationTests(unittest.TestCase):
    def run_preparation(self, *, kind='audiobook', terminal='HLS transcode completed',
                        completed_id='audio-preparation', preparation_status=202,
                        preparation_body=b'{"state":"queued"}', adoption_status=200,
                        active=False, malformed=False, adoption_headers=None, startup_id='audio-preparation'):
        calls, clock, case = [], [0.0], {'itemKind': kind}

        def http(path, method='GET', body=None, **options):
            calls.append((method, path))
            if method == 'POST':
                return preparation_status, preparation_body, {'X-Request-ID': 'audio-preparation'}
            self.assertGreater(options['timeout'], 0)
            self.assertLessEqual(options['timeout'], 20)
            return adoption_status, b'#EXTM3U\n', adoption_headers or {}

        def logs(*args):
            if malformed:
                return b'{broken}\n'
            entries = [{'msg': 'HLS startup preparation', 'request_id': startup_id, 'state': 'ready'}]
            if terminal:
                entries.append({'msg': terminal, 'request_id': completed_id})
            return ('\n'.join(json.dumps(v) for v in entries) + '\n').encode()

        def count(*args):
            calls.append(('sample', None))
            return int(active)

        def sleep(seconds):
            clock[0] += seconds

        api = SimpleNamespace(http=http)
        log = SimpleNamespace(stat=lambda: SimpleNamespace(st_size=0))
        with patch('hls_followon_public.bounded_bytes', side_effect=logs), \
                patch('hls_followon_public.encoder_count', side_effect=count), \
                patch('hls_followon_public.time.monotonic', side_effect=lambda: clock[0]), \
                patch('hls_followon_public.time.sleep', side_effect=sleep):
            try:
                prepare_once(api, '/prepare', '/hls/fixture/index.m3u8', log, object(), object(), case)
            except (RuntimeError, ValueError) as error:
                return case, calls, clock[0], error
        return case, calls, clock[0], None

    def test_audio_adopts_before_join_and_requires_successful_completion(self):
        for kind in ['audio', 'audiobook']:
            with self.subTest(kind=kind):
                case, calls, _, error = self.run_preparation(kind=kind)
                self.assertIsNone(error)
                self.assertEqual(calls[:2], [('POST', '/prepare'), ('GET', '/hls/fixture/index.m3u8')])
                self.assertEqual(sum(method == 'GET' for method, _ in calls), 1)
                self.assertEqual(case['preparationAttempt']['joinedSamples'], 3)
                self.assertTrue(case['preparationAttempt']['encoderCompleted'])

    def test_partial_ready_cannot_substitute_for_failed_paused_or_missing_completion(self):
        for terminal in ['HLS transcode failed', 'HLS transcode paused after playback became inactive', None]:
            with self.subTest(terminal=terminal):
                _, calls, elapsed, error = self.run_preparation(terminal=terminal)
                self.assertIsNotNone(error)
                self.assertLessEqual(elapsed, 20.05)
                self.assertEqual(sum(method == 'POST' for method, _ in calls), 1)
                self.assertEqual(sum(method == 'GET' for method, _ in calls), 1)

    def test_wrong_request_completion_and_live_worker_fail_with_original_deadline(self):
        for value in [{'completed_id': 'different-request'}, {'active': True}]:
            with self.subTest(value=value):
                _, calls, elapsed, error = self.run_preparation(**value)
                self.assertIsNotNone(error)
                self.assertGreaterEqual(elapsed, 20)
                self.assertLessEqual(elapsed, 20.05)
                self.assertEqual(sum(method == 'GET' for method, _ in calls), 1)

    def test_bad_preparation_rejects_before_adoption_or_process_sampling(self):
        for value in [{'preparation_status': 401}, {'preparation_body': b'not-json'},
                      {'preparation_body': b'x' * (512 * 1024 + 1)}]:
            with self.subTest(value=value):
                _, calls, _, error = self.run_preparation(**value)
                self.assertIsNotNone(error)
                self.assertEqual(calls, [('POST', '/prepare')])

    def test_failed_adoption_and_malformed_logs_never_join(self):
        _, calls, _, error = self.run_preparation(adoption_status=503)
        self.assertIsNotNone(error)
        self.assertEqual(calls, [('POST', '/prepare'), ('GET', '/hls/fixture/index.m3u8')])
        _, _, _, error = self.run_preparation(malformed=True)
        self.assertIsNotNone(error)

    def test_video_preparation_keeps_unadopted_window_contract(self):
        case, calls, _, error = self.run_preparation(kind='video', terminal=None)
        self.assertIsNone(error)
        self.assertEqual(sum(method == 'GET' for method, _ in calls), 0)
        self.assertEqual(case['preparationAttempt']['completionState'], 'ready')


    def test_join_witness_distinguishes_each_missing_join_conjunct(self):
        for options, expected in [({'terminal': None}, (True, False, 0)),
                                  ({'completed_id': 'other'}, (True, False, 0)),
                                  ({'startup_id': 'other'}, (False, True, 0)),
                                  ({'active': True}, (True, True, 1))]:
            with self.subTest(options=options):
                case, _, _, error = self.run_preparation(**options)
                self.assertEqual(str(error), 'one_shot_preparation_not_joined')
                witness = case['preparationAttempt']['joinWitness']
                self.assertEqual((bool(witness['matchedTerminalStates']), witness['encoderCompleted'],
                                  witness['ownedFFmpeg'] or 0), expected)
                self.assertTrue(witness['deadlineExpired'])
                self.assertLessEqual(witness['elapsedMs'], 20050)
                self.assertEqual(witness['joinedSamples'], 0)

    def test_adoption_request_identity_is_separate_bounded_and_never_a_new_gate(self):
        for header, present, matches in [('audio-preparation', True, True), ('other', True, False),
                                         ('PRIVATE?token=secret', False, False), ('x' * 81, False, False)]:
            with self.subTest(header=header):
                case, _, _, error = self.run_preparation(adoption_headers={'X-Request-ID': header})
                self.assertIsNone(error)
                witness = case['preparationAttempt']['joinWitness']
                self.assertEqual(witness['getRequestIDPresent'], present)
                self.assertEqual(witness['getRequestIDMatchesPost'], matches)
                self.assertNotIn(header, json.dumps(witness))
                self.assertEqual(witness['stage'], 'joined')
                self.assertEqual(witness['joinedSamples'], 3)

    def test_terminal_failure_witness_survives_without_replacing_original_failure(self):
        for terminal, flag in [('HLS transcode failed', 'encoderFailed'),
                               ('HLS transcode paused after playback became inactive', 'encoderPaused')]:
            with self.subTest(terminal=terminal):
                case, _, _, error = self.run_preparation(terminal=terminal)
                self.assertEqual(str(error), 'audio_playback_not_completed')
                witness = case['preparationAttempt']['joinWitness']
                self.assertTrue(witness[flag])
                self.assertEqual(witness['stage'], 'join-sampling')
                self.assertFalse(witness['deadlineExpired'])

    def test_invalid_public_state_rejects_before_get_or_encoder_sampling(self):
        for body in [b'null', b'[]', b'false', b'{}', b'{"state":false}', b'{"state":1}',
                     b'{"state":"busy"}', b'{"state":"PRIVATE-unknown"}',
                     b'{"state":"queued","state":"ready"}']:
            with self.subTest(body=body):
                case, calls, _, error = self.run_preparation(preparation_body=body)
                self.assertIsNotNone(error)
                self.assertEqual(calls, [('POST', '/prepare')])
                self.assertNotIn('PRIVATE', json.dumps(case))
        for state in ['ready', 'queued']:
            case, _, _, error = self.run_preparation(preparation_body=json.dumps({'state': state}).encode())
            self.assertIsNone(error)
            self.assertEqual(case['preparationAttempt']['publicState'], state)


class PublicTimeoutTests(unittest.TestCase):
    def test_invalid_timeout_rejects_before_open(self):
        for value in [None, True, '20', 0, -1, 41, float('nan'), float('inf')]:
            with self.subTest(value=value):
                api = PublicServer('http://localhost:1')
                with patch.object(api.opener, 'open') as opened:
                    with self.assertRaisesRegex(RuntimeError, 'public_timeout_invalid'):
                        api.http('/hls/fixture/index.m3u8', timeout=value)
                    opened.assert_not_called()

    def test_adoption_budget_and_existing_default_reach_opener_once(self):
        for value, expected in [({}, 40), ({'timeout': 19.5}, 19.5)]:
            with self.subTest(value=value):
                api = PublicServer('http://localhost:1')
                with patch.object(api.opener, 'open') as opened:
                    opened.return_value.read.return_value = b''
                    opened.return_value.getcode.return_value = 200
                    opened.return_value.headers = {}
                    self.assertEqual(api.http('/hls/fixture/index.m3u8', **value)[0], 200)
                    self.assertEqual(opened.call_count, 1)
                    self.assertEqual(opened.call_args.kwargs['timeout'], expected)


class PresentationOracleTests(unittest.TestCase):
    def test_negative_clock_is_unqualified_and_never_trimmed_without_origin_proof(self):
        rows = [(-0.5, 'a' * 32), (0, 'b' * 32), (1 / 24, 'c' * 32)]
        actual = frame_facts(rows)
        self.assertEqual(actual['rawFrames'], 3)
        self.assertEqual(actual['negativeTimestampFrames'], 1)
        self.assertFalse(actual['presentationQualified'])
        self.assertEqual(actual['presentedFrames'], 3)
        self.assertNotEqual(actual['identity'], frame_facts(rows[1:])['identity'])

    def test_opening_frame_at_zero_cannot_be_trimmed_by_matching_reference(self):
        actual = frame_facts([(0, 'a' * 32), (1 / 24, 'b' * 32)])
        reference = frame_facts([(0, 'b' * 32)])
        self.assertNotEqual(actual['identity'], reference['identity'])

    def test_missing_clock_and_oversized_evidence_are_rejected(self):
        for data in [b'0, 0, 0, 1, 12, ' + b'a' * 32,
                     b'#tb 0: 0/24\n', b'x' * (2 * 1024 * 1024 + 1)]:
            with self.assertRaises(RuntimeError):
                parse_frames(data)

    def test_clock_and_hash_are_parsed_without_rebasing(self):
        rows = parse_frames(b'#tb 0: 1/24\n0, -1, -1, 1, 12, ' + b'a' * 32 +
                            b'\n0, 2, 2, 1, 12, ' + b'b' * 32 + b'\n')
        self.assertEqual(rows, [(-1 / 24, 'a' * 32), (2 / 24, 'b' * 32)])


if __name__ == '__main__':
    unittest.main()
