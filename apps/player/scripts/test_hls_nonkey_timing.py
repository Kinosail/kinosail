"""Integrity gaps that valid public media cannot deliberately inject."""
import os
from pathlib import Path
import struct
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock
from hls_followon_frames import audio_sequence


class FloatingAudioCenters(unittest.TestCase):
    def test_fractional_centers_select_actual_sample_windows(self):
        result = SimpleNamespace(returncode=0, stdout=struct.pack('<f', 0.1) * 16000)
        with mock.patch('hls_followon_frames.subprocess.run', return_value=result):
            facts = audio_sequence(Path('synthetic'), 1, centers=[0.25, 0.5, 0.75])
        self.assertEqual(len(facts['windows']), 3)
        self.assertTrue(all(v['available'] for v in facts['windows']))


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


if __name__ == '__main__':
    unittest.main()
