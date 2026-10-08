#!/usr/bin/env python3
"""Synthetic startup E2E with repeatable artifacts and supported loopback HTTP."""
import hashlib
import json
import os
import platform
from pathlib import Path
import socket
import subprocess
import time
import threading
import sys
from hls_presented_fixture import proof_enabled, build_fixture

try:
    presentation_proof = proof_enabled(os.environ)
except ValueError:
    print('invalid presentation proof option', file=sys.stderr)
    raise SystemExit(2)

root = Path(__file__).resolve().parents[3]
run = root / '.verification/startup' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
run.mkdir(parents=True, mode=0o700 if presentation_proof else 0o755)
media = run / 'media'
media.mkdir()
binary = run / 'kinosail-player'
env = dict(os.environ, GOCACHE='/tmp/kinosail-apple-go-cache')
revision = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True).strip()
working_diff = hashlib.sha256(subprocess.check_output(['git', 'diff', 'HEAD'], cwd=root)).hexdigest()
source_inputs = ['.github/workflows/app.yml', 'apps/player/scripts/test-startup-local.py', 'apps/player/scripts/hls_presented_fixture.py', 'apps/player/e2e/test-instance-startup.spec.ts', 'apps/player/e2e/hls-presented-seek.ts', 'apps/player/e2e/hls-presented-frame.mjs', 'apps/player/e2e/hls-presentation-state.mjs', 'apps/player/e2e/apple-playback.config.ts', 'apps/player/e2e/happy-path-helpers.ts', 'apps/player/e2e/startup-playback-exit.ts', 'apps/player/e2e/offline-browser-api.mjs', 'scripts/e2e/fixture-response.mjs', 'packages/webassets/static/player-core.js', 'packages/webassets/static/player-controls.js', 'packages/webassets/static/player-progress.js', 'apps/player/internal/server/static/player-streaming-adaptive.js', 'apps/player/internal/server/static/player-streaming-recovery.js', 'apps/player/internal/server/hls.go', 'apps/player/internal/server/hls_presentation.go', 'apps/player/internal/server/hls_seek.go', 'apps/player/internal/server/hls_playlist.go']
source_hashes = {name: hashlib.sha256((root / name).read_bytes()).hexdigest() for name in source_inputs}
build = ['go', '-C', 'apps/player', 'build', '-p=1', '-o', str(binary), './cmd/kinosail']
subprocess.run(build, cwd=root, env=env, check=True)
hdr = ['ffmpeg', '-nostdin', '-v', 'error', '-f', 'lavfi', '-i', 'testsrc2=s=1280x720:r=24:d=64',
       '-f', 'lavfi', '-i', 'sine=frequency=440:sample_rate=48000:duration=64', '-c:v', 'libx265',
       '-preset', 'ultrafast', '-crf', '32', '-pix_fmt', 'yuv420p10le', '-threads', '2',
       '-x265-params', 'pools=2:frame-threads=2:keyint=48:min-keyint=48:scenecut=0:log-level=error',
       '-color_primaries', 'bt2020', '-color_trc', 'smpte2084', '-colorspace', 'bt2020nc',
       '-c:a', 'eac3', str(media / 'Cold.mkv')]
direct = ['ffmpeg', '-nostdin', '-v', 'error', '-f', 'lavfi', '-i', 'testsrc2=s=640x360:r=24:d=64',
          '-f', 'lavfi', '-i', 'sine=frequency=440:duration=64', '-c:v', 'libx264', '-threads', '2',
          '-preset', 'ultrafast', '-crf', '32', '-pix_fmt', 'yuv420p', '-c:a', 'aac',
          '-movflags', '+faststart', str(media / 'Direct.mp4')]
for recipe in [hdr, direct]:
    subprocess.run(recipe, check=True)
presentation_fixture = build_fixture(media, run) if presentation_proof else None
for name in ['Warm.mkv', 'Adopt.mkv', 'Compete.mkv', 'Invalidation.mkv']:
    os.link(media / 'Cold.mkv', media / name)
for name in ['Cold', 'Warm', 'Adopt', 'Compete', 'Invalidation', 'Direct']:
    (media / (name + '.en.srt')).write_text('1\n00:00:00,000 --> 00:01:03,000\nSynthetic caption.\n')
with socket.socket() as listener:
    listener.bind(('127.0.0.1', 0))
    port = listener.getsockname()[1]
url = f'http://localhost:{port}'
env.update(KINOSAIL_LISTEN=f'127.0.0.1:{port}', KINOSAIL_AUTH_URL=url, KINOSAIL_TLS_ENABLED='false',
           KINOSAIL_DATA_DIR=str(run / 'config'), KINOSAIL_MEDIA_DIR=str(media),
           KINOSAIL_CACHE_DIR=str(run / 'cache'), KINOSAIL_BACKUP_DIR=str(run / 'backups'),
           KINOSAIL_BACKUP_KEY='synthetic-startup-key', KINOSAIL_STARTUP_E2E='1',
           KINOSAIL_E2E_URL=url, KINOSAIL_TEST_REVISION=revision,
           KINOSAIL_STARTUP_BURN_SUPPORTED='1' if ' subtitles ' in subprocess.check_output(['ffmpeg', '-hide_banner', '-filters'], stderr=subprocess.DEVNULL, text=True) else '0',
           KINOSAIL_STARTUP_RUN=str(run), KINOSAIL_E2E_OUTPUT_DIR=str(run / 'results'),
           KINOSAIL_E2E_REPORT=str(run / 'results.json'))
browser = ['node', 'node_modules/@playwright/test/cli.js', 'test', 'test-instance-startup.spec.ts',
           '--config=apple-playback.config.ts', '--workers=1']
result = None
stop_samples = threading.Event()
resource = {'samples': 0, 'peakFFmpeg': 0, 'peakSpeculativeFFmpeg': 0, 'peakCPUPercent': 0, 'peakRSSKiB': 0}
def browser_failures():
    report = run / 'results.json'
    if not report.exists():
        return {'reportPresent': False}
    data = json.loads(report.read_text())
    failures = []
    def visit(suites):
        for suite in suites:
            for spec in suite.get('specs', []):
                for test in spec.get('tests', []):
                    for attempt in test.get('results', []):
                        if attempt.get('status') in {'failed', 'timedOut', 'interrupted'}:
                            failures.append({'status': attempt['status'], 'durationMS': attempt.get('duration'),
                                'locations': [{'file': Path(error.get('location', {}).get('file', spec.get('file', ''))).name,
                                    'line': error.get('location', {}).get('line', spec.get('line')),
                                    'column': error.get('location', {}).get('column', spec.get('column'))}
                                    for error in attempt.get('errors', [])]})
            visit(suite.get('suites', []))
    visit(data.get('suites', []))
    return {'reportPresent': True, 'stats': data.get('stats'), 'failures': failures}
def encoder_diagnostics():
    log = run / 'server.log'
    if not log.exists():
        return {}
    events = {}
    categories = set()
    patterns = {'filter-output': 'Failed to configure output pad', 'resource-unavailable': 'Resource temporarily unavailable',
        'encoder-initialization': 'Error initializing', 'encoder-opening': 'Error opening encoder',
        'unknown-encoder': 'Unknown encoder', 'missing-filter': 'No such filter', 'filter-network': 'Error reinitializing filters',
        'conversion-failed': 'Conversion failed', 'unsupported-option': 'Unrecognized option', 'permission': 'Permission denied'}
    allowed = {'HLS transcode started', 'HLS transcode failed', 'HLS transcode completed', 'HLS segment preparation failed', 'HLS segment unavailable'}
    for line in log.read_text().splitlines():
        try:
            entry = json.loads(line)
        except json.JSONDecodeError:
            continue
        if entry.get('msg') not in allowed:
            continue
        event = entry['msg']
        events[event] = events.get(event, 0) + 1
        detail = str(entry.get('error', ''))
        categories.update(category for category, pattern in patterns.items() if pattern.lower() in detail.lower())
        for kind in ['MASTERING_DISPLAY_METADATA', 'CONTENT_LIGHT_LEVEL', 'DYNAMIC_HDR_PLUS', 'DOVI_RPU_BUFFER', 'DOVI_METADATA']:
            if kind in detail and any(marker in detail.lower() for marker in ['undefined', 'invalid', 'parse', 'not found']):
                categories.add('unsupported-hdr-side-data-' + kind.lower())
    return {'events': events, 'errorCategories': sorted(categories)}
def sample_resources(pid):
    while not stop_samples.wait(0.2):
        rows = subprocess.check_output(['ps', '-axo', 'pid=,ppid=,pcpu=,rss=,args='], text=True).splitlines()
        selected = []
        for row in rows:
            fields = row.split(None, 4)
            if len(fields) == 5 and (int(fields[0]) == pid or int(fields[1]) == pid):
                selected.append(fields)
        encoders = [row for row in selected if 'ffmpeg' in row[4] and '-hls_time' in row[4]]
        resource['samples'] += 1
        resource['peakFFmpeg'] = max(resource['peakFFmpeg'], len(encoders))
        resource['peakSpeculativeFFmpeg'] = max(resource['peakSpeculativeFFmpeg'], sum('-readrate 4' in row[4] for row in encoders))
        resource['peakCPUPercent'] = max(resource['peakCPUPercent'], sum(float(row[2]) for row in selected))
        resource['peakRSSKiB'] = max(resource['peakRSSKiB'], sum(int(row[3]) for row in selected))
        (run / 'resources.json').write_text(json.dumps(resource, indent=2))
try:
    with (run / 'server.log').open('w') as log:
        server = subprocess.Popen([str(binary)], env=env, stdout=log, stderr=log)
        samples = threading.Thread(target=sample_resources, args=(server.pid,), daemon=True)
        samples.start()
        try:
            with (run / 'browser.log').open('w') as log_browser:
                result = subprocess.run(browser, cwd=root / 'apps/player/e2e', env=env,
                                        stdout=log_browser, stderr=subprocess.STDOUT).returncode
        finally:
            stop_samples.set()
            samples.join(timeout=2)
            server.terminate()
            try:
                server.wait(timeout=10)
            except subprocess.TimeoutExpired:
                server.kill()
                server.wait()
finally:
    checksum = lambda path: hashlib.sha256(path.read_bytes()).hexdigest()
    metadata = json.loads(subprocess.check_output(['ffprobe', '-v', 'error', '-show_streams', '-show_format', '-of', 'json', str(media / 'Cold.mkv')]))
    metadata.get('format', {}).pop('filename', None)
    (run / 'receipt.json').write_text(json.dumps({'revision': env['KINOSAIL_TEST_REVISION'], 'result': result,
        'workingDiffSHA256': working_diff, 'resources': resource, 'browserResults': browser_failures(), 'encoderDiagnostics': encoder_diagnostics(),
        'command': 'python3 apps/player/scripts/test-startup-local.py', 'build': build, 'browser': browser,
        'encoderVersions': {tool: subprocess.check_output([tool, '-version'], text=True).splitlines()[0] for tool in ['ffmpeg', 'ffprobe']},
        'baseline': env.get('KINOSAIL_STARTUP_BASELINE') == '1', 'mediaCommands': [hdr, direct],
        'presentationProof': presentation_proof, 'presentationFixture': presentation_fixture,
        'sourceSHA256': source_hashes, 'sourceDrift': any(hashlib.sha256((root / name).read_bytes()).hexdigest() != digest for name, digest in source_hashes.items()),
        'fixtureMetadata': metadata, 'binarySHA256': checksum(binary),
        'environment': f'{platform.system()} {platform.machine()} {env.get("KINOSAIL_STARTUP_BROWSER_CHANNEL", "chrome")}; serial Go build; supported HTTP loopback; synthetic media',
        'boundary': '720p HEVC Main 10 PQ/EAC3 is codec-representative, not movie/Nox workload-equivalent. No device or production proof.',
        'mediaSHA256': {p.name: checksum(p) for p in media.iterdir()}}, indent=2) + '\n')
    (run / 'SHA256SUMS').write_text(''.join(f'{checksum(p)}  {p.relative_to(run)}\n' for p in sorted(run.rglob('*'))
        if p.is_file() and p.name not in {'SHA256SUMS', 'presentation-auth.json'} and not any(part in {'config', 'backups'} for part in p.relative_to(run).parts)))
    print(f'E2E artifact: {run}', flush=True)
raise SystemExit(result if result is not None else 1)
