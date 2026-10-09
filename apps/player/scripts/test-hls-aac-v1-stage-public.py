#!/usr/bin/env python3
"""Three private V1 stage arms against an actual complete baseline cache."""
import hashlib
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import sys
import time
from hls_aac_v1_stage_public import prove, qualify_clock, settle
from hls_aac_v2_compat_public import snapshot
from hls_aac_v2_lazy_public import indexed, seed
from hls_followon_public import bounded_bytes, check
from hls_remaining_nonkey_deadline import DiagnosticDeadline
from hls_timeline_fixture import fixture
from hls_timeline_http import sha, source_state

ROOT = Path(__file__).resolve().parents[3]
RUN = ROOT / '.verification/hls-aac-v1-stage' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
RUN.mkdir(parents=True)
receipt = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(),
    'tree': subprocess.check_output(['git', 'rev-parse', 'HEAD^{tree}'], text=True).strip(),
    'baselineRevision': '664f4e2ad40049b03a5da62dba444adc3caa97ec',
    'productionParent': '4c9153393b7811f6d253acf85d3134e8fb5916a3',
    'command': 'python3 apps/player/scripts/test-hls-aac-v1-stage-public.py',
    'environment': {'platform': platform.platform(), 'python': sys.version.split()[0],
        'codecPackage': 'jellyfin-ffmpeg8_8.1.2-5-noble_amd64.deb',
        'codecPackageSHA256': 'b4e72894ad26c809ed0104805f5415a97be75212b9fcf9d60b89ad25bb3d43e3'},
    'scope': 'Private CLI old-profile cuts0/4/9 with original complete V1 init; no canonical publication or candidate Server',
    'result': 'failed', 'cases': [], 'stageFeasibilityAcceptance': False,
    'productionAcceptance': False, 'nativeAcceptance': False,
    'remainingBoundaries': ['Server worker and GET hydration', 'observer FD accounting',
        'source/root/owner replacement and P2 migration races', 'historical Version1 AAC failures',
        'deleted-zero baseline remains unqualified', 'all50 and fourteen scanner holds']}
owners, cli_owners, source, before, complete, seal, retained_source = [], [], None, None, None, None, None
guard = DiagnosticDeadline(900)
guard.__enter__()


def run(command, timeout=60):
    result = subprocess.run(command, capture_output=True, timeout=min(timeout, guard.check(15)))
    check(result.returncode == 0 and len(result.stdout) <= 2 << 20 and len(result.stderr) <= 2 << 20,
        'v1_stage_setup_command_failed_or_unbounded')
    return result.stdout


try:
    check(sys.platform == 'linux' and hasattr(os, 'WNOWAIT'), 'v1_stage_linux_waitid_required')
    check(shutil.disk_usage(RUN).free >= 2 << 30, 'v1_stage_hosted_disk_budget')
    regular, _ = fixture(RUN, 'regular', 48, ','.join(str(v) for v in range(0, 32, 2)), frames=768)
    media = RUN / 'media'
    media.mkdir()
    source = media / 'Fixture.mp4'
    run(['ffmpeg', '-nostdin', '-v', 'error', '-i', str(regular), '-map', '0:v:0', '-map', '0:a:0',
        '-c', 'copy', str(source)], 45)
    before = source_state(source)
    check(before['sha256'] == '198a6808092cea2a5481afc24e79481e4966ae577cbb8c46c214737e2173a3ed',
        'v1_stage_fixed_source_identity')
    old = RUN / 'baseline-source'
    run(['git', 'worktree', 'add', '--detach', str(old), receipt['baselineRevision']], 30)
    baseline = RUN / 'baseline-kinosail'
    run(['go', '-C', str(old / 'apps/player'), 'build', '-p=1', '-o', str(baseline), './cmd/kinosail'], 180)
    (RUN / 'empty-cache').mkdir()
    _, _, _, complete = seed(ROOT, RUN, baseline, source, owners, receipt.setdefault('seed', {}))
    directory = indexed(complete)
    timeline = json.loads(bounded_bytes(directory / '.copy-timeline', 256 << 10, 'v1_stage_timeline_bound'))
    certificate = json.loads(bounded_bytes(directory / '.copy-clock', 4096, 'v1_stage_clock_bound'))
    original_init = bounded_bytes(directory / '360p/init.mp4', 2 << 20, 'v1_stage_init_bound')
    original_zero = bounded_bytes(directory / '360p/segment-00000.m4s', 64 << 20, 'v1_stage_zero_bound')
    # The certificate encodes fixed-length hash arrays. Require both independent bindings.
    check(list(hashlib.sha256(original_init).digest()) == certificate['initialization']
        and list(hashlib.sha256(original_zero).digest()) == certificate['first'],
        'v1_stage_complete_baseline_certificate')
    seal = snapshot(complete)
    receipt['completeCacheSealBeforeSHA256'] = hashlib.sha256(json.dumps(seal, sort_keys=True).encode()).hexdigest()
    rows = owners[-1].source_invocation_rows()
    qualify_clock(timeline, rows, source, directory / '360p', RUN, receipt)
    retained_source = source.open('rb')
    prove(shutil.which('ffmpeg'), source, directory / '360p', rows,
        timeline, RUN, guard, receipt, cli_owners, retained_source)
    if len(receipt['cases']) == 3 and all(row['result'] == 'observed' for row in receipt['cases']):
        receipt.update(result='observed', stageFeasibilityAcceptance=True)
except Exception as error:
    receipt['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
finally:
    with guard.cleanup():
        for process, row in list(cli_owners):
            try:
                settle(process, row)
                cli_owners.remove((process, row))
            except Exception:
                row['cleanupFailureClass'] = 'v1_stage_final_cli_join_failed'
                receipt.update(result='failed', cleanupFailureClass='v1_stage_final_cli_join_failed')
        receipt['unresolvedCLIOwners'] = len(cli_owners)
        if retained_source is not None and not cli_owners:
            retained_source.close()
        receipt['retainedSourceClosedAfterJoinedCLI'] = retained_source.closed if retained_source is not None else None
        receipt['ownedServerEvidence'] = []
        for owner in owners:
            try:
                owner.stop()
                receipt['ownedServerEvidence'].append({'label': owner.directory.name, 'sessions': owner.sessions})
            except Exception:
                receipt.update(result='failed', cleanupFailureClass='v1_stage_owned_join_failed')
        try:
            receipt['sourceUnchanged'] = source_state(source) == before if source is not None and before else None
            if complete is not None and seal is not None:
                after = snapshot(complete)
                receipt['completeCacheUnchanged'] = seal == after
                receipt['completeCacheSealAfterSHA256'] = hashlib.sha256(json.dumps(after, sort_keys=True).encode()).hexdigest()
                if not receipt['completeCacheUnchanged']:
                    receipt.update(result='failed', cacheGuardFailureClass='v1_stage_complete_cache_changed')
            if receipt['sourceUnchanged'] is False:
                receipt.update(result='failed', sourceGuardFailureClass='v1_stage_source_changed')
        except Exception:
            receipt.update(result='failed', sourceGuardFailureClass='v1_stage_source_guard_failed')
        if receipt['result'] != 'observed':
            receipt['stageFeasibilityAcceptance'] = False
        files = [Path(__file__), *[Path(__file__).with_name(name) for name in [
            'hls_aac_v1_stage_public.py', 'hls_aac_v2_lazy_public.py', 'hls_aac_v2_compat_public.py',
            'hls_aac_v2_public_http.py', 'hls_aac_v2_public_evidence.py', 'hls_followon_public.py',
            'hls_remaining_nonkey_deadline.py', 'hls_remaining_process.py', 'hls_timeline_http.py',
            'hls_timeline_fixture.py', 'hls_timeline_packets.py', 'hls_remaining_nonkey_evidence.py',
            'hls_remaining_nonkey_init.py', 'hls_remaining_nonkey_fragment.py', 'hls_followon_frames.py',
            'hls_remaining_nonkey_boundary.py', 'hls_nonkey_browser_audio_boundary.py',
            'hls_nonkey_browser_packet_association.py', 'hls_nonkey_browser_packet_clock.py']]]
        receipt['executedScriptSHA256'] = {str(path.relative_to(ROOT)): sha(path) for path in files}
        raw = json.dumps(receipt, separators=(',', ':'), allow_nan=False) + '\n'
        check(0 < len(raw.encode()) <= 32 << 20, 'v1_stage_receipt_bound')
        target = RUN / 'receipt.json'
        target.write_text(raw)
        (RUN / 'SHA256SUMS').write_text(sha(target) + '  receipt.json\n' +
            ''.join(sha(path) + '  ' + str(path.relative_to(ROOT)) + '\n' for path in files))
        print(json.dumps({'revision': receipt['revision'], 'tree': receipt['tree'],
            'result': receipt['result'], 'receiptSHA256': sha(target), 'failureClass': receipt.get('failureClass'),
            'cases': receipt['cases'], 'baselineClockWitness': receipt.get('baselineClockWitness'),
            'completeCacheUnchanged': receipt.get('completeCacheUnchanged'),
            'sourceUnchanged': receipt.get('sourceUnchanged'), 'ownedServerEvidence': receipt['ownedServerEvidence'],
            'cleanupFailureClass': receipt.get('cleanupFailureClass'),
            'unresolvedCLIOwners': receipt['unresolvedCLIOwners'],
            'retainedSourceClosedAfterJoinedCLI': receipt['retainedSourceClosedAfterJoinedCLI'],
            'sourceGuardFailureClass': receipt.get('sourceGuardFailureClass'),
            'cacheGuardFailureClass': receipt.get('cacheGuardFailureClass'),
            'stageFeasibilityAcceptance': receipt['stageFeasibilityAcceptance'],
            'productionAcceptance': False, 'nativeAcceptance': False}), flush=True)
    guard.__exit__()
raise SystemExit(0 if receipt['result'] == 'observed' else 1)
