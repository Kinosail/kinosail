#!/usr/bin/env python3
"""Fixed edit/demux/native boundary diagnosis; no production acceptance."""
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
from hls_remaining_nonkey_evidence import observed_media
from hls_remaining_nonkey_deadline import DiagnosticDeadline
from hls_remaining_nonkey_boundary import edit_binding, frame_clock_diagnosis, native_clock_rows, packet_tail, stream_identity

ROOT = Path(__file__).resolve().parents[3]
RUN = ROOT / '.verification/hls-nonkey-boundary' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
RUN.mkdir(parents=True)
deadline = time.monotonic() + 240
receipt = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(),
    'tree': subprocess.check_output(['git', 'rev-parse', 'HEAD^{tree}'], text=True).strip(),
    'trackedSourceClean': not subprocess.check_output(
        ['git', 'status', '--porcelain', '--untracked-files=no'], text=True).strip(),
    'result': 'failed', 'cases': [], 'productionAcceptance': False,
    'boundary': 'Source-bound raw/edit/demux/native clocks; every raw frame and sample retained; no discard map applied.',
    'sourceCitations': ['https://github.com/FFmpeg/FFmpeg/blob/n8.1.2/libavformat/hlsenc.c#L800',
        'https://github.com/FFmpeg/FFmpeg/blob/n8.1.2/libavformat/movenc.c#L6611']}

guard = DiagnosticDeadline(240)
guard.__enter__()


def run(command, timeout=30):
    budget = min(timeout, deadline - time.monotonic())
    check(budget > 0, 'bounded_diagnostic_deadline')
    result = subprocess.run(command, capture_output=True, timeout=budget)
    check(len(result.stdout) <= 4 << 20, 'mux_diagnostic_output_bound')
    if result.returncode:
        raise RuntimeError('mux_diagnostic_command_failed')
    return result.stdout


def project(row):
    value = row.get('observations', {})
    detail = row.get('boundaryDiagnosis', {})
    mapping = value.get('mapping', {})
    pcm = value.get('nativePCM', {})
    slim = lambda values: [{k: r[k] for k in ['trackID', 'handler', 'timeBase', 'edits',
        'rawDecodeDiscontinuities', 'allDemuxClocksMatchEdit', 'negativeEditedPTSRows',
        'discardFlagRows']} | {'firstRows': r['completeRows'][:3]} for r in values]
    return {'container': row['container'], 'label': row['label'], 'result': row['result'],
        'failureClass': row.get('failureClass'), 'observationFailure': row.get('observationFailure'),
        'rawFrames': len(mapping.get('actualSourceIndices', [])),
        'exactRequestedSequence': mapping.get('exactRequestedSequence'),
        'completedBoundaryStages': detail.get('completedStages'), 'currentStage': detail.get('currentStage'),
        'defaultEditBinding': slim(detail.get('defaultEditBinding', [])),
        'ignoreEditBinding': slim(detail.get('ignoreEditBinding', [])),
        'frameClocks': {k: v for k, v in detail.get('frameClocks', {}).items()
            if k not in ['allActualIndices', 'allActualRows', 'nonnegativeClockIndices']},
        'nonnegativeClockCount': len(detail.get('frameClocks', {}).get('nonnegativeClockIndices', [])),
        'nativeTargetRows': detail.get('nativeSourceClock', {}).get('targetRows'),
        'wholePublicCorrespondence': pcm.get('wholePublicCorrespondence'),
        'wholePublicEqualsReference': pcm.get('wholePublicEqualsReference'),
        'aacPayloadTail': {k: v for k, v in detail.get('aacPayloadTail', {}).items()
            if k not in ['completeSourceRows', 'completePublicRows']}}


try:
    check(receipt['trackedSourceClean'], 'mux_dirty_tracked_source')
    regular, metadata = fixture(RUN, 'regular', 48, ','.join(str(v) for v in range(0, 32, 2)), frames=768)
    mp4 = RUN / 'regular-copy.mp4'
    run(['ffmpeg', '-nostdin', '-v', 'error', '-i', str(regular), '-map', '0:v:0', '-map', '0:a:0',
        '-c', 'copy', str(mp4)], 45)
    outer = ['-avoid_negative_ts', 'disabled']
    legacy = 'movflags=+frag_discont+skip_sidx'
    normal = 'movflags=+skip_sidx'
    candidates = [
        ('legacy', {}, 12.5),
        ('normal-both', {'output': outer, 'segment': normal + ':avoid_negative_ts=disabled'}, 12.5)]
    for source in [regular, mp4]:
        before = source_state(source)
        facts = stream_metadata(source)
        origin = float(facts['format']['start_time'])
        check(math.isfinite(origin) and abs(origin) <= 0.1, 'mux_source_origin')
        source_frames = json.loads(run(['ffprobe', '-v', 'error', '-select_streams', 'v:0',
            '-show_frames', '-show_entries', 'frame=best_effort_timestamp_time',
            '-of', 'json', str(source)]))['frames']
        points = [float(row['best_effort_timestamp_time']) for row in source_frames]
        check(len(points) == 768 and all(math.isfinite(p) for p in points), 'mux_source_frame_clock')
        source_metadata = dict(metadata, sha256=sha(source), streamOrigins=facts,
            sourceTimeOriginSeconds=origin, sourceFramePTS=points)
        for label, options, offset in candidates:
            directory = RUN / (source.suffix[1:] + '-' + label)
            row = {'container': source.suffix[1:], 'label': label, 'options': options,
                   'inputSeekSeconds': offset, 'result': 'in-flight'}
            receipt['cases'].append(row)
            row.update(mux_case(run, source, source_metadata, directory, label, offset, options))
            if row['result'] == 'observed':
                try:
                    manifest = bounded_bytes(directory / 'index.m3u8', 65536, 'mux_manifest_bound')
                    _, names = manifest_facts(manifest)
                    row['observations'] = {}
                    observed_media(source, directory / 'joined.mp4',
                        bounded_bytes(directory / 'init.mp4', 1 << 20, 'mux_init_bound'),
                        [directory / name for name, _ in names], source_metadata, offset, row['observations'])
                    value = row['observations']
                    detail = {'completedStages': [], 'presentationQualification': False}
                    row['boundaryDiagnosis'] = detail
                    streams = json.loads(run(['ffprobe', '-v', 'error', '-show_streams',
                        '-show_entries', 'stream=index,id,codec_type,time_base', '-of', 'json',
                        str(directory / 'joined.mp4')]))['streams']
                    detail['publicStreamRows'] = streams
                    stream_identity(streams)
                    detail['currentStage'] = 'raw-edit-default-demux'
                    detail['defaultEditBinding'] = edit_binding(value['initialization'],
                        value['physicalFragments'], streams, value['publicPacketRows'])
                    detail['completedStages'].append(detail['currentStage'])
                    detail['currentStage'] = 'raw-ignore-edit-demux'
                    raw_packets = json.loads(run(['ffprobe', '-v', 'error', '-ignore_editlist', '1',
                        '-read_intervals', '%+#4097', '-show_packets', '-show_data_hash', 'sha256',
                        '-show_entries', 'packet=stream_index,pts,dts,duration,flags,data_hash,side_data_list',
                        '-of', 'json', str(directory / 'joined.mp4')]))['packets']
                    detail['completeIgnoreEditPacketRows'] = raw_packets
                    check(0 < len(raw_packets) <= 4096, 'boundary_complete_ignore_edit_packet_bound')
                    raw_init = dict(value['initialization'], tracks=[
                        dict(t, edits=[]) for t in value['initialization']['tracks']])
                    detail['ignoreEditBinding'] = edit_binding(raw_init,
                        value['physicalFragments'], streams, raw_packets)
                    detail['completedStages'].append(detail['currentStage'])
                    detail['currentStage'] = 'decoded-frame-clocks'
                    detail['frameClocks'] = frame_clock_diagnosis(value['mapping'])
                    detail['completedStages'].append(detail['currentStage'])
                    detail['currentStage'] = 'native-source-clock'
                    native_frames = json.loads(run(['ffprobe', '-v', 'error', '-select_streams', 'a:0',
                        '-show_frames', '-show_entries', 'frame=pts,nb_samples,side_data_list',
                        '-of', 'json', str(source)]))['frames']
                    detail['completeNativeSourceFrameRows'] = native_frames
                    detail['sourceStreamRows'] = facts['streams']
                    check({t['index']: t['codec_type'] for t in facts['streams']} ==
                          {0: 'video', 1: 'audio'}, 'boundary_source_stream_slots')
                    audio = [t for t in facts['streams'] if t['codec_type'] == 'audio']
                    check(len(audio) == 1, 'boundary_source_audio_identity')
                    corr = value['nativePCM']['wholePublicCorrespondence']
                    detail['nativeSourceClock'] = native_clock_rows(native_frames, audio[0]['time_base'],
                        600000, corr['uniqueStartSample'])
                    check(detail['nativeSourceClock']['nativeSampleSum'] ==
                        value['nativePCM']['source']['samples'], 'boundary_native_complete_sample_accounting')
                    detail['completedStages'].append(detail['currentStage'])
                    detail['currentStage'] = 'copied-aac-payload-tail'
                    detail['aacPayloadTail'] = packet_tail(value['sourcePacketRows'], value['publicPacketRows'])
                    detail['completedStages'].append(detail['currentStage'])
                    detail['currentStage'] = 'complete'
                except Exception as error:
                    row['observationFailure'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
            print(json.dumps(project(row), separators=(',', ':')), flush=True)
        check(source_state(source) == before, 'mux_source_changed')
    check(len(receipt['cases']) == 4, 'boundary_exact_counterfactual_count')
    receipt['result'] = 'observed'
except Exception as error:
    receipt['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
finally:
    with guard.cleanup():
        receipt['handledTerminationSignals'] = guard.signals
        target = RUN / 'receipt.json'
        files = {Path(__file__)} | {Path(m.__file__).resolve() for m in list(sys.modules.values())
            if getattr(m, '__file__', None) and Path(m.__file__).resolve().parent == Path(__file__).resolve().parent}
        receipt['executedScriptSHA256'] = {str(p.relative_to(ROOT)): sha(p) for p in sorted(files)}
        raw = json.dumps(receipt, separators=(',', ':'), allow_nan=False) + '\n'
        check(0 < len(raw.encode()) <= 32 << 20, 'mux_lossless_receipt_bound')
        target.write_text(raw)
        (RUN / 'SHA256SUMS').write_text(''.join(sha(p) + '  ' + str(p.relative_to(ROOT)) + '\n'
            for p in sorted(files)) + sha(target) + '  receipt.json\n')
        print(json.dumps({'result': receipt['result'], 'revision': receipt['revision'],
            'receiptSHA256': sha(target), 'cases': len(receipt['cases']),
            'failureClass': receipt.get('failureClass'), 'productionAcceptance': False}))
    guard.__exit__()
raise SystemExit(0 if receipt['result'] == 'observed' else 1)
