"""Raw edit/demux and native sample-clock diagnosis; no discard rule applied."""
from fractions import Fraction


def integer(value):
    if type(value) is int:
        return value
    if isinstance(value, str) and value.lstrip('-').isdigit() and len(value) <= 20:
        return int(value)
    raise RuntimeError('boundary_integer_clock')


def stream_identity(streams):
    if not isinstance(streams, list) or len(streams) != 2:
        raise RuntimeError('boundary_stream_identity')
    identifiers, indices = [], []
    for stream in streams:
        if not isinstance(stream, dict) or not {'id', 'index', 'codec_type', 'time_base'} <= stream.keys():
            raise RuntimeError('boundary_stream_fields')
        identifier = stream['id']
        if not isinstance(identifier, str) or not identifier.startswith('0x') or len(identifier) > 18:
            raise RuntimeError('boundary_stream_id')
        identifier = int(identifier, 0)
        index = stream['index']
        if identifier <= 0 or type(index) is not int or index not in [0, 1]:
            raise RuntimeError('boundary_stream_index')
        if Fraction(stream['time_base']) <= 0:
            raise RuntimeError('boundary_stream_clock')
        identifiers.append(identifier)
        indices.append(index)
    if len(set(identifiers)) != 2 or sorted(indices) != [0, 1]:
        raise RuntimeError('boundary_duplicate_stream')
    if {s['index']: s['codec_type'] for s in streams} != {0: 'video', 1: 'audio'}:
        raise RuntimeError('boundary_stream_codec')
    return streams


def edit_binding(initialization, fragments, streams, packets):
    result = []
    for track in initialization['tracks']:
        identifier = track['trackID']
        selected = [s for s in streams if isinstance(s.get('id'), str)
                    and int(s['id'], 0) == identifier]
        if len(selected) != 1:
            raise RuntimeError('boundary_track_identity')
        stream = selected[0]
        clock = Fraction(stream['time_base'])
        if clock <= 0 or clock != Fraction(1, track['mediaTimescale']):
            raise RuntimeError('boundary_track_clock')
        edits = track['edits']
        if len(edits) > 1 or any(e['rateInteger'] != 1 or e['rateFraction'] != 0
                               or e['mediaTime'] < 0 or e['duration'] != 0 for e in edits):
            raise RuntimeError('boundary_fragmented_edit_shape')
        shift = edits[0]['mediaTime'] if edits else 0
        raw = [sample for fragment in fragments for t in fragment['tracks']
               if t['trackID'] == identifier for sample in t['samples']]
        demuxed = [p for p in packets if p['stream_index'] == stream['index']]
        if not 0 < len(raw) == len(demuxed) <= 4096:
            raise RuntimeError('boundary_sample_packet_count')
        rows, discontinuities = [], []
        for number, (sample, packet) in enumerate(zip(raw, demuxed)):
            pts, dts = integer(packet['pts']), integer(packet['dts'])
            if number and sample['dts'] != raw[number - 1]['dts'] + raw[number - 1]['duration']:
                discontinuities.append(number)
            rows.append({'number': number, 'rawPTS': sample['pts'], 'rawDTS': sample['dts'],
                'duration': sample['duration'], 'editMediaTime': shift, 'demuxPTS': pts,
                'demuxDTS': dts, 'ptsMatchesEdit': pts == sample['pts'] - shift,
                'dtsMatchesEdit': dts == sample['dts'] - shift,
                'packetFlags': packet['flags'], 'discardFlag': 'D' in packet['flags'],
                'payloadSHA256': packet['data_hash']})
        result.append({'trackID': identifier, 'handler': track['handler'],
            'timeBase': str(clock), 'edits': edits, 'completeRows': rows,
            'rawDecodeDiscontinuities': discontinuities,
            'allDemuxClocksMatchEdit': all(r['ptsMatchesEdit'] and r['dtsMatchesEdit'] for r in rows),
            'negativeEditedPTSRows': [r['number'] for r in rows if r['demuxPTS'] < 0],
            'discardFlagRows': [r['number'] for r in rows if r['discardFlag']],
            'payloadExtractedFromRawFragment': False,
            'presentationOrDiscardRuleApplied': False})
    return result


def frame_clock_diagnosis(mapping):
    rows = mapping['publicRows']
    actual = mapping['actualSourceIndices']
    if len(rows) != len(actual) or any(r[0] != n for r, n in zip(rows, actual)):
        raise RuntimeError('boundary_frame_identity')
    # A separate reported clock subset; every actual row remains in the receipt.
    nonnegative = [r[0] for r in rows if Fraction(str(r[2])) >= 0]
    return {'allActualIndices': actual, 'allActualRows': rows,
        'negativeClockIndices': [r[0] for r in rows if Fraction(str(r[2])) < 0],
        'nonnegativeClockIndices': nonnegative,
        'nonnegativeClockSubsetEqualsRequested': nonnegative == mapping['expectedSourceIndices'],
        'rawRequestedSequenceExact': mapping['exactRequestedSequence'],
        'framesTrimmed': 0, 'rendererQualification': False,
        'boundary': 'Reported decoder clock subset is not a certified presentation or discard map'}


def native_clock_rows(frames, time_base, requested, matched):
    clock = Fraction(time_base)
    if clock <= 0 or not 0 < len(frames) <= 4096:
        raise RuntimeError('boundary_native_clock_shape')
    rows, start = [], 0
    targets = [requested] + ([] if matched is None else [matched])
    for number, frame in enumerate(frames):
        count = integer(frame['nb_samples'])
        pts = integer(frame['pts'])
        if not 0 < count <= 8192:
            raise RuntimeError('boundary_native_sample_count')
        timestamp_samples = pts * clock * 48000
        rows.append({'frame': number, 'pts': pts, 'timeBase': str(clock), 'samples': count,
            'nativeStartSample': start, 'timestampSampleNumerator': timestamp_samples.numerator,
            'timestampSampleDenominator': timestamp_samples.denominator,
            'clockMinusOrdinalSamples': str(timestamp_samples - start),
            'containsTargets': [t for t in targets if start <= t < start + count]})
        start += count
    return {'completeRows': rows, 'nativeSampleSum': start,
        'targetRows': [r for r in rows if r['containsTargets']],
        'sourceClockReinterpreted': False, 'samplesTrimmed': 0}


def packet_tail(source, public):
    def audio(rows):
        return [r for r in rows if r['stream_index'] == 1]
    left, right = audio(source), audio(public)
    hashes = [p['data_hash'] for p in left]
    wanted = [p['data_hash'] for p in right]
    matches = [n for n in range(len(hashes) - len(wanted) + 1)
               if wanted and hashes[n:n + len(wanted)] == wanted]
    start = matches[0] if len(matches) == 1 else None
    return {'sourcePackets': len(left), 'publicPackets': len(right),
        'completeSourceRows': left, 'completePublicRows': right,
        'sequenceMatches': len(matches), 'uniqueSourceStart': start,
        'wholePublicPacketTail': start is not None and start + len(right) == len(left),
        'firstSourceMatch': left[start] if start is not None else None,
        'packetsTrimmed': 0}
