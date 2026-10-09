#!/usr/bin/env python3
"""Fixed one-variable mux counterfactuals; no public or production acceptance."""
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

ROOT = Path(__file__).resolve().parents[3]
RUN = ROOT / '.verification/hls-nonkey-mux' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
RUN.mkdir(parents=True)
deadline = time.monotonic() + 360
receipt = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(),
    'tree': subprocess.check_output(['git', 'rev-parse', 'HEAD^{tree}'], text=True).strip(),
    'trackedSourceClean': not subprocess.check_output(
        ['git', 'status', '--porcelain', '--untracked-files=no'], text=True).strip(),
    'result': 'failed', 'cases': [], 'productionAcceptance': False,
    'boundary': 'Fixed installed-FFmpeg counterfactuals; all negative frames retained; no discard map applied.',
    'sourceCitations': ['https://github.com/FFmpeg/FFmpeg/blob/n8.1.2/libavformat/hlsenc.c#L800',
        'https://github.com/FFmpeg/FFmpeg/blob/n8.1.2/libavformat/movenc.c#L6611']}

guard = DiagnosticDeadline(360)
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
    observation = row.get('observations', {})
    mapping = observation.get('mapping', {})
    pcm = observation.get('nativePCM', {})
    return {'container': row['container'], 'label': row['label'], 'result': row['result'],
        'options': row['options'], 'failureClass': row.get('failureClass'),
        'observationFailure': row.get('observationFailure'),
        'rawFrames': row.get('presentation', {}).get('rawFrames'),
        'negativeFrames': row.get('presentation', {}).get('negativeTimestampFrames'),
        'firstPTS': row.get('presentation', {}).get('firstPresentedPTS'),
        'exactRequestedSequence': mapping.get('exactRequestedSequence'),
        'precedingFrames': mapping.get('precedingSourceFrames'),
        'firstIndices': mapping.get('actualSourceIndices', [])[:16],
        'initialization': observation.get('initialization'),
        'nativeSamples': {key: value.get('samples') for key, value in pcm.items()
                         if key in ['source', 'public', 'referenceSeek']},
        'wholePublicCorrespondence': pcm.get('wholePublicCorrespondence'),
        'wholePublicEqualsReference': pcm.get('wholePublicEqualsReference'),
        'publicMinusReferenceSamples': pcm.get('publicMinusReferenceSamples')}


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
        ('outer-disabled', {'output': outer}, 12.5),
        ('inner-disabled', {'segment': legacy + ':avoid_negative_ts=disabled'}, 12.5),
        ('both-disabled', {'output': outer, 'segment': legacy + ':avoid_negative_ts=disabled'}, 12.5),
        ('normal-inner', {'segment': normal + ':avoid_negative_ts=disabled'}, 12.5),
        ('normal-both', {'output': outer, 'segment': normal + ':avoid_negative_ts=disabled'}, 12.5),
        ('normal-both-edits', {'output': outer,
            'segment': normal + ':avoid_negative_ts=disabled:use_editlist=1'}, 12.5),
        ('normal-both-signed-cts', {'output': outer, 'segment':
            'movflags=+skip_sidx+negative_cts_offsets:avoid_negative_ts=disabled:use_editlist=1'}, 12.5),
        ('key-control', {}, 12),
        ('zero-control', {}, 0)]
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
                except Exception as error:
                    row['observationFailure'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
            print(json.dumps(project(row), separators=(',', ':')), flush=True)
        check(source_state(source) == before, 'mux_source_changed')
    check(len(receipt['cases']) == 20, 'mux_exact_counterfactual_count')
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
