#!/usr/bin/env python3
"""Real Go album queue proof; preserve all disposable state and private artifacts."""
import argparse
import base64
import hashlib
import hmac
import json
import math
import os
from pathlib import Path
import socket
import struct
import subprocess
import time
import traceback
import urllib.request
import wave
import zlib

parser = argparse.ArgumentParser(description=__doc__)
mode = parser.add_mutually_exclusive_group()
mode.add_argument('--red', action='store_true', help='Require the pre-fix metadata mismatch twice')
mode.add_argument('--fixture-only', type=Path, help='Prepare only a new disposable album directory; no Owner or Server setup')
args = parser.parse_args()
root = Path(__file__).resolve().parents[2]
run = root / '.verification/r08-now-playing' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
run.mkdir(parents=True)
media = args.fixture_only if args.fixture_only else run / 'media'
media.mkdir()
title = 'real album queue advances source and all Now Playing identity to the fictional second track'
action_title = 'real album queue keeps system previous and next current and exposes only fresh current-track actions'
sources = ['packages/webassets/static/player-progress.js', 'packages/webassets/static/player-presentation.js',
           'packages/webassets/static/player-audio-queue.js', 'packages/playerweb/audio_queue_template.go',
           'packages/playerweb/player_template.go', 'apps/player/e2e/test-instance-audio-queue.spec.ts',
           'apps/player/e2e/player-audio-policy.spec.ts', 'apps/player/e2e/player-progress.spec.ts', 'apps/player/e2e/static-sources.ts',
           'scripts/testing/test-player-audio-queue-local.py']
revision = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True).strip()
receipt = {'revision': revision, 'sourceSHA256': {name: hashlib.sha256((root / name).read_bytes()).hexdigest() for name in sources},
           'workingDiffSHA256': hashlib.sha256(subprocess.check_output(['git', 'diff', 'HEAD'], cwd=root)).hexdigest(),
           'command': 'GOMAXPROCS=2 python3 scripts/testing/test-player-audio-queue-local.py' + (' --red' if args.red else ' --fixture-only <new-disposable-directory>' if args.fixture_only else ''),
           'environment': 'Native Go Kinosail Server; loopback HTTP; one Chromium worker',
           'data': 'Two generated twelve-second WAV tracks, fictional NFO metadata and PNG covers; disposable Owner and TOTP. State preserved.',
           'boundaries': 'No real user data, container, deployment, physical device, or native OS-control panel proof.',
           'result': 'failed'}
phase = 'fixture-preparation'

def build_inputs():
    output = subprocess.check_output(['go', 'list', '-deps', '-json', '-p', '1', './cmd/kinosail'],
                                     cwd=root / 'apps/player', env={**os.environ, 'GOMAXPROCS': '2', 'GOPROXY': 'off'}, text=True)
    decoder, offset, paths, modules = json.JSONDecoder(), 0, set(), {}
    while offset < len(output):
        while offset < len(output) and output[offset].isspace():
            offset += 1
        if offset == len(output):
            break
        package, offset = decoder.raw_decode(output, offset)
        directory = Path(package['Dir'])
        if directory.is_relative_to(root):
            for group in ['GoFiles', 'CgoFiles', 'CFiles', 'CXXFiles', 'HFiles', 'SFiles', 'SysoFiles', 'EmbedFiles']:
                paths.update(directory / name for name in package.get(group, []))
        module = package.get('Module', {})
        if module.get('Version'):
            modules[module['Path']] = {key: module[key] for key in ['Version', 'Sum', 'GoModSum'] if key in module}
    for directory in [root, root / 'apps/player']:
        paths.update(path for name in ['go.work', 'go.work.sum', 'go.mod', 'go.sum'] if (path := directory / name).is_file())
    return {'repoSHA256': {str(path.relative_to(root)): hashlib.sha256(path.read_bytes()).hexdigest() for path in sorted(paths)},
            'modules': modules, 'toolchain': subprocess.check_output(['go', 'version'], text=True).strip()}

def cover(path, color):
    def chunk(kind, value):
        return struct.pack('>I', len(value)) + kind + value + struct.pack('>I', zlib.crc32(kind + value))
    pixels = b''.join(b'\x00' + bytes(tuple(min(255, value + x + y) for value in color)) * 64 for y in range(64) for x in [0])
    path.write_bytes(b'\x89PNG\r\n\x1a\n' + chunk(b'IHDR', struct.pack('>IIBBBBB', 64, 64, 8, 2, 0, 0, 0)) + chunk(b'IDAT', zlib.compress(pixels)) + chunk(b'IEND', b''))

try:
    tracks = [('01 Lantern', 'Lantern Start', 'Aster Vale', 440, (10, 50, 90)),
              ('02 Copper', 'Copper &lt;Moon&gt; &amp; Harbor', 'Mira Tide', 660, (20, 80, 40))]
    for number, (name, track_title, artist, frequency, color) in enumerate(tracks, 1):
        with wave.open(str(media / (name + '.wav')), 'wb') as audio:
            audio.setparams((1, 2, 16000, 0, 'NONE', 'not compressed'))
            audio.writeframes(b''.join(struct.pack('<h', round(4000 * math.sin(2 * math.pi * frequency * index / 16000))) for index in range(12 * 16000)))
        (media / (name + '.nfo')).write_text(f'<track><title>{track_title}</title><artist>{artist}</artist><albumartist>Fictional Ensemble</albumartist><album>R08 Fictional Session</album><disc>1</disc><track>{number}</track></track>')
        cover(media / (name + '.png'), color)
    receipt['fixtureSHA256'] = {path.name: hashlib.sha256(path.read_bytes()).hexdigest() for path in sorted(media.iterdir())}
    if args.fixture_only:
        receipt['environment'] = 'New disposable media directory only; no Server, Owner, or browser setup'
        receipt['data'] = 'Two generated twelve-second WAV tracks, fictional NFO metadata and PNG covers. No account state prepared.'
        receipt['result'] = 'fixture-prepared'
        raise SystemExit(0)
    binary = run / 'kinosail-player'
    phase = 'build-inputs'
    inputs = build_inputs()
    encoded = (json.dumps(inputs, indent=2, sort_keys=True) + '\n').encode()
    (run / 'build-inputs.json').write_bytes(encoded)
    receipt['buildInputsSHA256'] = hashlib.sha256(encoded).hexdigest()
    phase = 'native-build'
    subprocess.run(['go', 'build', '-p', '1', '-o', str(binary), './cmd/kinosail'], cwd=root / 'apps/player', env={**os.environ, 'GOMAXPROCS': '2', 'GOPROXY': 'off'}, check=True)
    if any(hashlib.sha256((root / name).read_bytes()).hexdigest() != wanted for name, wanted in inputs['repoSHA256'].items()):
        raise RuntimeError('Source changed while building the native proof product')
    receipt['binarySHA256'] = hashlib.sha256(binary.read_bytes()).hexdigest()
    receipt['sourceInputsUnchangedAfterBuild'] = True
    phase = 'server-startup'
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        port = listener.getsockname()[1]
    url = f'http://localhost:{port}'
    env = {**os.environ, 'GOMAXPROCS': '2', 'KINOSAIL_LISTEN': f'127.0.0.1:{port}', 'KINOSAIL_AUTH_URL': url,
           'KINOSAIL_TLS_ENABLED': 'false', 'KINOSAIL_DATA_DIR': str(run / 'config'), 'KINOSAIL_MEDIA_DIR': str(media),
           'KINOSAIL_CACHE_DIR': str(run / 'cache'), 'KINOSAIL_BACKUP_DIR': str(run / 'backups'), 'KINOSAIL_BACKUP_KEY': os.urandom(32).hex()}
    with (run / 'server-private.log').open('w') as server_log:
        server = subprocess.Popen([str(binary)], cwd=root, env=env, stdout=server_log, stderr=subprocess.STDOUT)
        try:
            for attempt in range(100):
                try:
                    with urllib.request.urlopen(url + '/healthz', timeout=2) as response:
                        if response.status == 200:
                            break
                except OSError:
                    time.sleep(.1)
            else:
                raise RuntimeError('Disposable Server readiness failed')
            def request(path, body, token='', method='POST'):
                headers = {'Content-Type': 'application/json'}
                if token:
                    headers['Authorization'] = 'Bearer ' + token
                value = urllib.request.Request(url + path, method=method, data=json.dumps(body).encode(), headers=headers)
                with urllib.request.urlopen(value, timeout=10) as response:
                    return json.load(response) if response.status != 204 else None
            phase = 'disposable-owner-preparation'
            created = request('/api/v1/setup', {'name': 'Owner', 'password': 'synthetic-queue-password', 'device': 'Disposable R08 proof', 'totp': True})
            token, secret = created['token'], created['totp']['secret']
            digest = hmac.new(base64.b32decode(secret), int(time.time() // 30).to_bytes(8, 'big'), hashlib.sha1).digest()
            offset = digest[-1] & 15
            code = str((int.from_bytes(digest[offset:offset + 4], 'big') & 0x7fffffff) % 1000000).zfill(6)
            request('/api/v1/me/mfa', {'code': code}, token, 'PUT')
            request('/api/v1/settings/onboarding', {'enabled': False}, token, 'PUT')
            browser_env = {**env, 'KINOSAIL_TEST_INSTANCE': '1', 'KINOSAIL_TEST_TOTP_SECRET': secret,
                           'KINOSAIL_E2E_OWNER_PASSWORD': 'synthetic-queue-password', 'KINOSAIL_E2E_URL': url,
                           'KINOSAIL_E2E_VIDEO': 'off', 'KINOSAIL_BROWSER_WORKERS': '1',
                           'KINOSAIL_E2E_OUTPUT_DIR': str(run / 'browser-artifacts'), 'PLAYWRIGHT_JSON_OUTPUT_FILE': str(run / 'browser-results.json')}
            command = ['node', 'node_modules/@playwright/test/cli.js', 'test', 'test-instance-audio-queue.spec.ts',
                       '--project=chromium', '--workers=1', '--repeat-each=2', '--retries=0', '--global-timeout=120000', '--reporter=line,json']
            if args.red:
                command += ['--grep', title]
            receipt['browserCommand'] = command
            phase = 'browser-execution'
            with (run / 'browser.log').open('w') as browser_log:
                result = subprocess.run(command, cwd=root / 'apps/player/e2e', env=browser_env, stdout=browser_log, stderr=subprocess.STDOUT, timeout=180)
            receipt['browserExitCode'] = result.returncode
            if any(hashlib.sha256((root / name).read_bytes()).hexdigest() != wanted for name, wanted in receipt['sourceSHA256'].items()):
                raise RuntimeError('Source or test harness changed while executing the native proof')
            phase = 'named-browser-result-verification'
            report = json.loads((run / 'browser-results.json').read_text())
            def results(suite, name):
                values = [result for spec in suite.get('specs', []) if spec['title'] == name for test in spec['tests'] for result in test['results']]
                return values + [result for child in suite.get('suites', []) for result in results(child, name)]
            named = {name: [case for suite in report['suites'] for case in results(suite, name)]
                     for name in ([title] if args.red else [title, action_title])}
            receipt['namedResults'] = {name: [case['status'] for case in cases] for name, cases in named.items()}
            if args.red:
                phase = 'named-defect-reproduction-verification'
                cases = named[title]
                if result.returncode == 0 or len(cases) != 2 or any(case['status'] != 'failed' for case in cases):
                    raise RuntimeError('Both pre-fix reproductions must fail after actual queue advance')
                for case in cases:
                    attached = next(value for value in case['attachments'] if value['name'] == 'now-playing-after-real-advance')
                    observed = json.loads(Path(attached['path']).read_text() if attached.get('path') else base64.b64decode(attached['body']))
                    if observed['title'] != 'Copper <Moon> & Harbor' or observed['heading'] != 'Lantern Start' or observed['metadataTitle'] != 'Lantern Start':
                        raise RuntimeError('Pre-fix runtime mismatch was not reproduced')
                receipt['result'] = 'reproduced'
            elif result.returncode == 0 and all(len(cases) == 2 and all(case['status'] == 'passed' for case in cases) for cases in named.values()):
                receipt['result'] = 'passed'
            else:
                raise RuntimeError('Album queue proof failed')
        finally:
            server.terminate()
            try:
                server.wait(timeout=5)
            except subprocess.TimeoutExpired:
                server.kill()
                server.wait()
except Exception as error:
    receipt['failureClass'], receipt['failurePhase'] = type(error).__name__, phase
    (run / 'harness-private.log').write_text(traceback.format_exc())
finally:
    receipt['checksums'] = {str(path.relative_to(run)): hashlib.sha256(path.read_bytes()).hexdigest()
                            for path in run.rglob('*') if path.is_file() and path.name != 'kinosail-player'}
    (run / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print(f"R08 native album proof: {receipt['result']}; receipt: {run / 'receipt.json'}")
if receipt['result'] == 'failed':
    raise SystemExit(1)
