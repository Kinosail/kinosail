"""Admission of actual origin-refill argv before any encoder callback; no media replay."""
import copy
import hashlib
import json
from pathlib import Path
import sys
import tempfile
import time
import unittest

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'apps/player/scripts'))
from hls_remaining_audio import replay_refill
from hls_remaining_process import source_snapshot


class EncoderReached(Exception):
    pass


class OriginRefillReplayTests(unittest.TestCase):
    def attempt(self, pacing=None, edit=None, edit_case=None):
        with tempfile.TemporaryDirectory() as tmp:
            directory = Path(tmp); source = directory / 'Fixture.flac'
            source.write_bytes(b'pure-control-source-not-media')
            executable = directory / 'codec'; executable.write_bytes(b'pure-control-codec-not-executable')
            root = directory / 'cache' / '0123456789abcdef-plan-a'; root.mkdir(parents=True)
            generation = root.stat()
            fixture = source_snapshot(source)
            codec = {'executableSHA256': hashlib.sha256(executable.read_bytes()).hexdigest()}
            case = {'ownedProcessJoin': {'confirmedZeroSamples': 2, 'remainingOwnedPIDs': [],
                'forcedOwnedGroupStop': False, 'qualificationFailures': []}, 'ownedFFmpegBeforeTeardown': 0,
                'cleanupFailures': [], 'sourceUnchanged': True, 'fixture': fixture,
                'retainedRefillFragments': [{'name': 'segment-00004.m4s'}],
                'encoderStarts': [{'input_seek_ms': 8000, 'segment_start': 4,
                    'mode': 'audio-transcode', 'workClass': 'playback'}],
                'physicalBeforeFirstGET': {'generationInode': generation.st_ino,
                    'generationDevice': generation.st_dev}, 'planDurationSeconds': 10,
                'originSelectedAudio': {'codec': 'flac', 'sampleRate': 48000, 'channels': 2,
                    'layout': 'stereo', 'index': 0, 'sourceIndex': 0}, 'actualCodecInvocation': codec}
            if pacing is not None:
                case['testOnlyRealCodecPacing'] = dict(codec, readrate=pacing)
            args = ['-hide_banner', '-loglevel', 'error', '-y', '-avoid_negative_ts', 'disabled', '-max_delay', '5000000',
                '-ss', '0.000', *(['-readrate', str(pacing)] if pacing is not None else []), '-i', str(source),
                '-map', '0:a:0', '-vn', '-sn', '-dn', '-c:a', 'aac', '-ac', '2', '-b:a', '192000',
                '-output_ts_offset', str(8 - 382976 / 48000), '-bsf:a', 'noise=amount=0:drop=lt(pts\\,382976)',
                '-f', 'hls', '-hls_time', '2', '-hls_playlist_type', 'event', '-hls_segment_type', 'fmp4',
                '-hls_segment_options', 'movflags=+frag_discont+skip_sidx', '-hls_flags', 'temp_file',
                '-hls_fmp4_init_filename', 'init.mp4', '-start_number', '4', '-hls_segment_filename',
                str(root / 'audio/segment-%05d.m4s'), str(root / '.seek-4/audio/index.m3u8')]
            if edit:
                edit(args)
            if edit_case:
                edit_case(case)
            (directory / 'refill-recipe-private.json').write_text(json.dumps(args))
            calls = []
            def encoder(command, timeout):
                calls.append(command)
                raise EncoderReached()
            if edit or edit_case:
                with self.assertRaises((RuntimeError, ValueError, KeyError)):
                    replay_refill(encoder, time.monotonic()+10, directory, source, case, executable)
                self.assertEqual(calls, [])
                self.assertFalse((directory / 'isolated-refill-replay').exists())
            else:
                with self.assertRaises(EncoderReached):
                    replay_refill(encoder, time.monotonic()+10, directory, source, case, executable)
                self.assertEqual(len(calls), 1)
                self.assertTrue(case['isolatedFreshRefillReplay']['closedActualTemplateQualified'])
                self.assertEqual(calls[0][1:-3], args[:-3])
                self.assertEqual(source_snapshot(source), fixture)

    def test_exact_unpaced_and_paced_origin_refill_reach_encoder_callback(self):
        for pacing in [None, 1.25]:
            with self.subTest(pacing=pacing):
                self.attempt(pacing)

    def test_legacy_and_arbitrary_argument_sequences_reject_before_encoding(self):
        edits = [lambda a: a.__setitem__(a.index('-ss')+1, '8.000'),
            lambda a: a.__setitem__(a.index('-i')+1, '/foreign/source.flac'),
            lambda a: a.__setitem__(a.index('-output_ts_offset')+1, '8.000'),
            lambda a: a.__setitem__(a.index('-bsf:a')+1, 'noise=amount=0:drop=lt(pts\\,382977)'),
            lambda a: a.append('-unknown'), lambda a: a.__setitem__(slice(0,2), a[:2][::-1]),
            lambda a: a.__setitem__(a.index('-start_number')+1, '5'),
            lambda a: a.__setitem__(a.index('-hls_segment_filename')+1, '/foreign/output'),
            lambda a: a.__setitem__(slice(-1,None), []),
            lambda a: a.append('x'*4097), lambda a: a.extend(['x']*100)]
        for edit in edits:
            with self.subTest(edit=edit):
                self.attempt(1.25, edit=edit)

    def test_wrong_source_codec_track_duration_or_cleanup_rejects_before_encoding(self):
        edits = [lambda c: c['originSelectedAudio'].update(codec='aac'),
            lambda c: c['originSelectedAudio'].update(sourceIndex=1),
            lambda c: c['originSelectedAudio'].update(index=False),
            lambda c: c['originSelectedAudio'].update(channels=1),
            lambda c: c.update(planDurationSeconds=float('inf')),
            lambda c: c.update(planDurationSeconds=8),
            lambda c: c['actualCodecInvocation'].update(executableSHA256='a'*64),
            lambda c: c['ownedProcessJoin'].update(remainingOwnedPIDs=[42])]
        for edit in edits:
            with self.subTest(edit=edit):
                self.attempt(1.25, edit_case=edit)


if __name__ == '__main__':
    unittest.main()
