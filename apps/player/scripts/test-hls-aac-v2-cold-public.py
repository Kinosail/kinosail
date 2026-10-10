#!/usr/bin/env python3
"""Actual interrupted unindexed cold reopen: prefix preservation, not lazy-cut certification."""
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import shutil
import subprocess
import sys
import time
from hls_aac_v2_public_http import ActualServer, idle
from hls_followon_public import bounded_bytes, check, encoder_count
from hls_remaining_nonkey_deadline import DiagnosticDeadline
from hls_timeline_fixture import fixture
from hls_timeline_http import sha, source_state
from hls_timeline_packets import manifest_facts
from hls_timeline_seek_diagnostics import first_frames

ROOT = Path(__file__).resolve().parents[3]
RUN = ROOT / '.verification/hls-aac-v2-cold' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
RUN.mkdir(parents=True)
receipt = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(),
    'tree': subprocess.check_output(['git', 'rev-parse', 'HEAD^{tree}'], text=True).strip(),
    'command': 'python3 apps/player/scripts/test-hls-aac-v2-cold-public.py',
    'environment': {'platform': platform.platform(), 'python': sys.version.split()[0],
        'codecPackage': 'jellyfin-ffmpeg8_8.1.2-5-noble_amd64.deb',
        'codecPackageSHA256': 'b4e72894ad26c809ed0104805f5415a97be75212b9fcf9d60b89ad25bb3d43e3'},
    'result': 'failed', 'cases': [], 'productionAcceptance': False, 'nativeAcceptance': False,
    'scope': 'Genuine incomplete P0 H264 MP4 and HEVC MKV; joined actual cold Server reopen and exact cached prefix'}
owner = None
guard = DiagnosticDeadline(600)
guard.__enter__()


def run(command, timeout=60):
    result = subprocess.run(command, capture_output=True, timeout=min(timeout, guard.check(15)))
    check(result.returncode == 0 and len(result.stdout) <= 2 << 20 and len(result.stderr) <= 2 << 20,
          'cold_command_failed_or_unbounded')
    return result.stdout


def wait_until(condition, failure, seconds):
    limit = time.monotonic() + min(seconds, guard.check(25))
    while time.monotonic() < limit:
        if condition():
            return
        time.sleep(0.05)
    raise RuntimeError(failure)


def snapshot(paths):
    return {path.name: {'inode': path.stat().st_ino, 'bytes': path.stat().st_size,
        'mtimeNs': path.stat().st_mtime_ns, 'sha256': hashlib.sha256(
            bounded_bytes(path, 2 << 20, 'cold_prefix_bytes_bound')).hexdigest()} for path in paths}


def paced_server(directory, source, binary):
    value = ActualServer(ROOT, directory, source, binary)
    adapter = Path(value.env['KINOSAIL_FFMPEG'])
    adapter.write_text('#!/usr/bin/python3\nimport json,os,sys\n'
        'args=sys.argv[1:]\npaced=False\n'
        'if "-hls_time" in args and args.count("-i")==1 and args[args.index("-i")+1]==os.environ["P0_ACTUAL_SOURCE"]:\n'
        '    args[args.index("-i"):args.index("-i")]=["-readrate","0.2"]\n    paced=True\n'
        'if "-hls_time" in args:\n'
        '    with open(os.environ["V2_ARGV_CAPTURE"],"a") as stream:\n'
        '        stream.write(json.dumps({"args":args,"parent":os.getppid(),"testOnlyReadrate":paced})+"\\n")\n'
        'os.execv(os.environ["V2_REAL_FFMPEG"],[os.environ["V2_REAL_FFMPEG"],*args])\n')
    value.env['P0_ACTUAL_SOURCE'] = str(source)
    return value, sha(adapter)


def journey(label, original, binary):
    global owner
    row = {'label': label, 'result': 'failed', 'testOnlyReadrate': 0.2,
        'boundary': 'Real source HLS argv is paced before exec to expose the existing 45-second abandonment watchdog'}
    receipt['cases'].append(row)
    directory = RUN / label
    media = directory / 'media'
    media.mkdir(parents=True)
    source = media / ('Fixture' + original.suffix)
    shutil.copy2(original, source)
    before = source_state(source)
    owner, row['pacingWrapperSHA256'] = paced_server(directory, source, binary)
    try:
        owner.start(authorize=True)
        item = next(v for v in owner.api.call('/api/v1/library')['items'] if v['title'] == 'Fixture')
        plan = owner.api.call('/api/v1/items/' + item['id'] + '/playback?videoCodecs=h264,hevc&audioCodecs=aac')
        check(plan['compatiblePlan']['mode'] == 'remux', 'cold_plan_not_remux')
        selected = plan['compatible']
        check(owner.api.http(selected)[0] == 200, 'cold_initial_master_failed')
        roots = list((directory / 'cache').glob(item['id'] + '-plan-*'))
        check(len(roots) == 1, 'cold_generation_count')
        root = roots[0]
        variants = list(root.glob('*p/index.m3u8'))
        check(len(variants) == 1, 'cold_rendition_count')
        physical = variants[0]
        def prefix_live():
            return physical.is_file() and b'segment-00000.m4s' in bounded_bytes(physical, 2 << 20, 'cold_manifest_bound')
        wait_until(prefix_live, 'cold_real_prefix_missing', 20)
        check(b'#EXT-X-ENDLIST' not in bounded_bytes(physical, 2 << 20, 'cold_manifest_bound') and encoder_count(owner.process, source) == 1,
              'cold_interruption_not_real_incomplete_producer')
        row['interruption'] = 'Public cold HLS abandoned; existing 45-second inactivity watchdog'
        wait_until(lambda: encoder_count(owner.process, source) == 0 and (root / '.seekable').is_file(),
            'cold_abandoned_worker_not_joined', 55)
        row['idleBeforeStop'] = idle(owner.api, owner.process, source)
        data = bounded_bytes(physical, 2 << 20, 'cold_manifest_bound')
        _, segments = manifest_facts(data)
        binding = bounded_bytes(root / '.source', 16 << 10, 'cold_binding_bound')
        check(0 < len(segments) < 16 and b'#EXT-X-ENDLIST' not in data
              and not (root / '.copy-timeline').exists() and not (root / '.copy-clock').exists()
              and not (root / '.startup').exists() and b':copied-aac=2' not in binding,
              'cold_prefix_is_indexed_prepared_or_complete')
        paths = [physical.parent / 'init.mp4', physical.parent / segments[0][0]]
        preserved = snapshot(paths)
        calls = len(owner.source_invocation_rows())
        check(calls == 1 and owner.source_invocation_rows()[0]['testOnlyReadrate'] is True,
              'cold_initial_source_invocation_not_one_paced_actual_producer')
        row.update(interruptedPrefixSegments=len(segments), prefixBefore=preserved, sourceCallsBefore=calls,
            unindexedIncompletePrecondition=True)
        owner.stop()
        check(snapshot(paths) == preserved, 'cold_stop_changed_prefix')
        owner.start()
        base = selected.removesuffix('index.m3u8') + physical.parent.name + '/'
        public = []
        for name in ['init.mp4', segments[0][0]]:
            status, body, _ = owner.api.http(base + name)
            check(status == 200 and 0 < len(body) <= 2 << 20, 'cold_reopened_prefix_route_failed')
            check(hashlib.sha256(body).hexdigest() == preserved[name]['sha256'], 'cold_reopened_prefix_bytes_changed')
            public.append(body)
        check(owner.api.http(selected)[0] == 200, 'cold_reopened_master_failed')
        status, variant, _ = owner.api.http(base + 'index.m3u8')
        check(status == 200, 'cold_reopened_rendition_failed')
        facts, _ = manifest_facts(variant)
        check(facts['segmentCount'] > len(segments), 'cold_legacy_cadence_projection_lost')
        prefix = directory / 'public-prefix.mp4'
        prefix.write_bytes(b''.join(public))
        reference = first_frames(source)
        check(reference['exitStatus'] == 0 and reference['frames'] == 3
              and first_frames(prefix) == reference, 'cold_reopened_first_three_source_frames_changed')
        row['idleAfterReopen'] = idle(owner.api, owner.process, source)
        row.update(prefixAfter=snapshot(paths), sourceCallsAfter=len(owner.source_invocation_rows()),
            sourceUnchanged=source_state(source) == before, firstThreeSourceFramesMatch=True)
        check(row['prefixAfter'] == preserved and row['sourceCallsAfter'] == calls and row['sourceUnchanged']
              and not (root / '.copy-timeline').exists() and not (root / '.copy-clock').exists(),
              'cold_reopen_replaced_or_encoded_prefix')
        row['result'] = 'observed'
    except Exception as error:
        row['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
    finally:
        try:
            owner.stop()
        except Exception:
            row['sessions'] = owner.sessions
            raise RuntimeError('cold_owned_join_failed') from None
        row['sessions'] = owner.sessions
        row['finalSourceUnchanged'] = source_state(source) == before
        if row['result'] == 'observed':
            row['finalSourceCalls'] = len(owner.source_invocation_rows())
            row['finalPrefix'] = snapshot(paths)
            if row['finalSourceCalls'] != calls or row['finalPrefix'] != preserved or not row['finalSourceUnchanged']:
                row.update(result='failed', failureClass='cold_late_prefix_or_source_mutation')
        owner = None


try:
    check(shutil.disk_usage(RUN).free >= 2 << 30, 'cold_hosted_disk_budget')
    regular, _ = fixture(RUN, 'regular', 48, ','.join(str(v) for v in range(0, 32, 2)), frames=768)
    mp4 = RUN / 'cold-h264.mp4'
    run(['ffmpeg', '-nostdin', '-v', 'error', '-i', str(regular),
        '-map', '0:v:0', '-map', '0:a:0', '-c', 'copy', str(mp4)], 45)
    check(sha(mp4) == '198a6808092cea2a5481afc24e79481e4966ae577cbb8c46c214737e2173a3ed', 'cold_fixed_mp4_identity')
    hevc = RUN / 'cold-hevc.mkv'
    run(['ffmpeg', '-nostdin', '-v', 'error', '-i', str(regular), '-c:v', 'libx265',
        '-threads', '2', '-x265-params', 'pools=2:frame-threads=2:keyint=48:min-keyint=48:scenecut=0',
        '-preset', 'ultrafast', '-crf', '35', '-c:a', 'copy', str(hevc)], 90)
    receipt['fixtures'] = {'h264MP4SHA256': sha(mp4), 'hevcMKVSHA256': sha(hevc)}
    binary = RUN / 'kinosail'
    run(['go', '-C', str(ROOT / 'apps/player'), 'build', '-p=1', '-o', str(binary), './cmd/kinosail'], 180)
    receipt['binarySHA256'] = sha(binary)
    journey('interrupted-cold-h264-mp4', mp4, binary)
    journey('interrupted-cold-hevc-mkv', hevc, binary)
    if len(receipt['cases']) == 2 and all(v['result'] == 'observed' for v in receipt['cases']):
        receipt['result'] = 'observed'
except Exception as error:
    receipt['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
finally:
    with guard.cleanup():
        if owner is not None:
            try:
                owner.stop()
            except Exception:
                receipt.update(result='failed', cleanupFailureClass='cold_owned_join_failed')
        names = ['hls_aac_v2_argument_retry.py', 'hls_aac_v2_source_identity.py', 'hls_aac_v2_live_observer.py',
            'hls_aac_v2_public_http.py', 'hls_aac_v2_public_evidence.py', 'hls_followon_public.py',
            'hls_remaining_nonkey_deadline.py', 'hls_remaining_process.py', 'hls_timeline_http.py',
            'hls_timeline_fixture.py', 'hls_timeline_packets.py', 'hls_timeline_seek_diagnostics.py',
            'hls_remaining_nonkey_evidence.py', 'hls_remaining_nonkey_boundary.py',
            'hls_nonkey_browser_public.py', 'hls_nonkey_browser_audio_boundary.py',
            'hls_nonkey_browser_packet_association.py', 'hls_nonkey_browser_packet_clock.py']
        files = [Path(__file__), *(Path(__file__).with_name(name) for name in names)]
        receipt['executedScriptSHA256'] = {str(p.relative_to(ROOT)): sha(p) for p in files}
        raw = json.dumps(receipt, separators=(',', ':'), allow_nan=False) + '\n'
        check(0 < len(raw.encode()) <= 32 << 20, 'cold_receipt_bound')
        target = RUN / 'receipt.json'
        target.write_text(raw)
        (RUN / 'SHA256SUMS').write_text(sha(target) + '  receipt.json\n' +
            ''.join(sha(p) + '  ' + str(p.relative_to(ROOT)) + '\n' for p in files))
        print(json.dumps({'revision': receipt['revision'], 'tree': receipt['tree'], 'result': receipt['result'],
            'failureClass': receipt.get('failureClass'), 'receiptSHA256': sha(target),
            'cases': [{k: v.get(k) for k in ['label', 'result', 'failureClass', 'interruptedPrefixSegments',
                'unindexedIncompletePrecondition', 'sourceCallsBefore', 'sourceCallsAfter',
                'firstThreeSourceFramesMatch', 'sourceUnchanged']} for v in receipt['cases']],
            'productionAcceptance': False, 'nativeAcceptance': False}), flush=True)
    guard.__exit__()
raise SystemExit(0 if receipt['result'] == 'observed' else 1)
