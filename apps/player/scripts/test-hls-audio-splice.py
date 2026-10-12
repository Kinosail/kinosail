#!/usr/bin/env python3
"""Verify long converted-audio splices through the populated Player watch page."""
import argparse
import json
import os
from pathlib import Path
import platform
import shlex
import socket
import subprocess
import sys
import time

from hls_audio_capture import capture_audio, decode_source, packet_grid
from hls_followon_public import bounded_bytes, check, encoder_count
from hls_timeline_fixture import LONG_RESUME_KEYS, fixture
from hls_timeline_http import PublicServer, sha, source_state
from hls_timeline_preparation import prepare_scene

ROOT = Path(__file__).resolve().parents[3]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--audio-sample-rate', type=int, choices=[8000, 32000, 44100, 48000], default=48000)
parser.add_argument('--recovery', choices=['tail', 'middle', 'early'], default='tail')
parser.add_argument('--video-offset', type=float, choices=[0, .13], default=0)
args = parser.parse_args()
run = ROOT / '.verification/hls-audio-splice' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
media = run / 'media'
media.mkdir(parents=True)
build = ['go', 'build', '-p', '2', '-o', str(run / 'player'), './apps/player/cmd/kinosail']
receipt = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip(),
           'environment': platform.platform(), 'result': 'failed', 'captures': [], 'build': build,
           'command': shlex.join([sys.executable, str(Path(__file__).resolve()), *sys.argv[1:]]),
           'audioSampleRate': args.audio_sample_rate, 'recovery': args.recovery,
           'videoOffset': args.video_offset,
           'boundary': 'Real disposable Go Player, generated AC3 or ALAC source, original watch page and Chromium AAC decode. Recorder uses a CSP exception; no security, native or perceptual-audibility claim.'}


def settled(server, source):
    limit = time.monotonic() + 15
    while encoder_count(server, source) and time.monotonic() < limit:
        time.sleep(.05)
    check(encoder_count(server, source) == 0, 'encoder_not_settled')


try:
    frame_rate, frames = (25, 1862) if args.video_offset else (24, 1788)
    source, metadata = fixture(media, 'Resume Sample', frame_rate * 6, ','.join(map(str, LONG_RESUME_KEYS)),
                               rate=str(frame_rate), frames=frames, extension='.mp4', noise=True)
    converted = run / source.name
    conversion = ['ffmpeg', '-nostdin', '-v', 'error', '-i', str(source), '-map', '0:v:0', '-c:v', 'copy',
                  '-map', '0:a:0', '-c:a', 'alac' if args.audio_sample_rate == 8000 else 'ac3', '-ar', str(args.audio_sample_rate), str(converted)]
    subprocess.run(conversion, check=True, capture_output=True, timeout=60)
    converted.replace(source)
    metadata.update(audioConversionCommand=conversion, sha256=sha(source))
    if args.video_offset:
        shifted = source.with_suffix('.mkv')
        command = ['ffmpeg', '-nostdin', '-v', 'error', '-i', str(source), '-itsoffset', str(args.video_offset),
                   '-i', str(source), '-map', '1:v:0', '-map', '0:a:0', '-c', 'copy', str(shifted)]
        subprocess.run(command, check=True, capture_output=True, timeout=60)
        source.unlink()  # Replace only this owned synthetic input before startup.
        source = shifted
        metadata.update(shiftCommand=command, sha256=sha(source), durationSeconds=frames / frame_rate + args.video_offset)
    before = source_state(source)
    receipt.update(fixture=metadata, sourceBefore=before)
    source_pcm, receipt['decodeCommand'] = decode_source(source)
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
               KINOSAIL_BACKUP_DIR=str(run / 'backups'), KINOSAIL_BACKUP_KEY='synthetic-splice-key')
    with (run / 'server-private.log').open('w') as log:
        server = subprocess.Popen([str(run / 'player')], cwd=ROOT, env=env, stdout=log, stderr=log)
        try:
            api = PublicServer(origin)
            deadline = time.monotonic() + 120
            while time.monotonic() < deadline:
                check(server.poll() is None, 'fixture_exited')
                try:
                    if api.http('/healthz', authenticated=False)[0] == 200:
                        break
                except OSError:
                    pass
                time.sleep(.2)
            else:
                raise RuntimeError('fixture_boot_timeout')
            api.authorize()
            api.call('/api/v1/settings/subtitle-picker', 'PUT', {'limited': True})
            item = next(i for i in api.call('/api/v1/library')['items'] if i['title'] == 'Resume Sample')
            plan = api.call('/api/v1/items/' + item['id'] + '/playback?videoCodecs=h264&audioCodecs=aac')
            hls = plan['compatible'].replace('/index.m3u8', '-o12500/index.m3u8')
            receipt['preparation'] = {}
            prepared, initialization = prepare_scene(api, '/api/v1/items/' + item['id'] + '/playback-prepare', hls,
                run / 'cache', item['id'], server, source, receipt['preparation'], encoder_count, check, bounded_bytes, False)
            rendition = next(prepared.glob('*p/index.m3u8')).parent
            keys = [key + args.video_offset for key in LONG_RESUME_KEYS if key >= 12]
            base = hls.removesuffix('index.m3u8') + rendition.name + '/'
            for number in range(len(keys)):
                check(api.http(base + f'segment-{number:05d}.m4s')[0] == 200, 'population_status')
            settled(server, source)
            receipt['initialGrid'] = packet_grid(rendition)
            target = len(keys) - 1 if args.recovery == 'tail' else 1 if args.recovery == 'early' else 10
            point, span = keys[target] - 1, 3 if args.recovery == 'tail' else keys[target + 1] - keys[target] + 2
            receipt['captures'].append(capture_audio(ROOT, run, api, item, 'initial-join', 20 + args.video_offset, 3, source_pcm))
            receipt['captures'].append(capture_audio(ROOT, run, api, item, 'control', point, span, source_pcm))
            fragment = rendition / f'segment-{target:05d}.m4s'
            check(fragment.exists(), 'eviction_requires_existing_fragment')
            neighbors = [rendition / f'segment-{n:05d}.m4s' for n in (target - 1, target + 1) if 0 <= n < len(keys)]
            saved = {p.name: bounded_bytes(p, 64 << 20, 'neighbor_bound') for p in neighbors}
            receipt['neighborHashesBefore'] = {p.name: sha(p) for p in neighbors}
            stable = [prepared / name for name in ['index.m3u8', '.copy-timeline', '.copy-clock']]
            stable += [rendition / 'index.m3u8', rendition / 'init.mp4']
            stable_hashes = {p.name + str(n): sha(p) for n, p in enumerate(stable)}
            fragment.unlink()  # Only the missing segment in this run's disposable cache.
            receipt['captures'].append(capture_audio(ROOT, run, api, item, 'recovered', point, span, source_pcm))
            settled(server, source)
            receipt['neighborHashesAfterRecovery'] = {p.name: sha(p) for p in neighbors}
            if args.recovery != 'tail':
                # A client may already hold the old following fragment. Restore
                # only these owned fixture bytes to test both old/new joins.
                for p in neighbors:
                    temporary = p.with_suffix('.qa-pending')
                    temporary.write_bytes(saved[p.name])
                    temporary.replace(p)
                receipt['captures'].append(capture_audio(ROOT, run, api, item, 'retained-neighbors', point, span, source_pcm))
                receipt['neighborHashesRetained'] = {p.name: sha(p) for p in neighbors}
                check(receipt['neighborHashesBefore'] == receipt['neighborHashesRetained'], 'retained_neighbor_changed')
            receipt['recoveredGrid'] = packet_grid(rendition)
            receipt['metadataUnchanged'] = stable_hashes == {p.name + str(n): sha(p) for n, p in enumerate(stable)}
            check(fragment.exists() and (rendition / 'init.mp4').read_bytes() == initialization, 'recovery_assets')
            check(receipt['metadataUnchanged'], 'recovery_metadata_changed')
            child = dict(os.environ, QA_RESUME_ORIGIN=origin, QA_RESUME_TOKEN=api.token, QA_RESUME_ITEM=item['id'],
                         QA_RESUME_OUTPUT=str(run / 'video.json'), QA_RESUME_ENGINE='chromium', QA_RESUME_SECONDS='12.5',
                         QA_RESUME_FORCE_AUDIO_CONVERSION='true', QA_RESUME_EXPECTED_MODE='audio-transcode',
                         QA_RESUME_FRAME_RATE=str(frame_rate), QA_RESUME_DURATION=str(metadata['durationSeconds']))
            with (run / 'video-private.log').open('w') as output:
                video = subprocess.run(['node', str(ROOT / 'apps/player/scripts/hls-resume-browser.mjs')],
                                       cwd=ROOT, env=child, stdout=output, stderr=output, timeout=300)
            receipt['videoExitCode'] = video.returncode
            check(video.returncode == 0, 'video_or_eof_changed')
            check(all(row['passed'] for row in receipt['captures']), 'decoded_audio_splice')
            check(all(row['passed'] for key in ['initialGrid', 'recoveredGrid'] for row in receipt[key]), 'audio_packet_grid')
        finally:
            server.terminate()
            try:
                server.wait(timeout=15)
            except subprocess.TimeoutExpired:
                server.kill()
                server.wait()
            receipt.update(sourceAfter=source_state(source), sourceUnchanged=source_state(source) == before,
                           serverSettled=server.poll() is not None)
    check(receipt['sourceUnchanged'] and receipt['serverSettled'], 'fixture_not_settled')
    receipt['result'] = 'passed'
except Exception as error:
    receipt['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
finally:
    receipt['artifactsSHA256'] = {p.name: sha(p) for p in run.iterdir() if p.is_file() and p.name != 'player'}
    (run / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print(json.dumps({'receipt': str(run / 'receipt.json'), 'result': receipt['result']}), flush=True)
raise SystemExit(0 if receipt['result'] == 'passed' else 1)
