"""Raw timing evidence integrity checks authored before fragment parser code."""
import struct
import unittest
from hls_nonkey_fragment import fragment_metadata


def box(kind, data):
    return struct.pack('>I4s', 8 + len(data), kind) + data


def full(kind, version, flags, data):
    return box(kind, bytes([version]) + flags.to_bytes(3, 'big') + data)


def track(identifier=1, base=100, duration=10, offsets=(0, -2), version=1):
    header = full(b'tfhd', 0, 0x28, struct.pack('>III', identifier, duration, 0x10000))
    clock = full(b'tfdt', 1, 0, struct.pack('>Q', base))
    shape = '>i' if version == 1 else '>I'
    run = full(b'trun', version, 0x800, struct.pack('>I', len(offsets)) +
               b''.join(struct.pack(shape, value) for value in offsets))
    return box(b'traf', header + clock + run)


class FragmentTimingTests(unittest.TestCase):
    def test_signed_composition_offsets_and_decode_order_are_retained(self):
        self.assertEqual(fragment_metadata(box(b'moof', track())), [{'trackID': 1,
            'samples': [{'dts': 100, 'pts': 100, 'duration': 10, 'flags': 0x10000},
                        {'dts': 110, 'pts': 108, 'duration': 10, 'flags': 0x10000}]}])

    def test_unsigned_base_clock_is_never_reinterpreted_as_negative(self):
        value = fragment_metadata(box(b'moof', track(base=2**64 - 1, offsets=(0,))))
        self.assertEqual(value[0]['samples'][0]['dts'], 2**64 - 1)

    def test_explicit_per_sample_values_and_multiple_runs_preserve_every_row(self):
        header = full(b'tfhd', 0, 0, struct.pack('>I', 2))
        clock = full(b'tfdt', 0, 0, struct.pack('>I', 0))
        run = full(b'trun', 0, 0x700, struct.pack('>I', 1) + struct.pack('>III', 4, 8, 0))
        value = fragment_metadata(box(b'moof', box(b'traf', header + clock + run + run)))
        self.assertEqual(value[0]['samples'], [{'dts': 0, 'pts': 0, 'duration': 4, 'flags': 0},
                                             {'dts': 4, 'pts': 4, 'duration': 4, 'flags': 0}])

    def test_rejects_missing_clock_duration_and_duplicate_tracks_or_clocks(self):
        valid = track()[8:]
        for value in [box(b'moof', track() * 2), box(b'moof', box(b'traf', valid +
                full(b'tfdt', 0, 0, struct.pack('>I', 1)))),
                box(b'moof', box(b'traf', full(b'tfhd', 0, 0, struct.pack('>I', 1)) +
                    full(b'tfdt', 0, 0, struct.pack('>I', 0)) +
                    full(b'trun', 0, 0, struct.pack('>I', 1)))),
                box(b'moof', box(b'traf', valid.replace(full(b'tfdt', 1, 0, struct.pack('>Q', 100)), b'')))]:
            with self.subTest(length=len(value)), self.assertRaises(RuntimeError):
                fragment_metadata(value)

    def test_rejects_versions_unknown_flags_truncation_and_count_bounds(self):
        valid = box(b'moof', track())
        malformed = [b'', valid[:-1], b'x' * (2 * 1024 * 1024 + 1),
                     box(b'moof', b''.join(track(identifier=n) for n in range(1, 10)))]
        for kind in [b'tfhd', b'tfdt', b'trun']:
            value = bytearray(valid); value[value.index(kind) + 4] = 2; malformed.append(bytes(value))
            value = bytearray(valid); value[value.index(kind) + 5] = 0x80; malformed.append(bytes(value))
        value = bytearray(valid); index = value.index(b'trun') + 8
        value[index:index + 4] = struct.pack('>I', 4097); malformed.append(bytes(value))
        for value in malformed:
            with self.subTest(length=len(value)), self.assertRaises(RuntimeError):
                fragment_metadata(value)


if __name__ == '__main__':
    unittest.main()
