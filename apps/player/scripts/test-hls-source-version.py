#!/usr/bin/env python3
"""Real public HTTP/FFmpeg proof: source version wins over source clock skew.

Before production edits, protect future/normal/past timestamps, backwards source
changes, size changes, stale master identity, and two concurrent cache rebuilds.
Only disposable synthetic media/cache is modified. No production endpoint runs.
"""
import base64
import concurrent.futures
import hashlib
import hmac
import json
import os
from pathlib import Path
import shutil
import socket
import struct
import subprocess
import time
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parents[3]
RUN = ROOT / '.verification/hls-source-version' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
RUN.mkdir(parents=True)
revision = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
receipt = {'revision': revision, 'command': 'python3 apps/player/scripts/test-hls-source-version.py',
           'environment': 'disposable loopback HTTP Server; synthetic H264 High/AC3; real FFmpeg decode',
           'cases': [], 'result': 'failed', 'productionMediaOrCacheModified': False}
binary, probe = RUN / 'player', RUN / 'readiness'
fixture = RUN / 'fixture.mkv'
builds = [['go', '-C', 'apps/player', 'build', '-p=1', '-o', str(binary), './cmd/kinosail'],
          ['go', '-C', 'apps/player', 'build', '-p=1', '-o', str(probe), './scripts/hls-readiness']]
media_command = ['ffmpeg', '-nostdin', '-v', 'error', '-f', 'lavfi', '-i', 'testsrc2=s=640x360:r=24:d=32',
                 '-f', 'lavfi', '-i', 'sine=frequency=440:sample_rate=48000:duration=32',
                 '-c:v', 'libx264', '-threads', '2', '-preset', 'veryfast', '-profile:v', 'high', '-crf', '30',
                 '-pix_fmt', 'yuv420p', '-g', '48', '-c:a', 'ac3', '-ac', '6', str(fixture)]


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, msg, headers, newurl):
        return None


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def journey(name, source_time):
    case = {'name': name, 'requestedSourceMtimeSeconds': source_time, 'result': 'failed'}
    receipt['cases'].append(case)
    directory = RUN / name
    media = directory / 'media'
    media.mkdir(parents=True)
    source = media / 'Fixture.mkv'
    shutil.copyfile(fixture, source)
    os.utime(source, (source_time, source_time))
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        port = listener.getsockname()[1]
    url = f'http://localhost:{port}'
    env = dict(os.environ, KINOSAIL_LISTEN=f'127.0.0.1:{port}', KINOSAIL_AUTH_URL=url,
               KINOSAIL_TLS_ENABLED='false', KINOSAIL_DATA_DIR=str(directory / 'config'),
               KINOSAIL_MEDIA_DIR=str(media), KINOSAIL_CACHE_DIR=str(directory / 'cache'),
               KINOSAIL_BACKUP_DIR=str(directory / 'backups'), KINOSAIL_BACKUP_KEY='synthetic-hls-source-key')
    token = ''
    opener = urllib.request.build_opener(NoRedirect)

    def http(path, method='GET', body=None, authenticated=True):
        headers = {'Content-Type': 'application/json'}
        if authenticated and token:
            headers['Authorization'] = 'Bearer ' + token
        request = urllib.request.Request(url + path, data=json.dumps(body).encode() if body is not None else None,
                                         headers=headers, method=method)
        try:
            with opener.open(request, timeout=40) as response:
                return response.status, response.read()
        except urllib.error.HTTPError as error:
            status, data = error.code, error.read()
            error.close()
            return status, data

    def call(path, method='GET', body=None, status=200):
        actual, data = http(path, method, body)
        if actual != status:
            raise RuntimeError('public_http_status_' + str(actual))
        return json.loads(data) if data else None

    def projection(root):
        return json.loads(subprocess.check_output([str(probe), str(source), str(root)], timeout=10))

    def roots():
        cache = directory / 'cache'
        return [entry for entry in cache.iterdir() if entry.name.startswith(item_id + '-plan-')]

    def check(condition, failure):
        if not condition:
            raise RuntimeError(failure)

    def master():
        status, data = http(hls)
        check(status == 200, 'master_http_' + str(status))
        return data

    with (directory / 'server.log').open('w') as log:
        server = subprocess.Popen([str(binary)], cwd=ROOT, env=env, stdout=log, stderr=log)
        try:
            limit = time.monotonic() + 30
            while time.monotonic() < limit:
                try:
                    if http('/healthz', authenticated=False)[0] == 200:
                        break
                except OSError:
                    pass
                time.sleep(0.1)
            owner = call('/api/v1/setup', 'POST', {'name': 'Owner', 'password': 'synthetic-source-password', 'totp': True}, status=201)
            token = owner['token']
            secret = base64.b32decode(owner['totp']['secret'])
            value = hmac.new(secret, struct.pack('>Q', int(time.time() / 30)), hashlib.sha1).digest()
            offset = value[-1] & 15
            code = f'{(struct.unpack(">I", value[offset:offset + 4])[0] & 0x7fffffff) % 1000000:06d}'
            check(call('/api/v1/me/mfa', 'PUT', {'code': code}) == {'enabled': True}, 'mfa_confirmation')
            check(http('/onboarding/finish')[0] == 303, 'onboarding_completion')
            items = call('/api/v1/library')['items']
            item_id = next(item['id'] for item in items if item['title'] == 'Fixture')
            plan = call('/api/v1/items/' + item_id + '/playback?videoCodecs=h264')
            hls = plan['compatible']
            check('/p/a-' in hls, 'fixture_must_use_audio_transcode')
            check(http(hls, authenticated=False)[0] == 401, 'unauthenticated_hls_denial')
            check(not roots(), 'unauthenticated_cache_side_effect')
            preparation = '/api/v1/items/' + item_id + '/playback-prepare'
            check(http(preparation, 'POST', {'source': hls})[0] == 202, 'preparation_accepted')
            limit = time.monotonic() + 20
            observed = None
            while time.monotonic() < limit:
                candidates = roots()
                if len(candidates) == 1:
                    observed = projection(candidates[0])
                    if observed['variants'] and all(v['boundedRead'] and v['manifestBytes'] > 0 for v in observed['variants']):
                        break
                time.sleep(0.05)
            case['exactGoBeforeMaster'] = observed
            status, data = http(hls)
            case['initialMasterHTTP'] = status
            check(observed is not None and bool(observed['variants']), 'readiness_not_observed')
            check(status == 200, 'future_or_control_master_http_' + str(status))
            root = roots()[0]
            case['exactGoAfterMaster'] = projection(root)
            check(case['exactGoAfterMaster']['variantsReady'], 'exact_go_variants_not_ready')
            original = digest(root / 'index.m3u8')
            check(master() == data, 'unchanged_source_cache_reuse')
            rendition = next(line for line in data.decode().splitlines() if line.endswith('/index.m3u8'))
            base = hls.removesuffix('index.m3u8') + rendition.removesuffix('index.m3u8')
            fragments = []
            for file in ['init.mp4'] + [f'segment-{index:05d}.m4s' for index in range(6)]:
                fragment_status, fragment = http(base + file)
                check(fragment_status == 200, 'fragment_http_' + str(fragment_status))
                fragments.append(fragment)
            decoded = directory / 'decoded.mp4'
            decoded.write_bytes(b''.join(fragments))
            result = subprocess.run(['ffmpeg', '-nostdin', '-v', 'error', '-xerror', '-threads', '2',
                '-i', str(decoded), '-progress', 'pipe:1', '-f', 'null', '-'], capture_output=True, timeout=30)
            frames = [int(line.split('=')[1]) for line in result.stdout.decode().splitlines() if line.startswith('frame=')]
            check(result.returncode == 0 and max(frames, default=0) >= 240, 'decoded_moving_frames')
            case['decodedFramesAcrossWarmWindow'] = max(frames)
            case['originalInitSHA256'] = hashlib.sha256(fragments[0]).hexdigest()
            # Move source mtime backwards: a timestamp-only cache check would accept old output.
            os.utime(source, (315532800, 315532800))
            first_changed = master()
            check(hashlib.sha256(first_changed).hexdigest() != original, 'backwards_mtime_stale_reuse')
            case['backwardsMtimeInvalidated'] = True
            # Change bytes while retaining exactly that mtime, then join one rebuild twice.
            with source.open('ab') as output:
                output.write(b'\0')
            os.utime(source, (315532800, 315532800))
            with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
                rebuilt = list(pool.map(lambda _: master(), range(2)))
            check(rebuilt[0] == rebuilt[1] and rebuilt[0] != first_changed, 'size_change_concurrent_rebuild')
            case['changedSizeInvalidatedAndConcurrentMastersMatch'] = True
            # A stale validated-policy marker must never be returned as the current master.
            current = root / 'index.m3u8'
            previous = current.read_bytes()
            lines = previous.splitlines(keepends=True)
            current.write_bytes(b''.join(b'#KINOSAIL-TRANSCODER:stale\n' if line.startswith(b'#KINOSAIL-TRANSCODER:') else line for line in lines))
            check(master() == previous, 'stale_policy_master_reuse')
            case['staleMasterIdentityRegenerated'] = True
            case['result'] = 'passed'
        except Exception as error:
            # Safe categories only; HTTP bodies, tokens, private targets and encoder stderr stay private.
            case['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
        finally:
            server.terminate()
            try:
                server.wait(timeout=10)
            except subprocess.TimeoutExpired:
                server.kill()
                server.wait()


try:
    for command in builds:
        subprocess.run(command, cwd=ROOT, check=True)
    subprocess.run(media_command, check=True)
    receipt['binarySHA256'] = digest(binary)
    receipt['exactGoProbeSHA256'] = digest(probe)
    receipt['fixtureSHA256'] = digest(fixture)
    receipt['fixtureMediaCommand'] = media_command
    for name, stamp in [('normal', int(time.time())), ('past', 946684800), ('future2097', 4039365600)]:
        journey(name, stamp)
    if all(case['result'] == 'passed' for case in receipt['cases']) and len(receipt['cases']) == 3:
        receipt['result'] = 'passed'
except Exception as error:
    receipt['failureClass'] = type(error).__name__
finally:
    (RUN / 'receipt.json').write_text(json.dumps(receipt, indent=2, allow_nan=False) + '\n')
    checksums = {str(path.relative_to(ROOT)): digest(path) for path in [Path(__file__), ROOT / 'apps/player/scripts/hls-readiness/main.go']}
    checksums['receipt.json'] = digest(RUN / 'receipt.json')
    (RUN / 'SHA256SUMS').write_text(''.join(f'{value}  {key}\n' for key, value in checksums.items()))
    print(json.dumps({'result': receipt['result'], 'cases': receipt['cases'], 'receiptSHA256': checksums['receipt.json']}, allow_nan=False))
raise SystemExit(0 if receipt['result'] == 'passed' else 1)
