"""Complete non-key evidence only; never trims or qualifies delivered preroll."""
from collections import Counter
import json
import hashlib
import math
import re
import subprocess
import time
from hls_followon_frames import decode_frames, remaining_timeout, decode_audio_pcm
from hls_followon_public import check, bounded_bytes
from hls_nonkey_initialization import initialization_metadata
from hls_nonkey_mux import experiments, presentation_experiment, fragment_evidence


def packet_rows(path, deadline=None):
    result = subprocess.run(['ffprobe', '-v', 'error', '-read_intervals', '%+#4096',
        '-show_packets', '-show_entries', 'packet=stream_index,pts_time,dts_time,duration_time,flags',
        '-of', 'json', str(path)], capture_output=True, timeout=remaining_timeout(deadline, 30))
    check(result.returncode == 0 and len(result.stdout) <= 2 * 1024 * 1024,
          'nonkey_packet_probe_bound')
    rows = json.loads(result.stdout).get('packets', [])
    check(0 < len(rows) < 4096, 'nonkey_packet_count_bound')
    safe = []
    for row in rows:
        index, flags = row.get('stream_index'), row.get('flags', '')
        check(type(index) is int and 0 <= index < 8 and re.fullmatch(r'[A-Z_]{1,16}', flags),
              'nonkey_packet_identity')
        times = [float(row[key]) if key in row else None
                 for key in ['pts_time', 'dts_time', 'duration_time']]
        check(all(value is None or math.isfinite(value) for value in times)
              and (times[2] is None or 0 < times[2] <= 1), 'nonkey_packet_clock')
        # Compact JSON rows keep every packet in the bounded uploaded receipt.
        # Missing demux timestamps remain explicit nulls, never inferred clocks.
        safe.append(json.dumps([index, *times, flags], separators=(',', ':'), allow_nan=False))
    return {'columns': ['streamIndex', 'ptsSeconds', 'dtsSeconds', 'durationSeconds', 'flags'],
            'encoding': 'Each row is a complete JSON array; null means unavailable', 'rows': safe}


def frame_mapping(source_rows, public_rows, source_pts, requested):
    check(len(source_rows) == len(source_pts) and all(abs(row[0] - point) <= 0.001
          for row, point in zip(source_rows, source_pts)), 'nonkey_independent_source_pts')
    counts = Counter(value for _, value in source_rows)
    index = {value: number for number, (_, value) in enumerate(source_rows) if counts[value] == 1}
    mapped = [index.get(value) for _, value in public_rows]
    qualified = all(value == 1 for value in counts.values()) and all(v is not None for v in mapped)
    expected = [n for n, point in enumerate(source_pts) if point >= requested - 0.000001]
    presented = Counter(mapped)
    return {'boundary': 'Diagnostic correspondence only; no frame discarded or reordered',
        'sourceCorrespondenceQualified': qualified, 'requestedSourcePTS': requested,
        'expectedSourceIndices': expected, 'publicSourceIndices': mapped,
        'missingRequestedIndices': [n for n in expected if n not in presented] if qualified else None,
        'precedingSourceIndices': [n for n in mapped if n not in expected] if qualified else None,
        'duplicatePublicSourceIndices': [n for n, count in presented.items() if count > 1] if qualified else None,
        'ambiguousSourceHashes': sum(value > 1 for value in counts.values()),
        'unknownPublicFrames': sum(n is None for n in mapped),
        'completePublicFrameColumns': ['publicPTS', 'md5', 'sourceIndex', 'sourcePTS'],
        'completePublicFrames': [json.dumps([point, value, mapped[n],
            source_pts[mapped[n]] if mapped[n] is not None else None], separators=(',', ':'))
            for n, (point, value) in enumerate(public_rows)]}


def nonkey_evidence(source, public, init, directory, metadata, offset, case):
    result = {'boundary': 'Diagnostics only; strict public assertions and failures retained',
              'variantLimit': 5, 'offlineDeadlineSeconds': 120}
    case['nonKeyEvidence'] = result
    source_facts, source_rows = decode_frames(source)
    public_facts, public_rows = decode_frames(public)
    reference, reference_rows = decode_frames(source, offset)
    requested = metadata['sourceTimeOriginSeconds'] + offset
    check(reference['presentedFrames'] == case['referenceClock']['expectedFrames'],
          'nonkey_independent_reference_frames')
    result.update(source=source_facts, public=public_facts, reference=reference,
        sourceStreamOrigins=metadata['streamOrigins'], publicStreamOrigins=public_facts['containerPresentation'],
        sourceDecodedFramePTS=metadata['sourceFramePTS'],
        sourceFrameRowColumns=['pts', 'md5'],
        completeSourceFrames=[json.dumps(row, separators=(',', ':')) for row in source_rows],
        completeReferenceFrames=[json.dumps(row, separators=(',', ':')) for row in reference_rows],
        frameMapping=frame_mapping(source_rows, public_rows, metadata['sourceFramePTS'], requested))
    result['sourcePackets'] = packet_rows(source)
    result['publicPackets'] = packet_rows(public)
    result['initialization'] = initialization_metadata(bounded_bytes(init, 1024 * 1024, 'nonkey_init_bound'))
    result['firstFragmentSamples'] = fragment_evidence(bounded_bytes(directory / 'nonkey-first-fragment.m4s',
                                                                   2 * 1024 * 1024, 'nonkey_first_fragment_bound'))
    result['outputZeroDecoder'] = presentation_experiment(public, reference)
    result['offlineVariants'] = experiments(source, directory, offset, metadata, source_rows,
        reference, packet_rows, frame_mapping, time.monotonic() + 120)
    if metadata.get('audioTimeMarked'):
        audio_endpoint_proof(source, public, offset, case)


def pcm_tail_correspondence(source, public, requested):
    if len(source) % 4 or len(public) % 4 or not public or type(requested) is not int:
        raise RuntimeError('aac_pcm_correspondence_shape')
    count = len(public) // 4
    matches = [n for n in range(max(0, requested - 64), min(len(source) // 4, requested + 64) + 1)
        if source[n*4:n*4 + len(public)] == public]
    start = matches[0] if len(matches) == 1 else None
    return {'boundary': 'Whole public PCM diagnostic; no public sample trimmed, padded or rewritten',
        'searchRadiusSamples': 64, 'publicSamples': count, 'sequenceMatches': len(matches),
        'uniqueStartSample': start, 'requestedStartDeltaSamples': start - requested if start is not None else None,
        'uniqueSourceTailComplete': start is not None and start + count == len(source) // 4,
        'publicPCM_SHA256': hashlib.sha256(public).hexdigest()}


def audio_endpoint_proof(source, public, offset, case):
    control = case['markedAAC']['decoderBudgetControl']
    evidence = case['markedAAC']['nativeEndpointControl'] = {'complete': False, 'phase': 'scope',
        'boundary': 'Offline native-rate and resampled endpoint diagnostics; original audio/window/case failures retained; audible acceptance separate'}
    try:
        clocks = [control['sourceClock'], control['publicClock']]
        if not all(c.get('sampleRate') == 48000 and c.get('codecName') == 'aac' and c.get('channels') == 2
                and c['frameRows'] and max(p + n / 48000 for p, n in c['frameRows']) <= 32.1 for c in clocks):
            raise RuntimeError('aac_pcm_scope')
        evidence['phase'] = 'source_native'
        whole_source, left = decode_audio_pcm(source, 48000)
        evidence['sourceNative'] = whole_source
        evidence['phase'] = 'source_seek'
        source_seek, reference = decode_audio_pcm(source, 48000, offset)
        evidence['sourceSeekNative'] = source_seek
        evidence['phase'] = 'public_native'
        whole_public, observed = decode_audio_pcm(public, 48000)
        evidence['publicNative'] = whole_public
        evidence['phase'] = 'resampled_eof'
        resampled, _ = decode_audio_pcm(public, 16000)
        evidence['publicResampledEOF'] = resampled
        requested = round(offset * 48000)
        evidence.update(complete=True, phase='complete',
            sourceNativeMatchesFrameAccounting=whole_source['samples'] == clocks[0]['decodedSamplesAtSourceRate'],
            publicNativeMatchesFrameAccounting=whole_public['samples'] == clocks[1]['decodedSamplesAtSourceRate'],
            nativeSeekSampleDifference=whole_public['samples'] - source_seek['samples'],
            resampledEOFEqualsBudgetControl=resampled['sha256'] == control['public']['decodedPCMSHA256'],
            publicSourceCorrespondence=pcm_tail_correspondence(left, observed, requested),
            sourceSeekCorrespondence=pcm_tail_correspondence(left, reference, requested))
    except (RuntimeError, OSError, ValueError, KeyError, subprocess.TimeoutExpired) as error:
        safe = str(error) if isinstance(error, RuntimeError) else type(error).__name__
        evidence['failureClass'] = safe if re.fullmatch('[a-z_]{1,64}', safe) else 'aac_endpoint_error'
        case['failures'].append('aac_endpoint_unqualified')
