#!/usr/bin/env python3
"""Run scrub E2E against an isolated Player with generated video and a receipt."""
import base64
import hashlib
import hmac
import json
import os
from pathlib import Path
import platform
import socket
import struct
import subprocess
import time
import urllib.request

root = Path(__file__).resolve().parents[3]
run = root / '.verification/scrub' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
run.mkdir(parents=True)
media = run / 'media'
media.mkdir()
binary = run / 'kinosail-player'
revision = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True).strip()
sources = ['packages/webassets/static/player-preview.js', 'packages/webassets/static/player-controls.js',
           'packages/webassets/static/player-presentation.js', 'packages/webassets/static/player-status.js',
           'packages/webassets/static/player-stage.css', 'packages/webassets/webassets.go',
           'apps/player/e2e/player-scrub-process.spec.ts', 'apps/player/scripts/test-scrub-local.py']
source_hashes = {path: hashlib.sha256((root / path).read_bytes()).hexdigest() for path in sources}
build = ['go', '-C', 'apps/player', 'build', '-p=2', '-o', str(binary), './cmd/kinosail']
fixture = ['ffmpeg', '-nostdin', '-v', 'error', '-f', 'lavfi', '-i', 'testsrc2=s=1280x720:r=24:d=30',
           '-c:v', 'libx264', '-preset', 'ultrafast', '-threads', '2', '-g', '120',
           '-movflags', '+faststart', str(media / 'Scrub Sample.mp4')]
subprocess.run(build, cwd=root, check=True)
subprocess.run(fixture, check=True)
with socket.socket() as listener:
    listener.bind(('127.0.0.1', 0))
    port = listener.getsockname()[1]
url = f'http://localhost:{port}'
env = dict(os.environ, KINOSAIL_LISTEN=f'127.0.0.1:{port}', KINOSAIL_AUTH_URL=url,
           KINOSAIL_TLS_ENABLED='false', KINOSAIL_DATA_DIR=str(run / 'config'),
           KINOSAIL_MEDIA_DIR=str(media), KINOSAIL_CACHE_DIR=str(run / 'cache'),
           KINOSAIL_BACKUP_DIR=str(run / 'backups'), KINOSAIL_BACKUP_KEY='synthetic-scrub-key',
           KINOSAIL_E2E_URL=url, KINOSAIL_SCRUB_PROCESS='1', KINOSAIL_BROWSER_MATRIX='full',
           KINOSAIL_E2E_OUTPUT_DIR=str(run / 'results'))
client = urllib.request.build_opener(urllib.request.ProxyHandler({}))


def request(path, data, token='', method='POST'):
    headers = {'Content-Type': 'application/json'}
    if token:
        headers['Authorization'] = 'Bearer ' + token
    with client.open(urllib.request.Request(url + path, data=json.dumps(data).encode(),
                                           headers=headers, method=method), timeout=20) as response:
        body = response.read()
        return json.loads(body) if body else None


browser = ['pnpm', '--dir', 'apps/player/e2e', 'exec', 'playwright', 'test',
           'player-scrub-process.spec.ts', '--workers=1', '--reporter=line']
result = None
with (run / 'server.log').open('w') as log:
    server = subprocess.Popen([str(binary)], env=env, stdout=log, stderr=log)
    try:
        for _ in range(240):
            try:
                client.open(url + '/healthz', timeout=1).close()
                break
            except OSError:
                if server.poll() is not None:
                    raise RuntimeError('Player exited; see server.log') from None
                time.sleep(.25)
        else:
            raise RuntimeError('Player did not become healthy')
        owner = request('/api/v1/setup', {'name': 'Owner', 'password': 'test-instance-password',
                                        'device': 'Synthetic scrub E2E', 'totp': True})
        secret, token = owner['totp']['secret'], owner['token']
        digest = hmac.new(base64.b32decode(secret), struct.pack('>Q', int(time.time() / 30)), hashlib.sha1).digest()
        offset = digest[-1] & 15
        code = str((int.from_bytes(digest[offset:offset + 4], 'big') & 0x7fffffff) % 1_000_000).zfill(6)
        request('/api/v1/me/mfa', {'code': code}, token, 'PUT')
        request('/api/v1/settings/onboarding', {'enabled': False}, token, 'PUT')
        request('/api/v1/settings/subtitle-picker', {'limited': True}, token, 'PUT')
        env['KINOSAIL_TEST_TOTP_SECRET'] = secret
        with (run / 'browser.log').open('w') as browser_log:
            result = subprocess.run(browser, cwd=root, env=env, stdout=browser_log, stderr=subprocess.STDOUT).returncode
    finally:
        server.terminate()
        try:
            server.wait(timeout=10)
        except subprocess.TimeoutExpired:
            server.kill()
            server.wait()
        (run / 'receipt.json').write_text(json.dumps({
            'revision': revision, 'workingDiffSHA256': hashlib.sha256(subprocess.check_output(['git', 'diff', 'HEAD'], cwd=root)).hexdigest(),
            'command': 'python3 apps/player/scripts/test-scrub-local.py', 'build': build, 'fixture': fixture,
            'browser': browser, 'result': result, 'environment': platform.platform(), 'sourceSHA256': source_hashes,
            'binarySHA256': hashlib.sha256(binary.read_bytes()).hexdigest(),
            'logSHA256': {name: hashlib.sha256((run / name).read_bytes()).hexdigest() for name in ['server.log', 'browser.log'] if (run / name).exists()},
            'mediaSHA256': hashlib.sha256((media / 'Scrub Sample.mp4').read_bytes()).hexdigest(),
            'boundary': 'Generated 720p media, custom Web Player, local process. No production server or physical-device proof.'
        }, indent=2) + '\n')
        print(f'E2E artifact: {run}', flush=True)
raise SystemExit(result if result is not None else 1)
