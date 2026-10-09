#!/usr/bin/env python3
"""Version1 playlist-only older-client diagnostic; safe rejection is a release blocker."""
import hashlib
import json
from pathlib import Path
import platform
import re
import shutil
import subprocess
import sys
import time
from hls_aac_v2_public_http import ActualServer, idle
from hls_followon_public import bounded_bytes, check, prepare_once
from hls_remaining_nonkey_deadline import DiagnosticDeadline
from hls_timeline_fixture import fixture
from hls_timeline_http import sha, source_state

ROOT = Path(__file__).resolve().parents[3]
RUN = ROOT / '.verification/hls-aac-v2-legacy' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
RUN.mkdir(parents=True)
receipt = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(),
    'tree': subprocess.check_output(['git', 'rev-parse', 'HEAD^{tree}'], text=True).strip(),
    'command': 'python3 apps/player/scripts/test-hls-aac-v2-legacy-public.py',
    'environment': {'platform': platform.platform(), 'python': sys.version.split()[0],
        'codecPackage': 'jellyfin-ffmpeg8_8.1.2-5-noble_amd64.deb',
        'codecPackageSHA256': 'b4e72894ad26c809ed0104805f5415a97be75212b9fcf9d60b89ad25bb3d43e3'},
    'baselineRevision': '664f4e2ad40049b03a5da62dba444adc3caa97ec',
    'scope': 'Actual older-client master/rendition GET and HEAD only; no candidate prepare POST or GET regeneration',
    'result': 'failed', 'cases': [], 'releaseBlocker': 'supported-client-version1-request-path-unverified',
    'olderClientAcceptance': False, 'productionAcceptance': False, 'nativeAcceptance': False}
owner, source, before = None, None, None
guard = DiagnosticDeadline(600)
guard.__enter__()


def run(command, timeout=60):
    result = subprocess.run(command, capture_output=True, timeout=min(timeout, guard.check(15)))
    check(result.returncode == 0 and len(result.stdout) <= 2 << 20 and len(result.stderr) <= 2 << 20,
          'legacy_command_failed_or_unbounded')
    return result.stdout


def snapshot(cache):
    result = {}
    paths = sorted(cache.rglob('*'))
    check(len(paths) <= 96, 'binding_cache_path_bound')
    for path in paths:
        if path.is_dir():
            continue
        check(path.is_file() and not path.is_symlink(), 'binding_cache_kind')
        stat = path.stat()
        result[str(path.relative_to(cache))] = {'inode': stat.st_ino, 'bytes': stat.st_size,
            'mtimeNs': stat.st_mtime_ns, 'sha256': hashlib.sha256(
                bounded_bytes(path, 2 << 20, 'binding_cache_bytes_bound')).hexdigest()}
    return result



def rejection_diagnostic(log_path, request_id):
    valid_id = re.fullmatch(r'[a-zA-Z0-9_-]{8,96}', request_id) is not None
    rows = []
    for line in bounded_bytes(log_path, 2 << 20, 'legacy_private_log_bound').splitlines():
        try:
            entry = json.loads(line)
        except json.JSONDecodeError:
            continue
        if entry.get('msg') != 'HLS copied playlist rejected':
            continue
        valid = (set(entry) == {'time', 'level', 'msg', 'request_id', 'playback_session', 'failure_class'}
            and entry.get('level') == 'WARN' and entry.get('failure_class') == 'invalid-source-binding'
            and entry.get('request_id') == request_id and entry.get('playback_session') == '')
        rows.append({'boundedFieldsAndCorrelationValid': valid})
    check(valid_id and len(rows) == 1 and all(row['boundedFieldsAndCorrelationValid'] for row in rows),
        'legacy_rejection_diagnostic_level_fields_or_correlation')
    return rows


def arm(label, method, asset, candidate, baseline, seed_cache, selected):
    global owner
    row = {'label': label, 'method': method, 'result': 'failed', 'candidatePreparePOST': False}
    receipt['cases'].append(row)
    directory = RUN / label
    directory.mkdir()
    shutil.copytree(seed_cache, directory / 'cache')
    original = snapshot(directory / 'cache')
    owner = ActualServer(ROOT, directory, source, candidate)
    try:
        owner.start(authorize=True, binary=baseline)
        item = next(v for v in owner.api.call('/api/v1/library')['items'] if v['title'] == 'Fixture')
        plan = owner.api.call('/api/v1/items/' + item['id'] + '/playback?videoCodecs=h264&audioCodecs=aac')
        check(plan['compatible'].replace('/index.m3u8', '-o12000/index.m3u8') == selected,
            'legacy_clone_identity')
        target = selected if asset == 'master' else selected.removesuffix('index.m3u8') + '360p/index.m3u8'
        baseline_status, baseline_body, _ = owner.api.http(target, method=method)
        check(baseline_status == 200, 'legacy_baseline_playlist_control_failed')
        idle(owner.api, owner.process, source)
        check(snapshot(directory / 'cache') == original and len(owner.source_invocation_rows()) == 0,
            'legacy_baseline_silently_regenerated')
        owner.stop()
        check(snapshot(directory / 'cache') == original, 'legacy_baseline_stop_changed_cache')
        owner.start()
        status, body, headers = owner.api.http(target, method=method)
        immediate, immediate_calls = snapshot(directory / 'cache'), len(owner.source_invocation_rows())
        request_id = next((v for k, v in headers.items() if k.lower() == 'x-request-id'), '')
        row.update(baselineStatus=baseline_status, status=status, bodyBytes=len(body),
            bodySHA256=hashlib.sha256(body).hexdigest(), cacheUnchanged=immediate == original,
            sourceCalls=immediate_calls, requestID=request_id)
        idle(owner.api, owner.process, source)
        row['diagnostics'] = rejection_diagnostic(owner.log_path, request_id)
        owner.stop()
        row.update(finalCache=snapshot(directory / 'cache'), finalSourceCalls=len(owner.source_invocation_rows()),
            sourceUnchanged=source_state(source) == before)
        check(status == 404 and immediate == original and immediate_calls == 0
            and row['finalCache'] == original and row['finalSourceCalls'] == 0 and row['sourceUnchanged'],
            'legacy_playlist_only_admitted_regenerated_or_mutated')
        owner.start(binary=baseline)
        restored_status, restored_body, _ = owner.api.http(target, method=method)
        check(restored_status == baseline_status and restored_body == baseline_body,
            'legacy_candidate_rejection_destroyed_baseline_exact_playlist')
        row.update(result='observed', olderClientPlaylistPlayable=False, baselineRestoredExactBody=True)
    except Exception as error:
        row['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
    finally:
        try:
            owner.stop()
        except Exception:
            row['sessions'] = owner.sessions
            raise RuntimeError('legacy_owned_join_failed') from None
        row['sessions'] = owner.sessions
        if row['result'] == 'observed' and (snapshot(directory / 'cache') != original
            or len(owner.source_invocation_rows()) != 0 or source_state(source) != before):
            row.update(result='failed', failureClass='legacy_late_cache_or_source_mutation')
        owner = None


try:
    check(shutil.disk_usage(RUN).free >= 2 << 30, 'legacy_hosted_disk_budget')
    regular, _ = fixture(RUN, 'regular', 48, ','.join(str(v) for v in range(0, 32, 2)), frames=768)
    seed = RUN / 'seed'
    media = seed / 'media'
    media.mkdir(parents=True)
    source = media / 'Fixture.mp4'
    run(['ffmpeg', '-nostdin', '-v', 'error', '-i', str(regular), '-map', '0:v:0', '-map', '0:a:0',
        '-c', 'copy', str(source)], 45)
    before = source_state(source)
    check(before['sha256'] == '198a6808092cea2a5481afc24e79481e4966ae577cbb8c46c214737e2173a3ed',
        'legacy_fixed_source_identity')
    old_source = RUN / 'baseline-source'
    run(['git', 'worktree', 'add', '--detach', str(old_source), receipt['baselineRevision']], 30)
    baseline, candidate = RUN / 'baseline-kinosail', RUN / 'candidate-kinosail'
    run(['go', '-C', str(old_source / 'apps/player'), 'build', '-p=1', '-o', str(baseline), './cmd/kinosail'], 180)
    run(['go', '-C', str(ROOT / 'apps/player'), 'build', '-p=1', '-o', str(candidate), './cmd/kinosail'], 180)
    owner = ActualServer(ROOT, seed, source, candidate)
    owner.start(authorize=True, binary=baseline)
    item = next(v for v in owner.api.call('/api/v1/library')['items'] if v['title'] == 'Fixture')
    plan = owner.api.call('/api/v1/items/' + item['id'] + '/playback?videoCodecs=h264&audioCodecs=aac')
    selected = plan['compatible'].replace('/index.m3u8', '-o12000/index.m3u8')
    prepared = {}
    prepare_once(owner.api, '/api/v1/items/' + item['id'] + '/playback-prepare',
        selected, owner.log_path, owner.process, source, prepared)
    check(prepared['preparationAttempt']['completionState'] == 'ready', 'legacy_real_seed_not_ready')
    idle(owner.api, owner.process, source)
    check(len(owner.source_invocation_rows()) == 1, 'legacy_seed_source_invocation_control')
    # Adopt only the disposable baseline seed before sealing independent clones.
    check(owner.api.http(selected)[0] == 200
        and owner.api.http(selected.removesuffix('index.m3u8') + '360p/index.m3u8')[0] == 200,
        'legacy_seed_baseline_adoption_failed')
    idle(owner.api, owner.process, source)
    check(len(owner.source_invocation_rows()) == 1, 'legacy_seed_adoption_refilled_source')
    certificates = list((seed / 'cache').glob('*/.copy-clock'))
    check(len(certificates) == 1 and json.loads(bounded_bytes(certificates[0], 4096, 'legacy_certificate_bound'))['version'] == 1,
        'legacy_real_version1_control')
    owner.stop()
    check(not certificates[0].with_name('.startup').exists(), 'legacy_seed_unadopted_startup_marker')
    receipt['seedSessions'] = owner.sessions
    owner = None
    for asset in ['master', 'rendition']:
        for method in ['GET', 'HEAD']:
            arm(asset + '-' + method.lower(), method, asset, candidate, baseline, seed / 'cache', selected)
    if len(receipt['cases']) == 4 and all(v['result'] == 'observed' for v in receipt['cases']):
        receipt['result'] = 'observed'
except Exception as error:
    receipt['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
finally:
    with guard.cleanup():
        if owner is not None:
            try:
                owner.stop()
            except Exception:
                receipt.update(result='failed', cleanupFailureClass='legacy_owned_join_failed')
        try:
            receipt['sourceUnchanged'] = source_state(source) == before if source is not None and before is not None else None
        except Exception:
            receipt['sourceUnchanged'] = False
        if receipt['sourceUnchanged'] is False:
            receipt.update(result='failed', failureClass='legacy_source_changed')
        names = ['hls_aac_v2_public_http.py', 'hls_aac_v2_public_evidence.py', 'hls_followon_public.py',
            'hls_remaining_nonkey_deadline.py', 'hls_remaining_process.py', 'hls_timeline_http.py',
            'hls_timeline_fixture.py', 'hls_timeline_packets.py', 'hls_remaining_nonkey_evidence.py',
            'hls_remaining_nonkey_boundary.py', 'hls_nonkey_browser_public.py',
            'hls_nonkey_browser_audio_boundary.py', 'hls_nonkey_browser_packet_association.py',
            'hls_nonkey_browser_packet_clock.py']
        files = [Path(__file__), *(Path(__file__).with_name(name) for name in names)]
        receipt['executedScriptSHA256'] = {str(p.relative_to(ROOT)): sha(p) for p in files}
        raw = json.dumps(receipt, separators=(',', ':'), allow_nan=False) + '\n'
        check(0 < len(raw.encode()) <= 32 << 20, 'legacy_receipt_bound')
        target = RUN / 'receipt.json'
        target.write_text(raw)
        (RUN / 'SHA256SUMS').write_text(sha(target) + '  receipt.json\n' +
            ''.join(sha(p) + '  ' + str(p.relative_to(ROOT)) + '\n' for p in files))
        print(json.dumps({'revision': receipt['revision'], 'tree': receipt['tree'], 'result': receipt['result'],
            'failureClass': receipt.get('failureClass'), 'receiptSHA256': sha(target),
            'cases': [{k: v.get(k) for k in ['label', 'result', 'failureClass', 'baselineStatus', 'status',
                'cacheUnchanged', 'sourceCalls', 'sourceUnchanged', 'olderClientPlaylistPlayable',
                'baselineRestoredExactBody', 'diagnostics']} for v in receipt['cases']],
            'olderClientAcceptance': False, 'releaseBlocker': receipt['releaseBlocker'],
            'productionAcceptance': False, 'nativeAcceptance': False}), flush=True)
    guard.__exit__()
raise SystemExit(0 if receipt['result'] == 'observed' else 1)
