#!/usr/bin/env python3
"""Compare resumed compatible playback with the original in real browser decoders."""
import argparse
import json
import math
import os
from pathlib import Path
import platform
import socket
import subprocess
import time

from hls_timeline_fixture import fixture
from hls_timeline_http import PublicServer, sha, source_state
from hls_timeline_preparation import prepare_scene
from hls_followon_public import bounded_bytes, check, encoder_count

ROOT = Path(__file__).resolve().parents[3]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--engines', choices=['chromium', 'webkit', 'webkit-native', 'both'], default='both')
parser.add_argument('--prepared', action='store_true')
parser.add_argument('--tail-recovery', action='store_true')
parser.add_argument('--controls', action='store_true', help='Run injected metadata failures and recovery controls')
args = parser.parse_args()
if args.tail_recovery and not args.prepared:
    parser.error('--tail-recovery requires --prepared')
run = ROOT / '.verification/hls-resume-browser' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
media = run / 'media'
media.mkdir(parents=True)
engines = ['chromium', 'webkit'] if args.engines == 'both' else [args.engines]
build = ['go', 'build', '-p', '2', '-o', str(run / 'player'), './apps/player/cmd/kinosail']
receipt = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip(),
           'environment': platform.platform(), 'result': 'failed', 'cases': [], 'build': build,
           'command': 'python3 apps/player/scripts/test-hls-resume-browser.py --engines ' + args.engines + (' --prepared' if args.prepared else '') + (' --tail-recovery' if args.tail_recovery else '') + (' --controls' if args.controls else ''),
           'prepared': args.prepared,
           'boundary': 'Real loopback Player, generated media, saved progress and unmodified watch page; physical Safari and devices separate.'}


def audio_boundaries(rendition):
    """Check actual output clocks independently of browser video screenshots."""
    initialization = (rendition / 'init.mp4').read_bytes()
    rows, previous = [], None
    for fragment in sorted(rendition.glob('segment-*.m4s')):
        result = subprocess.run(['ffprobe', '-v', 'error', '-show_packets', '-show_entries',
            'packet=stream_index,pts_time,duration_time', '-of', 'json', '-i', 'pipe:0'],
            input=initialization + fragment.read_bytes(), capture_output=True, timeout=15, check=True)
        check(len(result.stdout) <= 1024 * 1024, 'fragment_packet_bound')
        packets = json.loads(result.stdout)['packets']
        video = [p for p in packets if p['stream_index'] == 0]
        audio = [p for p in packets if p['stream_index'] == 1]
        check(video and audio, 'fragment_missing_track')
        start = min(float(p['pts_time']) for p in audio)
        end = max(float(p['pts_time']) + float(p['duration_time']) for p in audio)
        video_end = max(float(p['pts_time']) + float(p['duration_time']) for p in video)
        gap = start - previous if previous is not None else None
        check(all(math.isfinite(v) for v in (start, end, video_end)), 'fragment_clock_nonfinite')
        rows.append({'fragment': fragment.name, 'audioPackets': len(audio), 'audioStart': start,
                     'audioEnd': end, 'videoEnd': video_end, 'gapSeconds': gap})
        previous = end
    check(len(rows) == 10, 'tail_fragment_count')
    check(all(r['gapSeconds'] is None or abs(r['gapSeconds']) <= .05 for r in rows), 'fragment_audio_discontinuity')
    check(abs(rows[-1]['audioEnd'] - rows[-1]['videoEnd']) <= .05, 'tail_audio_video_end')
    return rows


try:
    source, metadata = fixture(media, 'Resume Sample', 48, ','.join(str(n) for n in range(0, 32, 2)),
                               frames=732 if args.tail_recovery else 768, extension='.mp4')
    before = source_state(source)
    receipt.update(fixture=metadata, sourceBefore=before)
    with (run / 'build-private.log').open('w') as log:
        subprocess.run(build, cwd=ROOT, check=True, stdout=log, stderr=log, timeout=180)
    receipt['binarySHA256'] = sha(run / 'player')
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        port = listener.getsockname()[1]
    origin = f'http://localhost:{port}'
    env = dict(os.environ, KINOSAIL_LISTEN=f'127.0.0.1:{port}', KINOSAIL_AUTH_URL=origin,
               KINOSAIL_TLS_ENABLED='false', KINOSAIL_DATA_DIR=str(run / 'config'),
               KINOSAIL_MEDIA_DIR=str(media), KINOSAIL_CACHE_DIR=str(run / 'cache'),
               KINOSAIL_BACKUP_DIR=str(run / 'backups'), KINOSAIL_BACKUP_KEY='synthetic-resume-key')
    with (run / 'server-private.log').open('w') as log:
        server = subprocess.Popen([str(run / 'player')], cwd=ROOT, env=env, stdout=log, stderr=log)
        try:
            api, boot = PublicServer(origin), time.monotonic()
            # Startup on a busy development Mac is distinct from media readiness.
            while time.monotonic() - boot < 120:
                if server.poll() is not None:
                    raise RuntimeError('fixture_process_exited')
                try:
                    if api.http('/healthz', authenticated=False)[0] == 200:
                        break
                except OSError:
                    pass
                time.sleep(.2)
            else:
                raise RuntimeError('fixture_boot_timeout')
            receipt['bootSeconds'] = time.monotonic() - boot
            api.authorize()
            api.call('/api/v1/settings/subtitle-picker', 'PUT', {'limited': True})
            item = next(i for i in api.call('/api/v1/library')['items'] if i['title'] == 'Resume Sample')
            if args.prepared:
                plan = api.call('/api/v1/items/' + item['id'] + '/playback?videoCodecs=h264&audioCodecs=aac')
                hls = plan['compatible'].replace('/index.m3u8', '-o12500/index.m3u8')
                receipt['preparation'] = {}
                prepared_root, prepared_init = prepare_scene(api, '/api/v1/items/' + item['id'] + '/playback-prepare', hls,
                                                            run / 'cache', item['id'], server, source, receipt['preparation'],
                                                            encoder_count, check, bounded_bytes, False)
            for engine in engines:
                case = {'engine': engine}
                receipt['cases'].append(case)
                if args.tail_recovery:
                    rendition = next(prepared_root.glob('*p/index.m3u8')).parent
                    base = hls.removesuffix('index.m3u8') + rendition.name + '/'
                    for number in range(10):
                        status, _, _ = api.http(base + f'segment-{number:05d}.m4s')
                        check(status == 200, 'tail_population_fragment_status')
                    limit = time.monotonic() + 15
                    while encoder_count(server, source) and time.monotonic() < limit:
                        time.sleep(.05)
                    check(encoder_count(server, source) == 0, 'tail_recovery_worker_not_joined')
                    # The synthetic source independently defines ten keys from
                    # 12 through 30; its half-second tail is shorter than preroll.
                    tail = rendition / 'segment-00009.m4s'
                    case['tailPresentBeforeEviction'] = tail.exists()
                    check(case['tailPresentBeforeEviction'], 'tail_eviction_requires_existing_fragment')
                    case['audioBeforeEviction'] = audio_boundaries(rendition)
                    tail.unlink(missing_ok=True)  # Only this run's disposable cache.
                    case['tailMissingBeforePlayback'] = not tail.exists()
                child = dict(os.environ, QA_RESUME_ORIGIN=origin, QA_RESUME_TOKEN=api.token,
                             QA_RESUME_ITEM=item['id'], QA_RESUME_OUTPUT=str(run / f'{engine}.json'),
                             QA_RESUME_ENGINE=engine, QA_RESUME_DURATION=str(metadata['durationSeconds']))
                with (run / f'{engine}-private.log').open('w') as output:
                    script = 'hls-resume-controls.mjs' if args.controls else 'hls-resume-browser.mjs'
                    result = subprocess.run(['node', str(ROOT / 'apps/player/scripts' / script)],
                                            cwd=ROOT, env=child, stdout=output, stderr=subprocess.STDOUT, timeout=300)
                case['exitCode'] = result.returncode
                if args.tail_recovery:
                    case.update(tailRestored=tail.exists(), initializationUnchanged=(rendition / 'init.mp4').read_bytes() == prepared_init)
                    if case['tailRestored']:
                        case['audioAfterRecovery'] = audio_boundaries(rendition)
                    if not case['tailRestored'] or not case['initializationUnchanged']:
                        case['exitCode'] = 1
                print(json.dumps({k: v for k, v in case.items() if not k.startswith('audio')}), flush=True)
        finally:
            server.terminate()
            try:
                server.wait(timeout=15)
            except subprocess.TimeoutExpired:
                server.kill()
                server.wait()
            receipt.update(sourceAfter=source_state(source), sourceUnchanged=source_state(source) == before,
                           serverSettled=server.poll() is not None)
    if (len(receipt['cases']) == len(engines) and all(c['exitCode'] == 0 for c in receipt['cases'])
            and receipt['sourceUnchanged'] and receipt['serverSettled']):
        receipt['result'] = 'passed'
except Exception as error:
    receipt['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
finally:
    receipt['artifactsSHA256'] = {p.name: sha(p) for p in run.iterdir() if p.is_file() and p.name != 'player'}
    (run / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print(json.dumps({'receipt': str(run / 'receipt.json'), 'result': receipt['result']}), flush=True)
raise SystemExit(0 if receipt['result'] == 'passed' else 1)
