"""One-field edit-list counterfactual on generated diagnostic copies only."""
import copy
import hashlib
import struct
from hls_remaining_nonkey_init import boxes, initialization_metadata


def offsets(data, base=0):
    # Reuse strict extent parsing, then retain physical payload positions.
    position = 0
    result = []
    for kind, payload in boxes(data):
        header = 16 if struct.unpack_from('>I', data, position)[0] == 1 else 8
        result.append((kind, base + position + header, bytes(payload)))
        position += header + len(payload)
    return result


def unique(values, kind):
    matches = [v for v in values if v[0] == kind]
    if len(matches) != 1:
        raise RuntimeError('aac_edit_atom_identity')
    return matches[0]


def change_generated_audio_edit(data, delta):
    if type(delta) is not int or not -32 <= delta <= 32:
        raise RuntimeError('aac_edit_delta_bound')
    before = initialization_metadata(data)
    audio = [t for t in before['tracks'] if t['handler'] == 'soun']
    if len(audio) != 1 or audio[0]['mediaTimescale'] != 48000:
        raise RuntimeError('aac_edit_audio_identity')
    edits = audio[0]['edits']
    if len(edits) != 1 or edits[0]['duration'] != 0 or edits[0]['mediaTime'] < 32 or (
            edits[0]['rateInteger'], edits[0]['rateFraction']) != (1, 0):
        raise RuntimeError('aac_edit_generated_shape')
    _, moov_at, moov = unique(offsets(data), b'moov')
    tracks = [v for v in offsets(moov, moov_at) if v[0] == b'trak']
    if len(tracks) != len(before['tracks']):
        raise RuntimeError('aac_edit_track_count')
    selected = [entry for track, entry in zip(before['tracks'], tracks)
                if track['trackID'] == audio[0]['trackID']]
    if len(selected) != 1:
        raise RuntimeError('aac_edit_track_identity')
    _, track_at, track = selected[0]
    _, edts_at, edts = unique(offsets(track, track_at), b'edts')
    _, elst_at, elst = unique(offsets(edts, edts_at), b'elst')
    version = elst[0]
    at, shape = (elst_at + 12, '>i') if version == 0 else (elst_at + 16, '>q')
    prior = struct.unpack_from(shape, data, at)[0]
    if prior != edits[0]['mediaTime']:
        raise RuntimeError('aac_edit_original_field_binding')
    modified = bytearray(data)
    struct.pack_into(shape, modified, at, prior + delta)
    modified = bytes(modified)
    expected = copy.deepcopy(before)
    for track in expected['tracks']:
        if track['trackID'] == audio[0]['trackID']:
            track['edits'][0]['mediaTime'] += delta
    if initialization_metadata(modified) != expected:
        raise RuntimeError('aac_edit_metadata_change_extent')
    width = struct.calcsize(shape)
    if data[:at] != modified[:at] or data[at+width:] != modified[at+width:]:
        raise RuntimeError('aac_edit_byte_change_extent')
    return modified, {'boundary': 'Generated copy counterfactual only; original initialization untouched',
        'audioTrackID': audio[0]['trackID'], 'version': version, 'fieldOffset': at,
        'fieldBytes': width, 'oldMediaTime': prior, 'newMediaTime': prior + delta,
        'deltaSamples': delta, 'allOtherBytesUnchanged': True,
        'originalSHA256': hashlib.sha256(data).hexdigest(),
        'modifiedSHA256': hashlib.sha256(modified).hexdigest(),
        'productionAcceptance': False}


def payload_orders(original, modified):
    def order(rows, stream=None):
        return [(p['stream_index'], p['data_hash']) for p in rows
                if stream is None or p['stream_index'] == stream]
    for rows in [original, modified]:
        if not 0 < len(rows) <= 4096 or {p['stream_index'] for p in rows} != {0, 1}:
            raise RuntimeError('aac_edit_packet_stream_identity')
    tracks = {str(n): {'original': order(original, n), 'modified': order(modified, n),
        'exactOrderCountHashes': order(original, n) == order(modified, n)} for n in [0, 1]}
    return {'perStream': tracks, 'allPerStreamOrderCountHashes': all(
        t['exactOrderCountHashes'] for t in tracks.values()),
        'globalInterleavedOrderEqual': order(original) == order(modified),
        'boundary': 'Every packet retained; inter-track DTS reordering is separately reported'}
