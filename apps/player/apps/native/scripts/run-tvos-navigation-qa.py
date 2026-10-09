"""Opt-in, serial native UI verification with a disposable loopback catalog."""
import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import time
import urllib.request
import uuid

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--device', required=True, type=uuid.UUID)
parser.add_argument('--media', required=True, type=Path, help='Generated MP4, never original library media')
parser.add_argument('--evidence', required=True, type=Path)
parser.add_argument('--mode', choices=['loaded', 'pending', 'empty', 'failed'], default='loaded')
args = parser.parse_args()
native = Path(__file__).resolve().parent.parent
movie = args.media.resolve(strict=True)
if not movie.is_file() or not 0 < movie.stat().st_size <= 32 * 1024 * 1024:
    parser.error('The generated MP4 must be a regular file of at most 32 MiB.')
output = args.evidence.resolve()
output.mkdir(parents=True, exist_ok=False)
revision = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=native, text=True).strip()
if subprocess.check_output(['git', 'status', '--porcelain', '--', '.'], cwd=native):
    revision += '-working-tree'
identity = 'tv-polish-qa-' + uuid.uuid4().hex
environment = dict(os.environ)
for name, value in {'KINOSAIL_TV_QUALITY_SEED': '1', 'KINOSAIL_TV_FIXTURE_QA': '1',
                    'KINOSAIL_TV_QUALITY_FIXTURE_MODE': args.mode,
                    'KINOSAIL_TV_QUALITY_FIXTURE_ID': identity}.items():
    environment['TEST_RUNNER_' + name] = value
inputs = [p for folder in ['Sources', 'Tests', 'RemoteUITests', 'scripts']
          for p in (native / folder).rglob('*') if p.is_file() and p.suffix in ['.swift', '.py']]
device = str(args.device).upper()
manifest = {'revision': revision, 'device': device, 'mode': args.mode,
            'fixtureIdentity': identity, 'generatedMediaSHA256': hashlib.sha256(movie.read_bytes()).hexdigest(),
            'inputs': {str(p.relative_to(native)): hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(inputs)}}
(output / 'source.json').write_text(json.dumps(manifest, indent=2) + '\n')
base = ['xcodebuild', '-project', 'Kinosail.xcodeproj', '-configuration', 'Debug',
        '-destination', 'platform=tvOS Simulator,id=' + device,
        '-derivedDataPath', '.build/tvos-simulator', '-parallel-testing-enabled', 'NO',
        '-maximum-concurrent-test-simulator-destinations', '1', '-test-timeouts-enabled', 'YES',
        '-default-test-execution-time-allowance', '600', '-maximum-test-execution-time-allowance', '630',
        '-collect-test-diagnostics', 'never', 'CODE_SIGNING_ALLOWED=YES', 'CODE_SIGN_IDENTITY=-',
        'KINOSAIL_SOURCE_REVISION=' + revision]
receipt = {'startedAt': datetime.datetime.now(datetime.timezone.utc).isoformat(),
           'fixtureHealthFailures': [], 'phases': [], 'state': 'starting'}

def save():
    (output / 'run.json').write_text(json.dumps(receipt, indent=2) + '\n')

def health():
    with urllib.request.urlopen('http://127.0.0.1:38359/qa/health', timeout=2) as response:
        if json.load(response)['status'] != 'ready':
            raise RuntimeError('Fixture unavailable')

def settle(process):
    for action, allowance in [(lambda: process.send_signal(signal.SIGINT), 10),
                              (process.terminate, 5), (process.kill, 5)]:
        if process.poll() is not None:
            return
        action()
        try:
            process.wait(timeout=allowance)
        except subprocess.TimeoutExpired:
            continue
    if process.poll() is None:
        raise RuntimeError('Task-owned process did not join after bounded stop attempts')

def run(name, scheme, selection, timeout):
    command = base + ['-scheme', scheme, '-resultBundlePath', str(output / (name + '.xcresult'))] + selection + ['test']
    with (output / (name + '.log')).open('w') as log:
        process = subprocess.Popen(command, cwd=native, env=environment, stdout=log,
                                   stderr=subprocess.STDOUT, start_new_session=True)
        phase = {'name': name, 'command': command, 'pid': process.pid, 'timedOut': False}
        receipt['phases'].append(phase)
        started = time.monotonic()
        try:
            while process.poll() is None:
                if time.monotonic() - started > timeout:
                    phase['timedOut'] = True
                    break
                try:
                    health()
                    if fixture.poll() is not None:
                        raise RuntimeError('Fixture exited')
                    receipt['lastFixtureHealthAt'] = datetime.datetime.now(datetime.timezone.utc).isoformat()
                except Exception as error:
                    receipt['fixtureHealthFailures'].append(type(error).__name__)
                    break
                save()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    pass
        finally:
            try:
                settle(process)
            finally:
                phase.update(exitCode=process.returncode, elapsedSeconds=time.monotonic() - started,
                             processJoined=process.poll() is not None)
                save()
    return 124 if phase['timedOut'] or receipt['fixtureHealthFailures'] else process.returncode

with (output / 'fixture.log').open('w') as log:
    fixture = subprocess.Popen([sys.executable, '-u', str(native / 'scripts/tvos-navigation-fixture.py'), str(movie)],
                               stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
    receipt['fixturePID'] = fixture.pid
    save()
    try:
        for attempt in range(20):
            if fixture.poll() is not None:
                raise RuntimeError('Fixture exited before readiness; check whether port 38359 is occupied')
            try:
                health()
                break
            except Exception:
                if attempt == 19:
                    raise
                time.sleep(0.1)
        receipt['state'] = 'running'
        code = run('seed', 'Kinosail-tvOS', ['-only-testing:Kinosail-tvOSTests/TVQualitySessionSeed'], 150)
        if code == 0:
            if args.mode == 'loaded':
                selection = ['-only-testing:Kinosail-tvOSRemoteUITests/' + case for case in [
                    'RemoteQualityBaselineTests', 'RemoteHomeScrollingTests',
                    'RemoteDetailMovementTests/testHomeDirectionalRowsAndLeftEdge',
                    'RemoteMovementTests/testTopBarAndBrowseReturnPaths', 'RemotePlaybackOptionsTests']]
            else:
                method = {'empty': 'testEmptyHomeBackReachesNavigation',
                          'failed': 'testFailedHomeBackAndRetryRemainUsable',
                          'pending': 'testPendingHomeKeepsNavigationReachableAndRemovesSkeleton'}[args.mode]
                selection = ['-only-testing:Kinosail-tvOSRemoteUITests/RemoteHomeStateTests/' + method]
            code = run('remote', 'Kinosail-tvOS-Remote', selection, 780 if args.mode == 'loaded' else 120)
    finally:
        try:
            settle(fixture)
        finally:
            receipt.update(fixtureExitCode=fixture.returncode, fixtureJoined=fixture.poll() is not None,
                           state='completed' if fixture.poll() is not None else 'failed')
            save()
print(json.dumps(receipt))
sys.exit(code)
