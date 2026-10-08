#!/usr/bin/env python3
"""Measured CLI-clock AAC edits; raw failed model and video boundary remain preserved."""
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
from hls_remaining_nonkey_evidence import observed_media, native_pcm, packet_rows
from hls_remaining_nonkey_boundary import native_clock_rows, packet_tail
from hls_remaining_nonkey_aac_edit import change_generated_audio_edit, payload_orders
from hls_remaining_nonkey_deadline import DiagnosticDeadline
from hls_remaining_nonkey_aac_normalized import measure_cli_clock, derive_normalized_audio_edit
from hls_remaining_nonkey_aac_prefix import measure_prefix, qualify_prefix, derive_prefix_edit, qualify_negative_controls

ROOT = Path(__file__).resolve().parents[3]
RUN = ROOT / '.verification/hls-nonkey-aac-prefix' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
RUN.mkdir(parents=True)
receipt = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(),
    'tree': subprocess.check_output(['git', 'rev-parse', 'HEAD^{tree}'], text=True).strip(),
    'trackedSourceClean': not subprocess.check_output(
        ['git', 'status', '--porcelain', '--untracked-files=no'], text=True).strip(),
    'result': 'failed', 'cases': [], 'sourceMeasurements': [], 'productionAcceptance': False,
    'boundary': 'Bounded-prefix clocks compared with complete fixed controls; no production acceptance.'}
source_guards = []
guard = DiagnosticDeadline(240)
guard.__enter__()


def run(command, timeout=30):
    result = subprocess.run(command, capture_output=True, timeout=timeout)
    check(len(result.stdout) <= 4 << 20 and len(result.stderr) <= 4 << 20, 'aac_edit_output_bound')
    check(result.returncode == 0, 'aac_edit_command_failed')
    return result.stdout


def metadata_for(source, original):
    facts = stream_metadata(source)
    origin = float(facts['format']['start_time'])
    check(math.isfinite(origin) and abs(origin) <= .1, 'aac_edit_source_origin')
    frames = json.loads(run(['ffprobe', '-v', 'error', '-select_streams', 'v:0', '-show_frames',
        '-show_entries', 'frame=best_effort_timestamp_time', '-of', 'json', str(source)]))['frames']
    points = [float(row['best_effort_timestamp_time']) for row in frames]
    check(len(points) == 768 and all(math.isfinite(p) for p in points), 'aac_edit_source_frame_clock')
    return dict(original, sha256=sha(source), streamOrigins=facts,
                sourceTimeOriginSeconds=origin, sourceFramePTS=points), facts


def projection(row):
    value = row.get('observations', {})
    pcm = row.get('nativePCM') or value.get('nativePCM', {}).get('public', {})
    return {'container': row['container'], 'seekSeconds': row['seekSeconds'], 'label': row['label'], 'result': row['result'],
        'failureClass': row.get('failureClass'), 'fieldChange': row.get('fieldChange'),
        'prefixNegativeControls': row.get('prefixNegativeControls'),
        'nativeSamples': pcm.get('samples'), 'nativeSHA256': pcm.get('sha256'),
        'completeEOFAccounted': pcm.get('completeEOFAccounted'),
        'wholeReferenceEquals': row.get('wholeReferenceEquals'),
        'referenceSamples': row.get('referenceSamples'), 'referenceSHA256': row.get('referenceSHA256'),
        'hypothesisExpectedEquality': row.get('hypothesisExpectedEquality'),
        'hypothesisMatched': row.get('hypothesisMatched'),
        'allPacketPayloadsUnchanged': row.get('allPacketPayloadsUnchanged'),
        'globalPacketOrderUnchanged': row.get('globalPacketOrderUnchanged'),
        'videoRowsUnchanged': row.get('videoRowsUnchanged'), 'originalAssetsUnchanged': row.get('originalAssetsUnchanged'),
        'sourceUnchanged': row.get('sourceUnchanged'),
        'nativeFirstCopiedPacketClock': row.get('nativeFirstCopiedPacketClock'),
        'derivedClock': row.get('derivedClock'),
        'wholePublicCorrespondence': value.get('nativePCM', {}).get('wholePublicCorrespondence'),
        'rawFrames': len(row.get('completeVideoRows') or value.get('mapping', {}).get('actualSourceIndices', [])),
        'rawRequestedSequenceExact': row.get('rawRequestedSequenceExact', value.get('mapping', {}).get('exactRequestedSequence'))}


try:
    check(receipt['trackedSourceClean'], 'aac_edit_dirty_tracked_source')
    regular, original = fixture(RUN, 'regular', 48, ','.join(str(v) for v in range(0, 32, 2)), frames=768)
    mp4 = RUN / 'regular-copy.mp4'
    run(['ffmpeg', '-nostdin', '-v', 'error', '-i', str(regular), '-map', '0:v:0', '-map', '0:a:0',
         '-c', 'copy', str(mp4)], 45)
    for source in [regular, mp4]:
        before = source_state(source)
        source_guards.append((source, before))
        metadata, facts = metadata_for(source, original)
        measured = {'container': source.suffix[1:], 'result': 'in-flight'}
        receipt['sourceMeasurements'].append(measured)
        measure_cli_clock(source, measured)
        prefix = {}
        measured['boundedPrefix'] = prefix
        try:
            prefix_document = measure_prefix(source, prefix)
        except Exception as error:
            prefix.update(result='failed', failedStage=prefix.get('currentStage'),
                failureClass=str(error) if isinstance(error, RuntimeError) else type(error).__name__)
            raise
        print(json.dumps({'container': source.suffix[1:], 'prefixElapsed': prefix['elapsedSeconds'],
            'prefixFrames': len(prefix['nativeFrames']), 'normalizedFrames': len(prefix['normalizedFilter']['completeRows']),
            'sharedDeadlineSeconds': 2, 'productionAcceptance': False,
            'outputFilterClockEquivalent': prefix['outputFilterClockEquivalent'],
            'sourceStream': prefix['sourceStream'], 'leadingNativeFrames': prefix['nativeFrames'][:3],
            'leadingSourcePackets': prefix['sourcePackets'][:3],
            'leadingNormalizedFrames': prefix['normalizedFilter']['completeRows'][:3]}), flush=True)
        check(source_state(source) == before, 'aac_normalized_measurement_source_changed')
        measured['sourceUnchanged'] = True
        for offset in [12.5, 13.5, 18.2]:
            directory = RUN / (source.suffix[1:] + '-' + str(offset))
            base = {'container': source.suffix[1:], 'seekSeconds': offset,
                    'label': 'original-normal-both', 'result': 'in-flight'}
            receipt['cases'].append(base)
            options = {'output': ['-avoid_negative_ts', 'disabled'],
                       'segment': 'movflags=+skip_sidx:avoid_negative_ts=disabled'}
            base.update(mux_case(run, source, metadata, directory, base['label'], offset, options))
            check(base['result'] == 'observed', 'aac_edit_base_mux_unobserved')
            manifest = bounded_bytes(directory / 'index.m3u8', 65536, 'aac_edit_manifest_bound')
            _, names = manifest_facts(manifest)
            init = bounded_bytes(directory / 'init.mp4', 1 << 20, 'aac_edit_init_bound')
            fragments = [directory / name for name, _ in names]
            hashes = {p.name: sha(p) for p in [directory / 'init.mp4', *fragments]}
            base['observations'] = {}
            observed_media(source, directory / 'joined.mp4', init, fragments, metadata, offset, base['observations'])
            value = base['observations']
            source_frames = json.loads(run(['ffprobe', '-v', 'error', '-select_streams', 'a:0', '-show_frames',
                '-show_entries', 'frame=pts,nb_samples,side_data_list', '-of', 'json', str(source)]))['frames']
            audio = [s for s in facts['streams'] if s['codec_type'] == 'audio']
            check(len(audio) == 1 and {s['index']: s['codec_type'] for s in facts['streams']} ==
                  {0: 'video', 1: 'audio'}, 'aac_edit_source_stream_slots')
            base['completeNativeSourceFrameRows'] = source_frames
            base['sourceStreamRows'] = facts['streams']
            clock = native_clock_rows(source_frames, audio[0]['time_base'], round(offset * 48000),
                                      value['nativePCM']['wholePublicCorrespondence']['uniqueStartSample'])
            base['nativeSourceClock'] = clock
            check(clock['nativeSampleSum'] == value['nativePCM']['source']['samples'] and
                  value['nativePCM']['source']['completeEOFAccounted'], 'aac_derived_complete_source_accounting')
            tail = packet_tail(value['sourcePacketRows'], value['publicPacketRows'])
            base['aacPayloadTail'] = tail
            check(tail['wholePublicPacketTail'], 'aac_edit_original_payload_tail')
            first_pts = int(tail['firstSourceMatch']['pts'])
            matched = [r for r in clock['completeRows'] if r['pts'] == first_pts]
            check(len(matched) == 1, 'aac_edit_source_frame_packet_pts_identity')
            audio_track = [t for t in value['initialization']['tracks'] if t['handler'] == 'soun']
            check(len(audio_track) == 1, 'aac_edit_output_audio_identity')
            shift = audio_track[0]['edits'][0]['mediaTime']
            base['nativeFirstCopiedPacketClock'] = dict(matched[0], firstSourcePacket=tail['firstSourceMatch'],
                editMediaTime=shift, predictedNativeStartSample=matched[0]['nativeStartSample'] + shift,
                observedNativeStartSample=value['nativePCM']['wholePublicCorrespondence']['uniqueStartSample'],
                association='Unique exact source AAC packet/frame integer PTS, not an inferred timestamp')
            check(measured['sourcePCM']['sha256'] == value['nativePCM']['source']['sha256'] and
                  measured['sourcePCM']['samples'] == clock['nativeSampleSum'], 'aac_normalized_measurement_source_identity')
            qualify_prefix(prefix, measured, clock['completeRows'], source_frames, prefix_document, source)
            full_derived = derive_normalized_audio_edit(clock, measured, first_pts, offset, shift)
            derived = derive_prefix_edit(prefix, tail['firstSourceMatch'], offset, shift)
            check(all(derived[field] == full_derived[field] for field in
                ['desiredNativeStartSample', 'derivedMediaTime', 'deltaSamples']),
                'aac_prefix_full_derived_edit_equivalence')
            base['prefixNegativeControls'] = qualify_negative_controls(prefix, tail['firstSourceMatch'], offset, shift)
            base['fullControlDerivedClock'] = full_derived
            base['derivedClock'] = derived
            reference = value['nativePCM']['referenceSeek']
            base.update(referenceSamples=reference['samples'], referenceSHA256=reference['sha256'],
                wholeReferenceEquals=value['nativePCM']['wholePublicEqualsReference'],
                hypothesisExpectedEquality=derived['deltaSamples'] == 0)
            base['hypothesisMatched'] = base['wholeReferenceEquals'] == base['hypothesisExpectedEquality']
            joined = bounded_bytes(directory / 'joined.mp4', 32 << 20, 'aac_edit_joined_bound')
            check(joined.startswith(init), 'aac_edit_joined_initialization_identity')
            remaining = joined[len(init):]
            delta = derived['deltaSamples']
            adjacent = delta + 1 if delta < 32 else delta - 1
            deltas = [delta, adjacent]
            for delta in deltas:
                row = {'container': source.suffix[1:], 'seekSeconds': offset,
                       'label': 'generated-audio-edit-' + str(delta),
                       'result': 'in-flight', 'hypothesisExpectedEquality': delta == derived['deltaSamples'],
                       'derivedClock': derived}
                receipt['cases'].append(row)
                try:
                    changed, fields = change_generated_audio_edit(init, delta)
                    row['fieldChange'] = fields
                    target = directory / ('generated-audio-edit-' + str(delta) + '.mp4')
                    target.write_bytes(changed + remaining)
                    row['nativePCM'], _ = native_pcm(target)
                    row['completePacketRows'], row['packetMissingFields'] = packet_rows(target)
                    row['packetPayloadComparison'] = payload_orders(value['publicPacketRows'], row['completePacketRows'])
                    row['allPacketPayloadsUnchanged'] = row['packetPayloadComparison']['allPerStreamOrderCountHashes']
                    row['globalPacketOrderUnchanged'] = row['packetPayloadComparison']['globalInterleavedOrderEqual']
                    row['videoDecode'], row['completeVideoRows'] = decode_frames(target)
                    row['videoRowsUnchanged'] = row['completeVideoRows'] == value['publicFrameRows']
                    row['rawRequestedSequenceExact'] = value['mapping']['exactRequestedSequence']
                    row['originalAssetsUnchanged'] = hashes == {p.name: sha(p) for p in [directory / 'init.mp4', *fragments]}
                    row['sourceUnchanged'] = source_state(source) == before
                    row.update(referenceSamples=reference['samples'], referenceSHA256=reference['sha256'])
                    row['wholeReferenceEquals'] = (row['nativePCM']['samples'] == reference['samples'] and
                        row['nativePCM']['sha256'] == reference['sha256'])
                    row['hypothesisMatched'] = row['wholeReferenceEquals'] == row['hypothesisExpectedEquality']
                    check(row['allPacketPayloadsUnchanged'] and row['videoRowsUnchanged'] and
                          row['originalAssetsUnchanged'] and row['sourceUnchanged'] and row['nativePCM']['completeEOFAccounted'],
                          'aac_edit_counterfactual_extent_or_eof')
                    row['result'] = 'observed'
                except Exception as error:
                    if isinstance(error, RuntimeError) and str(error) == 'bounded_diagnostic_deadline':
                        raise
                    row.update(result='failed', failureClass=str(error) if isinstance(error, RuntimeError) else type(error).__name__)
                print(json.dumps(projection(row), separators=(',', ':')), flush=True)
            base['originalAssetsUnchanged'] = hashes == {p.name: sha(p) for p in [directory / 'init.mp4', *fragments]}
            base['sourceUnchanged'] = source_state(source) == before
            check(base['sourceUnchanged'] and base['originalAssetsUnchanged'], 'aac_edit_original_source_changed')
            print(json.dumps(projection(base), separators=(',', ':')), flush=True)
    check(len(receipt['cases']) == 18, 'aac_prefix_fixed_case_count')
    receipt['result'] = 'observed'
except Exception as error:
    receipt['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
finally:
    with guard.cleanup():
        receipt['sourceFinalChecks'] = []
        for source, before in source_guards:
            row = {'container': source.suffix[1:], 'sourceUnchanged': False}
            try:
                row['sourceUnchanged'] = source_state(source) == before
            except OSError as error:
                row['failureClass'] = type(error).__name__
            receipt['sourceFinalChecks'].append(row)
        if not all(row['sourceUnchanged'] for row in receipt['sourceFinalChecks']):
            receipt.update(result='failed', sourceFinalCheckFailed=True,
                failureClass=receipt.get('failureClass') or 'aac_prefix_source_changed')
        receipt['handledTerminationSignals'] = guard.signals
        files = {Path(__file__)} | {Path(m.__file__).resolve() for m in list(sys.modules.values())
            if getattr(m, '__file__', None) and Path(m.__file__).resolve().parent == Path(__file__).resolve().parent}
        receipt['executedScriptSHA256'] = {str(p.relative_to(ROOT)): sha(p) for p in sorted(files)}
        raw = json.dumps(receipt, separators=(',', ':'), allow_nan=False) + '\n'
        check(0 < len(raw.encode()) <= 32 << 20, 'aac_edit_lossless_receipt_bound')
        target = RUN / 'receipt.json'
        target.write_text(raw)
        (RUN / 'SHA256SUMS').write_text(''.join(sha(p) + '  ' + str(p.relative_to(ROOT)) + '\n'
            for p in sorted(files)) + sha(target) + '  receipt.json\n')
        print(json.dumps({'boundedPrefixComparisons': [{
            'container': m['container'], 'sourceStream': m.get('boundedPrefix', {}).get('sourceStream'),
            'outputFilterClockEquivalent': m.get('boundedPrefix', {}).get('outputFilterClockEquivalent'),
            'leadingNativeFrames': m.get('boundedPrefix', {}).get('nativeFrames', [])[:3],
            'leadingSourcePackets': m.get('boundedPrefix', {}).get('sourcePackets', [])[:3],
            'leadingNormalizedFrames': m.get('boundedPrefix', {}).get('normalizedFilter', {}).get('completeRows', [])[:3],
            'leadingOutputRows': m.get('boundedPrefix', {}).get('normalizedOutputRows', [])[:3]}
            for m in receipt['sourceMeasurements']], 'productionAcceptance': False}))
        print(json.dumps({'result': receipt['result'], 'revision': receipt['revision'],
            'receiptSHA256': sha(target), 'cases': len(receipt['cases']),
            'failureClass': receipt.get('failureClass'),
            'boundedPrefixStages': [{k: m.get('boundedPrefix', {}).get(k) for k in
                ['currentStage', 'completedStages', 'elapsedSeconds', 'failedStage', 'probeSHA256', 'processFacts']}
                for m in receipt['sourceMeasurements']], 'productionAcceptance': False}))
    guard.__exit__()
raise SystemExit(0 if receipt['result'] == 'observed' else 1)
