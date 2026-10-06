"""Evidence-integrity failures generated public MP4 initialization cannot inject.

These parser checks precede the diagnostic parser. They protect rejection of
malformed edit metadata; they do not stand in for public playback acceptance.
"""
import struct
import unittest
from hls_nonkey_initialization import initialization_metadata


def box(kind, data):
    return struct.pack('>I4s', 8 + len(data), kind) + data


def clock(kind, scale, version=0):
    before = bytes(8 if version == 0 else 16)
    after = bytes(4 if version == 0 else 8)
    return box(kind, bytes([version, 0, 0, 0]) + before + struct.pack('>I', scale) + after)


def track(identifier=1, handler=b'vide', edits=((500, 12000, 1, 0),), version=0, duplicate_edits=False):
    before = bytes(8 if version == 0 else 16)
    header = box(b'tkhd', bytes([version, 0, 0, 7]) + before + struct.pack('>II', identifier, 0))
    media = clock(b'mdhd', 24000, version) + box(b'hdlr', bytes(8) + handler + bytes(12))
    entry = '>Iihh' if version == 0 else '>Qqhh'
    edit = box(b'elst', bytes([version, 0, 0, 0]) + struct.pack('>I', len(edits)) +
               b''.join(struct.pack(entry, *value) for value in edits))
    return box(b'trak', header + box(b'mdia', media) + box(b'edts', edit * (2 if duplicate_edits else 1)))


def movie(tracks=None, timescale=1000, version=0):
    return box(b'ftyp', b'iso6' + bytes(4)) + box(b'moov', clock(b'mvhd', timescale, version) +
        (track(version=version) if tracks is None else tracks))


class InitializationEvidenceTests(unittest.TestCase):
    def test_records_signed_edits_and_timescales_without_applying_them(self):
        value = initialization_metadata(movie())
        self.assertEqual(value['movieTimescale'], 1000)
        self.assertEqual(value['tracks'], [{'trackID': 1, 'handler': 'vide', 'mediaTimescale': 24000,
            'edits': [{'duration': 500, 'mediaTime': 12000, 'rateInteger': 1, 'rateFraction': 0}]}])
        empty = initialization_metadata(movie(track(edits=((0, -1, 1, 0),))))
        self.assertEqual(empty['tracks'][0]['edits'][0]['mediaTime'], -1)

    def test_supports_version_one_without_narrowing_signed_time(self):
        value = initialization_metadata(movie(track(edits=((2**32, 2**33, 1, 0),), version=1), version=1))
        self.assertEqual(value['tracks'][0]['edits'][0]['duration'], 2**32)
        self.assertEqual(value['tracks'][0]['edits'][0]['mediaTime'], 2**33)

    def test_absent_edits_are_recorded_without_inventing_a_discard_map(self):
        header = box(b'tkhd', bytes(12) + struct.pack('>II', 1, 0))
        media = box(b'mdia', clock(b'mdhd', 48000) + box(b'hdlr', bytes(8) + b'soun' + bytes(12)))
        self.assertEqual(initialization_metadata(movie(box(b'trak', header + media)))['tracks'][0]['edits'], [])

    def test_rejects_truncation_overflow_and_oversized_input(self):
        for data in [b'', b'1234567', movie()[:-1], struct.pack('>I4s', 2, b'moov'),
                     struct.pack('>I4sQ', 1, b'moov', 2**63), b'x' * (1024 * 1024 + 1)]:
            with self.subTest(length=len(data)), self.assertRaises(RuntimeError):
                initialization_metadata(data)

    def test_rejects_duplicate_movie_clock_track_identity_and_edit_sections(self):
        edit = box(b'elst', bytes(4) + struct.pack('>I', 1) + struct.pack('>Iihh', 500, 12000, 1, 0))
        original = track()[8:]
        values = [movie() + movie(), movie(track() + track()),
            box(b'moov', clock(b'mvhd', 1000) * 2 + track()), movie(track(duplicate_edits=True)),
            movie(box(b'trak', original + box(b'edts', edit))),
            movie(box(b'trak', original.replace(box(b'edts', edit),
                box(b'edts', edit + box(b'elst', bytes(4) + struct.pack('>I', 1) +
                    struct.pack('>Iihh', 600, 13000, 1, 0))))))]
        for data in values:
            with self.subTest(length=len(data)), self.assertRaises(RuntimeError):
                initialization_metadata(data)

    def test_rejects_unsupported_versions_missing_clocks_and_excessive_entries(self):
        for data in [movie(timescale=0), movie(track(version=2)),
                     movie(track(edits=((1, 1, 1, 0),) * 9)),
                     box(b'moov', track()), movie(track(handler=b'xxxx')),
                     movie(b''.join(track(identifier=n) for n in range(1, 10))),
                     movie(box(b'moov', clock(b'mvhd', 1000) + track())),
                     movie() + box(b'free', b'') * 65]:
            with self.subTest(length=len(data)), self.assertRaises(RuntimeError):
                initialization_metadata(data)

    def test_rejects_each_unsupported_clock_and_edit_version_independently(self):
        for kind in [b'mvhd', b'mdhd', b'tkhd', b'elst']:
            data = bytearray(movie())
            offset = data.index(kind) + 4
            self.assertEqual(data[offset], 0)
            data[offset] = 2
            with self.subTest(box=kind), self.assertRaises(RuntimeError):
                initialization_metadata(bytes(data))

    def test_extended_box_and_unknown_leaf_do_not_change_metadata(self):
        ordinary = movie()
        extra = struct.pack('>I4sQ', 1, b'free', 20) + b'data'
        self.assertEqual(initialization_metadata(extra + ordinary), initialization_metadata(ordinary))


if __name__ == '__main__':
    unittest.main()
