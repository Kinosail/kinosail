import base64
import hashlib
import hmac
import json
import os
from pathlib import Path
import shutil
import socket
import ssl
import struct
import subprocess
import sys
import time
import urllib.error
import urllib.request

root, app, state_name = Path(sys.argv[1]).resolve(), sys.argv[2], sys.argv[3]
state = root / '.verification/test-overhaul' / state_name
artifact = Path(os.environ['KINOSAIL_E2E_ARTIFACT_DIR'])
shutil.copyfile(__file__, artifact / 'populated-driver.py')
binary = root / '.verification/test-overhaul' / ('kinosail-' + app)
if (state / 'config').exists() and any((state / 'config').iterdir()):
    raise RuntimeError('Choose a fresh isolated state directory')
(state / 'media').mkdir(parents=True, exist_ok=True)
recipes = [
    ['ffmpeg', '-hide_banner', '-loglevel', 'error', '-f', 'lavfi', '-i', 'color=c=blue:s=1280x720:d=12', '-c:v', 'ffv1', '-threads', '1', str(state / 'media/Arrival.mkv')],
    ['ffmpeg', '-hide_banner', '-loglevel', 'error', '-f', 'lavfi', '-i', 'testsrc2=size=640x360:rate=24:duration=12', '-f', 'lavfi', '-i', 'sine=frequency=440:duration=12', '-c:v', 'libx264', '-threads', '1', '-preset', 'veryfast', '-crf', '32', '-pix_fmt', 'yuv420p', '-c:a', 'aac', '-movflags', '+faststart', str(state / 'media/Direct Retry Control.mp4')],
]
for recipe in recipes:
    if not Path(recipe[-1]).exists():
        subprocess.run(recipe, check=True)
for name in ['Beta.mkv', 'Gamma.mkv']:
    target = state / 'media' / name
    if not target.exists():
        os.link(state / 'media/Arrival.mkv', target)
(state / 'media/Arrival.en.srt').write_text('1\n00:00:01,000 --> 00:00:02,000\nHello there.\n\n2\n00:00:03,000 --> 00:00:04,000\nAnother subtitle cue.\n')
with socket.socket() as sock:
    sock.bind(('127.0.0.1', 0))
    port = sock.getsockname()[1]
url = f'http://127.0.0.1:{port}'
environment = dict(os.environ, KINOSAIL_LISTEN=f'127.0.0.1:{port}',
    KINOSAIL_AUTH_URL=url, KINOSAIL_TLS_ENABLED='false', KINOSAIL_DATA_DIR=str(state / 'config'),
    KINOSAIL_MEDIA_DIR=str(state / 'media'), KINOSAIL_CACHE_DIR=str(state / 'cache'),
    KINOSAIL_BACKUP_DIR=str(state / 'backups'), KINOSAIL_BACKUP_KEY='synthetic-e2e-key')
for name in ['config', 'cache', 'backups']:
    (state / name).mkdir(parents=True, exist_ok=True)
(artifact / 'local-inputs.json').write_text(json.dumps({
    'binarySHA256': hashlib.sha256(binary.read_bytes()).hexdigest(),
    'serverEnvironment': {k: environment[k] for k in ['KINOSAIL_LISTEN','KINOSAIL_AUTH_URL','KINOSAIL_TLS_ENABLED','KINOSAIL_DATA_DIR','KINOSAIL_MEDIA_DIR','KINOSAIL_CACHE_DIR','KINOSAIL_BACKUP_DIR']},
    'ffmpegVersion': subprocess.check_output(['ffmpeg','-version'], text=True).splitlines()[0],
    'playwrightVersion': subprocess.check_output(['node', 'node_modules/@playwright/test/cli.js', '--version'], cwd=root / 'apps' / app / 'e2e', text=True).strip(),
    'mediaSHA256': {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in (state / 'media').iterdir()},
    'buildCommand': f'GOCACHE=/tmp/kinosail-testing-go-cache go -C apps/{app} build -o ../../.verification/test-overhaul/kinosail-{app} ./cmd/kinosail',
    'mediaRecipe': 'CC0 synthetic blue1280x72012s ffv1 threads1 Arrival.mkv; hardlinks Beta/Gamma; Arrival.en.srt two cues; testsrc2 640x36024fps12s+sine440 libx264 threads1 crf32 AAC faststart Direct Retry Control.mp4',
    'mediaCommands': recipes,
    'fixtureGeneratorCommand': f'KINOSAIL_UI_FIXTURE_DIR=<own output> GOCACHE=/tmp/kinosail-testing-go-cache go -C apps/{app} test ./internal/server -run ^TestWriteUIStateFixturesSubtitleInspector$ -count=1' if app == 'subtitles' else None,
    'fixtureSHA256': {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest() for p in (root / '.verification/test-overhaul' / ('ui-fixtures-' + app)).rglob('*') if p.is_file()},
    'boundary': 'Real native Go Server on application-supported loopback HTTP, persistent own database/media; API-created MFA Owner; Chrome macOS ARM64. Browser ignoreHTTPSErrors=false; certificate bypass launch flags removed. HTTP run makes no TLS claim. Template fixture specs remain fixtures.',
}, indent=2) + '\n')
with (artifact / 'server.log').open('w') as log:
    server = subprocess.Popen([str(binary)], env=environment, stdout=log, stderr=log)
    try:
        context = ssl.create_default_context()
        for attempt in range(120):
            if server.poll() is not None:
                raise RuntimeError(f'Server exited {server.returncode}')
            try:
                with urllib.request.urlopen(url + '/healthz', context=context, timeout=1) as response:
                    if json.load(response) == {'status': 'ok'}:
                        break
            except (OSError, ValueError, subprocess.CalledProcessError):
                time.sleep(0.25)
        else:
            raise RuntimeError('Server failed loopback HTTP readiness')
        def call(path, method, body, token=''):
            headers = {'Content-Type': 'application/json'}
            if token:
                headers['Authorization'] = 'Bearer ' + token
            request = urllib.request.Request(url + path, data=json.dumps(body).encode(), headers=headers, method=method)
            try:
                with urllib.request.urlopen(request, context=context, timeout=10) as response:
                    return json.load(response)
            except urllib.error.HTTPError as error:
                raise RuntimeError(f'API setup {path} returned {error.code}') from None
        owner = call('/api/v1/setup', 'POST', {'name': 'Owner', 'password': 'test-instance-password', 'totp': True})
        secret = owner['totp']['secret']
        digest = hmac.new(base64.b32decode(secret), struct.pack('>Q', int(time.time() / 30)), hashlib.sha1).digest()
        offset = digest[-1] & 15
        code = f'{(struct.unpack(">I", digest[offset:offset + 4])[0] & 0x7fffffff) % 1000000:06d}'
        call('/api/v1/me/mfa', 'PUT', {'code': code}, owner['token'])
        request = urllib.request.Request(url + '/onboarding/finish', headers={'Authorization': 'Bearer ' + owner['token']})
        with urllib.request.urlopen(request, context=context, timeout=10) as response:
            response.read()
        (artifact / 'setup-result.json').write_text(json.dumps({'loopbackHTTPHealth': 'pass', 'MFAOwnerSetup': 'pass', 'onboardingFinish': 'pass', 'browserCertificateBypasses':'disabled'}) + '\n')
        config = artifact / 'browser.config.ts'
        config.write_text('import config from ' + json.dumps(str(root / 'apps' / app / 'e2e/playwright.config.ts')) + ';\nexport default { ...config, testDir: ' + json.dumps(str(root / 'apps' / app / 'e2e')) + ', use: { ...config.use, ignoreHTTPSErrors: false }, projects: config.projects.map(project => ({ ...project, use: { ...project.use, ignoreHTTPSErrors: false, launchOptions: { ...project.use.launchOptions, args: (project.use.launchOptions?.args ?? []).filter(arg => !/ignore-certificate-errors|allow-insecure-localhost/.test(arg)) } } })) };\n')
        result = subprocess.run(['node', 'node_modules/@playwright/test/cli.js', 'test', '--config', str(config), *sys.argv[4:], '--project=chromium', '--workers=1'],
            cwd=root / 'apps' / app / 'e2e', env=dict(os.environ, KINOSAIL_E2E_URL=url,
            KINOSAIL_TEST_INSTANCE='1', KINOSAIL_TEST_TOTP_SECRET=secret, KINOSAIL_E2E_MEDIA_DIR=str(state / 'media'),
            KINOSAIL_TEST_REVISION=subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True).strip(),
            KINOSAIL_UI_FIXTURE_DIR=str(root / '.verification/test-overhaul' / ('ui-fixtures-' + app)), PLAYWRIGHT_CHANNEL='chrome'))
    finally:
        server.terminate()
        try:
            server.wait(timeout=10)
        except subprocess.TimeoutExpired:
            server.kill()
            server.wait()
sys.exit(result.returncode)
