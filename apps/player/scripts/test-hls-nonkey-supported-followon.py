#!/usr/bin/env python3
"""Selected supported-seek follow-on; independent from the failed raw producer gate."""
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
from hls_followon_frames import decode_frames, stream_metadata
from hls_followon_public import bounded_bytes, check
from hls_remaining_mux import mux_case
from hls_remaining_nonkey_evidence import observed_media, native_pcm
from hls_remaining_nonkey_boundary import packet_tail
from hls_remaining_nonkey_deadline import DiagnosticDeadline
from hls_nonkey_supported_readers import nal_evidence, read_options, reader_case

ROOT = Path(__file__).resolve().parents[3]
RUN = ROOT / '.verification/hls-nonkey-mux' / ('seek-followon-' + time.strftime('%Y%m%dT%H%M%SZ', time.gmtime()))
RUN.mkdir(parents=True)
receipt = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(),
    'tree': subprocess.check_output(['git', 'rev-parse', 'HEAD^{tree}'], text=True).strip(),
    'trackedSourceClean': not subprocess.check_output(
        ['git', 'status', '--porcelain', '--untracked-files=no'], text=True).strip(),
    'result': 'failed', 'cases': [], 'sources': [], 'productionAcceptance': False,
    'priorProducerGateRemainsEmpty': True, 'dependenciesChanged': False,
    'boundary': 'New consumer-capability gate; default producer oracle remains unchanged; no native/cache admission.'}
guard = DiagnosticDeadline(240)
guard.__enter__()


def failure(error):
    return str(error) if isinstance(error, RuntimeError) else type(error).__name__


def run(command, timeout=30, bound=4 << 20):
    guard.check()
    process = subprocess.run(command, capture_output=True, timeout=timeout)
    check(len(process.stdout) <= bound, 'followon_command_output_bound')
    if process.returncode:
        receipt.setdefault('failedCommands', []).append({'argv': command, 'returncode': process.returncode,
            'stderrSHA256': hashlib.sha256(process.stderr).hexdigest(),
            'stderrTail': process.stderr[-2048:].decode(errors='replace')})
        raise RuntimeError('followon_command_failed')
    return process.stdout


def project(row):
    observed = row.get('observations', {})
    mapping = observed.get('mapping', {})
    return {'container': row['container'], 'request': row['requestedRelativeSeconds'], 'label': row['label'],
        'result': row['result'], 'failureClass': row.get('failureClass'),
        'rawProducerExact': mapping.get('exactRequestedSequence'),
        'rawFrames': len(observed.get('publicFrameRows', [])),
        'actualFirstSourceIndices': mapping.get('actualSourceIndices', [])[:4],
        'requiredIDRPTS': row.get('requiredPrecedingIDR', {}).get('pts_time'),
        'actualIDR': row.get('actualFirstSourcePacket'), 'firstIDRMatchesRequired': row.get('firstIDRMatchesRequired'),
        'videoCompletePayloadTail': row.get('videoCompletePayloadTail'),
        'aacCompletePayloadTail': observed.get('audioPayloadTail', {}).get('wholePublicPacketTail'),
        'rawCompletePCMEqualsReference': observed.get('nativePCM', {}).get('wholePublicEqualsReference'),
        'sourceAndPublicEOF': observed.get('nativePCM', {}).get('publicAndSourceCompleteEOFAccounted'),
        'expectedFrames': len(mapping.get('expectedSourceIndices', [])),
        'independentReferencePCM': row.get('referencePCM', {}).get('sha256'),
        'referenceSamples': row.get('referencePCM', {}).get('samples'),
        'mediaUnchanged': row.get('mediaUnchanged'), 'readerClock': row.get('readerClock'),
        'readers': [{'label': r.get('label'), 'result': r.get('result'), 'copyts': r.get('copytsApplied'),
            'frames': len(r.get('completeFrameRows', [])), 'videoExact': r.get('seekedConsumerExact'),
            'videoEOF': r.get('videoEOFAccounted'), 'audio': r.get('audio'),
            'failureClass': r.get('failureClass')} for r in row.get('readers', [])],
        'consumerCapabilityQualified': row.get('consumerCapabilityQualified')}


try:
    check(receipt['trackedSourceClean'], 'followon_dirty_tracked_source')
    receipt['codecVersion'] = run(['ffmpeg', '-version'], 10, 65536).decode().splitlines()[0]
    regular, base = fixture(RUN, 'regular', 48, ','.join(str(v) for v in range(0, 32, 2)), frames=768)
    receipt['fixtureCommand'] = base['command']
    mp4 = RUN / 'regular-copy.mp4'
    copy = ['ffmpeg', '-nostdin', '-v', 'error', '-i', str(regular), '-map', '0:v:0', '-map', '0:a:0',
            '-c', 'copy', str(mp4)]
    run(copy, 45)
    receipt['mp4CopyCommand'] = copy
    for source in [regular, mp4]:
        before = source_state(source)
        facts = {'container': source.suffix[1:], 'path': str(source), 'state': before, 'currentStage': 'metadata'}
        receipt['sources'].append(facts)
        streams = stream_metadata(source)
        facts['metadata'] = streams
        origin = Fraction(streams['format']['start_time'])
        check(abs(float(origin)) <= 0.1, 'followon_source_origin')
        probe = ['ffprobe', '-v', 'error', '-select_streams', 'v:0', '-show_frames',
                 '-show_entries', 'frame=best_effort_timestamp_time', '-of', 'json', str(source)]
        points = [float(r['best_effort_timestamp_time']) for r in json.loads(run(probe))['frames']]
        facts.update(frameProbeCommand=probe, sourceFramePTS=points, currentStage='source-decode')
        source_decode, source_rows = decode_frames(source)
        facts.update(sourceDecode=source_decode, completeSourceFrameRows=source_rows, currentStage='source-nal')
        check(len(points) == len(source_rows) == 768 and all(math.isfinite(p) and abs(p-r[0]) <= 0.0000011
              for p, r in zip(points, source_rows)), 'followon_independent_frame_clock')
        source_nals = nal_evidence(run, source)
        facts.update(sourceNALs=source_nals, currentStage='selected-cases')
        check(source_nals['completeVideoPacketCount'] == 768, 'followon_complete_source_payloads')
        metadata = dict(base, sha256=sha(source), sourceTimeOriginSeconds=float(origin),
                        streamOrigins=streams, sourceFramePTS=points)
        for relative in [12.5, 13.5, 18.2, 12]:
            requested = origin + Fraction(str(relative))
            keys = [p for p in source_nals['rows'] if p['containsIDR'] and Fraction(p['pts_time']) <= requested]
            check(keys, 'followon_preceding_idr')
            required = max(keys, key=lambda p: Fraction(p['pts_time']))
            key_time = Fraction(required['pts_time'])
            delta = requested - key_time
            reference_info, reference_pcm = native_pcm(source, relative)
            for no_prior in [False, True]:
                guard.check(10)
                label = 'shift-no-prior' if no_prior else 'shift-default-prior'
                directory = RUN / (source.suffix[1:] + '-' + str(relative) + '-' + label)
                output = ['-avoid_negative_ts', 'disabled', '-output_ts_offset', str(-float(delta))]
                if no_prior:
                    output += ['-copypriorss:v', '0']
                options = {'output': output, 'segment': 'movflags=+skip_sidx:avoid_negative_ts=disabled:use_editlist=1'}
                row = {'container': source.suffix[1:], 'requestedRelativeSeconds': relative, 'label': label,
                    'requestedSourcePTS': str(requested), 'requiredPrecedingIDR': required,
                    'measuredDeltaSeconds': str(delta), 'referencePCM': reference_info, 'result': 'in-flight',
                    'commands': [], 'readers': [], 'observations': {}}
                receipt['cases'].append(row)
                def case_run(command, timeout=30, bound=4 << 20):
                    entry = {'argv': command, 'result': 'in-flight'}
                    row['commands'].append(entry)
                    try:
                        data = run(command, timeout, bound)
                        entry.update(result='settled-success', stdoutBytes=len(data),
                                     stdoutSHA256=hashlib.sha256(data).hexdigest())
                        return data
                    except Exception as error:
                        entry.update(result='settled-failure', failureClass=failure(error))
                        raise
                try:
                    row.update(mux_case(case_run, source, metadata, directory, label, float(key_time-origin), options))
                    check(row['result'] == 'observed', 'followon_mux_not_observed')
                    _, names = manifest_facts(bounded_bytes(directory / 'index.m3u8', 65536, 'followon_manifest_bound'))
                    observed = row['observations']
                    observed_media(source, directory / 'joined.mp4',
                        bounded_bytes(directory / 'init.mp4', 1 << 20, 'followon_init_bound'),
                        [directory / name for name, _ in names], metadata, relative, observed)
                    public_nal = nal_evidence(case_run, directory / 'joined.mp4', True)
                    row['firstPublicAccessUnit'] = public_nal
                    first = public_nal['rows'][0]
                    public_video = [p for p in observed['publicPacketRows'] if p['stream_index'] == 0]
                    source_hashes = [p['data_hash'].removeprefix('SHA256:') for p in observed['sourcePacketRows']
                                     if p['stream_index'] == 0]
                    check(source_hashes == [p['payloadSHA256'] for p in source_nals['rows']],
                          'followon_independent_source_payloads')
                    check(public_video[0]['data_hash'] == 'SHA256:' + first['payloadSHA256'],
                          'followon_independent_first_payload')
                    matches = [p for p in source_nals['rows'] if p['payloadSHA256'] == first['payloadSHA256']]
                    check(len(matches) == 1 and first['containsIDR'] and matches[0]['containsIDR'],
                          'followon_first_idr_payload')
                    row['actualFirstSourcePacket'] = matches[0]
                    row['firstIDRMatchesRequired'] = matches[0]['payloadSHA256'] == required['payloadSHA256']
                    delivered = [p['data_hash'].removeprefix('SHA256:') for p in public_video]
                    starts = [n for n in range(len(source_hashes)-len(delivered)+1)
                              if source_hashes[n:n+len(delivered)] == delivered]
                    row['videoPayloadTailStarts'] = starts
                    row['videoCompletePayloadTail'] = len(starts) == 1 and starts[0]+len(delivered) == len(source_hashes)
                    tail = packet_tail(observed['sourcePacketRows'], observed['publicPacketRows'])
                    # Full rows remain lossless in the independently retained packet lists above.
                    tail.pop('completeSourceRows')
                    tail.pop('completePublicRows')
                    tail.update(completeSourceRowsReference='observations.sourcePacketRows where stream_index=1',
                                completePublicRowsReference='observations.publicPacketRows where stream_index=1')
                    observed['audioPayloadTail'] = tail
                    assets = [directory / 'init.mp4', *[directory / name for name, _ in names]]
                    row['mediaSHA256Before'] = {p.name: sha(p) for p in assets}
                    clock, absolute, local = read_options(observed['publicFrameRows'], source_rows,
                                                        requested, stream_metadata(directory / 'joined.mp4'))
                    row['readerClock'] = clock
                    settings = [('accurate-zero-normal-clock', ['-seek_timestamp', '1', '-ss', '0', '-accurate_seek'], False),
                        ('inaccurate-zero-control', ['-seek_timestamp', '1', '-ss', '0', '-noaccurate_seek'], False),
                        ('no-seek-normal-clock-control', [], False),
                        ('accurate-zero-retained-clock-control', ['-seek_timestamp', '1', '-ss', '0', '-accurate_seek'], True)]
                    for reader_label, argv, copyts in settings:
                        reader = {}
                        row['readers'].append(reader)
                        reader_case(case_run, directory / 'joined.mp4', reader_label, argv, source_rows,
                                    requested, reference_pcm, reader, copyts=copyts)
                    row['mediaSHA256After'] = {p.name: sha(p) for p in assets}
                    row['mediaUnchanged'] = row['mediaSHA256Before'] == row['mediaSHA256After']
                    check(row['mediaUnchanged'], 'followon_media_changed')
                    positive = row['readers'][0]
                    row['consumerCapabilityQualified'] = (positive.get('result') == 'observed'
                        and positive.get('seekedConsumerExact', False) and positive.get('videoEOFAccounted', False)
                        and positive.get('audio', {}).get('completeEqualsIndependentReference', False)
                        and row['videoCompletePayloadTail'] and observed['audioPayloadTail']['wholePublicPacketTail']
                        and observed['nativePCM']['publicAndSourceCompleteEOFAccounted'])
                    row['result'] = 'observed'
                except Exception as error:
                    row.update(result='observation-failed', failureClass=failure(error))
                    guard.check()
                print(json.dumps(project(row), separators=(',', ':')), flush=True)
        facts['currentStage'] = 'complete'
        facts['sourceUnchangedAfterCases'] = source_state(source) == before
        check(facts['sourceUnchangedAfterCases'], 'followon_source_changed')
    check(len(receipt['cases']) == 16 and all(r['result'] == 'observed' for r in receipt['cases']),
          'followon_complete_case_count')
    receipt['qualifiedConsumerCases'] = [r['container'] + ':' + str(r['requestedRelativeSeconds']) + ':' + r['label']
                                         for r in receipt['cases'] if r.get('consumerCapabilityQualified')]
    receipt['result'] = 'observed'
except Exception as error:
    receipt['failureClass'] = failure(error)
finally:
    with guard.cleanup():
        receipt['handledTerminationSignals'] = guard.signals
        for facts in receipt['sources']:
            try:
                facts['finalSourceState'] = source_state(Path(facts['path']))
                facts['sourceUnchangedAtCleanup'] = facts['finalSourceState'] == facts['state']
                check(facts['sourceUnchangedAtCleanup'], 'followon_final_source_changed')
            except Exception as error:
                facts['finalSourceCheckFailure'] = failure(error)
                receipt.update(result='failed', failureClass='followon_final_source_check_failed')
        receipt['allInstrumentedCommandsSettled'] = all(c['result'] != 'in-flight'
            for row in receipt['cases'] for c in row['commands'])
        files = {Path(__file__)} | {Path(m.__file__).resolve() for m in list(sys.modules.values())
            if getattr(m, '__file__', None) and Path(m.__file__).resolve().parent == Path(__file__).resolve().parent}
        receipt['executedScriptSHA256'] = {str(p.relative_to(ROOT)): sha(p) for p in sorted(files)}
        target = RUN / 'receipt.json'
        raw = json.dumps(receipt, separators=(',', ':'), allow_nan=False) + '\n'
        check(0 < len(raw.encode()) <= 32 << 20, 'followon_lossless_receipt_bound')
        target.write_text(raw)
        (RUN / 'SHA256SUMS').write_text(''.join(sha(p) + '  ' + str(p.relative_to(ROOT)) + '\n'
            for p in sorted(files)) + sha(target) + '  receipt.json\n')
        print(json.dumps({'result': receipt['result'], 'revision': receipt['revision'], 'tree': receipt['tree'],
            'receiptSHA256': sha(target), 'cases': len(receipt['cases']), 'failureClass': receipt.get('failureClass'),
            'qualifiedConsumerCases': receipt.get('qualifiedConsumerCases', []), 'priorProducerGateRemainsEmpty': True,
            'productionAcceptance': False}))
    guard.__exit__()
raise SystemExit(0 if receipt['result'] == 'observed' else 1)
