#!/usr/bin/env python3
"""Actual missing-binding HTTP arms; preparation ordering cannot be tested by the retained renderer alone."""
import hashlib
import json
import platform
import sys
from pathlib import Path
import shutil
import subprocess
import time
import urllib.error
import urllib.request
from hls_aac_v2_public_http import ActualServer, cached_media, idle
from hls_aac_v2_public_evidence import assert_fixed_source_grid, cache_state
from hls_followon_public import bounded_bytes, check, prepare_once
from hls_remaining_nonkey_deadline import DiagnosticDeadline
from hls_timeline_fixture import fixture
from hls_timeline_http import sha, source_state

ROOT = Path(__file__).resolve().parents[3]
RUN = ROOT / '.verification/hls-aac-v2-binding' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
RUN.mkdir(parents=True)
receipt = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(),
    'tree': subprocess.check_output(['git', 'rev-parse', 'HEAD^{tree}'], text=True).strip(),
    'result': 'failed', 'cases': [], 'productionAcceptance': False, 'nativeAcceptance': False,
    'scope': 'Independent warm/cold real HTTP missing-source binding arms on fixed qualified MP4',
    'command': 'python3 apps/player/scripts/test-hls-aac-v2-binding-public.py',
    'environment': {'platform': platform.platform(), 'python': sys.version.split()[0],
        'codecPackage': 'jellyfin-ffmpeg8_8.1.2-5-noble_amd64.deb',
        'codecPackageSHA256': 'b4e72894ad26c809ed0104805f5415a97be75212b9fcf9d60b89ad25bb3d43e3'}}
owner, source, before = None, None, None
guard = DiagnosticDeadline(600)
guard.__enter__()


def run(command, timeout=60):
    result = subprocess.run(command, capture_output=True, timeout=min(timeout, guard.check(15)))
    check(result.returncode == 0 and len(result.stdout) <= 2 << 20 and len(result.stderr) <= 2 << 20,
          'binding_command_failed_or_unbounded')
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


def request(api, path, method, byte_range):
    headers = {'Authorization': 'Bearer ' + api.token}
    if byte_range:
        headers['Range'] = byte_range
    req = urllib.request.Request(api.url + path, method=method, headers=headers)
    try:
        response = api.opener.open(req, timeout=min(40, guard.check(15)))
    except urllib.error.HTTPError as error:
        response = error
    with response:
        data = response.read((2 << 20) + 1)
        check(len(data) <= 2 << 20, 'binding_public_body_bound')
        return response.getcode(), data


def arm(warm, label, method, asset, byte_range, binary, seed_cache, seed_selected):
    global owner
    guard.check(25)
    row = {'label': ('warm-' if warm else 'cold-') + label, 'result': 'failed', 'method': method,
           'range': byte_range, 'sessions': []}
    receipt['cases'].append(row)
    directory = RUN / row['label']
    directory.mkdir()
    shutil.copytree(seed_cache, directory / 'cache')
    original = snapshot(directory / 'cache')
    owner = ActualServer(ROOT, directory, source, binary)
    try:
        owner.start(authorize=True)
        item = next(v for v in owner.api.call('/api/v1/library')['items'] if v['title'] == 'Fixture')
        plan = owner.api.call('/api/v1/items/' + item['id'] + '/playback?videoCodecs=h264&audioCodecs=aac')
        selected = plan['compatible'].replace('/index.m3u8', '-o12000/index.m3u8')
        check(selected == seed_selected and plan['compatiblePlan']['mode'] == 'remux', 'binding_clone_identity')
        check(owner.api.http(selected)[0] == 200, 'binding_valid_warm_control')
        idle(owner.api, owner.process, source)
        root, _, state = cache_state(directory / 'cache')
        check(len(state['segments']) == 10, 'binding_complete_control')
        target = selected if asset == 'master' else selected.removesuffix('index.m3u8') + '360p/' + asset
        good_status, good_body = request(owner.api, target, method, byte_range)
        check(good_status == (206 if byte_range else 200), 'binding_valid_route_control')
        idle(owner.api, owner.process, source)
        row['cloneUnchanged'] = snapshot(directory / 'cache') == original
        row['cloneSourceCalls'] = len(owner.source_invocation_rows())
        check(row['cloneUnchanged'] and row['cloneSourceCalls'] == 0, 'binding_clone_silently_regenerated')
        if not warm:
            owner.stop()
        backup = directory / 'binding-backup'
        (root / '.source').rename(backup)  # The backup survives any accidental generation deletion.
        sealed = snapshot(directory / 'cache')
        count = len(owner.source_invocation_rows())
        if not warm:
            owner.start()
        status, body = request(owner.api, target, method, byte_range)
        after = snapshot(directory / 'cache')
        after_count = len(owner.source_invocation_rows())
        row.update(status=status, bodyBytes=len(body), bodySHA256=hashlib.sha256(body).hexdigest(),
            sourceCallsBefore=count, sourceCallsAfter=after_count, cacheUnchanged=after == sealed,
            sealedCache=sealed, cacheAfter=after, sourceUnchanged=source_state(source) == before)
        idle(owner.api, owner.process, source)
        settled = snapshot(directory / 'cache')
        settled_count = len(owner.source_invocation_rows())
        row.update(quiescentCache=settled, quiescentCacheUnchanged=settled == sealed,
            quiescentSourceCalls=settled_count)
        check(status == 404 and after == sealed and after_count == count and row['sourceUnchanged']
              and settled == sealed and settled_count == count,
              'binding_http_admitted_or_mutated_missing_generation')
        owner.stop()
        joined = snapshot(directory / 'cache')
        joined_count = len(owner.source_invocation_rows())
        row.update(joinedCache=joined, joinedCacheUnchanged=joined == sealed, joinedSourceCalls=joined_count)
        check(joined == sealed and joined_count == count, 'binding_late_missing_generation_mutation')
        backup.rename(root / '.source')
        owner.start()
        restored_status, restored_body = request(owner.api, target, method, byte_range)
        check(restored_status == good_status and restored_body == good_body,
              'binding_restored_route_changed_exact_bytes')
        row.update(result='observed', restoredExactBody=True)
    except Exception as error:
        row['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
    finally:
        try:
            owner.stop()
        except Exception:
            row['cleanupFailureClass'] = 'binding_owned_join_failed'
            row['sessions'] = owner.sessions
            # Retain the owner and stop creating arms until final bounded cleanup joins it.
            raise RuntimeError('binding_owned_join_failed') from None
        row['sessions'] = owner.sessions
        row['finalSourceCalls'] = len(owner.source_invocation_rows())
        row['finalCache'] = snapshot(directory / 'cache')
        if row['result'] == 'observed' and (row['finalSourceCalls'] != 0 or row['finalCache'] != original):
            row.update(result='failed', failureClass='binding_late_restored_generation_mutation')
        owner = None


try:
    check(shutil.disk_usage(RUN).free >= 2 << 30, 'binding_hosted_disk_budget')
    regular, _ = fixture(RUN, 'regular', 48, ','.join(str(v) for v in range(0, 32, 2)), frames=768)
    directory = RUN / 'seed'
    media = directory / 'media'
    media.mkdir(parents=True)
    source = media / 'Fixture.mp4'
    run(['ffmpeg', '-nostdin', '-v', 'error', '-i', str(regular),
         '-map', '0:v:0', '-map', '0:a:0', '-c', 'copy', str(source)], 45)
    before = source_state(source)
    check(before['sha256'] == '198a6808092cea2a5481afc24e79481e4966ae577cbb8c46c214737e2173a3ed',
          'binding_fixed_source_identity')
    grid = assert_fixed_source_grid(json.loads(run(['ffprobe', '-v', 'error', '-select_streams', 'v:0',
        '-show_packets', '-show_streams', '-show_entries', 'packet=pts,dts,duration,flags:stream=time_base',
        '-of', 'json', str(source)], 30)))
    receipt['sourceVideoGrid'] = grid
    binary = RUN / 'kinosail'
    run(['go', '-C', str(ROOT / 'apps/player'), 'build', '-p=1', '-o', str(binary), './cmd/kinosail'], 180)
    owner = ActualServer(ROOT, directory, source, binary)
    owner.start(authorize=True)
    item = next(v for v in owner.api.call('/api/v1/library')['items'] if v['title'] == 'Fixture')
    plan = owner.api.call('/api/v1/items/' + item['id'] + '/playback?videoCodecs=h264&audioCodecs=aac')
    selected = plan['compatible'].replace('/index.m3u8', '-o12000/index.m3u8')
    prepared = {}
    prepare_once(owner.api, '/api/v1/items/' + item['id'] + '/playback-prepare',
        selected, owner.log_path, owner.process, source, prepared)
    check(prepared['preparationAttempt']['completionState'] == 'ready', 'binding_seed_not_ready')
    cached_media(owner, selected, RUN / 'seed-public', grid)
    owner.stop()
    receipt['seedSessions'] = owner.sessions
    owner = None
    cases = [('master-get', 'GET', 'master', None), ('master-head', 'HEAD', 'master', None),
        ('rendition-get', 'GET', 'index.m3u8', None), ('rendition-head', 'HEAD', 'index.m3u8', None),
        ('init-get', 'GET', 'init.mp4', None), ('init-head', 'HEAD', 'init.mp4', None),
        ('first-get', 'GET', 'segment-00000.m4s', None),
        ('init-range', 'GET', 'init.mp4', 'bytes=0-31'),
        ('first-range', 'GET', 'segment-00000.m4s', 'bytes=0-31')]
    for warm in [True, False]:
        for label, method, asset, byte_range in cases:
            arm(warm, label, method, asset, byte_range, binary, directory / 'cache', selected)
    if len(receipt['cases']) == 18 and all(row['result'] == 'observed' for row in receipt['cases']):
        receipt['result'] = 'observed'
except Exception as error:
    receipt['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
finally:
    with guard.cleanup():
        if owner is not None:
            try:
                owner.stop()
            except Exception:
                receipt.update(result='failed', cleanupFailureClass='binding_owned_join_failed')
        try:
            receipt['sourceUnchanged'] = source_state(source) == before if source is not None and before is not None else None
        except Exception:
            receipt['sourceUnchanged'] = False
        if receipt['sourceUnchanged'] is False:
            receipt.update(result='failed', failureClass='binding_source_changed')
        files = [Path(__file__), *(Path(__file__).with_name(name) for name in [
            'hls_aac_v2_argument_retry.py', 'hls_aac_v2_source_identity.py', 'hls_aac_v2_live_observer.py',
            'hls_aac_v2_public_http.py', 'hls_aac_v2_public_evidence.py', 'hls_timeline_fixture.py',
            'hls_timeline_http.py', 'hls_timeline_packets.py', 'hls_followon_public.py',
            'hls_remaining_process.py', 'hls_remaining_nonkey_deadline.py',
            'hls_remaining_nonkey_evidence.py', 'hls_remaining_nonkey_boundary.py',
            'hls_nonkey_browser_public.py', 'hls_nonkey_browser_audio_boundary.py',
            'hls_nonkey_browser_packet_association.py', 'hls_nonkey_browser_packet_clock.py'])]
        receipt['executedScriptSHA256'] = {str(path.relative_to(ROOT)): sha(path) for path in files}
        target = RUN / 'receipt.json'
        raw = json.dumps(receipt, separators=(',', ':'), allow_nan=False) + '\n'
        check(0 < len(raw.encode()) <= 32 << 20, 'binding_safe_receipt_bound')
        target.write_text(raw)
        (RUN / 'SHA256SUMS').write_text(sha(target) + '  receipt.json\n' +
            ''.join(sha(path) + '  ' + str(path.relative_to(ROOT)) + '\n' for path in files))
        print(json.dumps({'revision': receipt['revision'], 'tree': receipt['tree'], 'result': receipt['result'],
            'failureClass': receipt.get('failureClass'), 'sourceUnchanged': receipt.get('sourceUnchanged'),
            'cases': [{key: row.get(key) for key in ['label', 'result', 'failureClass', 'status',
                'sourceCallsBefore', 'sourceCallsAfter', 'cacheUnchanged', 'restoredExactBody']} for row in receipt['cases']],
            'receiptSHA256': sha(target), 'productionAcceptance': False, 'nativeAcceptance': False}), flush=True)
    guard.__exit__()
raise SystemExit(0 if receipt['result'] == 'observed' else 1)
