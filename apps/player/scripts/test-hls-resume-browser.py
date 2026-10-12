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

from hls_timeline_fixture import LONG_RESUME_KEYS, fixture
from hls_resume_audio import audio_packets, uninterrupted_audio_reference
from hls_timeline_http import PublicServer, sha, source_state
from hls_timeline_preparation import prepare_scene
from hls_followon_public import bounded_bytes, check, encoder_count

ROOT = Path(__file__).resolve().parents[3]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--engines', choices=['chromium', 'webkit', 'webkit-native', 'both'], default='both')
parser.add_argument('--prepared', action='store_true')
parser.add_argument('--long-remux', action='store_true', help='Use a 74.5-second source with irregular source keyframes')
parser.add_argument('--resume-seconds', type=float, choices=[.5, 2.5, 12, 12.5, 64.5], default=12.5)
parser.add_argument('--force-audio-conversion', action='store_true')
parser.add_argument('--audio-sample-rate', type=int, choices=[32000, 44100, 48000], default=48000)
parser.add_argument('--audio-mode', choices=['aac', 'ac3', 'none'], default='aac')
parser.add_argument('--audio-reference', action='store_true', help='Compare converted AAC with uninterrupted encoding')
parser.add_argument('--tail-recovery', action='store_true')
parser.add_argument('--first-recovery', action='store_true')
parser.add_argument('--controls', action='store_true', help='Run injected metadata failures and recovery controls')
args = parser.parse_args()
recovery = args.tail_recovery or args.first_recovery
if recovery and (not args.prepared or args.audio_mode == 'none') or args.tail_recovery and args.first_recovery:
    parser.error('choose one recovery target, with --prepared')
if args.audio_reference and (not recovery or args.audio_mode != 'ac3' or not args.force_audio_conversion):
    parser.error('--audio-reference requires converted-audio recovery')
if args.long_remux and (args.audio_mode != 'aac' or args.force_audio_conversion or args.resume_seconds != 12.5):
    parser.error('--long-remux uses AAC copied audio and the 12.5-second saved position')
reference_audio = None
fractional = args.resume_seconds == 2.5
frame_rate = 25 if fractional or args.resume_seconds == 64.5 else 24
duration = 74.5 if args.long_remux else 22.4 if fractional else 84 if args.resume_seconds == 64.5 else 18 if args.resume_seconds < 2 else 30.5 if args.tail_recovery else 32
gop = frame_rate * 6 if args.long_remux else 16 if fractional else frame_rate * 2
keys = LONG_RESUME_KEYS if args.long_remux else [n / frame_rate for n in range(0, round(duration * frame_rate), gop)]
source_keys = [point + (.13 if fractional else 0) for point in keys]
source_origin = max(point for point in source_keys if point <= args.resume_seconds)
fragment_count = sum(point >= source_origin for point in source_keys)
run = ROOT / '.verification/hls-resume-browser' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
media = run / 'media'
media.mkdir(parents=True)
engines = ['chromium', 'webkit'] if args.engines == 'both' else [args.engines]
build = ['go', 'build', '-p', '2', '-o', str(run / 'player'), './apps/player/cmd/kinosail']
receipt = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip(),
           'environment': platform.platform(), 'result': 'failed', 'cases': [], 'build': build,
           'command': 'python3 apps/player/scripts/test-hls-resume-browser.py --engines ' + args.engines + (' --prepared' if args.prepared else '') + (' --long-remux' if args.long_remux else '') + (' --tail-recovery' if args.tail_recovery else '') + (' --first-recovery' if args.first_recovery else '') + (' --controls' if args.controls else '') + ' --audio-sample-rate ' + str(args.audio_sample_rate) + ' --audio-mode ' + args.audio_mode + ' --resume-seconds ' + str(args.resume_seconds) + (' --force-audio-conversion' if args.force_audio_conversion else '') + (' --audio-reference' if args.audio_reference else ''),
           'prepared': args.prepared, 'longRemux': args.long_remux, 'audioMode': args.audio_mode, 'audioSampleRate': args.audio_sample_rate, 'resumeSeconds': args.resume_seconds, 'forceAudioConversion': args.force_audio_conversion,
           'boundary': 'Real loopback Player, generated media, saved progress and unmodified watch page; physical Safari and devices separate.'}


def audio_boundaries(rendition):
    """Check actual output clocks independently of browser video screenshots."""
    initialization = (rendition / 'init.mp4').read_bytes()
    rows, previous = [], None
    for fragment in sorted(rendition.glob('segment-*.m4s')):
        if reference_audio is not None:
            check(audio_packets(initialization, fragment) == reference_audio.get(fragment.name), 'converted_audio_content_changed')
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
    check(len(rows) == fragment_count, 'tail_fragment_count')
    check(all(r['gapSeconds'] is None or abs(r['gapSeconds']) <= .05 for r in rows), 'fragment_audio_discontinuity')
    if args.audio_mode == 'ac3':
        check(all(r['gapSeconds'] is None or r['gapSeconds'] <= .000002 for r in rows), 'converted_audio_gap')
    check(abs(rows[-1]['audioEnd'] - rows[-1]['videoEnd'] + (.13 if fractional else 0)) <= .05, 'tail_audio_video_end')
    return rows


try:
    source, metadata = fixture(media, 'Resume Sample', gop, ','.join(map(str, keys)),
                               rate=str(frame_rate), frames=round(duration * frame_rate), extension='.mp4')
    if args.audio_mode != 'aac':
        converted = run / source.name
        conversion = ['ffmpeg', '-nostdin', '-v', 'error', '-i', str(source), '-map', '0:v:0', '-c:v', 'copy']
        conversion += ['-map', '0:a:0', '-c:a', 'ac3', '-ar', str(args.audio_sample_rate)] if args.audio_mode == 'ac3' else ['-an']
        conversion += [str(converted)]
        subprocess.run(conversion, check=True, capture_output=True, timeout=60)
        converted.replace(source)
        metadata['audioConversionCommand'] = conversion
        metadata['sha256'] = sha(source)
    if fractional:
        shifted = source.with_suffix('.mkv')
        shift = ['ffmpeg', '-nostdin', '-v', 'error', '-i', str(source), '-itsoffset', '0.130',
                 '-i', str(source), '-map', '1:v:0', '-map', '0:a:0', '-c', 'copy', str(shifted)]
        subprocess.run(shift, check=True, capture_output=True, timeout=60)
        source.unlink()  # Only this run's generated input before Server startup.
        source = shifted
        metadata.update(shiftCommand=shift, sha256=sha(source), keyframesSeconds=source_keys,
                        durationSeconds=duration + .13, videoDurationSeconds=duration + .13)
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
                hls = plan['compatible'].replace('/index.m3u8', f'-o{round(args.resume_seconds * 1000)}/index.m3u8')
                receipt['preparation'] = {}
                prepared_root, prepared_init = prepare_scene(api, '/api/v1/items/' + item['id'] + '/playback-prepare', hls,
                                                            run / 'cache', item['id'], server, source, receipt['preparation'],
                                                            encoder_count, check, bounded_bytes, False)
            if args.audio_reference:
                reference_init, reference_audio, receipt['audioReference'] = uninterrupted_audio_reference(source, prepared_root, run / 'uninterrupted-audio')
                check(reference_init == prepared_init, 'reference_initialization_changed')
            for engine in engines:
                case = {'engine': engine}
                receipt['cases'].append(case)
                if recovery:
                    rendition = next(prepared_root.glob('*p/index.m3u8')).parent
                    base = hls.removesuffix('index.m3u8') + rendition.name + '/'
                    for number in range(fragment_count):
                        status, _, _ = api.http(base + f'segment-{number:05d}.m4s')
                        check(status == 200, 'tail_population_fragment_status')
                    limit = time.monotonic() + 15
                    while encoder_count(server, source) and time.monotonic() < limit:
                        time.sleep(.05)
                    check(encoder_count(server, source) == 0, 'tail_recovery_worker_not_joined')
                    # Derive the last fragment from the independent fixture key schedule.
                    target = 0 if args.first_recovery else fragment_count - 1
                    fragment = rendition / f'segment-{target:05d}.m4s'
                    case['evictedFragment'] = fragment.name
                    case['fragmentPresentBeforeEviction'] = fragment.exists()
                    check(case['fragmentPresentBeforeEviction'], 'eviction_requires_existing_fragment')
                    previous_fragment = sha(fragment)
                    stable_paths = [prepared_root / name for name in ['index.m3u8', '.copy-timeline', '.copy-clock']]
                    stable_paths.append(rendition / 'index.m3u8')
                    previous_metadata = [sha(path) for path in stable_paths]
                    case['audioBeforeEviction'] = audio_boundaries(rendition)
                    fragment.unlink()  # Only this run's disposable cache.
                    case['fragmentMissingBeforePlayback'] = not fragment.exists()
                    if args.tail_recovery:
                        case['tailPresentBeforeEviction'] = case['fragmentPresentBeforeEviction']
                        case['tailMissingBeforePlayback'] = case['fragmentMissingBeforePlayback']
                child = dict(os.environ, QA_RESUME_ORIGIN=origin, QA_RESUME_TOKEN=api.token,
                             QA_RESUME_ITEM=item['id'], QA_RESUME_OUTPUT=str(run / f'{engine}.json'),
                             QA_RESUME_ENGINE=engine, QA_RESUME_SECONDS=str(args.resume_seconds), QA_RESUME_FORCE_AUDIO_CONVERSION=str(args.force_audio_conversion).lower(), QA_RESUME_EXPECTED_MODE=('audio-transcode' if args.force_audio_conversion else 'audio-transcode,remux') if args.audio_mode == 'ac3' else 'remux', QA_RESUME_FRAME_RATE=str(frame_rate), QA_RESUME_DURATION=str(metadata['durationSeconds']))
                with (run / f'{engine}-private.log').open('w') as output:
                    script = 'hls-resume-controls.mjs' if args.controls else 'hls-resume-browser.mjs'
                    result = subprocess.run(['node', str(ROOT / 'apps/player/scripts' / script)],
                                            cwd=ROOT, env=child, stdout=output, stderr=subprocess.STDOUT, timeout=300)
                case['exitCode'] = result.returncode
                if recovery:
                    case.update(fragmentRestored=fragment.exists(), initializationUnchanged=(rendition / 'init.mp4').read_bytes() == prepared_init)
                    if case['fragmentRestored']:
                        case['audioAfterRecovery'] = audio_boundaries(rendition)
                    if args.first_recovery:
                        case['firstFragmentIdentical'] = fragment.exists() and sha(fragment) == previous_fragment
                        case['metadataUnchanged'] = [sha(path) for path in stable_paths] == previous_metadata
                        if not case['firstFragmentIdentical'] or not case['metadataUnchanged']:
                            case['exitCode'] = 1
                    else:
                        case['tailRestored'] = case['fragmentRestored']
                    if not case['fragmentRestored'] or not case['initializationUnchanged']:
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
