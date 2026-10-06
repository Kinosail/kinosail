#!/usr/bin/env python3
"""Test-first public non-key resumes and H264 audio conversion, separate from493."""
import hashlib
import json
import math
import os
from pathlib import Path
import re
import shutil
import socket
import subprocess
import threading
import time
from hls_timeline_fixture import fixture
from hls_timeline_http import PublicServer, sha, source_state
from hls_timeline_packets import safe_encoder_lifecycle, safe_seek_phases
from hls_timeline_preparation import prepare_scene
from hls_followon_public import check, bounded_bytes, encoder_count, sample_resources, prepare_once, measure
from hls_followon_frames import stream_metadata
from hls_followon_controls import controls

ROOT = Path(__file__).resolve().parents[3]
RUN = ROOT / '.verification/hls-followon' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
RUN.mkdir(parents=True)
binary = RUN / 'player'
receipt = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip(),
    'result': 'failed', 'cases': [], 'command': 'python3 apps/player/scripts/test-hls-followon.py',
    'boundary': 'Synthetic authenticated public Server delivery; native/Safari/iOS and Nox acceptance separate.',
    'productionMediaOrCacheModified': False, 'fixtureSeconds': 32, 'fixtureFrameRate': 24}


def packet_identity(path, stream):
    value = subprocess.check_output(['ffprobe', '-v', 'error', '-select_streams', stream,
        '-read_intervals', '%+#4096', '-show_packets', '-show_data_hash', 'sha256',
        '-show_entries', 'packet=data_hash', '-of', 'json', str(path)], timeout=30)
    check(len(value) <= 2 * 1024 * 1024, 'fixture_packet_bound')
    hashes = [p.get('data_hash', '') for p in json.loads(value)['packets']]
    check(0 < len(hashes) < 4096 and all(re.fullmatch(r'SHA256:[a-f0-9]{64}', h) for h in hashes),
          'fixture_packet_identity')
    return {'packets': len(hashes), 'sha256': hashlib.sha256('\n'.join(hashes).encode()).hexdigest()}


def convert(original, metadata, name, ac3=False):
    path = RUN / (name + ('.mkv' if ac3 else '.mp4'))
    command = ['ffmpeg', '-nostdin', '-v', 'error', '-i', str(original)]
    if ac3:
        command += ['-f', 'lavfi', '-i', 'aevalsrc=0.2*sin(2*PI*(440+110*floor(t/4))*t):s=48000:d=32',
                    '-map', '0:v:0', '-map', '1:a:0', '-c:v', 'copy', '-c:a', 'ac3', '-b:a', '192k', '-ac', '2']
    else:
        command += ['-map', '0:v:0', '-map', '0:a:0', '-c', 'copy']
    command += [str(path)]
    subprocess.run(command, check=True, timeout=90, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    duration = float(subprocess.check_output(['ffprobe', '-v', 'error', '-show_entries', 'format=duration',
        '-of', 'csv=p=0', str(path)], timeout=20))
    result = dict(metadata, command=command, sha256=sha(path), durationSeconds=duration)
    result = reprobe_source(path, result)
    result['copiedVideoIdentity'] = packet_identity(path, 'v:0')
    check(result['copiedVideoIdentity'] == packet_identity(original, 'v:0'), 'fixture_video_reencoded')
    if not ac3:
        result['copiedAudioIdentity'] = packet_identity(path, 'a:0')
        check(result['copiedAudioIdentity'] == packet_identity(original, 'a:0'), 'fixture_audio_changed')
    return path, result


def reprobe_source(path, metadata):
    facts = stream_metadata(path)
    data = subprocess.check_output(['ffprobe', '-v', 'error', '-select_streams', 'v:0', '-show_frames',
        '-show_entries', 'frame=best_effort_timestamp_time,key_frame', '-of', 'json', str(path)], timeout=30)
    check(len(data) <= 256 * 1024, 'source_frame_clock_bound')
    frames = json.loads(data)['frames']
    times = [float(v['best_effort_timestamp_time']) for v in frames]
    keys = [float(v['best_effort_timestamp_time']) for v in frames if v['key_frame']]
    origin = float(facts['format']['start_time'])
    check(len(times) == 768 and math.isfinite(origin) and all(math.isfinite(v) for v in times)
          and all(a < b for a, b in zip(times, times[1:])), 'source_presentation_clock')
    video = next(v for v in facts['streams'] if v['codec_type'] == 'video')
    numerator, denominator = video['avg_frame_rate'].split('/')
    rate = float(numerator) / float(denominator)
    check(0 < len(keys) <= 100 and abs(rate - 24) < 0.001, 'source_key_rate')
    return dict(metadata, keyframesSeconds=keys, frameRate=rate, streamOrigins=facts,
                sourceFramePTS=times, sourceTimeOriginSeconds=origin)


def journey(name, original, metadata, offset=0, cold=False, one_shot=False, audio_conversion=False):
    case = {'name': name, 'result': 'failed', 'fixture': metadata, 'resumeOffsetSeconds': offset, 'failures': []}
    receipt['cases'].append(case)
    directory, media = RUN / name, RUN / name / 'media'
    media.mkdir(parents=True)
    source = media / ('Fixture' + original.suffix)
    shutil.copy2(original, source)
    before = source_state(source)
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        port = listener.getsockname()[1]
    url = f'http://localhost:{port}'
    env = dict(os.environ, KINOSAIL_LISTEN=f'127.0.0.1:{port}', KINOSAIL_AUTH_URL=url,
        KINOSAIL_TLS_ENABLED='false', KINOSAIL_DATA_DIR=str(directory / 'config'),
        KINOSAIL_MEDIA_DIR=str(media), KINOSAIL_CACHE_DIR=str(directory / 'cache'),
        KINOSAIL_BACKUP_DIR=str(directory / 'backups'), KINOSAIL_BACKUP_KEY='synthetic-followon-key')
    api = PublicServer(url)
    resources = {'samples': 0, 'peakOwnedFFmpeg': 0, 'samplingErrors': 0}
    case['resources'] = resources
    log_path = directory / 'server.log'
    with log_path.open('w') as log:
        server = subprocess.Popen([str(binary)], cwd=ROOT, env=env, stdout=log, stderr=log)
        stop = threading.Event()
        sampler = threading.Thread(target=sample_resources, args=(server, source, stop, resources), daemon=True)
        sampler.start()
        try:
            api.authorize()
            item_id = next(i['id'] for i in api.call('/api/v1/library')['items'] if i['title'] == 'Fixture')
            plan = api.call('/api/v1/items/' + item_id + '/playback?videoCodecs=h264&audioCodecs=aac')
            case['mode'] = plan['compatiblePlan']['mode']
            check(case['mode'] == ('audio-transcode' if audio_conversion else 'remux'), 'expected_compatibility_mode')
            hls = plan['compatible']
            origin = metadata['sourceTimeOriginSeconds']
            case['referenceClock'] = {'originSeconds': origin, 'requestedSourcePTS': origin + offset,
                'expectedFrames': sum(point >= origin + offset - 0.000001 for point in metadata['sourceFramePTS']),
                'method': 'independent ffprobe decoded source PTS minus demux format start_time'}
            if offset:
                check(all(abs(k - origin - offset) > 0.05 for k in metadata['keyframesSeconds']), 'fixture_offset_is_key')
                hls = hls.replace('/index.m3u8', '-o' + str(round(offset * 1000)) + '/index.m3u8')
            check(re.fullmatch(r'/hls/[a-f0-9]{16}/p/[ra]-[a-zA-Z0-9-]+/index\.m3u8', hls), 'planned_recipe_route')
            case['expectedTimelineSeconds'] = plan['duration'] - offset
            check(abs(plan['duration'] - metadata['durationSeconds']) < 0.1, 'source_duration')
            check(plan['media']['fileVersion'] == str(before['sizeBytes']) + ':' + before['mtimeNanoseconds'],
                  'source_snapshot_binding')
            prepare, cache = '/api/v1/items/' + item_id + '/playback-prepare', directory / 'cache'
            roots = lambda: sorted(p.name for p in cache.iterdir() if p.name.startswith(item_id))
            check(not roots() and encoder_count(server, source) == 0, 'planning_side_effect')
            status, _, _ = api.http(prepare, 'POST', {'source': hls}, authenticated=False)
            check(status == 401 and not roots() and encoder_count(server, source) == 0, 'unauthenticated_side_effect')
            case['unauthenticatedPreparation'] = {'status': status, 'cacheUnchanged': True}
            prepared_init = None
            if one_shot:
                prepare_once(api, prepare, hls, log_path, server, source, case)
                case['preparationReady'] = False
            else:
                _, prepared_init = prepare_scene(api, prepare, hls, cache, item_id, server, source, case,
                    encoder_count, check, bounded_bytes, cold)
            measure(api, hls, directory, source, metadata, offset, prepared_init, case)
            case['result'] = 'passed' if not case['failures'] else 'failed'
        except Exception as error:
            case['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
        finally:
            join_limit = time.monotonic() + 15
            while encoder_count(server, source) > 0 and time.monotonic() < join_limit:
                time.sleep(0.05)
            case['ownedFFmpegBeforeTeardown'] = encoder_count(server, source)
            stop.set()
            sampler.join(timeout=5)
            server.terminate()
            try:
                server.wait(timeout=10)
            except subprocess.TimeoutExpired:
                server.kill()
                server.wait(timeout=5)
            log.flush()
            private = bounded_bytes(log_path, 2 * 1024 * 1024, 'private_log_bound').decode()
            case.update(safe_seek_phases(private))
            lifecycle = safe_encoder_lifecycle(private)
            case['encoderLifecycle'] = lifecycle
            case['workerBound'] = (resources['samples'] > 0 and resources['peakOwnedFFmpeg'] <= 1
                and resources['samplingErrors'] == 0 and lifecycle['validSequence'] and lifecycle['peakActive'] == 1
                and case['ownedFFmpegBeforeTeardown'] == 0)
            if not case['workerBound']:
                case['failures'].append('owned_encoder_bound')
                case['result'] = 'failed'
            case['sourceUnchanged'] = before == source_state(source)
            case['sourceBefore'], case['sourceAfter'] = before, source_state(source)
            if not case['sourceUnchanged']:
                case['failures'].append('source_mutated')
                case['result'] = 'failed'


try:
    subprocess.run(['go', '-C', 'apps/player', 'build', '-p=1', '-o', str(binary), './cmd/kinosail'],
                   cwd=ROOT, check=True, timeout=180)
    receipt['binarySHA256'] = sha(binary)
    receipt['encoderVersions'] = {t: subprocess.check_output([t, '-version'], text=True).splitlines()[0]
                                  for t in ['ffmpeg', 'ffprobe']}
    regular, regular_metadata = fixture(RUN, 'regular', 48, ','.join(str(v) for v in range(0, 32, 2)), frames=768)
    regular_metadata = reprobe_source(regular, regular_metadata)
    mp4, mp4_metadata = convert(regular, regular_metadata, 'regular-copy')
    journey('nonkey-mkv', regular, regular_metadata, 12.5, one_shot=True)
    journey('nonkey-mp4', mp4, mp4_metadata, 12.5, one_shot=True)
    ac3, ac3_metadata = convert(regular, regular_metadata, 'regular-ac3', ac3=True)
    journey('audio-regular-prepared', ac3, ac3_metadata, audio_conversion=True)
    sparse, sparse_metadata = fixture(RUN, 'sparse', 2400, '0,15,16,18', frames=768)
    sparse_metadata = reprobe_source(sparse, sparse_metadata)
    ac3, ac3_metadata = convert(sparse, sparse_metadata, 'sparse-ac3', ac3=True)
    journey('audio-sparse-cold', ac3, ac3_metadata, cold=True, audio_conversion=True)
    journey('audio-sparse-prepared', ac3, ac3_metadata, audio_conversion=True)
    controls(ROOT, RUN, binary, receipt)
    if len(receipt['cases']) == 8 and all(c['result'] == 'passed' for c in receipt['cases']):
        receipt['result'] = 'passed'
except Exception as error:
    receipt['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
finally:
    target = RUN / 'receipt.json'
    target.write_text(json.dumps(receipt, indent=2, allow_nan=False) + '\n')
    files = [Path(__file__)] + [Path(__file__).with_name(n) for n in
        ['hls_followon_public.py', 'hls_followon_frames.py', 'hls_followon_controls.py', 'hls_timeline_http.py',
         'hls_timeline_packets.py', 'hls_timeline_fixture.py', 'hls_timeline_preparation.py']]
    checksums = {str(p.relative_to(ROOT)): sha(p) for p in files} | {'receipt.json': sha(target)}
    (RUN / 'SHA256SUMS').write_text(''.join(f'{v}  {k}\n' for k, v in checksums.items()))
    print(json.dumps({'result': receipt['result'], 'receiptSHA256': sha(target), 'failureClass': receipt.get('failureClass'),
        'cases': [{k: c.get(k) for k in ['name', 'result', 'failureClass', 'failures', 'mode', 'publicVariant',
                                       'presentation', 'reference', 'preparationAttempt', 'workerBound', 'sourceUnchanged']}
                  for c in receipt['cases']]}))
raise SystemExit(0 if receipt['result'] == 'passed' else 1)
