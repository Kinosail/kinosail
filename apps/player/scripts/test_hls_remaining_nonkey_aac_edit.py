"""Adversarial one-field generated edit controls; no production parser changes."""
import struct
import unittest
from hls_remaining_nonkey_init import initialization_metadata
from hls_remaining_nonkey_aac_edit import change_generated_audio_edit, payload_orders


def box(kind, payload):
    return struct.pack('>I4s', 8 + len(payload), kind) + payload


def track(identifier, handler, value=27600, version=0, scale=48000, rate=1):
    before = bytes(8 if version == 0 else 16)
    clock = bytes([version, 0, 0, 0]) + before + struct.pack('>I', scale)
    clock += bytes(4 if version == 0 else 8)
    tkhd = bytes([version, 0, 0, 7]) + before + struct.pack('>II', identifier, 0)
    mdia = box(b'mdhd', clock) + box(b'hdlr', bytes(8) + handler + bytes(12))
    shape = '>Iihh' if version == 0 else '>Qqhh'
    elst = bytes([version, 0, 0, 0]) + struct.pack('>I', 1) + struct.pack(shape, 0, value, rate, 0)
    return box(b'trak', box(b'tkhd', tkhd) + box(b'mdia', mdia) +
               box(b'edts', box(b'elst', elst)))


def movie(audio=None, version=0):
    before = bytes(8 if version == 0 else 16)
    clock = bytes([version, 0, 0, 0]) + before + struct.pack('>I', 1000)
    clock += bytes(4 if version == 0 else 8)
    return box(b'ftyp', b'iso6' + bytes(4)) + box(b'moov', box(b'mvhd', clock) +
        track(1, b'vide', 9328, version, 16000) +
        (track(2, b'soun', version=version) if audio is None else audio))


class EditControls(unittest.TestCase):
    def test_only_audio_media_time_changes(self):
        data = movie()
        original = initialization_metadata(data)
        result, facts = change_generated_audio_edit(data, -16)
        self.assertEqual(initialization_metadata(result)['tracks'][1]['edits'][0]['mediaTime'], 27584)
        self.assertEqual(initialization_metadata(result)['tracks'][0], original['tracks'][0])
        self.assertEqual(initialization_metadata(data), original)
        self.assertTrue(facts['allOtherBytesUnchanged'])
        self.assertEqual(facts['fieldBytes'], 4)
        self.assertFalse(facts['productionAcceptance'])

    def test_version_one_preserves_wide_field(self):
        result, facts = change_generated_audio_edit(movie(version=1), 1)
        self.assertEqual(initialization_metadata(result)['tracks'][1]['edits'][0]['mediaTime'], 27601)
        self.assertEqual(facts['fieldBytes'], 8)

    def test_adjacent_controls_and_zero_are_exact(self):
        for delta in [-1, 0, 1]:
            data = movie()
            result, facts = change_generated_audio_edit(data, delta)
            self.assertEqual(facts['newMediaTime'], 27600 + delta)
            if delta == 0:
                self.assertEqual(result, data)

    def test_delta_does_not_accept_bool_float_or_unbounded(self):
        for delta in [True, 1.0, -33, 33]:
            with self.assertRaises(RuntimeError):
                change_generated_audio_edit(movie(), delta)

    def test_wrong_rate_or_audio_clock_rejected(self):
        for audio in [track(2, b'soun', rate=0), track(2, b'soun', scale=44100)]:
            with self.assertRaises(RuntimeError):
                change_generated_audio_edit(movie(audio), 1)

    def test_missing_or_duplicate_audio_rejected(self):
        for audio in [b'', track(2, b'soun') + track(3, b'soun')]:
            with self.assertRaises(RuntimeError):
                change_generated_audio_edit(movie(audio), 1)

    def test_malformed_extent_cannot_patch(self):
        for data in [movie()[:-1], b'elst' * 10]:
            with self.assertRaises(RuntimeError):
                change_generated_audio_edit(data, 1)

    def test_other_atoms_with_edit_text_are_untouched(self):
        data = movie() + box(b'free', b'ignore elst bytes here')
        result, _ = change_generated_audio_edit(data, 1)
        self.assertEqual(result[-29:], data[-29:])


    def packet_rows(self):
        return [{'stream_index': 0, 'data_hash': 'video-a'},
                {'stream_index': 1, 'data_hash': 'audio-a'},
                {'stream_index': 0, 'data_hash': 'video-b'},
                {'stream_index': 1, 'data_hash': 'audio-b'}]

    def test_cross_track_order_is_reported_without_false_payload_failure(self):
        original = self.packet_rows()
        value = payload_orders(original, [original[1], original[0], original[3], original[2]])
        self.assertTrue(value['allPerStreamOrderCountHashes'])
        self.assertFalse(value['globalInterleavedOrderEqual'])
        self.assertEqual(len(value['perStream']['0']['original']), 2)
        self.assertEqual(len(value['perStream']['1']['modified']), 2)

    def test_same_stream_reorder_or_omission_fails(self):
        original = self.packet_rows()
        for modified in [[original[2], original[1], original[0], original[3]], original[:-1]]:
            self.assertFalse(payload_orders(original, modified)['allPerStreamOrderCountHashes'])

    def test_unknown_or_missing_stream_fails_closed(self):
        original = self.packet_rows()
        for modified in [[original[0]], [dict(p, stream_index=2) for p in original]]:
            with self.assertRaises(RuntimeError):
                payload_orders(original, modified)


if __name__ == '__main__':
    unittest.main()
