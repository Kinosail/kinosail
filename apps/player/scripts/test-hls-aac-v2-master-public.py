#!/usr/bin/env python3
"""Actual Version1 and Version2 malformed master GET/HEAD rejection controls."""
import json
from pathlib import Path
import platform
import shutil
import subprocess
import sys
import time
from hls_aac_v2_lazy_public import clone_arm, seed, selected_path
from hls_aac_v2_master_public import FAULTS, master_path, reject
from hls_aac_v2_public_evidence import assert_fixed_source_grid, cache_state
from hls_aac_v2_public_http import cached_media, diagnostic_producer_rows, idle
from hls_aac_v2_compat_public import diagnostics, requests, responses, require_diagnostics, snapshot
from hls_followon_frames import decode_frames
from hls_followon_public import bounded_bytes, check, prepare_once
from hls_remaining_nonkey_deadline import DiagnosticDeadline
from hls_timeline_fixture import fixture
from hls_timeline_http import sha, source_state

ROOT = Path(__file__).resolve().parents[3]
RUN = ROOT / '.verification/hls-aac-v2-master' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
RUN.mkdir(parents=True)
receipt = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(),
    'tree': subprocess.check_output(['git', 'rev-parse', 'HEAD^{tree}'], text=True).strip(),
    'baselineRevision': '664f4e2ad40049b03a5da62dba444adc3caa97ec',
    'command': 'python3 apps/player/scripts/test-hls-aac-v2-master-public.py',
    'environment': {'platform': platform.platform(), 'python': sys.version.split()[0],
        'codecPackage': 'jellyfin-ffmpeg8_8.1.2-5-noble_amd64.deb',
        'codecPackageSHA256': 'b4e72894ad26c809ed0104805f5415a97be75212b9fcf9d60b89ad25bb3d43e3'},
    'scope': 'Actual V1/V2 master and rendition GET/HEAD reject malformed or init-mismatched masters without writes or encoders',
    'result': 'failed', 'controls': [], 'cases': [], 'masterRejectionAcceptance': False,
    'olderClientAcceptance': False, 'productionAcceptance': False, 'nativeAcceptance': False}
owners, source, before = [], None, None
guard = DiagnosticDeadline(900)
guard.__enter__()


def run(command, timeout=60):
    result = subprocess.run(command, capture_output=True, timeout=min(timeout, guard.check(15)))
    check(result.returncode == 0 and len(result.stdout) <= 2 << 20 and len(result.stderr) <= 2 << 20,
        'master_command_failed_or_unbounded')
    return result.stdout


def older_marker_controls(cache, baseline, candidate, source, selected, reference):
    template = RUN / 'older-marker-cache'
    shutil.copytree(cache, template)
    path = master_path(template)
    original = bounded_bytes(path, 256 << 10, 'master_older_marker_bound')
    check(original.count(b'#KINOSAIL-BANDWIDTH:2\n') == 1, 'master_older_marker_fixture')
    changed = original.replace(b'#KINOSAIL-BANDWIDTH:2\n', b'', 1)
    path.write_bytes(changed)
    expected = None
    for label, binary in [('baseline', baseline), ('candidate', candidate)]:
        row = {'version': 1, 'format': 'before-bandwidth-marker', 'arm': label,
            'result': 'failed', 'candidatePreparePOST': False}
        receipt['controls'].append(row)
        owner = clone_arm(ROOT, RUN, 'older-marker-' + label, binary, source, template, owners)
        steps = [*requests('master', 'HEAD', selected), *requests('rendition', 'HEAD', selected),
            *requests('journey', 'GET', selected)]
        replies = responses(owner, steps)
        row.update(statuses=[v[0] for v in replies], headBodiesEmpty=all(not v[1] for v in replies[:2]),
            immediateCacheUnchanged=snapshot(owner.directory / 'cache') == owner.clone_snapshot,
            immediateSourceCalls=len(owner.source_invocation_rows()))
        row['idle'] = idle(owner.api, owner.process, source)
        row['idleCacheUnchanged'] = snapshot(owner.directory / 'cache') == owner.clone_snapshot
        row['diagnostics'] = diagnostics(owner, replies)
        require_diagnostics(row['diagnostics'], 0)
        owner.stop()
        row.update(cacheUnchanged=snapshot(owner.directory / 'cache') == owner.clone_snapshot,
            sourceCalls=len(owner.source_invocation_rows()), sessions=owner.sessions,
            masterSHA256=sha(master_path(owner.directory / 'cache')))
        check(row['statuses'] == [200] * 15 and row['headBodiesEmpty']
            and row['immediateCacheUnchanged'] and row['idleCacheUnchanged'] and row['cacheUnchanged']
            and row['immediateSourceCalls'] == row['sourceCalls'] == 0, 'master_older_marker_client_control')
        bodies = [v[1] for v in replies[2:]]
        if expected is None:
            expected = bodies
        row['exactBaselineBodies'] = bodies == expected
        check(row['exactBaselineBodies'], 'master_older_marker_exact_baseline_bodies')
        joined = owner.directory / 'older-joined.mp4'
        joined.write_bytes(b''.join(v[1] for v in replies[4:]))
        _, actual = decode_frames(joined)
        row.update(decodedFrames=len(actual), exactSourceFrames=[v[1] for v in actual] == [v[1] for v in reference])
        check(len(actual) == 480 and row['exactSourceFrames'], 'master_older_marker_exact_source_frames')
        row['result'] = 'observed'


try:
    check(shutil.disk_usage(RUN).free >= 2 << 30, 'master_hosted_disk_budget')
    regular, _ = fixture(RUN, 'regular', 48, ','.join(str(v) for v in range(0, 32, 2)), frames=768)
    media = RUN / 'media'
    media.mkdir()
    source = media / 'Fixture.mp4'
    run(['ffmpeg', '-nostdin', '-v', 'error', '-i', str(regular), '-map', '0:v:0', '-map', '0:a:0',
        '-c', 'copy', str(source)], 45)
    before = source_state(source)
    check(before['sha256'] == '198a6808092cea2a5481afc24e79481e4966ae577cbb8c46c214737e2173a3ed',
        'master_fixed_source_identity')
    grid = assert_fixed_source_grid(json.loads(run(['ffprobe', '-v', 'error',
        '-select_streams', 'v:0', '-show_packets', '-show_streams',
        '-show_entries', 'stream=time_base:packet=pts,dts,duration,flags', '-of', 'json', str(source)])))
    old = RUN / 'baseline-source'
    run(['git', 'worktree', 'add', '--detach', str(old), receipt['baselineRevision']], 30)
    baseline, candidate = RUN / 'baseline-kinosail', RUN / 'candidate-kinosail'
    run(['go', '-C', str(old / 'apps/player'), 'build', '-p=1', '-o', str(baseline), './cmd/kinosail'], 180)
    run(['go', '-C', str(ROOT / 'apps/player'), 'build', '-p=1', '-o', str(candidate), './cmd/kinosail'], 180)
    (RUN / 'empty-cache').mkdir()
    selected, _, _, complete = seed(ROOT, RUN, baseline, source, owners, receipt.setdefault('v1Seed', {}))
    v2 = clone_arm(ROOT, RUN, 'v2-seed', candidate, source, RUN / 'empty-cache', owners)
    item, v2_selected = selected_path(v2)
    preparation = receipt.setdefault('v2Seed', {})
    prepare_once(v2.api, '/api/v1/items/' + item['id'] + '/playback-prepare',
        v2_selected, v2.log_path, v2.process, source, preparation)
    check(preparation['preparationAttempt']['completionState'] == 'ready', 'master_v2_seed_not_ready')
    joined, _, _, _ = cached_media(v2, v2_selected, RUN / 'v2-media', grid)
    _, frames = decode_frames(joined)
    _, reference = decode_frames(source, offset=12)
    preparation.update(decodedFrames=len(frames), referenceFrames=len(reference),
        exactSourceFrames=[v[1] for v in frames] == [v[1] for v in reference])
    check(len(frames) == len(reference) == 480 and preparation['exactSourceFrames'],
        'master_v2_seed_source_frames')
    preparation['certificateVersion'] = cache_state(v2.directory / 'cache')[2]['certificateVersion']
    check(preparation['certificateVersion'] == 2, 'master_v2_real_origin_control')
    idle(v2.api, v2.process, source)
    v2.stop()
    preparation.update(sessions=v2.sessions, sourceCalls=len(v2.source_invocation_rows()),
        sourceAudit=diagnostic_producer_rows(v2.invocation_rows(), source, v2.owned_pids))
    seeds = [(1, complete, selected), (2, v2.directory / 'cache', v2_selected)]
    for version, cache, path in seeds:
        row = {'version': version, 'result': 'failed', 'candidatePreparePOST': False}
        receipt['controls'].append(row)
        owner = clone_arm(ROOT, RUN, 'valid-v' + str(version), candidate, source, cache, owners)
        heads = responses(owner, [*requests('master', 'HEAD', path), *requests('rendition', 'HEAD', path)])
        row.update(headStatuses=[v[0] for v in heads], headBodiesEmpty=all(not v[1] for v in heads),
            headCacheUnchanged=snapshot(owner.directory / 'cache') == owner.clone_snapshot,
            headSourceCalls=len(owner.source_invocation_rows()))
        check(row['headStatuses'] == [200, 200] and row['headBodiesEmpty']
            and row['headCacheUnchanged'] and row['headSourceCalls'] == 0, 'master_positive_head_control')
        replies = responses(owner, requests('journey', 'GET', path))
        row.update(statuses=[v[0] for v in replies],
            immediateCacheUnchanged=snapshot(owner.directory / 'cache') == owner.clone_snapshot,
            immediateSourceCalls=len(owner.source_invocation_rows()))
        row['idle'] = idle(owner.api, owner.process, source)
        row['idleCacheUnchanged'] = snapshot(owner.directory / 'cache') == owner.clone_snapshot
        row['diagnostics'] = diagnostics(owner, [*heads, *replies])
        require_diagnostics(row['diagnostics'], 0)
        owner.stop()
        row.update(cacheUnchanged=snapshot(owner.directory / 'cache') == owner.clone_snapshot,
            sourceCalls=len(owner.source_invocation_rows()), sessions=owner.sessions)
        check(row['statuses'] == [200] * 13 and row['immediateCacheUnchanged']
            and row['idleCacheUnchanged'] and row['cacheUnchanged']
            and row['immediateSourceCalls'] == row['sourceCalls'] == 0,
            'master_positive_client_control')
        joined = owner.directory / 'valid-joined.mp4'
        joined.write_bytes(b''.join(v[1] for v in replies[2:]))
        _, actual = decode_frames(joined)
        row.update(decodedFrames=len(actual), exactSourceFrames=[v[1] for v in actual] == [v[1] for v in reference])
        check(len(actual) == 480 and row['exactSourceFrames'], 'master_positive_exact_source_frames')
        row['result'] = 'observed'
        for fault in FAULTS:
            label = 'v' + str(version) + '-' + fault
            row = {'label': label, 'version': version, 'fault': fault, 'result': 'failed',
                'candidatePreparePOST': False}
            receipt['cases'].append(row)
            try:
                reject(ROOT, RUN, label, candidate, source, cache, path, owners, before, row)
            except Exception as error:
                row['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
                for owned in owners:
                    owned.stop()
    older_marker_controls(complete, baseline, candidate, source, selected, reference)
    if len(receipt['controls']) == 4 and len(receipt['cases']) == 24 and (
        all(v['result'] == 'observed' for v in [*receipt['controls'], *receipt['cases']])):
        receipt.update(result='observed', masterRejectionAcceptance=True)
except Exception as error:
    receipt['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
finally:
    with guard.cleanup():
        receipt['ownedServerEvidence'] = []
        for owner in owners:
            try:
                owner.stop()
            except Exception:
                receipt['result'] = 'failed'
                receipt.setdefault('cleanupFailureClasses', []).append('master_owned_join_failed')
            evidence = {'label': owner.directory.name, 'sessions': owner.sessions}
            receipt['ownedServerEvidence'].append(evidence)
            try:
                evidence.update(sourceCalls=len(owner.source_invocation_rows()),
                    sourceAudit=diagnostic_producer_rows(owner.invocation_rows(), source, owner.owned_pids))
            except Exception as error:
                evidence['sourceAuditFailureClass'] = type(error).__name__
                receipt['result'] = 'failed'
        try:
            receipt['sourceUnchanged'] = source_state(source) == before if source is not None and before is not None else None
        except Exception:
            receipt['sourceUnchanged'] = False
        if receipt['sourceUnchanged'] is False:
            receipt.update(result='failed', sourceGuardFailureClass='master_source_changed')
        names = ['hls_aac_v2_master_public.py', 'hls_aac_v2_lazy_public.py',
            'hls_aac_v2_compat_public.py', 'hls_aac_v2_public_http.py', 'hls_aac_v2_public_evidence.py',
            'hls_followon_public.py', 'hls_remaining_nonkey_deadline.py', 'hls_remaining_process.py',
            'hls_timeline_http.py', 'hls_timeline_fixture.py', 'hls_timeline_packets.py',
            'hls_remaining_nonkey_evidence.py', 'hls_remaining_nonkey_boundary.py',
            'hls_nonkey_browser_public.py', 'hls_nonkey_browser_audio_boundary.py',
            'hls_nonkey_browser_packet_association.py', 'hls_nonkey_browser_packet_clock.py', 'hls_followon_frames.py']
        files = [Path(__file__), *(Path(__file__).with_name(name) for name in names)]
        receipt['executedScriptSHA256'] = {str(p.relative_to(ROOT)): sha(p) for p in files}
        raw = json.dumps(receipt, separators=(',', ':'), allow_nan=False) + '\n'
        check(0 < len(raw.encode()) <= 32 << 20, 'master_receipt_bound')
        target = RUN / 'receipt.json'
        target.write_text(raw)
        (RUN / 'SHA256SUMS').write_text(sha(target) + '  receipt.json\n' +
            ''.join(sha(p) + '  ' + str(p.relative_to(ROOT)) + '\n' for p in files))
        print(json.dumps({'revision': receipt['revision'], 'tree': receipt['tree'], 'result': receipt['result'],
            'failureClass': receipt.get('failureClass'), 'receiptSHA256': sha(target),
            'v1Seed': receipt.get('v1Seed'), 'v2Seed': receipt.get('v2Seed'),
            'controls': receipt['controls'], 'cases': receipt['cases'],
            'ownedServerEvidence': receipt['ownedServerEvidence'],
            'sourceGuardFailureClass': receipt.get('sourceGuardFailureClass'),
            'cleanupFailureClasses': receipt.get('cleanupFailureClasses'),
            'masterRejectionAcceptance': receipt['masterRejectionAcceptance'],
            'olderClientAcceptance': False, 'productionAcceptance': False, 'nativeAcceptance': False}), flush=True)
    guard.__exit__()
raise SystemExit(0 if receipt['result'] == 'observed' else 1)
