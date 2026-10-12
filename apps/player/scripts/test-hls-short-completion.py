#!/usr/bin/env python3
"""Prove public short-HEVC readiness with controlled real-encoder completion."""
import argparse
from concurrent.futures import ThreadPoolExecutor
import json
import os
from pathlib import Path
import shutil
import socket
import subprocess
import sys
import time

from hls_followon_public import check
from hls_timeline_http import PublicServer, sha, source_state

ROOT = Path(__file__).resolve().parents[3]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--case', choices=['all', 'success', 'late-failure', 'timeout', 'background', 'marker-recovery'], default='all')
parser.add_argument('--duration', type=float, choices=[10, 29.9], default=10)
args = parser.parse_args()
run = ROOT / '.verification/hls-short-completion' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
run.mkdir(parents=True)
media = run / 'media'
media.mkdir()
source = media / 'Fixture.mkv'
binary = run / 'player'
receipt = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(),
    'result': 'failed', 'cases': [], 'command': f'python3 apps/player/scripts/test-hls-short-completion.py --case {args.case} --duration {args.duration}',
    'boundary': 'Real authenticated Server and FFmpeg media; a test-owned wrapper holds or fails the producer after real EOF. No browser or physical-device proof.'}
patch = run / 'tested-source.patch'
patch.write_bytes(subprocess.check_output(['git', 'diff', 'HEAD', '--']))
receipt['sourceSnapshot'] = {'patchSHA256': sha(patch), 'harnessSHA256': sha(Path(__file__)),
    'helpers': {name: sha(Path(__file__).with_name(name))
                for name in ['hls_followon_public.py', 'hls_timeline_http.py']}}


def wait_for(condition, seconds, failure):
    limit = time.monotonic() + seconds
    while time.monotonic() < limit:
        if condition():
            return
        time.sleep(.025)
    raise RuntimeError(failure)


def wrapper(path, directory, fault):
    path.write_text('#!' + sys.executable + '\n' + f'''
import os,subprocess,sys,time
from pathlib import Path
real={shutil.which('ffmpeg')!r}
directory=Path({str(directory)!r})
args=sys.argv[1:]
if {str(source)!r} not in args or '-f' not in args or args[args.index('-f')+1]!='hls':
    os.execv(real,[real]+args)
with (directory/'wrapper-pids').open('a') as output:output.write(str(os.getpid())+'\\n')
result=subprocess.run([real]+args).returncode
if result:raise SystemExit(result)
with (directory/'encoder-completions').open('a') as output:output.write('complete\\n')
deadline=time.monotonic()+45
while not (directory/'release').exists() and time.monotonic()<deadline:time.sleep(.025)
raise SystemExit({42 if fault == 'late-failure' else 0})
''')
    path.chmod(0o700)


def completion_count(directory):
    path = directory / 'encoder-completions'
    return len(path.read_text().splitlines()) if path.exists() else 0


def workers_settled(directory):
    path = directory / 'wrapper-pids'
    for value in path.read_text().splitlines() if path.exists() else []:
        try:
            os.kill(int(value), 0)
            return False
        except ProcessLookupError:
            pass
    return True


def prepare_background(api, item, plan, directory, case):
    started = time.monotonic()
    prepare = '/api/v1/items/' + item['id'] + '/playback-prepare'
    body = {'source': plan['compatible']}
    value = api.call(prepare, 'POST', body, 202)
    check(value['state'] == 'queued', 'background_not_queued')
    wait_for(lambda: completion_count(directory) == 1, 35, 'background_producer_not_observed')
    time.sleep(.3)
    value = api.call(prepare, 'POST', body, 202)
    case['backgroundStateWhileProducerActive'] = value['state']
    check(value['state'] == 'queued', 'background_ready_before_producer_settled')
    (directory / 'release').write_text('release')
    wait_for(lambda: api.call(prepare, 'POST', body, 202)['state'] == 'ready', 10, 'background_not_ready')
    status, data, _ = api.http(plan['compatible'])
    case['responses'].append({'attempt': 1, 'status': status, 'playlistPublished': data.startswith(b'#EXTM3U')})
    check(status == 200 and completion_count(directory) == 1, 'background_output_not_reused')
    case['seconds'] = round(time.monotonic() - started, 3)
    case['result'] = 'passed'


def recover_markers(api, plan, directory, case):
    (directory / 'release').write_text('release')
    sentinel = directory / 'external-marker-sentinel'
    sentinel.write_text('must not change')
    original = sha(sentinel)
    for attempt, corruption in enumerate(['initial', 'missing', 'mismatched', 'symlink', 'missing-source']):
        if corruption != 'initial':
            markers = list((directory / 'cache').rglob('.hevc-complete'))
            check(len(markers) == 1, 'completion_marker_count')
            marker = markers[0]
            if corruption == 'missing-source':
                marker.with_name('.source').unlink()
            else:
                marker.unlink()
                if corruption == 'mismatched':
                    marker.write_text('different-policy')
                elif corruption == 'symlink':
                    marker.symlink_to(sentinel)
        status, data, _ = api.http(plan['compatible'])
        case['responses'].append({'attempt': attempt + 1, 'corruption': corruption, 'status': status,
            'playlistPublished': data.startswith(b'#EXTM3U'), 'producerCompletions': completion_count(directory)})
        check(status == 200 and completion_count(directory) == attempt + 1, 'invalid_marker_cache_reused')
        check(sha(sentinel) == original, 'external_marker_modified')
    case['externalSentinelUnchanged'] = True
    case['result'] = 'passed'


def exercise(fault):
    directory = run / fault
    directory.mkdir()
    encoder = directory / 'ffmpeg-wrapper.py'
    wrapper(encoder, directory, fault)
    case = {'fault': fault, 'result': 'failed', 'responses': []}
    receipt['cases'].append(case)
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        port = listener.getsockname()[1]
    origin = f'http://localhost:{port}'
    env = dict(os.environ, KINOSAIL_LISTEN=f'127.0.0.1:{port}', KINOSAIL_AUTH_URL=origin,
        KINOSAIL_TLS_ENABLED='false', KINOSAIL_FFMPEG=str(encoder), KINOSAIL_DATA_DIR=str(directory / 'config'),
        KINOSAIL_MEDIA_DIR=str(media), KINOSAIL_CACHE_DIR=str(directory / 'cache'),
        KINOSAIL_BACKUP_DIR=str(directory / 'backups'), KINOSAIL_BACKUP_KEY='synthetic-completion-key')
    session = 'qa-hevc-' + fault
    api = PublicServer(origin)
    api.opener.addheaders = [('X-Playback-Session', session)]
    with (directory / 'server-private.log').open('w') as log:
        server = subprocess.Popen([str(binary)], env=env, stdout=log, stderr=log)
        try:
            api.authorize()
            item = next(v for v in api.call('/api/v1/library')['items'] if v['title'] == 'Fixture')
            plan = api.call('/api/v1/items/' + item['id'] + '/playback?videoCodecs=hevc&audioCodecs=aac')
            check(plan['compatiblePlan']['mode'] == 'audio-transcode', 'fixture_mode')
            if fault == 'background':
                prepare_background(api, item, plan, directory, case)
                return
            if fault == 'marker-recovery':
                recover_markers(api, plan, directory, case)
                return
            for attempt in range(2 if fault == 'late-failure' else 1):
                (directory / 'release').unlink(missing_ok=True)
                started = time.monotonic()
                with ThreadPoolExecutor(max_workers=1) as pool:
                    pending = pool.submit(api.http, plan['compatible'])
                    wait_for(lambda: pending.done() or completion_count(directory) > attempt, 35, 'producer_not_observed')
                    held = not pending.done()
                    if held:
                        time.sleep(.15)
                        held = not pending.done()
                    case.setdefault('withheldWhileProducerActive', []).append(held)
                    if fault != 'timeout':
                        (directory / 'release').write_text('release')
                    status, data, headers = pending.result(timeout=40)
                request_id = next((v for k, v in headers.items() if k.lower() == 'x-request-id'), '')
                case['responses'].append({'attempt': attempt + 1, 'status': status,
                    'seconds': round(time.monotonic() - started, 3), 'requestID': request_id,
                    'playlistPublished': data.startswith(b'#EXTM3U')})
            expected = 200 if fault == 'success' else 503
            check(all(case['withheldWhileProducerActive']), 'playlist_before_producer_settled')
            check(all(r['status'] == expected for r in case['responses']), 'completion_response_status')
            check(fault == 'success' or not any(r['playlistPublished'] for r in case['responses']), 'failed_output_published')
            if fault == 'timeout':
                check(28 <= case['responses'][0]['seconds'] <= 35, 'completion_deadline')
            case['result'] = 'passed'
        except Exception as error:
            case['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
        finally:
            (directory / 'release').write_text('release')
            server.terminate()
            try:
                server.wait(timeout=10)
            except subprocess.TimeoutExpired:
                server.kill()
                server.wait(timeout=5)
            case['serverSettled'] = server.poll() is not None
            try:
                wait_for(lambda: workers_settled(directory), 5, 'owned_worker_not_settled')
            except RuntimeError:
                case['result'] = 'failed'
            case['ownedWorkersSettled'] = workers_settled(directory)
            log.flush()
            private = (directory / 'server-private.log').read_text()
            check(len(private) <= 2 * 1024 * 1024, 'private_log_bound')
            entries = []
            for line in private.splitlines():
                if line.startswith('{'):
                    entry = json.loads(line)
                    if entry.get('msg') == 'HLS playlist preparation failed':
                        entries.append({k: entry.get(k) for k in ['level', 'request_id', 'playback_session', 'mode', 'encoder_phase']})
            case['safeFailureLogs'] = entries
            case['noCredentialsLogged'] = api.token not in private if api.token else False
            if fault in ['late-failure', 'timeout']:
                correlated = all(any(e['level'] == 'ERROR' and e['request_id'] == r['requestID']
                    and e['playback_session'] == session for e in entries) for r in case['responses'] if r['status'] == 503)
                case['failureLogsCorrelated'] = correlated
                if not correlated:
                    case['result'] = 'failed'
            if not case['noCredentialsLogged']:
                case['result'] = 'failed'
            case['producerCompletions'] = completion_count(directory)
            case['wrapperSHA256'] = sha(encoder)
            case['privateLogSHA256'] = sha(directory / 'server-private.log')


try:
    command = ['ffmpeg', '-nostdin', '-v', 'error', '-f', 'lavfi', '-i', f'testsrc2=s=320x180:r=24:d={args.duration}',
        '-f', 'lavfi', '-i', f'sine=frequency=440:sample_rate=48000:duration={args.duration}',
        '-c:v', 'libx265', '-preset', 'ultrafast', '-threads', '1',
        '-x265-params', 'pools=1:frame-threads=1:keyint=48:min-keyint=48:scenecut=0',
        '-c:a', 'ac3', '-ac', '2', str(source)]
    subprocess.run(command, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=60)
    receipt['fixture'] = {'command': command, **source_state(source)}
    before = source_state(source)
    build = ['go', 'build', '-p', '2', '-o', str(binary), './apps/player/cmd/kinosail']
    receipt['build'] = build
    with (run / 'build-private.log').open('w') as log:
        subprocess.run(build, check=True, stdout=log, stderr=log, timeout=180)
    receipt['binarySHA256'] = sha(binary)
    for fault in ['success', 'late-failure', 'timeout', 'background', 'marker-recovery'] if args.case == 'all' else [args.case]:
        exercise(fault)
    receipt['sourceUnchanged'] = source_state(source) == before
    if receipt['sourceUnchanged'] and all(v['result'] == 'passed' and v['serverSettled'] and v['ownedWorkersSettled'] for v in receipt['cases']):
        receipt['result'] = 'passed'
except Exception as error:
    receipt['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
finally:
    (run / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print(json.dumps({'receipt': str(run.relative_to(ROOT) / 'receipt.json'), 'result': receipt['result']}))
sys.exit(0 if receipt['result'] == 'passed' else 1)
