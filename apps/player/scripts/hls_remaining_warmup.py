"""Isolated AAC warmup diagnosis; packet cuts and content remain separate facts."""
from array import array
import hashlib
import json
import math
import sys
from hls_followon_public import bounded_bytes, check
from hls_remaining_audio import packet_evidence
from hls_remaining_process import asset_snapshot, native_pcm, source_snapshot
from hls_timeline_packets import manifest_facts


def fixed_windows(reference, observed):
    def samples(data):
        values = array('h')
        values.frombytes(data)
        if sys.byteorder != 'little':
            values.byteswap()
        return values
    source, public = samples(reference), samples(observed)
    rows = []
    for label, start in [('before', 372000), ('seam', 382976), ('after', 396000), ('eof', 477952)]:
        end, phase = start + 2048, 1024
        row = {'label': label, 'sourceStartSample': start, 'windowSamples': 2048, 'declaredPhaseSamples': phase,
            'referenceSHA256': hashlib.sha256(reference[start * 4:end * 4]).hexdigest(),
            'complete': end * 2 <= len(source) and (end + phase) * 2 <= len(public), 'channels': []}
        rows.append(row)
        if not row['complete']:
            row.update(requiredObservedEndSample=end + phase, observedSamples=len(public) // 2)
            continue
        for channel in [0, 1]:
            a = source[start * 2 + channel:end * 2:2]
            b = public[(start + phase) * 2 + channel:(end + phase) * 2:2]
            energy, norm = sum(v * v for v in a), sum(v * v for v in b)
            check(energy > 0 and norm > 0, 'warmup_window_energy')
            error = math.sqrt(sum((x - y) ** 2 for x, y in zip(a, b)) / energy)
            correlation = sum(x * y for x, y in zip(a, b)) / math.sqrt(energy * norm)
            check(math.isfinite(error) and math.isfinite(correlation), 'warmup_window_finite')
            row['channels'].append({'channel': channel, 'normalizedRMSError': error, 'correlationAtDeclaredPhase': correlation})
    check(len({v['referenceSHA256'] for v in rows}) == len(rows), 'warmup_reference_windows_unique')
    return rows


def warmup_counterfactual(run, deadline, directory, result, interrupted, control, executable):
    result.update(boundary='Isolated warmup/packet-suffix/content diagnosis; no production or audible-loss admission.',
        stage='original-binding', rawPacketsNeverTrimmedFromEvidence=True, phaseOptimizationApplied=False)
    case_root = directory / interrupted['name']
    source = case_root / 'media/Fixture.flac'
    before = source_snapshot(source)
    for case in [interrupted, control]:
        join = case.get('ownedProcessJoin', {})
        check(case.get('sourceUnchanged') and case.get('workerBound') and join.get('confirmedZeroSamples') == 2
            and not join.get('forcedOwnedGroupStop') and not join.get('remainingOwnedPIDs')
            and not join.get('qualificationFailures') and not case.get('cleanupFailures'), 'warmup_original_owned_source')
    check(all(interrupted['fixture'].get(k) == v for k, v in before.items())
        and interrupted['fixture']['sha256'] == control['fixture']['sha256'], 'warmup_source_pair')
    actual = interrupted['isolatedFreshRefillReplay']
    check(actual['result'] == 'qualified' and actual['matchesCanonicalPCM']
        and actual['outputPolicyCounterfactual']['exactPublicPacketRowsMatch'], 'warmup_faithful_control')
    raw = bounded_bytes(case_root / 'refill-recipe-private.json', 8192, 'warmup_recipe_bound')
    check(hashlib.sha256(raw).hexdigest() == actual['privateActualArgvSHA256'], 'warmup_closed_recipe_binding')
    args = json.loads(raw)
    check(hashlib.sha256(bounded_bytes(executable, 256 << 20, 'warmup_codec_bound')).hexdigest() ==
        interrupted['testOnlyRealCodecPacing']['executableSHA256'], 'warmup_installed_codec')
    reference = bounded_bytes(case_root / 'Fixture.flac.pcm', 2 << 20, 'warmup_reference_bound')
    public = bounded_bytes(case_root / 'public.mp4.pcm', 2 << 20, 'warmup_public_bound')
    control_pcm = bounded_bytes(directory / control['name'] / 'public.mp4.pcm', 2 << 20, 'warmup_control_bound')
    check(len(reference) == 480000 * 4 and hashlib.sha256(reference).hexdigest() == interrupted['fullEOFNativeSamples']['sourceSHA256']
        and hashlib.sha256(public).hexdigest() == interrupted['fullEOFNativeSamples']['publicSHA256']
        and hashlib.sha256(control_pcm).hexdigest() == control['fullEOFNativeSamples']['publicSHA256']
        and control['fullEOFNativeSamples']['sourceSHA256'] == interrupted['fullEOFNativeSamples']['sourceSHA256'], 'warmup_native_source_binding')
    result.update(source=before, interruptedWindows=fixed_windows(reference, public), uninterruptedWindows=fixed_windows(reference, control_pcm))
    roots = list((case_root / 'cache').glob('*-plan-*'))
    check(len(roots) == 1, 'warmup_generation_count')
    root = roots[0]
    generation = root.stat()
    check((generation.st_ino, generation.st_dev) == (interrupted['physicalBeforeFirstGET']['generationInode'],
        interrupted['physicalBeforeFirstGET']['generationDevice']), 'warmup_generation_identity')
    canonical, identity = asset_snapshot(root / 'audio/init.mp4', 2 << 20)
    check(identity['sha256'] == interrupted['initializationSHA256'], 'warmup_canonical_init')
    args[args.index('-ss') + 1] = '7.936'
    offset = 8 - 2048 / 48000
    args[args.index('-output_ts_offset') + 1] = str(offset)
    result.update(sourceSeekSeconds=7.936, warmupSourceSamples=3072, outputOffsetSeconds=offset, cases=[])
    try:
        for label, filtered in [('unfiltered', False), ('filtered', True)]:
            row = {'label': label, 'result': 'unqualified', 'fragmentIdentities': []}
            result['cases'].append(row)
            stage = case_root / ('isolated-warmup-' + label)
            stage.mkdir()
            command = list(args)
            if filtered:
                command[command.index('-f'):command.index('-f')] = ['-bsf:a', 'noise=amount=0:drop=lt(pts\\,2048)']
            command[command.index('-hls_segment_filename') + 1] = str(stage / 'segment-%05d.m4s')
            command[-1] = str(stage / 'index.m3u8')
            result['stage'] = label + '-encode'
            run([str(executable), *command], 30)
            init, init_identity = asset_snapshot(stage / 'init.mp4', 2 << 20)
            row['initialization'] = init_identity
            manifest, names = manifest_facts(bounded_bytes(stage / 'index.m3u8', 65536, 'warmup_manifest_bound'))
            row['manifest'] = manifest
            check(init == canonical and manifest['endlist'] and [n for n, _ in names] ==
                [v['name'] for v in interrupted['retainedRefillFragments']], 'warmup_init_eof_sequence')
            data = init
            fragments = []
            for name, _ in names:
                part = bounded_bytes(stage / name, 8 << 20, 'warmup_fragment_bound')
                fragments.append(part)
                data += part
                row['fragmentIdentities'].append({'name': name, 'sha256': hashlib.sha256(part).hexdigest()})
                check(len(data) <= 16 << 20, 'warmup_join_bound')
            tail = stage / 'tail.mp4'
            tail.write_bytes(data)
            row['packets'] = packet_evidence(run, tail)
            streams = json.loads(run(['ffprobe', '-v', 'error', '-select_streams', 'a:0', '-show_entries',
                'stream=time_base', '-of', 'json', str(tail)]))['streams']
            check(len(streams) == 1 and streams[0]['time_base'] == '1/48000', 'warmup_encoder_timebase')
            row['encoderTimeBase'] = streams[0]['time_base']
            pcm, facts = native_pcm(tail, deadline, case_root)
            row.update(nativeEOF=facts, pcmSHA256=hashlib.sha256(pcm).hexdigest())
            if not filtered:
                packets = row['packets']
                check(len(packets) > 3 and all(abs((float(v['pts_time']) - offset) * 48000 - (n * 1024 - 1024)) <= 0.06
                    and math.isfinite(float(v['duration_time'])) and 0 < float(v['duration_time']) <= 1024 / 48000 + 0.000001
                    and v['pts_time'] == v['dts_time'] for n, v in enumerate(packets)), 'warmup_observed_priming_grid')
                row.update(observedEncoderPTS=[n * 1024 - 1024 for n in range(len(packets))], result='observed')
                continue
            row['exactUnfilteredSuffix'] = row['packets'] == result['cases'][0]['packets'][3:]
            check(row['exactUnfilteredSuffix'] and abs(float(row['packets'][0]['pts_time']) - 8) <= 1 / 48000, 'warmup_exact_three_packet_suffix')
            joined = canonical
            for name in interrupted['physicalBeforeFirstGET']['segments']:
                prefix = bounded_bytes(case_root / 'retained-fragments' / name, 8 << 20, 'warmup_retained_prefix_bound')
                expected = next(v['sha256'] for v in interrupted['physicalBeforeFirstGET']['assets'] if v['name'] == name)
                check(hashlib.sha256(prefix).hexdigest() == expected, 'warmup_prefix_binding')
                joined += prefix
                check(len(joined) <= 16 << 20, 'warmup_public_join_bound')
            for fragment in fragments:
                joined += fragment
                check(len(joined) <= 16 << 20, 'warmup_public_join_bound')
            path = stage / 'public.mp4'
            path.write_bytes(joined)
            row['joinedPackets'] = packet_evidence(run, path)
            pcm, facts = native_pcm(path, deadline, case_root)
            points = [float(v['pts_time']) for v in row['joinedPackets']]
            gaps = [float(b['pts_time']) - float(a['pts_time']) - float(a['duration_time'])
                for a, b in zip(row['joinedPackets'], row['joinedPackets'][1:])]
            row.update(joinedNativeEOF=facts, joinedPCMSHA256=hashlib.sha256(pcm).hexdigest(), fixedWindows=fixed_windows(reference, pcm),
                packetClockOrderValid=all(a < b for a, b in zip(points, points[1:])),
                maximumPacketGapSeconds=max([0] + gaps), maximumPacketOverlapSeconds=max([0] + [-v for v in gaps]),
                candidatePacketAssertionsSatisfied=all(a < b for a, b in zip(points, points[1:]))
                    and all(math.isfinite(v) and abs(v) <= 1 / 48000 + 0.000001 for v in gaps), result='observed')
            row['fixedPhaseContentComparison'] = []
            for candidate, baseline in zip(row['fixedWindows'], result['uninterruptedWindows']):
                comparison = {'label': candidate['label'], 'complete': candidate['complete'] and baseline['complete'], 'channels': []}
                row['fixedPhaseContentComparison'].append(comparison)
                check(candidate['referenceSHA256'] == baseline['referenceSHA256'], 'warmup_same_reference_window')
                if comparison['complete']:
                    comparison['channels'] = [{'channel': a['channel'],
                        'normalizedRMSErrorDelta': a['normalizedRMSError'] - b['normalizedRMSError'],
                        'correlationDeltaAtDeclaredPhase': a['correlationAtDeclaredPhase'] - b['correlationAtDeclaredPhase']}
                        for a, b in zip(candidate['channels'], baseline['channels'])]
        result['result'] = 'observed'
    finally:
        result['sourceUnchanged'] = source_snapshot(source) == before
        result['canonicalInitUnchanged'] = asset_snapshot(root / 'audio/init.mp4', 2 << 20)[1] == identity
        check(result['sourceUnchanged'] and result['canonicalInitUnchanged'], 'warmup_source_init_changed')
