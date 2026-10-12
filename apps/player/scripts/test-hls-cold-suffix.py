#!/usr/bin/env python3
"""Public cold-resume indexing with real media and bounded probe failures."""
import argparse
from concurrent.futures import ThreadPoolExecutor
import json
import os
import platform
from pathlib import Path
import shutil
import socket
import subprocess
import sys
import time

from hls_followon_public import check
from hls_cold_suffix_faults import wrappers
from hls_timeline_fixture import LONG_RESUME_KEYS, fixture
from hls_timeline_packets import fragment_facts
from hls_timeline_http import PublicServer, sha, source_state

ROOT = Path(__file__).resolve().parents[3]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--case', choices=['all', 'success', 'clock-race', 'fractional-recovery', 'lookback', 'late-failure', 'timeout', 'early-bound', 'non-key', 'source-change', 'idr-failure', 'configuration-failure', 'aggregate-deadline', 'producer-failure', 'packet-limit', 'byte-limit', 'key-limit', 'unsupported-duration', 'endpoint-tolerance'], default='all')
args = parser.parse_args()
run = ROOT / '.verification/hls-cold-suffix' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
run.mkdir(parents=True)
receipt = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(), 'result': 'failed', 'cases': [],
    'command': f'python3 apps/player/scripts/test-hls-cold-suffix.py --case {args.case}',
    'environment': platform.platform(),
    'boundary': 'Authenticated real Server and synthetic H264 media; test-owned executable wrappers inject probe faults. Browser and device proof are separate.'}
patch = run / 'tested-source.patch'
patch.write_bytes(subprocess.check_output(['git', 'diff', 'HEAD', '--']))
receipt['patchSHA256'] = sha(patch)
receipt['helpersSHA256'] = {name: sha(Path(__file__).with_name(name)) for name in ['test-hls-cold-suffix.py', 'hls_cold_suffix_faults.py', 'hls_followon_public.py', 'hls_timeline_http.py', 'hls_timeline_fixture.py', 'hls_timeline_packets.py']}
binary = run / 'player'


def count(directory, name):
    path = directory / name
    return len(path.read_text().splitlines()) if path.exists() else 0


def settled(directory, name='wrapper-pids'):
    path = directory / name
    for pid in path.read_text().splitlines() if path.exists() else []:
        try:
            os.kill(int(pid), 0)
            return False
        except ProcessLookupError:
            pass
    return True


def wait_for(condition, seconds, failure):
    limit = time.monotonic() + seconds
    while time.monotonic() < limit:
        if condition():
            return
        time.sleep(.025)
    raise RuntimeError(failure)


def exercise(source_fixture, source_facts, fault):
    directory = run / fault
    media = directory / 'media'
    media.mkdir(parents=True)
    source = media / 'Fixture.mp4'
    shutil.copyfile(source_fixture, source)
    variant = None
    if fault in ['fractional-recovery', 'lookback']:
        source.unlink()
        frames = 732 if fault == 'fractional-recovery' else 2280
        source, variant = fixture(media, 'Fixture', 48, ','.join(str(n) for n in range(0, 32 if frames == 732 else 95, 2)), frames=frames, extension='.mp4')
    expected_origin = 64.0 if fault == 'lookback' else 0.0 if fault == 'endpoint-tolerance' else 12.0
    fragments = sum(point >= expected_origin for point in (variant or source_facts)['keyframesSeconds'])
    before = source_state(source)
    wrappers(directory, source, fault)
    case = {'fault': fault, 'result': 'failed', 'responses': [], 'sourceBefore': before}
    receipt['cases'].append(case)
    if variant:
        case['fixture'] = variant
    successful = fault in ['success', 'clock-race', 'fractional-recovery', 'lookback', 'endpoint-tolerance']
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        port = listener.getsockname()[1]
    origin = f'http://localhost:{port}'
    env = dict(os.environ, KINOSAIL_LISTEN=f'127.0.0.1:{port}', KINOSAIL_AUTH_URL=origin, KINOSAIL_TLS_ENABLED='false',
        KINOSAIL_FFMPEG=str(directory / 'ffmpeg-wrapper.py'), KINOSAIL_FFPROBE=str(directory / 'ffprobe-wrapper.py'),
        KINOSAIL_DATA_DIR=str(directory / 'config'), KINOSAIL_MEDIA_DIR=str(media), KINOSAIL_CACHE_DIR=str(directory / 'cache'),
        KINOSAIL_BACKUP_DIR=str(directory / 'backups'), KINOSAIL_BACKUP_KEY='synthetic-cold-suffix-key')
    api = PublicServer(origin)
    session = 'qa-cold-' + fault
    api.opener.addheaders = [('X-Playback-Session', session)]
    with (directory / 'server-private.log').open('w') as log:
        server = subprocess.Popen([str(binary)], env=env, stdout=log, stderr=log)
        try:
            api.authorize()
            item = next(v for v in api.call('/api/v1/library')['items'] if v['title'] == 'Fixture')
            plan = api.call('/api/v1/items/' + item['id'] + '/playback?videoCodecs=h264&audioCodecs=aac')
            check(plan['compatiblePlan']['mode'] == 'remux', 'fixture_mode')
            offset = 65100 if fault == 'lookback' else 500 if fault == 'endpoint-tolerance' else 12500
            path = plan['compatible'].replace('/index.m3u8', f'-o{offset}/index.m3u8')
            started = time.monotonic()
            with ThreadPoolExecutor(max_workers=2) as pool:
                if fault == 'clock-race':
                    pending = [pool.submit(api.http, path)]
                    wait_for(lambda: count(directory, 'clock-starts') == 1, 5, 'clock_probe_not_started')
                    wait_for(lambda: settled(directory, 'encoder-pids'), 1, 'encoder_not_settled_before_second_request')
                    pending.append(pool.submit(api.http, path))
                    time.sleep(.3)
                    case['encodersWhileClockHeld'] = count(directory, 'encoder-starts')
                    (directory / 'clock-release').write_text('release')
                else:
                    pending = [pool.submit(api.http, path) for _ in range(2 if successful else 1)]
                for request in pending:
                    status, data, headers = request.result(timeout=40)
                    normalized = {k.lower(): v for k, v in headers.items()}
                    case['responses'].append({'status': status, 'requestID': normalized.get('x-request-id'),
                        'origin': normalized.get('x-kinosail-hls-origin'), 'playlistPublished': data.startswith(b'#EXTM3U')})
            case['seconds'] = round(time.monotonic() - started, 3)
            if fault == 'producer-failure':
                wait_for(lambda: (directory / 'producer-failed').exists() and settled(directory, 'encoder-pids'), 15, 'producer_did_not_fail')
                status, data, headers = api.http(path)
                case['retryStatus'] = status
                check(status == 200 and data.startswith(b'#EXTM3U'), 'failed_prefix_not_recovered')
                check(count(directory, 'encoder-starts') == 2 and count(directory, 'suffix-starts') == 2, 'failed_prefix_not_replaced')
                case['result'] = 'passed'
                return
            if not successful:
                check(all(r['status'] == 503 and not r['playlistPublished'] for r in case['responses']), 'unsafe_suffix_published')
                check(count(directory, 'encoder-starts') == 0, 'rejected_index_started_encoder')
                check(not list((directory / 'cache').rglob('.copy-timeline')), 'rejected_index_bound')
                if fault == 'unsupported-duration':
                    check(count(directory, 'suffix-starts') == 0, 'invalid_duration_started_suffix_probe')
                if fault in ['timeout', 'aggregate-deadline']:
                    check(7 <= case['seconds'] <= 12, 'probe_deadline_not_bounded')
                if fault in ['early-bound', 'packet-limit', 'byte-limit', 'key-limit']:
                    check(case['seconds'] < 4, 'early_bound_waited_for_eof')
            else:
                check(all(r['status'] == 200 and r['origin'] == f'{expected_origin:.6f}' for r in case['responses']), 'cold_origin')
                check(count(directory, 'suffix-starts') == 1, 'concurrent_index_not_shared')
                check(count(directory, 'encoder-starts') == 1, 'concurrent_encoder_not_shared')
                timeline = next((directory / 'cache').rglob('.copy-timeline'))
                cache = timeline.parent
                rendition = next(cache.glob('*p/index.m3u8')).parent
                base = path.removesuffix('index.m3u8') + rendition.name + '/'
                for number in range(fragments):
                    check(api.http(base + f'segment-{number:05d}.m4s')[0] == 200, 'initial_fragment_status')
                wait_for(lambda: settled(directory), 5, 'index_probe_not_settled')
                metadata = [cache / name for name in ['.copy-timeline', '.copy-clock', '.source', 'index.m3u8']]
                metadata += [rendition / name for name in ['init.mp4', 'index.m3u8']]
                snapshots = [sha(p) for p in metadata]
                case['recoveries'] = []
                for number in sorted({0, fragments // 2, fragments - 1}):
                    fragment = rendition / f'segment-{number:05d}.m4s'
                    digest = sha(fragment)
                    previous = fragment_facts(rendition, fragment)
                    preceding = fragment_facts(rendition, rendition / f'segment-{number-1:05d}.m4s') if number else None
                    fragment.unlink()
                    status, data, _ = api.http(base + fragment.name)
                    check(status == 200 and fragment.exists(), 'fragment_not_restored')
                    current = fragment_facts(rendition, fragment)
                    check(current['videoHashes'] == previous['videoHashes'] and current['videoPTS'] == previous['videoPTS'], 'fragment_video_changed')
                    if number == fragments - 1:
                        check(abs(current['audioEnd'] - current['videoEnd']) <= .05, 'fragment_audio_end')
                    if preceding:
                        check(abs(current['audioStart'] - preceding['audioEnd']) <= .05, 'fragment_audio_gap')
                    if number == 0:
                        check(sha(fragment) == digest, 'first_fragment_changed')
                    check([sha(p) for p in metadata] == snapshots, 'certificate_or_initialization_changed')
                    check(count(directory, 'suffix-starts') == 1, 'refill_rescanned_index')
                    case['recoveries'].append({'segment': number, 'status': status, 'binaryIdentical': sha(fragment) == digest, 'videoFramesAndTimestampsIdentical': True, 'audioGapSeconds': current['audioStart'] - preceding['audioEnd'] if preceding else None, 'metadataUnchanged': True})
                if fault == 'success':
                    server.terminate()
                    server.wait(timeout=10)
                    server = subprocess.Popen([str(binary)], env=env, stdout=log, stderr=log)
                    deadline = time.monotonic() + 15
                    while time.monotonic() < deadline:
                        try:
                            if api.http('/healthz', authenticated=False)[0] == 200:
                                break
                        except OSError:
                            pass
                        time.sleep(.05)
                    status, _, headers = api.http(path)
                    normalized = {k.lower(): v for k, v in headers.items()}
                    check(status == 200 and normalized.get('x-kinosail-hls-origin') == f'{expected_origin:.6f}', 'warm_origin_changed')
                    check([sha(p) for p in metadata] == snapshots and count(directory, 'suffix-starts') == 1, 'warm_certificate_changed')
                    case['warmRestart'] = 'passed'
                    case['damagedMetadata'] = []
                    for damage in ['corrupt', 'missing']:
                        before_encoders = count(directory, 'encoder-starts')
                        before_probes = count(directory, 'suffix-starts')
                        if damage == 'corrupt':
                            timeline.write_text('{}')
                        else:
                            timeline.unlink()
                        status, data, headers = api.http(path)
                        normalized = {k.lower(): v for k, v in headers.items()}
                        case['responses'].append({'status': status, 'requestID': normalized.get('x-request-id'), 'playlistPublished': data.startswith(b'#EXTM3U')})
                        if status == 200:
                            wait_for(lambda: settled(directory, 'encoder-pids'), 5, 'replacement_encoder_not_settled')
                            check(normalized.get('x-kinosail-hls-origin') == f'{expected_origin:.6f}' and normalized.get('x-kinosail-hls-clock'), 'damaged_certificate_served_without_clock')
                            check([sha(p) for p in metadata] == snapshots, 'replacement_certificate_changed')
                            check(count(directory, 'encoder-starts') == before_encoders + 1 and count(directory, 'suffix-starts') == before_probes + 1, 'damaged_certificate_not_regenerated')
                            outcome = 'verified-regeneration'
                        else:
                            check(status == 503 and not data.startswith(b'#EXTM3U'), 'damaged_certificate_published')
                            check(count(directory, 'encoder-starts') == before_encoders, 'rejected_certificate_started_encoder')
                            outcome = 'rejected'
                        case['damagedMetadata'].append({'damage': damage, 'status': status, 'outcome': outcome})
            case['result'] = 'passed'
        except Exception as error:
            case['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
        finally:
            (directory / 'clock-release').write_text('release')
            server.terminate()
            try:
                server.wait(timeout=10)
            except subprocess.TimeoutExpired:
                server.kill()
                server.wait(timeout=5)
            case['serverSettled'] = server.poll() is not None
            case['ownedProbesSettled'] = settled(directory)
            case['suffixProbes'] = count(directory, 'suffix-starts')
            case['encoderStarts'] = count(directory, 'encoder-starts')
            case['sourceAfter'] = source_state(source)
            case['sourceUnchanged'] = case['sourceAfter'] == before
            log.flush()
            private = (directory / 'server-private.log').read_text()
            check(len(private) <= 2 * 1024 * 1024, 'private_log_bound')
            entries = [json.loads(line) for line in private.splitlines() if line.startswith('{')]
            case['safeFailureLogs'] = [{k: row.get(k) for k in ['level', 'request_id', 'playback_session', 'encoder_phase']}
                for row in entries if row.get('msg') == 'HLS playlist preparation failed']
            case['failureLogsCorrelated'] = all(any(row['level'] == 'ERROR' and row['request_id'] == response['requestID']
                and row['playback_session'] == session for row in case['safeFailureLogs']) for response in case['responses'] if response['status'] == 503)
            case['noCredentialsLogged'] = bool(api.token) and api.token not in private
            case['artifactsSHA256'] = {p.name: sha(p) for p in directory.iterdir() if p.is_file()}
            if not (case['serverSettled'] and case['ownedProbesSettled'] and case['noCredentialsLogged'] and case['failureLogsCorrelated']
                    and (case['sourceUnchanged'] or fault == 'source-change')):
                case['result'] = 'failed'


try:
    source, facts = fixture(run, 'Source', 144, ','.join(map(str, LONG_RESUME_KEYS)), frames=1788, extension='.mp4')
    receipt['fixture'] = facts
    build = ['go', 'build', '-p', '2', '-o', str(binary), './apps/player/cmd/kinosail']
    receipt['build'] = build
    with (run / 'build-private.log').open('w') as log:
        subprocess.run(build, check=True, stdout=log, stderr=log, timeout=180)
    receipt['binarySHA256'] = sha(binary)
    for fault in ['success', 'clock-race', 'fractional-recovery', 'lookback', 'late-failure', 'timeout', 'early-bound', 'non-key', 'source-change', 'idr-failure', 'configuration-failure', 'aggregate-deadline', 'producer-failure', 'packet-limit', 'byte-limit', 'key-limit', 'unsupported-duration', 'endpoint-tolerance'] if args.case == 'all' else [args.case]:
        exercise(source, facts, fault)
    if receipt['cases'] and all(case['result'] == 'passed' for case in receipt['cases']):
        receipt['result'] = 'passed'
except Exception as error:
    receipt['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
finally:
    (run / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print(json.dumps({'receipt': str(run.relative_to(ROOT) / 'receipt.json'), 'result': receipt['result']}))
sys.exit(0 if receipt['result'] == 'passed' else 1)
