"""Bounded raw first-fragment timing; unsigned clocks are never reinterpreted."""
import struct
from hls_nonkey_initialization import boxes, one

KNOWN = {b'moof', b'traf', b'mfhd', b'tfhd', b'tfdt', b'trun', b'mdat'}


def nested(data, allowed):
    values = boxes(data)
    if any(kind in KNOWN and kind not in allowed for kind, _ in values):
        raise RuntimeError('fragment_metadata_layout')
    return values


def full(data, versions, mask):
    if len(data) < 4 or data[0] not in versions or int.from_bytes(data[1:4]) & ~mask:
        raise RuntimeError('fragment_metadata_header')
    return int.from_bytes(data[1:4])


def number(data, offset, shape):
    size = struct.calcsize(shape)
    if offset + size > len(data):
        raise RuntimeError('fragment_metadata_extent')
    return struct.unpack_from(shape, data, offset)[0], offset + size


def header(data):
    flags = full(data, [0], 0x03003b)
    identifier, position = number(data, 4, '>I')
    values = {}
    for flag, name, shape in [(1, 'base', '>Q'), (2, 'description', '>I'),
            (8, 'duration', '>I'), (16, 'size', '>I'), (32, 'flags', '>I')]:
        if flags & flag:
            values[name], position = number(data, position, shape)
    if not identifier or position != len(data):
        raise RuntimeError('fragment_track_header')
    return identifier, values


def decode_time(data):
    full(data, [0, 1], 0)
    value, end = number(data, 4, '>Q' if data[0] else '>I')
    if end != len(data):
        raise RuntimeError('fragment_decode_clock_extent')
    return value


def run(data, base, defaults):
    flags = full(data, [0, 1], 0xf05)
    count, position = number(data, 4, '>I')
    if not 0 < count <= 4096 or flags & 4 and flags & 0x400:
        raise RuntimeError('fragment_sample_count_or_flags')
    if flags & 1:
        _, position = number(data, position, '>i')
    first_flags = None
    if flags & 4:
        first_flags, position = number(data, position, '>I')
    samples = []
    for index in range(count):
        values = dict(defaults)
        values['composition'] = 0
        for flag, name, shape in [(0x100, 'duration', '>I'), (0x200, 'size', '>I'),
                (0x400, 'flags', '>I'), (0x800, 'composition', '>i' if data[0] else '>I')]:
            if flags & flag:
                values[name], position = number(data, position, shape)
        duration = values.get('duration', 0)
        if not duration:
            raise RuntimeError('fragment_sample_duration_missing')
        samples.append({'dts': base, 'pts': base + values['composition'], 'duration': duration,
                        'flags': first_flags if index == 0 and first_flags is not None else values.get('flags')})
        base += duration
    if position != len(data):
        raise RuntimeError('fragment_sample_extent')
    return samples, base


def fragment_track(data):
    values = nested(data, {b'tfhd', b'tfdt', b'trun'})
    identifier, defaults = header(one(values, b'tfhd'))
    base = decode_time(one(values, b'tfdt'))
    runs = [value for key, value in values if key == b'trun']
    if not 0 < len(runs) <= 8:
        raise RuntimeError('fragment_run_bound')
    samples = []
    for value in runs:
        rows, base = run(value, base, defaults)
        samples += rows
        if len(samples) > 4096:
            raise RuntimeError('fragment_sample_bound')
    return {'trackID': identifier, 'samples': samples}


def fragment_metadata(data):
    if not isinstance(data, bytes) or not 0 < len(data) <= 2 * 1024 * 1024:
        raise RuntimeError('fragment_input_bound')
    movie = nested(one(nested(memoryview(data), {b'moof', b'mdat'}), b'moof'), {b'mfhd', b'traf'})
    tracks = [fragment_track(value) for key, value in movie if key == b'traf']
    if (not 0 < len(tracks) <= 8 or len({value['trackID'] for value in tracks}) != len(tracks)
            or sum(len(value['samples']) for value in tracks) > 4096):
        raise RuntimeError('fragment_track_bound')
    return tracks
