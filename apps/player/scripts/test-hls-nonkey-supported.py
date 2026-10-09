#!/usr/bin/env python3
"""Measured existing codec capabilities; no raw, renderer or production admission."""
from fractions import Fraction
import hashlib
import json
import math
from pathlib import Path
import subprocess
import sys
import time
from hls_timeline_fixture import fixture
from hls_timeline_http import sha, source_state
from hls_timeline_packets import manifest_facts
from hls_followon_frames import stream_metadata, decode_frames
from hls_followon_public import bounded_bytes, check
from hls_remaining_mux import mux_case
from hls_remaining_nonkey_evidence import observed_media, native_pcm, packet_rows, pcm_tail_correspondence
from hls_remaining_nonkey_init import initialization_metadata
from hls_remaining_nonkey_fragment import fragment_metadata
from hls_remaining_nonkey_deadline import DiagnosticDeadline
from hls_nonkey_supported_readers import nal_evidence, consumers

ROOT = Path(__file__).resolve().parents[3]
RUN = ROOT / '.verification/hls-nonkey-mux' / ('supported-' + time.strftime('%Y%m%dT%H%M%SZ', time.gmtime()))
RUN.mkdir(parents=True)
receipt = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(),
    'tree': subprocess.check_output(['git', 'rev-parse', 'HEAD^{tree}'], text=True).strip(),
    'trackedSourceClean': not subprocess.check_output(
        ['git', 'status', '--porcelain', '--untracked-files=no'], text=True).strip(),
    'result': 'failed', 'cases': [], 'sources': [], 'productionAcceptance': False,
    'boundary': 'Actual pinned-codec producer and consumer observations; all raw frames remain in the receipt.',
    'dependenciesChanged': False, 'requestedRelativeSeconds': 12.5, 'nativeOrBrowserAcceptance': False}
guard = DiagnosticDeadline(240)
guard.__enter__()


def run(command, timeout=30, bound=4 << 20):
    guard.check()
    result = subprocess.run(command, capture_output=True, timeout=timeout)
    check(len(result.stdout) <= bound, 'capability_command_output_bound')
    if result.returncode:
        receipt.setdefault('failedCommands', []).append({'argv': command, 'returncode': result.returncode,
            'stderrBytes': len(result.stderr), 'stderrSHA256': hashlib.sha256(result.stderr).hexdigest(),
            'stderrTail': result.stderr[-2048:].decode(errors='replace')})
        raise RuntimeError('capability_command_failed')
    return result.stdout


def failure(error):
    return str(error) if isinstance(error, RuntimeError) else type(error).__name__


def independent_tail(source, directory, offset, observation):
    # Preserve the unchanged oracle failure. Independent later stages may still be observable.
    operations = [
        ('initialization', lambda: initialization_metadata(bounded_bytes(directory / 'init.mp4', 1 << 20,
                                                                          'capability_init_bound'))),
        ('sourcePacketRows', lambda: packet_rows(source)[0]),
        ('publicPacketRows', lambda: packet_rows(directory / 'joined.mp4')[0])]
    observation['independentStageFailures'] = []
    for name, operation in operations:
        if name in observation:
            continue
        try:
            observation[name] = operation()
        except Exception as error:
            observation['independentStageFailures'].append({'stage': name, 'failureClass': failure(error)})
            guard.check()
    if 'physicalFragments' not in observation:
        try:
            _, names = manifest_facts(bounded_bytes(directory / 'index.m3u8', 65536, 'capability_manifest_bound'))
            observation['physicalFragments'] = []
            for name, _ in names:
                data = bounded_bytes(directory / name, 2 << 20, 'capability_fragment_bound')
                observation['physicalFragments'].append({'name': name, 'sha256': hashlib.sha256(data).hexdigest(),
                                                         'tracks': fragment_metadata(data)})
        except Exception as error:
            observation['independentStageFailures'].append({'stage': 'physicalFragments', 'failureClass': failure(error)})
            guard.check()
    pcm = observation.setdefault('nativePCM', {})
    if all(name in pcm for name in ['source', 'public', 'referenceSeek']):
        return
    data = {}
    for name, path, seek in [('source', source, None), ('public', directory / 'joined.mp4', None),
                             ('referenceSeek', source, offset)]:
        try:
            pcm[name], data[name] = native_pcm(path, seek)
        except Exception as error:
            observation['independentStageFailures'].append({'stage': name, 'failureClass': failure(error)})
            guard.check()
    if len(data) == 3:
        pcm.update(wholePublicEqualsReference=data['public'] == data['referenceSeek'],
            wholePublicCorrespondence=pcm_tail_correspondence(data['source'], data['public'], round(offset * 48000)),
            publicMinusReferenceSamples=pcm['public']['samples'] - pcm['referenceSeek']['samples'],
            publicAndSourceCompleteEOFAccounted=pcm['public']['completeEOFAccounted'] and pcm['source']['completeEOFAccounted'])


def inspect_case(case_run, source, directory, metadata, wanted, source_nals, source_rows, reference_pcm, row):
    public = directory / 'joined.mp4'
    if not public.is_file():
        return
    observation = row.setdefault('observations', {})
    try:
        _, names = manifest_facts(bounded_bytes(directory / 'index.m3u8', 65536, 'capability_manifest_bound'))
        observed_media(source, public, bounded_bytes(directory / 'init.mp4', 1 << 20, 'capability_init_bound'),
                       [directory / name for name, _ in names], metadata, 12.5, observation)
    except Exception as error:
        row['unchangedOracleFailure'] = failure(error)
        guard.check()
    independent_tail(source, directory, 12.5, observation)
    row['rawProducerExact'] = observation.get('mapping', {}).get('exactRequestedSequence', False)
    row['completePublicPCMEqualsReference'] = observation.get('nativePCM', {}).get('wholePublicEqualsReference', False)
    try:
        public_nals = nal_evidence(case_run, public, True)
        row['firstPublicAccessUnit'] = public_nals
        actual = public_nals['rows'][0]
        matches = [p for p in source_nals['rows'] if p['payloadSHA256'] == actual['payloadSHA256']]
        row['firstPublicSourcePacketMatches'] = matches
        row['firstAccessUnitMatchesRequiredIDR'] = len(matches) == 1 and matches[0]['payloadSHA256'] == row['requiredPrecedingIDR']['payloadSHA256']
        row['firstAccessUnitIDRPayloadQualified'] = len(matches) == 1 and actual['containsIDR'] and matches[0]['containsIDR']
        source_packets = [p['payloadSHA256'] for p in source_nals['rows']]
        public_packets = [p['data_hash'].removeprefix('SHA256:') for p in observation.get('publicPacketRows', [])
                          if p['stream_index'] == 0]
        starts = [n for n in range(len(source_packets) - len(public_packets) + 1)
                  if public_packets and source_packets[n:n + len(public_packets)] == public_packets]
        row['videoPayloadTail'] = {'sourcePacketCount': len(source_packets), 'publicPacketCount': len(public_packets),
            'sequenceMatches': starts, 'uniqueCompleteTail': len(starts) == 1 and starts[0] + len(public_packets) == len(source_packets),
            'allPublicPacketsCompared': True, 'packetsDroppedByOracle': 0}
    except Exception as error:
        row['nalOrPayloadObservationFailure'] = failure(error)
        guard.check()
    row['producerQualifiedForFurtherOffsets'] = (row['rawProducerExact'] and row['completePublicPCMEqualsReference']
        and row.get('firstAccessUnitIDRPayloadQualified', False) and row.get('firstAccessUnitMatchesRequiredIDR', False)
        and row.get('videoPayloadTail', {}).get('uniqueCompleteTail', False)
        and observation.get('nativePCM', {}).get('publicAndSourceCompleteEOFAccounted', False)
        and 'unchangedOracleFailure' not in row)
    if row['label'] in ['request-default', 'idr-default', 'idr-no-prior', 'idr-output-shift',
                         'idr-signed-cts', 'idr-input-shift'] and observation.get('publicFrameRows'):
        try:
            consumers(case_run, directory, source_rows, wanted, observation['publicFrameRows'], reference_pcm, row)
        except Exception as error:
            row['consumerObservationFailure'] = failure(error)
            guard.check()


def project(row):
    observation = row.get('observations', {})
    mapping = observation.get('mapping', {})
    return {'container': row['container'], 'label': row['label'], 'result': row.get('result'),
        'rawProducerExact': row.get('rawProducerExact'), 'rawFrames': len(observation.get('publicFrameRows', [])),
        'firstSourceIndices': mapping.get('actualSourceIndices', [])[:16],
        'completePublicPCMEqualsReference': row.get('completePublicPCMEqualsReference'),
        'firstAccessUnitIDRPayloadQualified': row.get('firstAccessUnitIDRPayloadQualified'),
        'producerQualifiedForFurtherOffsets': row.get('producerQualifiedForFurtherOffsets'),
        'failureClass': row.get('failureClass'), 'unchangedOracleFailure': row.get('unchangedOracleFailure'),
        'nalOrPayloadObservationFailure': row.get('nalOrPayloadObservationFailure'),
        'consumerObservationFailure': row.get('consumerObservationFailure'), 'readerClock': row.get('readerClock'),
        'readers': [{'label': r.get('label'), 'result': r.get('result'),
            'frames': len(r.get('completeFrameRows', [])), 'seekedConsumerExact': r.get('seekedConsumerExact'),
            'completeAudioEqualsReference': r.get('audio', {}).get('completeEqualsIndependentReference'),
            'failureClass': r.get('failureClass')} for r in row.get('readers', [])]}


try:
    check(receipt['trackedSourceClean'], 'capability_dirty_tracked_source')
    receipt['codecVersion'] = run(['ffmpeg', '-version'], 10, 65536).decode().splitlines()[0]
    regular, metadata = fixture(RUN, 'regular', 48, ','.join(str(v) for v in range(0, 32, 2)), frames=768)
    receipt['fixtureCommand'] = metadata['command']
    mp4 = RUN / 'regular-copy.mp4'
    copy_command = ['ffmpeg', '-nostdin', '-v', 'error', '-i', str(regular), '-map', '0:v:0', '-map', '0:a:0',
                    '-c', 'copy', str(mp4)]
    run(copy_command, 45)
    receipt['mp4CopyCommand'] = copy_command
    for source in [regular, mp4]:
        before = source_state(source)
        source_facts = stream_metadata(source)
        origin = Fraction(source_facts['format']['start_time'])
        check(abs(float(origin)) <= 0.1, 'capability_source_origin_bound')
        frame_probe = ['ffprobe', '-v', 'error', '-select_streams', 'v:0', '-show_frames',
                       '-show_entries', 'frame=best_effort_timestamp_time', '-of', 'json', str(source)]
        points = [float(r['best_effort_timestamp_time']) for r in json.loads(run(frame_probe))['frames']]
        check(len(points) == 768 and all(math.isfinite(p) for p in points), 'capability_complete_source_frame_clock')
        source_decode, source_rows = decode_frames(source)
        check(len(source_rows) == len(points) and all(abs(r[0] - p) <= 0.0000011
            for r, p in zip(source_rows, points)), 'capability_independent_source_pts')
        source_nals = nal_evidence(run, source)
        check(source_nals['completeVideoPacketCount'] == 768, 'capability_complete_source_payloads')
        wanted = origin + Fraction(25, 2)
        keys = [r for r in source_nals['rows'] if r['containsIDR'] and Fraction(r['pts_time']) <= wanted]
        check(keys, 'capability_preceding_idr_missing')
        key = max(keys, key=lambda r: Fraction(r['pts_time']))
        key_time = Fraction(key['pts_time'])
        delta = wanted - key_time
        source_metadata = dict(metadata, sha256=sha(source), streamOrigins=source_facts,
            sourceTimeOriginSeconds=float(origin), sourceFramePTS=points)
        reference_info, reference_pcm = native_pcm(source, 12.5)
        source_evidence = {'container': source.suffix[1:], 'state': before, 'metadata': source_facts,
            'frameProbeCommand': frame_probe, 'sourceFramePTS': points, 'sourceDecode': source_decode,
            'completeSourceFrameRows': source_rows, 'sourceNALs': source_nals,
            'requestedSourcePTS': str(wanted), 'requiredPrecedingIDR': key,
            'measuredDeltaSeconds': str(delta), 'independentAudioReference': reference_info}
        receipt['sources'].append(source_evidence)
        output = ['-avoid_negative_ts', 'disabled', '-output_ts_offset', str(-float(delta))]
        segment = 'movflags=+skip_sidx:avoid_negative_ts=disabled:use_editlist=1'
        candidates = [('request-default', {}, 12.5),
            ('request-no-prior', {'output': ['-copypriorss:v', '0']}, 12.5),
            ('request-keep-prior', {'output': ['-copypriorss:v', '1']}, 12.5),
            ('idr-default', {}, float(key_time - origin)),
            ('idr-no-prior', {'output': ['-copypriorss:v', '0']}, float(key_time - origin)),
            ('idr-output-shift', {'output': output, 'segment': segment}, float(key_time - origin)),
            ('idr-signed-cts', {'output': output, 'segment':
                'movflags=+skip_sidx+negative_cts_offsets:avoid_negative_ts=disabled:use_editlist=1'}, float(key_time - origin)),
            ('idr-input-shift', {'input': ['-itsoffset', str(-float(delta))],
                'output': ['-avoid_negative_ts', 'disabled'], 'segment': segment}, float(key_time - origin))]
        for label, options, seek in candidates:
            guard.check(10)
            directory = RUN / (source.suffix[1:] + '-' + label)
            row = {'container': source.suffix[1:], 'label': label, 'requestedSourcePTS': str(wanted),
                   'requiredPrecedingIDR': key, 'inputSeekSeconds': seek, 'result': 'in-flight',
                   'options': options, 'commands': []}
            receipt['cases'].append(row)
            def case_run(command, timeout=30, bound=4 << 20):
                record = {'argv': command, 'result': 'in-flight'}
                row['commands'].append(record)
                try:
                    data = run(command, timeout, bound)
                    record.update(result='settled-success', stdoutBytes=len(data), stdoutSHA256=hashlib.sha256(data).hexdigest())
                    return data
                except Exception as error:
                    record.update(result='settled-failure', failureClass=failure(error))
                    raise
            row.update(mux_case(case_run, source, source_metadata, directory, label, seek, options))
            inspect_case(case_run, source, directory, source_metadata, wanted, source_nals, source_rows, reference_pcm, row)
            row['assetSHA256'] = {p.name: sha(p) for p in sorted(directory.iterdir()) if p.is_file()}
            print(json.dumps(project(row), separators=(',', ':')), flush=True)
        source_evidence['sourceUnchanged'] = source_state(source) == before
        check(source_evidence['sourceUnchanged'], 'capability_source_changed')
    check(len(receipt['cases']) == 16, 'capability_exact_case_count')
    receipt['qualifyingProducerCases'] = [r['container'] + ':' + r['label'] for r in receipt['cases']
                                         if r.get('producerQualifiedForFurtherOffsets')]
    receipt['furtherOffsetQualificationRequired'] = bool(receipt['qualifyingProducerCases'])
    receipt['result'] = 'observed'
except Exception as error:
    receipt['failureClass'] = failure(error)
finally:
    with guard.cleanup():
        receipt['handledTerminationSignals'] = guard.signals
        receipt['elapsedScope'] = '240-second shared command deadline, then bounded receipt cleanup'
        receipt['allInstrumentedCommandsSettled'] = all(c['result'] != 'in-flight'
            for row in receipt['cases'] for c in row.get('commands', []))
        files = {Path(__file__)} | {Path(m.__file__).resolve() for m in list(sys.modules.values())
            if getattr(m, '__file__', None) and Path(m.__file__).resolve().parent == Path(__file__).resolve().parent}
        receipt['executedScriptSHA256'] = {str(p.relative_to(ROOT)): sha(p) for p in sorted(files)}
        target = RUN / 'receipt.json'
        raw = json.dumps(receipt, separators=(',', ':'), allow_nan=False) + '\n'
        check(0 < len(raw.encode()) <= 32 << 20, 'capability_lossless_receipt_bound')
        target.write_text(raw)
        (RUN / 'SHA256SUMS').write_text(''.join(sha(p) + '  ' + str(p.relative_to(ROOT)) + '\n'
            for p in sorted(files)) + sha(target) + '  receipt.json\n')
        print(json.dumps({'result': receipt['result'], 'revision': receipt['revision'], 'tree': receipt['tree'],
            'receiptSHA256': sha(target), 'cases': len(receipt['cases']), 'failureClass': receipt.get('failureClass'),
            'qualifyingProducerCases': receipt.get('qualifyingProducerCases', []), 'productionAcceptance': False}))
    guard.__exit__()
raise SystemExit(0 if receipt['result'] == 'observed' else 1)
