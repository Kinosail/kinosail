#!/usr/bin/env python3
"""Test-first actual adopted-lazy and speculative Version1 client GET journeys."""
import hashlib
import json
from pathlib import Path
import platform
import shutil
import subprocess
import sys
import time
from hls_aac_v2_lazy_public import indexed, journey, seed
from hls_aac_v2_public_http import diagnostic_producer_rows
from hls_followon_public import check
from hls_remaining_nonkey_deadline import DiagnosticDeadline
from hls_timeline_fixture import fixture
from hls_timeline_http import sha, source_state

ROOT = Path(__file__).resolve().parents[3]
RUN = ROOT / '.verification/hls-aac-v2-lazy' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
RUN.mkdir(parents=True)
receipt = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(),
    'tree': subprocess.check_output(['git', 'rev-parse', 'HEAD^{tree}'], text=True).strip(),
    'baselineRevision': '664f4e2ad40049b03a5da62dba444adc3caa97ec',
    'command': 'python3 apps/player/scripts/test-hls-aac-v2-lazy-public.py',
    'environment': {'platform': platform.platform(), 'python': sys.version.split()[0],
        'codecPackage': 'jellyfin-ffmpeg8_8.1.2-5-noble_amd64.deb',
        'codecPackageSHA256': 'b4e72894ad26c809ed0104805f5415a97be75212b9fcf9d60b89ad25bb3d43e3'},
    'scope': 'Real old-client HEAD and GET of genuine adopted lazy, speculative and missing0/4/9 Version1 caches; no candidate POST',
    'result': 'failed', 'cases': [], 'lazyVersion1Acceptance': False,
    'olderClientAcceptance': False, 'productionAcceptance': False, 'nativeAcceptance': False,
    'remainingBoundaries': ['concurrent lifecycle and generation publication races',
        'actual native client and all50', 'historical Version1 AAC failures', 'fourteen scanner holds', 'absent-media HEAD and Range',
        'pending init-mismatch and preflight-to-preparation races']}
owners, source, before = [], None, None
guard = DiagnosticDeadline(900)
guard.__enter__()


def run(command, timeout=60):
    result = subprocess.run(command, capture_output=True, timeout=min(timeout, guard.check(15)))
    check(result.returncode == 0 and len(result.stdout) <= 2 << 20 and len(result.stderr) <= 2 << 20,
        'lazy_command_failed_or_unbounded')
    return result.stdout


try:
    check(shutil.disk_usage(RUN).free >= 2 << 30, 'lazy_hosted_disk_budget')
    regular, _ = fixture(RUN, 'regular', 48, ','.join(str(v) for v in range(0, 32, 2)), frames=768)
    media = RUN / 'media'
    media.mkdir()
    source = media / 'Fixture.mp4'
    run(['ffmpeg', '-nostdin', '-v', 'error', '-i', str(regular), '-map', '0:v:0', '-map', '0:a:0',
        '-c', 'copy', str(source)], 45)
    before = source_state(source)
    check(before['sha256'] == '198a6808092cea2a5481afc24e79481e4966ae577cbb8c46c214737e2173a3ed',
        'lazy_fixed_source_identity')
    old = RUN / 'baseline-source'
    run(['git', 'worktree', 'add', '--detach', str(old), receipt['baselineRevision']], 30)
    baseline, candidate = RUN / 'baseline-kinosail', RUN / 'candidate-kinosail'
    run(['go', '-C', str(old / 'apps/player'), 'build', '-p=1', '-o', str(baseline), './cmd/kinosail'], 180)
    run(['go', '-C', str(ROOT / 'apps/player'), 'build', '-p=1', '-o', str(candidate), './cmd/kinosail'], 180)
    (RUN / 'empty-cache').mkdir()
    selected, speculative, adopted, complete = seed(ROOT, RUN, baseline, source, owners,
        receipt.setdefault('seed', {}))
    cases = [('adopted-lazy', adopted), ('speculative-lazy', speculative)]
    for number in [0, 4, 9]:
        cache = RUN / ('missing-' + str(number) + '-cache')
        shutil.copytree(complete, cache)
        indexed(cache).joinpath('360p/segment-' + str(number).zfill(5) + '.m4s').unlink()
        cases.append(('missing-' + str(number), cache))
    for label, cache in cases:
        row = {'label': label, 'result': 'failed', 'candidatePreparePOST': False,
            'baseline': {'result': 'failed'}, 'candidate': {'result': 'failed'}}
        receipt['cases'].append(row)
        try:
            control, replies = journey(ROOT, RUN, label + '-baseline', baseline, source, cache,
                selected, owners, before, False, row['baseline'])
            check(control['result'] == 'observed', 'lazy_baseline_control_not_qualified')
            actual, received = journey(ROOT, RUN, label + '-candidate', candidate, source, cache,
                selected, owners, before, True, row['candidate'])
            row['exactBaselinePlaylistBodies'] = all(a[:2] == b[:2]
                for a, b in zip(received[:2], replies[:2]))
            check(actual['result'] == 'observed' and row['exactBaselinePlaylistBodies'],
                'lazy_candidate_existing_client_regression')
            row['result'] = 'observed'
        except Exception as error:
            row['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
            for owner in owners:
                owner.stop()
    if len(receipt['cases']) == 5 and all(v['result'] == 'observed' for v in receipt['cases']):
        receipt.update(result='observed', lazyVersion1Acceptance=True)
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
                receipt.setdefault('cleanupFailureClasses', []).append('lazy_owned_join_failed')
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
            receipt.update(result='failed', sourceGuardFailureClass='lazy_source_changed')
        names = ['hls_aac_v2_lazy_public.py', 'hls_aac_v2_compat_public.py',
            'hls_aac_v2_public_http.py', 'hls_aac_v2_public_evidence.py', 'hls_followon_public.py',
            'hls_remaining_nonkey_deadline.py', 'hls_remaining_process.py', 'hls_timeline_http.py',
            'hls_timeline_fixture.py', 'hls_timeline_packets.py', 'hls_remaining_nonkey_evidence.py',
            'hls_remaining_nonkey_boundary.py', 'hls_nonkey_browser_public.py',
            'hls_nonkey_browser_audio_boundary.py', 'hls_nonkey_browser_packet_association.py',
            'hls_nonkey_browser_packet_clock.py', 'hls_followon_frames.py']
        files = [Path(__file__), *(Path(__file__).with_name(name) for name in names)]
        receipt['executedScriptSHA256'] = {str(p.relative_to(ROOT)): sha(p) for p in files}
        raw = json.dumps(receipt, separators=(',', ':'), allow_nan=False) + '\n'
        check(0 < len(raw.encode()) <= 32 << 20, 'lazy_receipt_bound')
        target = RUN / 'receipt.json'
        target.write_text(raw)
        (RUN / 'SHA256SUMS').write_text(sha(target) + '  receipt.json\n' +
            ''.join(sha(p) + '  ' + str(p.relative_to(ROOT)) + '\n' for p in files))
        print(json.dumps({'revision': receipt['revision'], 'tree': receipt['tree'], 'result': receipt['result'],
            'failureClass': receipt.get('failureClass'), 'receiptSHA256': sha(target),
            'seed': receipt.get('seed'), 'cases': receipt['cases'],
            'ownedServerEvidence': receipt['ownedServerEvidence'],
            'sourceGuardFailureClass': receipt.get('sourceGuardFailureClass'),
            'cleanupFailureClasses': receipt.get('cleanupFailureClasses'),
            'lazyVersion1Acceptance': receipt['lazyVersion1Acceptance'], 'olderClientAcceptance': False,
            'productionAcceptance': False, 'nativeAcceptance': False}), flush=True)
    guard.__exit__()
raise SystemExit(0 if receipt['result'] == 'observed' else 1)
