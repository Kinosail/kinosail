#!/usr/bin/env python3
"""Actual indexed MP4 API/cache proof; historical and native holds stay open."""
import json
from pathlib import Path
import re
import shutil
import subprocess
import sys
import time
from hls_aac_v2_public_http import ActualServer, actual_producer_rows, cached_media, diagnostic_producer_rows
from hls_aac_v2_public_evidence import assert_fixed_cache_grid, assert_fixed_source_grid, assert_fixed_timeline, cache_state, qualify
from hls_followon_frames import decode_frames
from hls_followon_public import bounded_bytes, check, prepare_once
from hls_nonkey_browser_public import public_media
from hls_remaining_nonkey_deadline import DiagnosticDeadline
from hls_timeline_fixture import fixture
from hls_timeline_http import sha, source_state
from hls_timeline_packets import manifest_facts, safe_encoder_lifecycle, safe_seek_phases

ROOT = Path(__file__).resolve().parents[3]
RUN = ROOT / '.verification/hls-aac-v2-public' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
RUN.mkdir(parents=True)
receipt = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(),
    'tree': subprocess.check_output(['git', 'rev-parse', 'HEAD^{tree}'], text=True).strip(),
    'result': 'failed', 'cases': [], 'sessions': [], 'productionAcceptance': False,
    'nativeAcceptance': False, 'browserAcceptance': False, 'historicalFailuresPreserved': True,
    'heldControls': ['P0 unindexed interrupted-cold/reopen', 'P2 missing-binding public master/rendition readiness', 'native tvOS AVPlayer'],
    'scope': 'Pinned fixed32s H264/AAC MP4; actual indexed0.1s prepare/refill/reopen/live-zero/cold-zero/V1 migration'}
owner, source, before = None, None, None
guard = DiagnosticDeadline(600)
guard.__enter__()


def run(command, timeout=60):
    guard.check()
    result = subprocess.run(command, capture_output=True, timeout=min(timeout, guard.check(15)))
    check(len(result.stdout) <= 2 << 20 and len(result.stderr) <= 2 << 20, 'v2_command_output_bound')
    check(result.returncode == 0, 'v2_command_failed')
    return result.stdout


def case(label, selected, metadata, original):
    row = {'label': label, 'result': 'failed'}
    receipt['cases'].append(row)
    joined, manifest, assets, delivery = public_media(owner.api, selected, RUN / label,
        owner.log_path, owner.process, source)
    facts, fragments = manifest_facts(manifest)
    row.update(manifestFacts=facts, segmentNames=[name for name, _ in fragments], delivery=delivery)
    assert_fixed_timeline(facts, fragments, metadata['sourceVideoGrid'])
    row['delivery'] = delivery
    row['manifestSHA256'] = __import__('hashlib').sha256(manifest).hexdigest()
    qualify(source, joined, assets, metadata, row)
    _, _, after = cache_state(owner.directory / 'cache')
    check(after['metadataSHA256'] == original['metadataSHA256']
          and after['generationInode'] == original['generationInode']
          and all(after['assetSHA256'].get(name) == digest for name, digest in original['assetSHA256'].items()),
          'v2_canonical_prefix_changed')
    row.update(result='observed', canonicalPrefixUnchanged=True, cacheAfter=after,
        actualProducerRows=actual_producer_rows(owner.source_invocation_rows()[getattr(owner, 'producer_start', 0):]))


try:
    check(shutil.disk_usage(RUN).free >= 2 << 30, 'v2_hosted_disk_budget')
    receipt['stage'] = 'pinned-source'
    receipt['codecVersion'] = run(['ffmpeg', '-version'], 10).decode().splitlines()[0]
    regular, facts = fixture(RUN, 'regular', 48, ','.join(str(v) for v in range(0, 32, 2)), frames=768)
    directory = RUN / 'actual'
    media = directory / 'media'
    media.mkdir(parents=True)
    source = media / 'Fixture.mp4'
    run(['ffmpeg', '-nostdin', '-v', 'error', '-i', str(regular),
        '-map', '0:v:0', '-map', '0:a:0', '-c', 'copy', str(source)], 45)
    before = source_state(source)
    check(before['sha256'] == '198a6808092cea2a5481afc24e79481e4966ae577cbb8c46c214737e2173a3ed',
          'v2_fixed_source_identity')
    _, frames = decode_frames(source)
    check(len(frames) == 768, 'v2_source_frame_count')
    source_grid = assert_fixed_source_grid(json.loads(run(['ffprobe', '-v', 'error', '-select_streams', 'v:0',
        '-show_packets', '-show_streams', '-show_entries', 'packet=pts,dts,duration,flags:stream=time_base',
        '-of', 'json', str(source)], 30)))
    receipt['sourceVideoGrid'] = source_grid
    metadata = dict(facts, sourceVideoGrid=source_grid, sourceFramePTS=[row[0] for row in frames], sourceTimeOriginSeconds=0)
    receipt['source'] = before
    receipt['stage'] = 'exact-source-build'
    binary = RUN / 'kinosail'
    run(['go', '-C', str(ROOT / 'apps/player'), 'build', '-p=1', '-o', str(binary), './cmd/kinosail'], 180)
    owner = ActualServer(ROOT, directory, source, binary)
    owner.start(authorize=True)
    item = next(value for value in owner.api.call('/api/v1/library')['items'] if value['title'] == 'Fixture')
    prepare = '/api/v1/items/' + item['id'] + '/playback-prepare'
    plan = owner.api.call('/api/v1/items/' + item['id'] + '/playback?videoCodecs=h264&audioCodecs=aac')
    check(plan['compatiblePlan']['mode'] == 'remux', 'v2_actual_remux_plan')
    selected = plan['compatible'].replace('/index.m3u8', '-o12000/index.m3u8')
    check(re.fullmatch(r'/hls/[a-f0-9]{16}/p/r-[a-zA-Z0-9-]+/index\.m3u8', selected), 'v2_recipe_allowlist')
    check(owner.api.http(prepare, 'POST', {'source': selected}, authenticated=False)[0] == 401,
          'v2_unauthorized_preparation')
    receipt['stage'] = 'actual-preparation'
    prepared = {}
    prepare_once(owner.api, prepare, selected, owner.log_path, owner.process, source, prepared)
    check(prepared['preparationAttempt']['completionState'] == 'ready', 'v2_actual_prepare_not_ready')
    _, _, prefix = cache_state(directory / 'cache')
    check(prefix['segments'] == ['segment-' + format(n, '05d') + '.m4s' for n in range(4)],
          'v2_real_lazy_prefix_required')
    assert_fixed_cache_grid(prefix, source_grid)
    origin = prefix['audioOrigin']
    check(origin['InitialSeekMicros'] == 12000000 and origin['FirstPTS'] == 571400
          and origin['Physical'] == -571400 and origin['Edit'] == 4600 and origin['SourceTrack'] == 1,
          'v2_actual_packet_origin')
    all_rows = owner.invocation_rows()
    private = bounded_bytes(owner.log_path, 2 << 20, 'v2_private_lifecycle_bound').decode()
    receipt.update(preparation=prepared, initialCache=prefix,
        producerCapture={'records': len(all_rows), 'fileExists': owner.invocations.exists(),
            'rows': diagnostic_producer_rows(all_rows, source, owner.owned_pids)},
        encoderLifecycle=safe_encoder_lifecycle(private), encoderSeekPhases=safe_seek_phases(private))
    rows = owner.source_invocation_rows()
    receipt['producerCapture']['sourceRecords'] = len(rows)
    check(len(rows) == 1, 'v2_prepare_encoder_count')
    receipt['stage'] = 'causal-public-refill4'
    case('causal-refill4', selected, metadata, prefix)
    producer = actual_producer_rows(owner.source_invocation_rows())
    check(len(producer) == 2 and producer[0]['seek'] == '12.000000'
          and producer[0]['startNumber'] == '0' and producer[1]['seek'] == '20.000000'
          and producer[1]['startNumber'] == '4' and producer[1]['muxOffset'] == '8.000000'
          and producer[1]['audioBSF'] == 'noise=amount=0:drop=lt(pts+960000\\,956001),setts=pts=PTS+4600:dts=DTS+4600',
          'v2_actual_source_derived_refill_argv')
    _, media_cache, complete = cache_state(directory / 'cache')
    owner.stop()
    receipt['stage'] = 'cold-server-reopen'
    owner.start()
    row = {'label': 'cold-reopen', 'result': 'failed'}
    receipt['cases'].append(row)
    joined, manifest, assets, idle = cached_media(owner, selected, RUN / 'cold-reopen', source_grid)
    qualify(source, joined, assets, metadata, row)
    check(len(owner.source_invocation_rows()) == 2, 'v2_reopen_started_encoder')
    _, _, reopened = cache_state(directory / 'cache')
    check(reopened == complete, 'v2_reopen_changed_generation')
    value = owner.api.call(prepare, 'POST', {'source': selected}, status=202)
    check(value.get('state') == 'ready', 'v2_reopen_lost_preparation_readiness')
    row.update(result='observed', cacheUnchanged=True, encoderCountUnchanged=True, idle=idle)
    receipt['stage'] = 'missing-zero-staged-regeneration'
    (media_cache / 'segment-00000.m4s').unlink()
    case('missing-zero', selected, metadata, prefix)
    check(len(owner.source_invocation_rows()) == 3, 'v2_zero_regeneration_encoder_count')
    owner.stop()
    receipt['stage'] = 'cold-server-missing-zero'
    (media_cache / 'segment-00000.m4s').unlink()
    owner.start()
    status, initialization, _ = owner.api.http(selected.removesuffix('index.m3u8') + '360p/init.mp4')
    check(status == 200 and __import__('hashlib').sha256(initialization).hexdigest() == prefix['assetSHA256']['init.mp4'],
          'v2_cold_zero_changed_initialization')
    check(len(owner.source_invocation_rows()) == 3, 'v2_cold_zero_initialization_started_encoder')
    case('cold-missing-zero', selected, metadata, prefix)
    check(len(owner.source_invocation_rows()) == 4, 'v2_cold_zero_regeneration_encoder_count')
    owner.stop()
    receipt['stage'] = 'legacy-version1-generation'
    baseline = RUN / 'baseline-source'
    run(['git', 'worktree', 'add', '--detach', str(baseline), '664f4e2ad40049b03a5da62dba444adc3caa97ec'], 30)
    old_binary = RUN / 'baseline-kinosail'
    run(['go', '-C', str(baseline / 'apps/player'), 'build', '-p=1', '-o', str(old_binary), './cmd/kinosail'], 180)
    legacy_directory = RUN / 'legacy'
    legacy_media = legacy_directory / 'media'
    legacy_media.mkdir(parents=True)
    legacy_source = legacy_media / 'Fixture.mp4'
    shutil.copy2(source, legacy_source)
    receipt['sessions'].extend(owner.sessions)
    receipt['completedSourceAudit'] = diagnostic_producer_rows(owner.invocation_rows(), source, owner.owned_pids)
    owner = ActualServer(ROOT, legacy_directory, legacy_source, binary)
    source, before = legacy_source, source_state(legacy_source)
    owner.start(authorize=True, binary=old_binary)
    item = next(value for value in owner.api.call('/api/v1/library')['items'] if value['title'] == 'Fixture')
    prepare = '/api/v1/items/' + item['id'] + '/playback-prepare'
    plan = owner.api.call('/api/v1/items/' + item['id'] + '/playback?videoCodecs=h264&audioCodecs=aac')
    selected = plan['compatible'].replace('/index.m3u8', '-o12000/index.m3u8')
    old_prepare = {}
    prepare_once(owner.api, prepare, selected, owner.log_path, owner.process, source, old_prepare)
    check(old_prepare['preparationAttempt']['completionState'] == 'ready', 'v2_legacy_prepare_control')
    certificates = list((legacy_directory / 'cache').glob('*/.copy-clock'))
    check(len(certificates) == 1, 'v2_legacy_certificate_count')
    legacy_clock = certificates[0]
    check(json.loads(legacy_clock.read_bytes())['version'] == 1, 'v2_legacy_version_control')
    old_metadata = {name: sha(legacy_clock.with_name(name)) for name in ['.source', '.copy-timeline', '.copy-clock']}
    owner.stop()
    receipt['stage'] = 'version1-direct-rejection-and-migration'
    owner.producer_start = len(owner.source_invocation_rows())
    owner.start()
    asset = selected.removesuffix('index.m3u8') + '360p/init.mp4'
    check(owner.api.http(asset)[0] == 404, 'v2_version1_direct_asset_bypass')
    check(old_metadata == {name: sha(legacy_clock.with_name(name)) for name in old_metadata},
          'v2_direct_rejection_mutated_version1')
    prepared = {}
    prepare_once(owner.api, prepare, selected, owner.log_path, owner.process, source, prepared)
    check(prepared['preparationAttempt']['completionState'] == 'ready', 'v2_migration_not_ready')
    _, _, migrated = cache_state(legacy_directory / 'cache')
    check(migrated['metadataSHA256'] != old_metadata, 'v2_version1_not_reindexed')
    case('version1-migration', selected, metadata, migrated)
    receipt['legacyPreparation'] = old_prepare
    receipt['migrationPreparation'] = prepared
    receipt['stage'] = 'complete'
    receipt['result'] = 'observed'
except Exception as error:
    receipt['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
finally:
    with guard.cleanup():
        if owner is not None:
            try:
                owner.stop()
            except Exception:
                receipt.update(result='failed', cleanupFailureClass='v2_owned_server_join_failed')
            receipt['sessions'].extend(owner.sessions)
            try:
                receipt['finalSourceAudit'] = diagnostic_producer_rows(owner.invocation_rows(), source, owner.owned_pids)
                receipt['finalSourceInvocationCount'] = len(owner.source_invocation_rows())
            except Exception as error:
                receipt.update(result='failed', invocationAuditFailureClass=str(error)
                    if isinstance(error, RuntimeError) else type(error).__name__)
        try:
            receipt['sourceUnchanged'] = source_state(source) == before if source is not None and before is not None else None
        except Exception:
            receipt['sourceUnchanged'] = False
        if receipt['sourceUnchanged'] is False:
            receipt.update(result='failed', sourceFailureClass='v2_source_changed')
            receipt.setdefault('failureClass', 'v2_source_changed')
        files = [Path(__file__), *(ROOT / 'apps/player/scripts' / name for name in [
            'hls_aac_v2_public_http.py', 'hls_aac_v2_public_evidence.py', 'hls_timeline_fixture.py',
            'hls_timeline_http.py', 'hls_timeline_packets.py', 'hls_followon_public.py',
            'hls_remaining_process.py', 'hls_remaining_nonkey_evidence.py',
            'hls_remaining_nonkey_boundary.py', 'hls_nonkey_browser_public.py',
            'hls_nonkey_browser_audio_boundary.py', 'hls_nonkey_browser_packet_association.py',
            'hls_nonkey_browser_packet_clock.py'])]
        receipt['executedScriptSHA256'] = {str(path.relative_to(ROOT)): sha(path) for path in files}
        raw = json.dumps(receipt, separators=(',', ':'), allow_nan=False) + '\n'
        check(0 < len(raw.encode()) <= 32 << 20, 'v2_safe_receipt_bound')
        target = RUN / 'receipt.json'
        target.write_text(raw)
        (RUN / 'SHA256SUMS').write_text(sha(target) + '  receipt.json\n' +
            ''.join(sha(path) + '  ' + str(path.relative_to(ROOT)) + '\n' for path in files))
        print(json.dumps({'revision': receipt['revision'], 'tree': receipt['tree'], 'result': receipt['result'],
            'stage': receipt.get('stage'), 'failureClass': receipt.get('failureClass'),
            'producerCapture': receipt.get('producerCapture'), 'encoderLifecycle': receipt.get('encoderLifecycle'),
            'encoderSeekPhases': receipt.get('encoderSeekPhases'),
            'initialTimeline': {key: receipt.get('initialCache', {}).get(key) for key in ['timelineEnd', 'timelineGrid', 'timelineKeys']},
            'sourceVideoGrid': receipt.get('sourceVideoGrid'),
            'finalSourceInvocationCount': receipt.get('finalSourceInvocationCount'),
            'finalSourceAudit': receipt.get('finalSourceAudit'),
            'cases': [{'label': value['label'], 'result': value['result'],
                       'manifestFacts': value.get('manifestFacts'), 'segmentNames': value.get('segmentNames'),
                       'publicPackets': value.get('tail', {}).get('publicPackets'),
                       'videoFrames': len(value.get('observations', {}).get('publicFrameRows', []))}
                      for value in receipt['cases']], 'receiptSHA256': sha(target),
            'productionAcceptance': False, 'nativeAcceptance': False}), flush=True)
    guard.__exit__()
raise SystemExit(0 if receipt['result'] == 'observed' else 1)
