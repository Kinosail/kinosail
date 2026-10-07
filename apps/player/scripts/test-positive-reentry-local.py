#!/usr/bin/env python3
"""One source-bound, real decoded-media native WebKit saved-reentry check."""
import argparse
import base64
import datetime
import hashlib
import hmac
import json
import os
from pathlib import Path
import re
import shutil
import socket
import subprocess
import tarfile
import time
import urllib.request
from positive_reentry_processes import run_owned_command, stop_owned_group, stop_observed_node_children
from positive_reentry_tls import HostedFixtureTrust, require_hosted_macos

parser = argparse.ArgumentParser()
parser.add_argument('--source', default='deb7e51a1cdd3ed9f71842677abda5cd0e9e7fc8')
parser.add_argument('--dependencies', type=Path, help='Existing pinned e2e node_modules; never installed by this runner')
args = parser.parse_args()
root = Path(__file__).resolve().parents[3]
revision = subprocess.check_output(['git', 'rev-parse', '--verify', args.source + '^{commit}'], cwd=root, text=True).strip()
run = root / '.verification/positive-reentry' / datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%S%fZ')
run.mkdir(parents=True)
checksum = lambda path: hashlib.sha256(path.read_bytes()).hexdigest()
receipt = {'sourceRevision': revision, 'sourceTree': subprocess.check_output(['git', 'rev-parse', revision + '^{tree}'], cwd=root, text=True).strip(),
           'harnessRevision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True).strip(),
           'testSHA256': checksum(root / 'apps/player/e2e/test-instance-positive-reentry.spec.ts'),
           'selectorControlTestSHA256': checksum(root / 'apps/player/e2e/test-instance-positive-selector.spec.ts'),
           'selectorAdmissionHelperSHA256': checksum(root / 'apps/player/e2e/positive-selector-admission.ts'),
           'runnerSHA256': checksum(Path(__file__)), 'processHelperSHA256': checksum(Path(__file__).with_name('positive_reentry_processes.py')),
           'tlsHelperSHA256': checksum(Path(__file__).with_name('positive_reentry_tls.py')), 'result': 'not-run', 'minimumFreeBytes': 3 * 1024**3,
           'boundaries': 'Immutable git export; verified loopback HTTPS; synthetic Owner/media; real macOS WebKit HLS in iPhone context; next-episode autoplay disabled; disposable hosted CA only; no production edits/Nox/UI/device proof'}
free = shutil.disk_usage(run).free
receipt['availableBytesBeforeRun'] = free
if free < receipt['minimumFreeBytes']:
    receipt['blocker'] = 'Insufficient disk headroom; no source export, build, encoder, browser or server started'
    (run / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print(json.dumps({'result': receipt['result'], 'blocker': receipt['blocker'], 'availableBytes': free, 'receipt': str(run / 'receipt.json')}))
    raise SystemExit(2)
require_hosted_macos()

dependencies = (args.dependencies or root.parent / 'kinosail/apps/player/e2e/node_modules').resolve()
package = dependencies / '@playwright/test/package.json'
expected_version = json.loads((root / 'apps/player/e2e/package.json').read_text())['devDependencies']['@playwright/test']
if not package.is_file() or json.loads(package.read_text()).get('version') != expected_version:
    receipt['blocker'] = 'Existing pinned Playwright dependencies are unavailable; no build started'
    (run / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    raise SystemExit(2)
borrowed_link = root / 'apps/player/e2e/node_modules'
borrowed_link_created = False
if borrowed_link.exists() or borrowed_link.is_symlink():
    if borrowed_link.resolve() != dependencies:
        raise RuntimeError('Preserve existing dependency path; pass its exact --dependencies value')
    borrowed_link = None

# Observe actual Node spawn/fork calls, including Playwright's detached browser.
# Record only owned PID ancestry and creation identity, never command arguments.
observer = run / 'owned-node-processes.cjs'
process_log = run / 'owned-node-processes.jsonl'
observer.write_text('''const cp = require('node:child_process');
const fs = require('node:fs');
for (const method of ['spawn', 'fork']) {
  const original = cp[method];
  cp[method] = function (...args) {
    const child = original.apply(this, args);
    child.once('spawn', () => {
      let identity = '', groupPID = 0;
      try {
        const row = cp.execFileSync('/bin/ps', ['-p', String(child.pid), '-o', 'pgid=,lstart='], {encoding:'utf8'}).trim();
        const match = /^(\\d+)\\s+(.+)$/.exec(row);
        if (match) { groupPID = Number(match[1]); identity = match[2]; }
      } catch {}
      fs.appendFileSync(process.env.KINOSAIL_OWNED_NODE_PROCESS_LOG, JSON.stringify({parentPID:process.pid,childPID:child.pid,groupPID,startIdentity:identity})+'\\n');
    });
    return child;
  };
}
''')
process_log.write_text('')

source = run / 'immutable-source'
receipt['result'] = 'failed'
receipt['stage'] = 'immutable-source-export'
(run / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
source.mkdir()
archive = subprocess.Popen(['git', 'archive', revision], cwd=root, stdout=subprocess.PIPE)
try:
    with tarfile.open(fileobj=archive.stdout, mode='r|') as stream:
        stream.extractall(source, filter='data')
finally:
    archive.stdout.close()
    if archive.wait() != 0:
        receipt['blocker'] = 'Immutable source export failed before build'
        (run / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
        raise RuntimeError('Immutable source export failed')
binary = run / 'kinosail-player'
media = run / 'media'
media.mkdir()
build = ['go', 'build', '-p', '1', '-o', str(binary), './cmd/kinosail']
environment = {key: os.environ[key] for key in ['PATH', 'HOME', 'TMPDIR', 'TMP', 'TEMP', 'LANG', 'LC_ALL', 'LC_CTYPE', 'TZ'] if key in os.environ}
environment.update(GOMAXPROCS='2', GOENV='off', GOFLAGS='', GOWORK=str(source / 'go.work') if (source / 'go.work').is_file() else 'off')
receipt['controlledGoEnvironment'] = {key: environment[key] for key in ['GOMAXPROCS', 'GOENV', 'GOFLAGS', 'GOWORK']}
receipt['build'] = build
generate = ['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', '-f', 'lavfi', '-i',
            'color=c=blue:s=320x180:r=24:d=4', '-f', 'lavfi', '-i', 'color=c=yellow:s=320x180:r=24:d=20',
            '-f', 'lavfi', '-i', 'sine=frequency=440:sample_rate=48000:duration=24', '-filter_complex_threads', '1',
            '-filter_complex', '[0:v][1:v]concat=n=2:v=1:a=0[v]', '-map', '[v]', '-map', '2:a',
            '-c:v', 'ffv1', '-threads', '1', '-c:a', 'pcm_s16le', str(media / 'Positive Reentry.mkv')]
receipt['mediaCommand'] = generate
selector_media = media / 'Positive Selector.mkv'
selector_generate = ['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', '-i', str(media / 'Positive Reentry.mkv'),
                     '-map', '0', '-c', 'copy', '-metadata', 'title=Positive Selector', str(selector_media)]
receipt['selectorMediaCommand'] = selector_generate
zero_media = media / 'Positive Zero.mkv'
zero_generate = ['ffmpeg', '-nostdin', '-hide_banner', '-loglevel', 'error', '-i', str(media / 'Positive Reentry.mkv'),
                 '-map', '0', '-c', 'copy', '-metadata', 'title=Positive Zero', str(zero_media)]
receipt['zeroMediaCommand'] = zero_generate
server = browser = None
trust = HostedFixtureTrust(run, receipt)

def project_browser_failure():
    """Project fixed error classes only; no messages, URLs, code or credentials."""
    report = run / 'report/results-webkit.json'
    projection = {'sourceRevision': revision, 'harnessRevision': receipt['harnessRevision'],
                  'reportPresent': report.is_file(), 'tests': [],
                  'boundary': 'Fixed diagnostic flags only; private reporter and process log remain private'}
    if report.is_file():
        projection['reportSHA256'] = checksum(report)
        try:
            parsed = json.loads(report.read_text())
            suites = list(parsed.get('suites', []))
            while suites and len(projection['tests']) < 4:
                suite = suites.pop(0)
                suites.extend(suite.get('suites', []))
                for spec in suite.get('specs', []):
                    for test_result in spec.get('tests', []):
                        for result in test_result.get('results', [])[:1]:
                            errors = []
                            for error in result.get('errors', [])[:4]:
                                message = error.get('message', '')
                                errors.append({
                                    'timeout': bool(re.search(r'timeout|Timeout', message)),
                                    'strictLocator': 'strict mode violation' in message,
                                    'notEditable': bool(re.search(r'not editable|readonly|read-only', message)),
                                    'targetClosed': 'has been closed' in message,
                                    'conditionalCredentialInit': 'PublicKeyCredential' in message or 'isConditionalMediationAvailable' in message,
                                    'operations': [value for value in ['page.goto', 'locator.fill', 'locator.click', 'page.waitForURL', 'page.evaluate', 'expect.poll'] if value in message],
                                    'knownLocators': [value for value in ['Name', 'Password', 'Authentication or recovery code', 'Sign in'] if re.search(r'[\'\"]' + re.escape(value) + r'[\'\"]', message)],
                                    'waitingFor': [value for value in ['visible', 'enabled', 'editable', 'stable', 'navigation'] if value in message],
                                    'testLine': error.get('location', {}).get('line') if isinstance(error.get('location', {}).get('line'), int) else None})
                            projection['tests'].append({'status': result.get('status') if result.get('status') in ['passed', 'failed', 'timedOut', 'skipped', 'interrupted'] else 'unknown',
                                                        'durationMS': result.get('duration') if isinstance(result.get('duration'), (int, float)) else None, 'errors': errors})
        except (OSError, ValueError, TypeError, AttributeError) as error:
            projection['projectionFailureClass'] = type(error).__name__
    (run / 'browser-admission.json').write_text(json.dumps(projection, indent=2) + '\n')

try:
    receipt['stage'] = 'build'
    with (run / 'build-private.log').open('w') as output:
        run_owned_command(build, environment, timeout=180, cwd=source / 'apps/player', output=output)
    receipt['binarySHA256'] = checksum(binary)
    receipt['stage'] = 'synthetic-media-generation'
    run_owned_command(generate, environment, timeout=60)
    receipt['mediaSHA256'] = checksum(media / 'Positive Reentry.mkv')
    run_owned_command(selector_generate, environment, timeout=30)
    receipt['selectorMediaSHA256'] = checksum(selector_media)
    run_owned_command(zero_generate, environment, timeout=30)
    receipt['zeroMediaSHA256'] = checksum(zero_media)
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        port = listener.getsockname()[1]
    url = f'https://localhost:{port}'
    environment.update(KINOSAIL_LISTEN=f'127.0.0.1:{port}', KINOSAIL_AUTH_URL=url, KINOSAIL_TLS_ENABLED='true',
                       KINOSAIL_DATA_DIR=str(run / 'data'), KINOSAIL_MEDIA_DIR=str(media), KINOSAIL_CACHE_DIR=str(run / 'cache'),
                       KINOSAIL_BACKUP_DIR=str(run / 'backups'), KINOSAIL_BACKUP_KEY=os.urandom(32).hex(),
                       KINOSAIL_SCAN_INTERVAL='24h', KINOSAIL_BACKUP_INTERVAL='24h')
    receipt['stage'] = 'hosted-fixture-tls'
    tls_context = trust.prepare(binary, environment, source)
    with (run / 'server-private.log').open('w') as log:
        receipt['stage'] = 'server-readiness'
        server = subprocess.Popen([str(binary)], cwd=source, env=environment, stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
        receipt['serverPID'] = server.pid
        for _ in range(100):
            if server.poll() is not None:
                raise RuntimeError('Owned Server exited before readiness')
            try:
                with urllib.request.urlopen(url + '/healthz', timeout=1, context=tls_context) as response:
                    if json.load(response) == {'status': 'ok'}:
                        break
            except (OSError, ValueError):
                time.sleep(.2)
        else:
            raise RuntimeError('Owned Server readiness failed')

        def request(path, body, token='', method='POST'):
            headers = {'Content-Type': 'application/json'}
            if token:
                headers['Authorization'] = 'Bearer ' + token
            value = urllib.request.Request(url + path, method=method, data=json.dumps(body).encode(), headers=headers)
            with urllib.request.urlopen(value, timeout=10, context=tls_context) as response:
                return json.load(response) if response.status != 204 else None

        password = 'synthetic-positive-' + os.urandom(8).hex()
        created = request('/api/v1/setup', {'name': 'Owner', 'password': password, 'device': 'Disposable reentry proof', 'totp': True})
        token, secret = created['token'], created['totp']['secret']
        digest = hmac.new(base64.b32decode(secret), int(time.time() // 30).to_bytes(8, 'big'), hashlib.sha1).digest()
        offset = digest[-1] & 15
        code = str((int.from_bytes(digest[offset:offset + 4], 'big') & 0x7fffffff) % 1000000).zfill(6)
        request('/api/v1/me/mfa', {'code': code}, token, 'PUT')
        request('/api/v1/settings/onboarding', {'enabled': False}, token, 'PUT')
        (run / 'source-receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
        environment.update(KINOSAIL_TEST_INSTANCE='1', KINOSAIL_TEST_TOTP_SECRET=secret, KINOSAIL_E2E_OWNER_PASSWORD=password,
                           KINOSAIL_E2E_URL=url, KINOSAIL_POSITIVE_REENTRY_E2E='1', KINOSAIL_POSITIVE_SOURCE_RECEIPT=str(run / 'source-receipt.json'),
                           KINOSAIL_E2E_VIDEO='off', KINOSAIL_BROWSER_WORKERS='1', KINOSAIL_BROWSER_PROJECT='webkit',
                           KINOSAIL_E2E_OUTPUT_DIR=str(run / 'artifacts'), KINOSAIL_E2E_ARTIFACT_DIR=str(run / 'report'))
        environment['NODE_OPTIONS'] = '--require=' + json.dumps(str(observer))
        environment['KINOSAIL_OWNED_NODE_PROCESS_LOG'] = str(process_log)
        if borrowed_link is not None:
            borrowed_link.symlink_to(dependencies, target_is_directory=True)
            borrowed_link_created = True
        command = ['node', 'node_modules/@playwright/test/cli.js', 'test', 'test-instance-positive-reentry.spec.ts', 'test-instance-positive-selector.spec.ts', '--project=webkit', '--workers=1', '--retries=0', '--headed']
        receipt['stage'] = 'decoded-browser-check'
        receipt['browserCommand'] = command
        with (run / 'browser-private.log').open('w') as log:
            browser = subprocess.Popen(command, cwd=root / 'apps/player/e2e', env=environment, stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
            receipt['browserPID'] = browser.pid
            exit_code = browser.wait(timeout=300)
        receipt['browserExitCode'] = exit_code
        project_browser_failure()
        receipt['result'] = 'passed' if exit_code == 0 else 'failed'
finally:
    for key, process in [('ownedBrowserGroupExited', browser), ('ownedServerGroupExited', server)]:
        try:
            receipt[key] = stop_owned_group(process)
        except (OSError, subprocess.TimeoutExpired) as error:
            receipt[key] = False
            receipt[key + 'FailureClass'] = type(error).__name__
    try:
        receipt['observedNodeChildrenExited'] = stop_observed_node_children(browser, process_log, receipt)
    except (OSError, ValueError, RuntimeError) as error:
        receipt['observedNodeChildrenExited'] = False
        receipt['observedNodeChildrenFailureClass'] = type(error).__name__
    if borrowed_link_created and borrowed_link.is_symlink() and borrowed_link.resolve() == dependencies:
        borrowed_link.unlink()
    try:
        receipt['ownedFixtureTrustRemoved'] = trust.cleanup(environment)
    except (OSError, RuntimeError, subprocess.SubprocessError) as error:
        receipt['ownedFixtureTrustRemoved'] = False
        receipt['fixtureTrustCleanupFailureClass'] = type(error).__name__
    if not receipt['ownedBrowserGroupExited'] or not receipt['ownedServerGroupExited'] or not receipt['observedNodeChildrenExited'] or not receipt['ownedFixtureTrustRemoved']:
        receipt['result'] = 'failed'
        receipt['blocker'] = 'Owned process or fixture trust cleanup did not finish'
    receipt['binaryUnchanged'] = checksum(binary) == receipt.get('binarySHA256') if binary.exists() else False
    receipt['completedUTC'] = datetime.datetime.now(datetime.timezone.utc).isoformat()
    receipt['artifactChecksums'] = {str(path.relative_to(run)): checksum(path) for path in run.rglob('*')
        if path.is_file() and path.parts[-1] not in {'receipt.json', 'source-receipt.json'}
        and not any(part in {'immutable-source', 'data', 'cache', 'backups'} for part in path.relative_to(run).parts)}
    (run / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print(json.dumps({'result': receipt['result'], 'receipt': str(run / 'receipt.json')}))
raise SystemExit(0 if receipt['result'] == 'passed' else 1)
