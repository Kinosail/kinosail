"""Full public AAC payload, skip and native-clock evidence with unchanged strict checks."""
import hashlib
import json
import math
import re
from array import array
import sys
from hls_followon_public import check, bounded_bytes
from hls_remaining_process import native_pcm, asset_snapshot, source_snapshot
from hls_timeline_packets import manifest_facts


def marker_clock(reference, observed):
    """Measure sample displacement; retain full PCM and make no lossy-hash identity claim."""
    def channel(data):
        samples = array('h')
        samples.frombytes(data)
        if sys.byteorder != 'little':
            samples.byteswap()
        return samples[::2]
    source, public = channel(reference), channel(observed)
    start, length, extent = 8 * 48000 - 1024, 2048, 2048
    check(start + length <= len(source) and start + length + extent <= len(public), 'marker_full_window_bound')
    template = source[start:start + length]
    mean = sum(template) / length
    centered = [v - mean for v in template]
    energy = sum(v * v for v in centered)
    check(energy > 0, 'marker_reference_energy')
    def score(lag):
        values = public[start + lag:start + lag + length]
        mean = sum(values) / length
        norm = sum((v - mean) ** 2 for v in values)
        return sum(x * (y - mean) for x, y in zip(centered, values)) / math.sqrt(energy * norm) if norm > 0 else 0
    coarse = max(range(-extent, extent + 1, 16), key=score)
    lag = max(range(max(-extent, coarse - 15), min(extent, coarse + 15) + 1), key=score)
    correlation = score(lag)
    check(math.isfinite(correlation), 'marker_correlation_finite')
    return {'templateSourceStartSample': start, 'windowSamples': length, 'searchLagSamples': [-extent, extent],
        'coarseStepSamples': 16, 'refinementStepSamples': 1, 'selectedChannel': 0, 'sampleRate': 48000,
        'bestLagSamples': lag, 'bestLagSeconds': lag / 48000, 'sampledMaximumCorrelation': correlation,
        'fullPCMUntrimmed': True, 'positiveLagMeansObservedContentLater': True}


def packet_evidence(run, path):
    raw = run(['ffprobe', '-v', 'error', '-select_streams', 'a:0',
        '-read_intervals', '%+#4096', '-show_packets', '-show_data_hash', 'sha256',
        '-show_entries', 'packet=pts_time,dts_time,duration_time,size,data_hash:packet_side_data=side_data_type,skip_samples,discard_padding,skip_reason,discard_reason',
        '-of', 'json', str(path)])
    rows = json.loads(raw).get('packets', [])
    check(0 < len(rows) < 4096, 'audio_packet_bound')
    for row in rows:
        check(re.fullmatch(r'SHA256:[a-f0-9]{64}', row.get('data_hash', '')), 'aac_payload_hash_missing')
        check(int(row['size']) > 0, 'aac_payload_size')
        side = row.get('side_data_list', [])
        check(len(side) <= 8 and all(set(v) <= {'side_data_type', 'skip_samples', 'discard_padding',
              'skip_reason', 'discard_reason'} for v in side), 'aac_skip_metadata_bound')
        check(all(v.get('side_data_type') == 'Skip Samples' and all(type(n) is int and 0 <= n <= 1 << 20
              for key, n in v.items() if key != 'side_data_type') for v in side), 'aac_skip_metadata_shape')
    return rows


def replay_refill(run, run_deadline, directory, source, case, executable):
    """Replay the private actual argv into owned output; qualify every encoded payload."""
    result = {'boundary': 'Isolated argv-bound replay, not capture of the ephemeral joined worker init.',
        'result': 'unqualified', 'fragmentIdentities': []}
    case['isolatedFreshRefillReplay'] = result
    join = case.get('ownedProcessJoin', {})
    check(join.get('confirmedZeroSamples') == 2 and join.get('remainingOwnedPIDs') == []
        and not join.get('forcedOwnedGroupStop') and not join.get('qualificationFailures')
        and case.get('ownedFFmpegBeforeTeardown') == 0 and not case.get('cleanupFailures')
        and not case.get('cleanupFailureClass') and not case.get('diagnosticFailureClass'), 'refill_replay_owned_teardown')
    before = source_snapshot(source)
    check(case.get('sourceUnchanged') and all(case['fixture'].get(k) == v for k, v in before.items()), 'refill_replay_original_source')
    raw = bounded_bytes(directory / 'refill-recipe-private.json', 8192, 'refill_recipe_bound')
    args = json.loads(raw)
    segments = case['retainedRefillFragments']
    number = int(segments[0]['name'].removeprefix('segment-').removesuffix('.m4s'))
    check(isinstance(args, list) and 0 < len(args) <= 96 and all(type(v) is str and len(v) <= 4096 for v in args), 'refill_recipe_shape')
    check(number == 4 and [v for v in case['encoderStarts'] if v['segment_start'] > 0] ==
        [{'input_seek_ms': 8000, 'segment_start': 4, 'mode': 'audio-transcode', 'workClass': 'playback'}], 'refill_recipe_phase')
    generations = list((directory / 'cache').glob('*-plan-*'))
    check(len(generations) == 1, 'refill_recipe_generation_count')
    root = generations[0]
    generation = root.stat()
    check((generation.st_ino, generation.st_dev) == (case['physicalBeforeFirstGET']['generationInode'],
        case['physicalBeforeFirstGET']['generationDevice']), 'refill_recipe_generation_identity')
    expected = ['-hide_banner', '-loglevel', 'error', '-y', '-avoid_negative_ts', 'disabled', '-max_delay', '5000000',
        '-ss', '8.000', '-readrate', str(case['testOnlyRealCodecPacing']['readrate']), '-i', str(source),
        '-map', '0:a:0', '-vn', '-sn', '-dn', '-c:a', 'aac', '-ac', '2', '-b:a', '192000', '-output_ts_offset', '8.000',
        '-f', 'hls', '-hls_time', '2', '-hls_playlist_type', 'event', '-hls_segment_type', 'fmp4',
        '-hls_segment_options', 'movflags=+frag_discont+skip_sidx', '-hls_flags', 'temp_file',
        '-hls_fmp4_init_filename', 'init.mp4', '-start_number', '4', '-hls_segment_filename',
        str(root / 'audio/segment-%05d.m4s'), str(root / '.seek-4/audio/index.m3u8')]
    check(args == expected, 'refill_recipe_closed_actual_template')
    check(hashlib.sha256(bounded_bytes(executable, 256 << 20, 'refill_codec_bound')).hexdigest() ==
        case['testOnlyRealCodecPacing']['executableSHA256'], 'refill_recipe_installed_codec')
    result.update(privateActualArgvSHA256=hashlib.sha256(raw).hexdigest(), closedActualTemplateQualified=True,
        modifications=['owned_output_paths_replaced'], actualPacingAndInputFlagsPreserved=True, stage='encode')
    stage = directory / 'isolated-refill-replay'
    stage.mkdir()
    replay = list(args)
    replay[replay.index('-hls_segment_filename') + 1] = str(stage / 'segment-%05d.m4s')
    replay[-1] = str(stage / 'index.m3u8')
    run([str(executable), *replay], 30)
    fresh, identity = asset_snapshot(stage / 'init.mp4', 2 << 20)
    manifest, names = manifest_facts(bounded_bytes(stage / 'index.m3u8', 65536, 'refill_replay_manifest_bound'))
    result.update(initialization=identity, manifest=manifest, stage='fragment-identity')
    check(manifest['endlist'], 'refill_replay_complete_eof')
    check([name for name, _ in names] == [v['name'] for v in segments], 'refill_replay_segment_sequence')
    joined = fresh
    retained = b''
    identities = result['fragmentIdentities']
    for name, _ in names:
        produced = bounded_bytes(stage / name, 8 << 20, 'refill_replay_fragment_bound')
        actual_bytes = bounded_bytes(directory / 'retained-fragments' / name, 8 << 20, 'retained_refill_bound')
        digest = hashlib.sha256(produced).hexdigest()
        actual_digest = hashlib.sha256(actual_bytes).hexdigest()
        expected_digest = next(v['sha256'] for v in segments if v['name'] == name)
        identities.append({'name': name, 'sha256': digest, 'actualSHA256': actual_digest, 'expectedSHA256': expected_digest})
        check(produced == actual_bytes and digest == expected_digest, 'refill_replay_fragment_identity')
        joined += produced
        retained += actual_bytes
        check(len(joined) <= 16 << 20, 'refill_replay_join_bound')
    path = directory / 'isolated-refill-replay.mp4'
    path.write_bytes(joined)
    actual = packet_evidence(run, path)
    packets = case['refillNativeEOF']['packets']
    match = [v['data_hash'] for v in actual] == [v['data_hash'] for v in packets]
    result.update(packets=actual, payloadSequenceMatchesActualRefill=match, stage='native-decode')
    check(match and source_snapshot(source) == before, 'refill_replay_payload_source_binding')
    # Same actual fragment bytes and replay-produced init; packet and decoded
    # frame multiplicities remain separate evidence.
    path.write_bytes(fresh + retained)
    pcm, facts = native_pcm(path, run_deadline, directory)
    result.update(nativeEOF=facts, pcmSHA256=hashlib.sha256(pcm).hexdigest(), stage='final-identity')
    check(asset_snapshot(stage / 'init.mp4', 2 << 20)[1] == identity and source_snapshot(source) == before, 'refill_replay_final_identity')
    result.update(result='qualified', sourceUnchanged=True, fragmentBytesMatchActualRefill=True, stage='complete')
    return result


def audio_output(api, hls, directory, case, source, run, run_deadline):
    status, master, _ = api.http(hls)
    check(status == 200, 'audio_master_status')
    check(re.findall(r'^audio/index\.m3u8$', master.decode(), re.M) == ['audio/index.m3u8'], 'audio_rendition')
    base = hls.removesuffix('index.m3u8') + 'audio/'
    status, raw, _ = api.http(base + 'index.m3u8')
    facts, segments = manifest_facts(raw)
    case['publicVariant'] = facts
    case['publicVariantSHA256'] = hashlib.sha256(raw).hexdigest()
    check(status == 200 and 0 < len(segments) <= 16, 'audio_variant_status_bound')
    if not facts['endlist'] or facts['playlistType'] != 'VOD':
        case['failures'].append('control_complete_timeline')
    if abs(facts['durationSeconds'] - case['planDurationSeconds']) > 0.1:
        case['failures'].append('control_full_timeline_duration')
    status, init, _ = api.http(base + 'init.mp4')
    check(status == 200 and init and len(init) <= 2 << 20, 'audio_init')
    case['initializationSHA256'] = hashlib.sha256(init).hexdigest()
    prepared = next(v for v in case['physicalBeforeFirstGET']['assets'] if v['name'] == 'init.mp4')
    check(prepared['sha256'] == case['initializationSHA256'], 'prepared_audio_initialization_changed')
    joined = directory / 'public.mp4'
    data = init
    retained_fragments = {}
    retained_directory = directory / 'retained-fragments'
    retained_directory.mkdir()
    case['publicPackets'] = []
    prior, prior_point, prior_packet = None, None, None
    for name, advertised in segments:
        check(re.fullmatch(r'segment-[0-9]{5}\.m4s', name), 'audio_segment_name')
        status, fragment, _ = api.http(base + name)
        check(status == 200 and fragment, 'audio_segment_status')
        data += fragment
        check(len(data) <= 16 << 20, 'audio_join_bound')
        retained_fragments[name] = fragment
        (retained_directory / name).write_bytes(fragment)
        part = directory / 'fragment.mp4'
        part.write_bytes(init + fragment)
        rows = packet_evidence(run, part)
        seam = {'previous': prior_packet, 'next': rows[0],
            'samePayload': prior_packet is not None and prior_packet['data_hash'] == rows[0]['data_hash']}
        gaps, order_valid = [], True
        for row in rows:
            point, duration = float(row['pts_time']), float(row['duration_time'])
            check(math.isfinite(point) and 0 < duration <= 1, 'audio_packet_clock')
            if prior is not None:
                gaps.append(point - prior)
            order_valid = order_valid and (prior_point is None or point > prior_point)
            prior_point = point
            prior = point + duration
        if any(abs(v) > 0.05 for v in gaps):
            case['failures'].append('audio_packet_discontinuity')
        if not order_valid:
            case['failures'].append('audio_packet_order')
        case['publicPackets'].append({'segment': name, 'advertisedSeconds': advertised,
            'count': len(rows), 'first': rows[0], 'last': rows[-1], 'seamsSeconds': gaps,
            'audioPacketOrderValid': order_valid, 'completePayloadAndSkipEvidence': rows, 'boundaryPayload': seam})
        prior_packet = rows[-1]
    joined.write_bytes(data)
    case['joinedPublicPacketPayloads'] = packet_evidence(run, joined)
    public, public_facts = native_pcm(joined, run_deadline, directory)
    reference, reference_facts = native_pcm(source, run_deadline, directory)
    case['nativePCMQualification'] = {'public': public_facts, 'source': reference_facts}
    if case['fixture'].get('markerNear8Seconds'):
        case['markerSourcePublicSampleClock'] = marker_clock(reference, public)
    if not public_facts['decodedClockOrderValid']:
        case['failures'].append('audio_decoded_frame_order')
    check(public_facts['stream']['codec_name'] == 'aac', 'public_aac_qualification')
    case['fullEOFNativeSamples'] = {'public': len(public) // 4, 'source': len(reference) // 4,
        'publicSHA256': hashlib.sha256(public).hexdigest(), 'sourceSHA256': hashlib.sha256(reference).hexdigest(),
        'decodedToEOF': True, 'trimmed': False, 'channels': 2, 'sampleRate': 48000,
        'rateConversionApplied': False, 'channelConversionApplied': False, 'format': 's16le'}
    if abs(len(public) - len(reference)) > 0.1 * 48000 * 4:
        case['failures'].append('control_audio_sample_count')
    physical_path = directory / 'physical.mp4'
    prefix, prefix_facts = native_pcm(physical_path, run_deadline, directory)
    case['physicalNativeEOF'] = dict(prefix_facts, pcmSHA256=hashlib.sha256(prefix).hexdigest())
    case['physicalPacketPayloads'] = packet_evidence(run, physical_path)
    accurate = directory / 'accurate-source-tail8.flac'
    if case['planDurationSeconds'] > 8.1:
        run(['ffmpeg', '-nostdin', '-v', 'error', '-xerror', '-i', str(source), '-ss', '8',
            '-map', '0:a:0', '-c:a', 'flac', str(accurate)])
    if accurate.exists():
        tail, tail_facts = native_pcm(accurate, run_deadline, directory)
        case['accurateSourceTail8'] = dict(tail_facts, pcmSHA256=hashlib.sha256(tail).hexdigest(),
            outputSideReferenceSeekSeconds=8, decodedToEOF=True, rateConversionApplied=False, channelConversionApplied=False)
    retained = set(case['physicalBeforeFirstGET']['segments'])
    refill = [(name, advertised) for name, advertised in segments if name not in retained]
    if refill:
        path = directory / 'public-refill.mp4'
        refill_data = init
        for name, _ in refill:
            fragment = retained_fragments[name]
            refill_data += fragment
            check(len(refill_data) <= 16 << 20, 'audio_refill_join_bound')
        path.write_bytes(refill_data)
        pcm, facts = native_pcm(path, run_deadline, directory)
        case['refillNativeEOF'] = dict(facts, pcmSHA256=hashlib.sha256(pcm).hexdigest(),
            segments=[name for name, _ in refill], packets=packet_evidence(run, path))
        generations = list((directory / 'cache').glob(hls.split('/')[2] + '-plan-*'))
        check(len(generations) == 1, 'refill_generation_bound')
        generation = generations[0].stat()
        check((generation.st_ino, generation.st_dev) == (case['physicalBeforeFirstGET']['generationInode'],
            case['physicalBeforeFirstGET']['generationDevice']), 'refill_generation_changed')
        case['retainedRefillFragments'] = [{'name': name,
            'sha256': hashlib.sha256(retained_fragments[name]).hexdigest()} for name, _ in refill]
    check(api.http(base + 'init.mp4')[1] == init, 'audio_init_changed')
