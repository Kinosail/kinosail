"""Bounded edit-list evidence only; never applies a presentation/discard map."""
import struct


def boxes(data):
    result, position = [], 0
    while position < len(data):
        if len(result) >= 64 or len(data) - position < 8:
            raise RuntimeError('initialization_box_bound')
        size, kind = struct.unpack_from('>I4s', data, position)
        header = 8
        if size == 1:
            if len(data) - position < 16:
                raise RuntimeError('initialization_extended_box')
            size = struct.unpack_from('>Q', data, position + 8)[0]
            header = 16
        elif size == 0:
            size = len(data) - position
        if size < header or size > len(data) - position:
            raise RuntimeError('initialization_box_extent')
        result.append((kind, data[position + header:position + size]))
        position += size
    return result


def one(values, kind, required=True):
    selected = [value for key, value in values if key == kind]
    if len(selected) > 1 or required and not selected:
        raise RuntimeError('initialization_metadata_identity')
    return selected[0] if selected else None


def version(data):
    if len(data) < 4 or data[0] not in (0, 1):
        raise RuntimeError('initialization_metadata_version')
    return data[0]


def clock(data):
    current = version(data)
    offset = 12 if current == 0 else 20
    if len(data) < offset + (8 if current == 0 else 12):
        raise RuntimeError('initialization_clock_extent')
    value = struct.unpack_from('>I', data, offset)[0]
    if not value:
        raise RuntimeError('initialization_clock_zero')
    return value


def edits(data):
    current = version(data)
    if len(data) < 8 or bytes(data[1:4]) != bytes(3):
        raise RuntimeError('initialization_edit_header')
    count = struct.unpack_from('>I', data, 4)[0]
    shape = '>Iihh' if current == 0 else '>Qqhh'
    width = struct.calcsize(shape)
    if count > 8 or len(data) != 8 + count * width:
        raise RuntimeError('initialization_edit_bound')
    names = ('duration', 'mediaTime', 'rateInteger', 'rateFraction')
    return [dict(zip(names, struct.unpack_from(shape, data, 8 + index * width)))
            for index in range(count)]


def track(data):
    children = boxes(data)
    header = one(children, b'tkhd')
    current = version(header)
    offset = 12 if current == 0 else 20
    if len(header) < offset + 8:
        raise RuntimeError('initialization_track_extent')
    identifier = struct.unpack_from('>I', header, offset)[0]
    media = boxes(one(children, b'mdia'))
    timescale = clock(one(media, b'mdhd'))
    handler = one(media, b'hdlr')
    if not identifier or len(handler) < 12 or bytes(handler[8:12]) not in (b'vide', b'soun'):
        raise RuntimeError('initialization_track_identity')
    section = one(children, b'edts', required=False)
    selected = one(boxes(section), b'elst') if section is not None else None
    return {'trackID': identifier, 'handler': bytes(handler[8:12]).decode('ascii'),
            'mediaTimescale': timescale, 'edits': edits(selected) if selected is not None else []}


def initialization_metadata(data):
    if not isinstance(data, bytes) or not 0 < len(data) <= 1024 * 1024:
        raise RuntimeError('initialization_input_bound')
    movie = boxes(one(boxes(memoryview(data)), b'moov'))
    timescale = clock(one(movie, b'mvhd'))
    tracks = [track(value) for key, value in movie if key == b'trak']
    if not 0 < len(tracks) <= 8 or len({value['trackID'] for value in tracks}) != len(tracks):
        raise RuntimeError('initialization_tracks_bound')
    return {'movieTimescale': timescale, 'tracks': tracks}
