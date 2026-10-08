#!/usr/bin/env python3
"""Observe CLI audio normalization before automatic output trimming; select no edits."""
from fractions import Fraction
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import time
from hls_timeline_fixture import fixture
from hls_timeline_http import sha, source_state
from hls_followon_frames import stream_metadata
from hls_followon_public import check
from hls_remaining_nonkey_evidence import native_pcm, pcm_tail_correspondence
from hls_remaining_nonkey_boundary import native_clock_rows
from hls_remaining_nonkey_aac_clock import frame_md5, filter_clock
from hls_remaining_nonkey_deadline import DiagnosticDeadline

ROOT = Path(__file__).resolve().parents[3]
RUN = ROOT / '.verification/hls-nonkey-aac-clock' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
RUN.mkdir(parents=True)
receipt = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(),
    'tree': subprocess.check_output(['git', 'rev-parse', 'HEAD^{tree}'], text=True).strip(),
    'trackedSourceClean': not subprocess.check_output(
        ['git', 'status', '--porcelain', '--untracked-files=no'], text=True).strip(),
    'result': 'failed', 'cases': [], 'productionAcceptance': False,
    'boundary': 'CLI user-filter clocks before automatic output trim, and separate PCM output packet clocks.'}
guard = DiagnosticDeadline(240)
guard.__enter__()


def command(arguments, timeout=45):
    process = subprocess.run(arguments, capture_output=True, timeout=timeout)
    check(len(process.stdout) <= 2 << 20 and len(process.stderr) <= 2 << 20, 'aac_clock_command_extent')
    check(process.returncode == 0, 'aac_clock_command_failed')
    return process.stdout, process.stderr


def observe(source, offset, pcm):
    args = ['ffmpeg', '-nostdin', '-v', 'info', '-xerror', '-threads', '2', '-i', str(source)]
    if offset is not None:
        args += ['-ss', str(offset)]
    args += ['-map', '0:a:0', '-vn', '-sn', '-dn', '-frames:a', '4097',
             '-af', 'ashowinfo', '-c:a', 'pcm_s16le', '-f', 'framemd5', 'pipe:1']
    stdout, stderr = command(args)
    output = frame_md5(stdout.decode(), pcm)
    filtered = filter_clock(stderr.decode())
    return {'outputPCMClock': output, 'preTrimUserFilterClock': filtered,
        'frameMD5OutputSHA256': hashlib.sha256(stdout).hexdigest(),
        'generatedMediaLogSHA256': hashlib.sha256(stderr).hexdigest(),
        'commandOptions': ['native 48k stereo PCM', 'ashowinfo before automatic output trim',
            'no requested rate/channel conversion', 'no copyts', 'no timestamp edit']}


def projection(row):
    filtered = row.get('preTrimUserFilterClock', {})
    output = row.get('outputPCMClock', {})
    return {'container': row['container'], 'seekSeconds': row['seekSeconds'], 'result': row['result'],
        'failureClass': row.get('failureClass'), 'sourceUnchanged': row.get('sourceUnchanged'),
        'sourceCompleteEOFAccounted': row.get('sourceCompleteEOFAccounted'),
        'sameFullPreTrimSequence': row.get('sameFullPreTrimSequence'),
        'normalizedFilterNativeFrameAssociation': row.get('normalizedFilterNativeFrameAssociation'),
        'filterPTSPrintedTimeQualification': row.get('filterPTSPrintedTimeQualification'),
        'filterResidualSamples': row.get('filterResidualSamples'),
        'preTrimFrames': len(filtered.get('completeRows', [])),
        'preTrimFirstRows': filtered.get('completeRows', [])[:6],
        'preTrimTargetRows': row.get('preTrimTargetRows'),
        'rawNativeFirstRows': row.get('rawNativeSourceClock', {}).get('completeRows', [])[:6],
        'outputFrames': len(output.get('completeRows', [])), 'outputSamples': output.get('samples'),
        'outputPCM_SHA256': output.get('pcmSHA256'), 'allOutputPacketPCMBytesBound': output.get('allOutputPacketPCMBytesBound'),
        'outputResidualSamples': output.get('residualSamples'), 'outputFirstRows': output.get('completeRows', [])[:6],
        'sourceTailCorrespondence': row.get('sourceTailCorrespondence'), 'productionAcceptance': False}


try:
    check(receipt['trackedSourceClean'], 'aac_clock_dirty_tracked_source')
    regular, _ = fixture(RUN, 'regular', 48, ','.join(str(v) for v in range(0, 32, 2)), frames=768)
    mp4 = RUN / 'regular-copy.mp4'
    command(['ffmpeg', '-nostdin', '-v', 'error', '-i', str(regular), '-map', '0:v:0', '-map', '0:a:0',
             '-c', 'copy', str(mp4)])
    for source in [regular, mp4]:
        before = source_state(source)
        metadata = stream_metadata(source)
        audio = [s for s in metadata['streams'] if s['codec_type'] == 'audio']
        check(len(audio) == 1 and audio[0]['sample_rate'] == '48000' and audio[0]['channels'] == 2,
              'aac_clock_native_source_format')
        raw, _ = command(['ffprobe', '-v', 'error', '-select_streams', 'a:0', '-show_frames',
            '-show_entries', 'frame=pts,nb_samples,side_data_list', '-of', 'json', str(source)])
        raw_frames = json.loads(raw)['frames']
        clock = native_clock_rows(raw_frames, audio[0]['time_base'], 0, None)
        original, whole_pcm = native_pcm(source)
        check(original['completeEOFAccounted'] and clock['nativeSampleSum'] == original['samples'],
              'aac_clock_source_complete_extent')
        baseline = None
        for offset in [None, 12.5, 13.5, 18.2]:
            row = {'container': source.suffix[1:], 'seekSeconds': offset, 'result': 'in-flight',
                'sourceCompleteEOFAccounted': True, 'rawNativeSourceFrameRows': raw_frames,
                'rawNativeSourceClock': clock, 'sourceStreamRows': metadata['streams'],
                'sourcePCM': original}
            receipt['cases'].append(row)
            try:
                facts, pcm = (original, whole_pcm) if offset is None else native_pcm(source, offset)
                row['nativePCM'] = facts
                row.update(observe(source, offset, pcm))
                filtered = row['preTrimUserFilterClock']['completeRows']
                row['sameFullPreTrimSequence'] = baseline is None or filtered == baseline
                if offset is None:
                    baseline = filtered
                row['normalizedFilterNativeFrameAssociation'] = (
                    len(filtered) == len(raw_frames) and
                    [r['samples'] for r in filtered] == [r['nb_samples'] for r in raw_frames])
                row['filterPTSPrintedTimeQualification'] = all(
                    abs(Fraction(r['ptsTime']) - Fraction(r['pts'], 48000)) <= Fraction(1, 100000)
                    for r in filtered)
                ordinal, residuals, targets = 0, set(), []
                for value in filtered:
                    residuals.add(value['pts'] - ordinal)
                    if offset is not None and value['pts'] <= round(offset * 48000) < value['pts'] + value['samples']:
                        targets.append(dict(value, nativeStartSample=ordinal))
                    ordinal += value['samples']
                row['filterResidualSamples'] = sorted(residuals)
                row['preTrimTargetRows'] = targets
                row['sourceTailCorrespondence'] = pcm_tail_correspondence(
                    whole_pcm, pcm, 0 if offset is None else round(offset * 48000))
                row['sourceUnchanged'] = source_state(source) == before
                check(row['sourceUnchanged'] and row['sameFullPreTrimSequence'] and
                      row['normalizedFilterNativeFrameAssociation'] and row['filterPTSPrintedTimeQualification'] and
                      ordinal == original['samples'] and row['sourceTailCorrespondence']['uniqueSourceTailComplete'],
                      'aac_clock_complete_source_binding')
                row['result'] = 'observed'
            except Exception as error:
                if isinstance(error, RuntimeError) and str(error) == 'bounded_diagnostic_deadline':
                    raise
                row.update(result='failed', failureClass=str(error) if isinstance(error, RuntimeError) else type(error).__name__)
            print(json.dumps(projection(row), separators=(',', ':')), flush=True)
    check(len(receipt['cases']) == 8, 'aac_clock_fixed_case_count')
    receipt['result'] = 'observed'
except Exception as error:
    receipt['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
finally:
    with guard.cleanup():
        receipt['handledTerminationSignals'] = guard.signals
        files = {Path(__file__)} | {Path(m.__file__).resolve() for m in list(sys.modules.values())
            if getattr(m, '__file__', None) and Path(m.__file__).resolve().parent == Path(__file__).resolve().parent}
        receipt['executedScriptSHA256'] = {str(p.relative_to(ROOT)): sha(p) for p in sorted(files)}
        raw = json.dumps(receipt, separators=(',', ':'), allow_nan=False) + '\n'
        check(0 < len(raw.encode()) <= 32 << 20, 'aac_clock_lossless_receipt_bound')
        target = RUN / 'receipt.json'
        target.write_text(raw)
        (RUN / 'SHA256SUMS').write_text(''.join(sha(p) + '  ' + str(p.relative_to(ROOT)) + '\n'
            for p in sorted(files)) + sha(target) + '  receipt.json\n')
        print(json.dumps({'result': receipt['result'], 'revision': receipt['revision'],
            'receiptSHA256': sha(target), 'cases': len(receipt['cases']),
            'failureClass': receipt.get('failureClass'), 'productionAcceptance': False}))
    guard.__exit__()
raise SystemExit(0 if receipt['result'] == 'observed' else 1)
