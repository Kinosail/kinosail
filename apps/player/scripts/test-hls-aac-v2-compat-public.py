#!/usr/bin/env python3
"""Actual Version1 older-client compatibility; no preparation POST or migration."""
import hashlib
import json
from pathlib import Path
import platform
import shutil
import subprocess
import sys
import time
from hls_followon_frames import decode_frames
from hls_aac_v2_compat_public import diagnostics, fault, query_controls, requests, require_diagnostics, responses, snapshot
from hls_aac_v2_public_http import ActualServer, diagnostic_producer_rows, idle
from hls_followon_public import bounded_bytes, check, prepare_once
from hls_remaining_nonkey_deadline import DiagnosticDeadline
from hls_timeline_fixture import fixture
from hls_timeline_http import sha, source_state

ROOT = Path(__file__).resolve().parents[3]
RUN = ROOT / '.verification/hls-aac-v2-compat' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
RUN.mkdir(parents=True)
receipt = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(),
    'tree': subprocess.check_output(['git', 'rev-parse', 'HEAD^{tree}'], text=True).strip(),
    'command': 'python3 apps/player/scripts/test-hls-aac-v2-compat-public.py',
    'environment': {'platform': platform.platform(), 'python': sys.version.split()[0],
        'codecPackage': 'jellyfin-ffmpeg8_8.1.2-5-noble_amd64.deb',
        'codecPackageSHA256': 'b4e72894ad26c809ed0104805f5415a97be75212b9fcf9d60b89ad25bb3d43e3'},
    'baselineRevision': '664f4e2ad40049b03a5da62dba444adc3caa97ec',
    'scope': 'Complete adopted Version1 master/rendition, init, every media fragment and Range; no candidate POST, migration or regeneration',
    'result': 'failed', 'cases': [], 'releaseBlocker': 'version1-lazy-startup-migration-still-unverified',
    'completeAdoptedVersion1Acceptance': False,
    'olderClientAcceptance': False, 'productionAcceptance': False, 'nativeAcceptance': False,
    'remainingBoundaries': ['Version1 lazy/interrupted continuation', 'speculative startup adoption',
        'concurrent migration and filesystem races', 'actual native client and all50', 'historical Version1 AAC failures']}
owner, source, before = None, None, None
guard = DiagnosticDeadline(900)
guard.__enter__()


def run(command, timeout=60):
    result = subprocess.run(command, capture_output=True, timeout=min(timeout, guard.check(15)))
    check(result.returncode == 0 and len(result.stdout) <= 2 << 20 and len(result.stderr) <= 2 << 20,
          'legacy_command_failed_or_unbounded')
    return result.stdout


def arm(label, method, asset, candidate, baseline, seed_cache, selected, invalid=None, warm=False, query=None, control_status=None):
    global owner
    row = {'label': label, 'method': method, 'result': 'failed', 'candidatePreparePOST': False,
        'expectedStatus': 404 if invalid else control_status or (206 if method == 'RANGE' else 200), 'warm': warm}
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
            'compat_clone_identity')
        steps = requests(asset, method, selected)
        if query is not None:
            steps = [(target + '?' + query, verb, ranged) for target, verb, ranged in steps]
        controls = responses(owner, steps)
        expected = control_status or (206 if method == 'RANGE' else 200)
        check(all(status == expected for status, _, _ in controls), 'compat_baseline_control')
        if asset == 'journey':
            reference = directory / 'baseline-joined.mp4'
            reference.write_bytes(b''.join(body for _, body, _ in controls[2:]))
            _, reference_rows = decode_frames(reference)
            _, source_rows = decode_frames(source, offset=12)
            check(len(reference_rows) == len(source_rows) == 480
                and [v[1] for v in reference_rows] == [v[1] for v in source_rows],
                'compat_baseline_exact_source_frame_count')
            row['baselineDecodedFrames'] = len(reference_rows)
        idle(owner.api, owner.process, source)
        check(snapshot(directory / 'cache') == original and len(owner.source_invocation_rows()) == 0,
            'compat_baseline_silently_regenerated')
        owner.stop()
        check(snapshot(directory / 'cache') == original, 'compat_baseline_stop_changed_cache')
        if invalid:
            fault(directory / 'cache', invalid)
            original = snapshot(directory / 'cache')
        owner.start()
        if warm:
            binding = next((directory / 'cache').glob('*/.source'))
            retained = directory / 'retained-binding'
            binding.rename(retained)
            absent = snapshot(directory / 'cache')
            status, _, _ = owner.api.http(selected)
            idle(owner.api, owner.process, source)
            check(status == 404 and snapshot(directory / 'cache') == absent
                and len(owner.source_invocation_rows()) == 0, 'compat_warm_missing_binding_control')
            retained.rename(binding)
            check(snapshot(directory / 'cache') == original, 'compat_warm_exact_binding_restore')
            row['warmMissingBindingRejectedWithoutWrites'] = True
        actual = responses(owner, steps)
        immediate, calls = snapshot(directory / 'cache'), len(owner.source_invocation_rows())
        row.update(baselineStatuses=[v[0] for v in controls], statuses=[v[0] for v in actual],
            bodySHA256=[hashlib.sha256(v[1]).hexdigest() for v in actual],
            cacheUnchanged=immediate == original, sourceCalls=calls)
        idle(owner.api, owner.process, source)
        row['diagnostics'] = diagnostics(owner, actual)
        owner.stop()
        row.update(finalCacheUnchanged=snapshot(directory / 'cache') == original,
            finalSourceCalls=len(owner.source_invocation_rows()), sourceUnchanged=source_state(source) == before)
        check(immediate == original and calls == 0 and row['finalCacheUnchanged']
            and row['finalSourceCalls'] == 0 and row['sourceUnchanged'], 'compat_request_changed_cache_or_source')
        if invalid:
            check(all(status == 404 for status, _, _ in actual), 'compat_invalid_generation_admitted')
        else:
            row['exactBaselineBodies'] = all(a[:2] == b[:2] for a, b in zip(actual, controls))
            check(row['exactBaselineBodies'], 'compat_existing_client_status_or_body_regression')
            if method == 'RANGE':
                check(len(actual[0][1]) == 25 and actual[0][2].get('Content-Range', '').startswith('bytes 7-31/'),
                    'compat_range_shape')
            if asset == 'journey':
                joined = directory / 'candidate-joined.mp4'
                joined.write_bytes(b''.join(body for _, body, _ in actual[2:]))
                _, rows = decode_frames(joined)
                check(rows == reference_rows, 'compat_existing_client_decoded_frames_changed')
                row['candidateDecodedFrames'] = len(rows)
            owner.start(binary=baseline)
            restored = responses(owner, steps)
            check(all(a[:2] == b[:2] for a, b in zip(restored, controls)), 'compat_baseline_restore')
            row['baselineRestoredExactBody'] = True
        require_diagnostics(row['diagnostics'], len(actual) if invalid else 0)
        row['result'] = 'observed'
    except Exception as error:
        row['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
    finally:
        try:
            owner.stop()
        except Exception:
            row['sessions'] = owner.sessions
            raise RuntimeError('compat_owned_join_failed') from None
        row['sessions'] = owner.sessions
        if row['result'] == 'observed' and (snapshot(directory / 'cache') != original
            or len(owner.source_invocation_rows()) != 0 or source_state(source) != before):
            row.update(result='failed', failureClass='compat_late_cache_or_source_mutation')
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
    # Existing baseline GET fills only this disposable seed before immutable clones.
    hydration = receipt['seedHydration'] = []
    hydration_bytes = []
    prefix = selected.removesuffix('index.m3u8') + '360p/'
    for name in ['init.mp4', *['segment-' + str(n).zfill(5) + '.m4s' for n in range(10)]]:
        paths = list((seed / 'cache').glob('*/360p/' + name))
        physical_before = len(paths) == 1 and paths[0].is_file()
        status, body, _ = owner.api.http(prefix + name)
        hydration.append({'asset': name, 'physicalBefore': physical_before, 'status': status,
            'bytes': len(body), 'sha256': hashlib.sha256(body).hexdigest()})
        check(status == 200 and 0 < len(body) <= 2 << 20, 'compat_baseline_seed_hydration')
        hydration_bytes.append(body)
    idle(owner.api, owner.process, source)
    receipt['seedSourceAudit'] = diagnostic_producer_rows(owner.invocation_rows(), source, owner.owned_pids)
    receipt['seedSourceInvocations'] = len(owner.source_invocation_rows())
    check(1 <= receipt['seedSourceInvocations'] <= 10, 'compat_seed_hydration_encoder_bound')
    certificates = list((seed / 'cache').glob('*/.copy-clock'))
    check(len(certificates) == 1 and json.loads(bounded_bytes(certificates[0], 4096, 'legacy_certificate_bound'))['version'] == 1,
        'legacy_real_version1_control')
    seed_joined = seed / 'baseline-joined.mp4'
    seed_joined.write_bytes(b''.join(hydration_bytes))
    _, seed_rows = decode_frames(seed_joined)
    _, seed_source_rows = decode_frames(source, offset=12)
    check(len(seed_rows) == len(seed_source_rows) == 480
        and [v[1] for v in seed_rows] == [v[1] for v in seed_source_rows],
        'compat_seed_exact_source_frames')
    receipt['seedDecodedSourceFrames'] = len(seed_rows)
    owner.stop()
    receipt['seedFinalSourceAudit'] = diagnostic_producer_rows(owner.invocation_rows(), source, owner.owned_pids)
    check(receipt['seedFinalSourceAudit'] == receipt['seedSourceAudit'], 'compat_seed_late_encoder')
    for row in hydration:
        paths = list((seed / 'cache').glob('*/360p/' + row['asset']))
        row['postJoinPhysicalSHA256'] = sha(paths[0]) if len(paths) == 1 and paths[0].is_file() else None
        check(row['postJoinPhysicalSHA256'] == row['sha256'], 'compat_seed_response_not_physical_file')
    check(not certificates[0].with_name('.startup').exists(), 'legacy_seed_unadopted_startup_marker')
    timelines = list((seed / 'cache').glob('*/.copy-timeline'))
    check(len(timelines) == 1, 'compat_seed_timeline_count')
    timeline = json.loads(bounded_bytes(timelines[0], 256 << 10, 'compat_seed_timeline_bound'))
    check(len(timeline['Keys']) == 10 and timeline.get('AudioOrigin') is None
        and timeline.get('Presentation') is None and timeline.get('Clock') is not None,
        'compat_seed_version1_contract')
    rendition = timelines[0].parent / '360p'
    for name in ['init.mp4', *['segment-' + str(n).zfill(5) + '.m4s' for n in range(10)]]:
        check((rendition / name).is_file(), 'compat_seed_all_media_physical')
    receipt['seedSessions'] = owner.sessions
    owner = None
    for asset in ['master', 'rendition']:
        for method in ['GET', 'HEAD']:
            arm(asset + '-' + method.lower(), method, asset, candidate, baseline, seed / 'cache', selected)
            arm(asset + '-' + method.lower() + '-warm', method, asset, candidate, baseline, seed / 'cache', selected, warm=True)
    for asset in ['init', 'first', 'last']:
        for method in ['GET', 'HEAD', 'RANGE']:
            arm(asset + '-' + method.lower(), method, asset, candidate, baseline, seed / 'cache', selected)
    arm('all-media-journey', 'GET', 'journey', candidate, baseline, seed / 'cache', selected)
    for invalid in ['missing-source', 'wrong-source', 'missing-clock', 'missing-timeline', 'wrong-version',
                    'wrong-timeline', 'wrong-master', 'wrong-init', 'wrong-first']:
        arm(invalid, 'GET', 'master', candidate, baseline, seed / 'cache', selected, invalid)
    arm('missing-timeline-warm', 'GET', 'master', candidate, baseline, seed / 'cache', selected,
        'missing-timeline', warm=True)
    for label, query, expected in query_controls():
        arm(label, 'GET', 'rendition', candidate, baseline, seed / 'cache', selected,
            query=query, control_status=expected)
    if len(receipt['cases']) == 39 and all(v['result'] == 'observed' for v in receipt['cases']):
        receipt['completeAdoptedVersion1Acceptance'] = True
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
            receipt['incompleteSeedSessions'] = owner.sessions
            try:
                receipt['incompleteSeedSourceAudit'] = diagnostic_producer_rows(
                    owner.invocation_rows(), source, owner.owned_pids)
            except Exception as error:
                receipt['seedAuditFailureClass'] = type(error).__name__
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
            'hls_nonkey_browser_packet_clock.py', 'hls_followon_frames.py', 'hls_aac_v2_compat_public.py']
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
            'seedHydration': receipt.get('seedHydration'),
            'seedDecodedSourceFrames': receipt.get('seedDecodedSourceFrames'),
            'seedSourceAudit': receipt.get('seedSourceAudit'),
            'seedFinalSourceAudit': receipt.get('seedFinalSourceAudit'),
            'cases': [{k: v.get(k) for k in ['label', 'result', 'failureClass', 'baselineStatuses', 'statuses',
                'cacheUnchanged', 'sourceCalls', 'sourceUnchanged', 'olderClientPlaylistPlayable',
                'baselineRestoredExactBody', 'exactBaselineBodies', 'baselineDecodedFrames', 'candidateDecodedFrames',
                'warm', 'warmMissingBindingRejectedWithoutWrites', 'diagnostics']} for v in receipt['cases']],
            'completeAdoptedVersion1Acceptance': receipt['completeAdoptedVersion1Acceptance'],
            'olderClientAcceptance': False, 'releaseBlocker': receipt['releaseBlocker'],
            'productionAcceptance': False, 'nativeAcceptance': False}), flush=True)
    guard.__exit__()
raise SystemExit(0 if receipt['result'] == 'observed' else 1)
