"""Protect complete renderer evidence; public generated rows cannot inject corruption."""
import copy
from pathlib import Path
import unittest
from hls_nonkey_renderer import renderer_facts
import hls_nonkey_renderer


def capture(count, first=0):
    return {'ended': True, 'errorCode': 0, 'captureErrors': 0, 'width': 640,
        'height': 360, 'gesture': 'player-keyboard-space',
        'gestureEvent': {'trusted': True, 'hasBeenActive': True, 'isActive': True},
        'beforeGesture': [[True, 0, 0, False, 1, 1, False, False], [True, 0, 1, False, 1, 1, False, False]],
        'videoPlaybackQuality': {'total': count, 'dropped': 0, 'corrupted': 0},
        'nativeFrames': [{'format': 'I420', 'visibleRect': [0, 0, 640, 360],
            'codedDimensions': [640, 368], 'displayDimensions': [640, 360],
            'rotation': 0, 'flip': False, 'colorSpace': {'primaries': 'bt709',
                'transfer': 'bt709', 'matrix': 'bt709', 'fullRange': False},
            'allocationBytes': 345600, 'layout': [[0, 640], [230400, 320], [288000, 320]]}],
        'rows': [[n / 24, n + 1, format(n + first, '064x'), round(n * 1000000 / 24), 0, 1]
                 for n in range(count)]}


class RendererIntegrity(unittest.TestCase):
    def test_complete_public_window_is_compared_without_hash_trimming(self):
        reference, public = capture(4), capture(2, 2)
        facts = renderer_facts(reference, public, [n / 24 for n in range(4)], 2 / 24)
        self.assertTrue(facts['referenceQualified'])
        self.assertTrue(facts['publicQualified'])
        self.assertTrue(facts['requestedIdentityMatches'])
        self.assertEqual(facts['publicSourceIndices'], [2, 3])
        preroll = renderer_facts(reference, capture(4), [n / 24 for n in range(4)], 2 / 24)
        self.assertFalse(preroll['requestedIdentityMatches'])
        self.assertEqual(preroll['precedingSourceIndices'], [0, 1])

    def test_missing_reference_start_or_tail_cannot_qualify(self):
        for reference in [capture(3), capture(4)]:
            if len(reference['rows']) == 4:
                reference['rows'][0][0] = 0.01
            facts = renderer_facts(reference, capture(2, 2), [n / 24 for n in range(4)], 2 / 24)
            self.assertFalse(facts['referenceQualified'])
            self.assertFalse(facts['requestedIdentityMatches'])

    def test_duplicate_hash_clock_skipped_counter_and_decoder_fault_remain_unqualified(self):
        for fault in ['hash', 'clock', 'counter', 'ended', 'errorCode', 'captureErrors', 'failureClass']:
            public = capture(2, 2)
            if fault == 'hash': public['rows'][1][2] = public['rows'][0][2]
            elif fault == 'clock': public['rows'][1][0] = public['rows'][0][0]
            elif fault == 'counter': public['rows'][1][1] += 1
            elif fault == 'ended': public['ended'] = False
            elif fault == 'failureClass': public['failureClass'] = 'renderer_deadline'
            else: public[fault] = 1
            with self.subTest(fault=fault):
                facts = renderer_facts(capture(4), public, [n / 24 for n in range(4)], 2 / 24)
                self.assertFalse(facts['publicQualified'])
                self.assertFalse(facts['requestedIdentityMatches'])

    def test_interior_and_tail_frame_loss_are_preserved(self):
        reference, public = capture(5), capture(2, 2)
        public['rows'][1][2] = format(4, '064x')
        facts = renderer_facts(reference, public, [n / 24 for n in range(5)], 2 / 24)
        self.assertFalse(facts['requestedIdentityMatches'])
        self.assertEqual(facts['missingRequestedIndices'], [3])
        self.assertEqual(len(facts['completePublicRows']), 2)

    def test_invalid_and_oversized_rows_fail_closed(self):
        for rows in [[[float('nan'), 1, '0' * 64]], [[0, True, '0' * 64]],
                     [[0, 1, 'credential']], [[0, 1, '0' * 64, 2]],
                     [[0, 1, '0' * 64]] * 4097]:
            bad = capture(2)
            bad['rows'] = rows
            with self.subTest(size=len(rows)):
                with self.assertRaises(RuntimeError):
                    renderer_facts(capture(4), bad, [n / 24 for n in range(4)], 2 / 24)

    def test_reference_ambiguity_is_retained_and_cannot_certify(self):
        reference = capture(4)
        reference['rows'][1][2] = reference['rows'][0][2]
        before = copy.deepcopy(reference)
        facts = renderer_facts(reference, capture(2, 2), [n / 24 for n in range(4)], 2 / 24)
        self.assertFalse(facts['referenceQualified'])
        self.assertFalse(facts['requestedIdentityMatches'])
        self.assertEqual(reference, before)

    def test_matching_pixels_cannot_hide_a_wrong_public_movie_clock(self):
        public = capture(2, 2)
        for n, row in enumerate(public['rows']):
            row[0], row[3] = (n + 2) / 24, round((n + 2) * 1000000 / 24)
        facts = renderer_facts(capture(4), public, [n / 24 for n in range(4)], 2 / 24)
        self.assertTrue(facts['publicSourceClockMatches'])
        public['rows'][1][0] += 0.002
        public['rows'][1][3] += 2000
        facts = renderer_facts(capture(4), public, [n / 24 for n in range(4)], 2 / 24)
        self.assertTrue(facts['requestedIdentityMatches'])
        self.assertFalse(facts['publicSourceClockMatches'])

    def test_all_initializations_and_unchanged_reference_are_required(self):
        network = {'master': 200, 'variant': 200, 'initializationSHA256s': ['a' * 64],
            'successfulFragments': 10, 'unexpectedMediaRequests': 0, 'failedMediaResponses': 0}
        self.assertTrue(hls_nonkey_renderer.renderer_delivery_matches(network, 'a' * 64, 10, True))
        self.assertFalse(hls_nonkey_renderer.renderer_delivery_matches(network, 'a' * 64, 10, False))
        network['initializationSHA256s'] = ['b' * 64, 'a' * 64]
        self.assertFalse(hls_nonkey_renderer.renderer_delivery_matches(network, 'a' * 64, 10, True))
        network['initializationSHA256s'] = []
        self.assertFalse(hls_nonkey_renderer.renderer_delivery_matches(network, 'a' * 64, 10, True))

    def test_complete_pixels_do_not_waive_failed_process_or_unjoined_browser(self):
        process = {'exitCode': 0, 'timedOut': False, 'ownedGroupJoined': True,
            'browserOwnershipVerified': True, 'liveOwnedProcesses': 0, 'joinedSamples': 2}
        self.assertTrue(hls_nonkey_renderer.renderer_process_accepted(process, {}))
        for key, bad in [('exitCode', 1), ('timedOut', True), ('ownedGroupJoined', False),
                         ('browserOwnershipVerified', False), ('liveOwnedProcesses', 1), ('joinedSamples', 1)]:
            value = process | {key: bad}
            with self.subTest(key=key):
                self.assertFalse(hls_nonkey_renderer.renderer_process_accepted(value, {}))
        self.assertFalse(hls_nonkey_renderer.renderer_process_accepted(process, {'failureClass': 'renderer_deadline'}))

    def test_native_timestamp_binding_geometry_and_gesture_are_required(self):
        for fault in ['timestamp', 'metadata', 'rotation', 'display', 'autoplay', 'advance', 'dropped']:
            public = capture(2, 2)
            if fault == 'timestamp': public['rows'][1][3] += 2000
            elif fault == 'metadata': public['rows'][1][4] = 1
            elif fault == 'rotation': public['nativeFrames'][0]['rotation'] = 90
            elif fault == 'display': public['nativeFrames'][0]['displayDimensions'] = [1280, 720]
            elif fault == 'autoplay': public['beforeGesture'][1][0] = False
            elif fault == 'advance': public['beforeGesture'][1][1] += 0.002
            else: public['videoPlaybackQuality']['dropped'] = 1
            with self.subTest(fault=fault):
                facts = renderer_facts(capture(4), public, [n / 24 for n in range(4)], 2 / 24)
                self.assertFalse(facts['publicQualified'])

    def test_native_copy_failure_retains_every_partial_callback(self):
        public = capture(2, 2)
        public['rows'][1][2], public['rows'][1][4] = '', None
        public['captureErrors'] = 1
        facts = renderer_facts(capture(4), public, [n / 24 for n in range(4)], 2 / 24)
        self.assertFalse(facts['publicQualified'])
        self.assertEqual(len(facts['completePublicRows']), 2)

    def test_matching_native_planes_retains_color_interpretation_difference(self):
        public = capture(2, 2)
        public['nativeFrames'][0]['colorSpace']['matrix'] = 'smpte170m'
        facts = renderer_facts(capture(4), public, [n / 24 for n in range(4)], 2 / 24)
        self.assertFalse(facts['colorInterpretationMatches'])
        self.assertEqual(facts['rowColumns'][-4:], ['nativeYUV420SHA256', 'nativeTimestampMicroseconds', 'nativeFrameMetadataIndex', 'playbackRate'])

    def test_real_rate_and_unmuted_gesture_startup_are_required(self):
        for fault in ['rate', 'muted', 'volume', 'gesture_rate', 'gesture', 'activation', 'sticky_activation', 'trusted_event']:
            public = capture(2, 2)
            if fault == 'rate': public['rows'][1][5] = 0.25
            elif fault == 'muted': public['beforeGesture'][1][3] = True
            elif fault == 'volume': public['beforeGesture'][1][4] = 0
            elif fault == 'gesture_rate': public['beforeGesture'][1][5] = 0.25
            elif fault == 'activation': public['beforeGesture'][1][7] = True
            elif fault == 'sticky_activation': public['beforeGesture'][1][6] = True
            elif fault == 'trusted_event': public['gestureEvent']['trusted'] = False
            else: public['gesture'] = 'evaluate-play'
            with self.subTest(fault=fault):
                self.assertFalse(renderer_facts(capture(4), public, [n / 24 for n in range(4)], 2 / 24)['publicQualified'])

    def test_normal_speed_observer_and_declared_rate_are_fixed(self):
        root = Path(__file__).resolve().parents[1]
        source = (root / 'e2e/hls-public-renderer.mjs').read_text()
        self.assertIn('const result = {playbackRate: 1, reference: {}, public: {}};', source)
        self.assertIn('media.defaultPlaybackRate = media.playbackRate = 1;', source)
        self.assertNotIn('playbackRate = 0.25', source)
        quarter = capture(2, 2)
        for row in quarter['rows']:
            row[5] = 0.25
        for sample in quarter['beforeGesture']:
            sample[5] = 0.25
        facts = renderer_facts(capture(4), quarter, [n / 24 for n in range(4)], 2 / 24)
        self.assertFalse(facts['publicQualified'])
        self.assertFalse(facts['requestedIdentityMatches'])

    def test_boolean_or_nonfinite_gesture_rate_cannot_mean_normal_speed(self):
        for rate in [True, False, float('nan'), float('inf'), None, '1']:
            public = capture(2, 2)
            public['beforeGesture'][1][5] = rate
            with self.subTest(kind=type(rate).__name__):
                self.assertFalse(renderer_facts(capture(4), public, [n / 24 for n in range(4)], 2 / 24)['publicQualified'])


if __name__ == '__main__':
    unittest.main()
