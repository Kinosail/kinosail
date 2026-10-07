#!/usr/bin/env python3
"""Strict public audio completion and non-key diagnostics; never production proof of preroll."""
import argparse
import hashlib
import json
import math
import os
from pathlib import Path
import re
import shutil
import signal
import socket
import subprocess
import threading
import time
from hls_remaining_process import annotate_case, finish_processes, native_pcm, physical, source_snapshot
from hls_timeline_fixture import fixture
from hls_timeline_http import PublicServer, sha
from hls_timeline_packets import manifest_facts
from hls_followon_frames import decode_frames, stream_metadata
from hls_followon_public import check, bounded_bytes, encoder_count, measure, prepare_once, sample_resources

ROOT = Path(__file__).resolve().parents[3]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--suite', choices=['remaining', 'audio-timing'], default='remaining')
SUITE = parser.parse_args().suite
RUN = ROOT / '.verification/hls-followon' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
RUN.mkdir(parents=True)
BINARY = RUN / 'player'
receipt = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(),
    'result': 'failed', 'cases': [], 'suite': SUITE,
    'command': 'python3 apps/player/scripts/test-hls-remaining.py --suite ' + SUITE,
    'boundary': 'Disposable authenticated public Server. Raw negative frames retained; browser/native/audible qualification separate.',
    'productionMediaOrCacheModified': False, 'expectedCases': 3 if SUITE == 'audio-timing' else 6}
RUN_DEADLINE = time.monotonic() + 500


def alarm(_signal, _frame):
    raise RuntimeError('bounded_diagnostic_deadline')


signal.signal(signal.SIGALRM, alarm)
signal.signal(signal.SIGTERM, alarm)


def remaining_alarm():
    signal.setitimer(signal.ITIMER_REAL, max(0.001, RUN_DEADLINE - time.monotonic()))


def run(command, seconds=30):
    remaining = RUN_DEADLINE - time.monotonic()
    check(remaining > 0, 'bounded_run_deadline')
    value = subprocess.run(command, capture_output=True, timeout=min(seconds, remaining))
    check(value.returncode == 0, 'diagnostic_command_failed')
    check(len(value.stdout) <= 2 * 1024 * 1024, 'diagnostic_output_bound')
    return value.stdout


def reprobe(path, metadata):
    facts = stream_metadata(path)
    frames = json.loads(run(['ffprobe', '-v', 'error', '-select_streams', 'v:0', '-show_frames',
        '-show_entries', 'frame=best_effort_timestamp_time,key_frame', '-of', 'json', str(path)]))['frames']
    times = [float(row['best_effort_timestamp_time']) for row in frames]
    keys = [float(row['best_effort_timestamp_time']) for row in frames if row['key_frame']]
    origin = float(facts['format']['start_time'])
    check(len(times) == 768 and math.isfinite(origin) and all(math.isfinite(v) for v in times)
        and all(a < b for a, b in zip(times, times[1:])), 'source_independent_clock')
    return dict(metadata, sourceFramePTS=times, keyframesSeconds=keys, sourceTimeOriginSeconds=origin,
        durationSeconds=float(facts['format']['duration']), streamOrigins=facts)


def audio_output(api, hls, directory, case, source):
    status, master, _ = api.http(hls)
    check(status == 200, 'audio_master_status')
    check(re.findall(r'^audio/index\.m3u8$', master.decode(), re.M) == ['audio/index.m3u8'], 'audio_rendition')
    base = hls.removesuffix('index.m3u8') + 'audio/'
    status, raw, _ = api.http(base + 'index.m3u8')
    facts, segments = manifest_facts(raw)
    case['publicVariant'] = facts
    case['publicVariantSHA256'] = hashlib.sha256(raw).hexdigest()
    check(status == 200 and 0 < len(segments) <= 16, 'audio_variant_status_bound')
    if not facts['endlist'] or facts['playlistType'] != 'VOD':
        case['failures'].append('control_complete_timeline')
    if abs(facts['durationSeconds'] - case['planDurationSeconds']) > 0.1:
        case['failures'].append('control_full_timeline_duration')
    status, init, _ = api.http(base + 'init.mp4')
    check(status == 200 and init and len(init) <= 2 << 20, 'audio_init')
    case['initializationSHA256'] = hashlib.sha256(init).hexdigest()
    prepared = next(v for v in case['physicalBeforeFirstGET']['assets'] if v['name'] == 'init.mp4')
    check(prepared['sha256'] == case['initializationSHA256'], 'prepared_audio_initialization_changed')
    joined = directory / 'public.mp4'
    data = init
    case['publicPackets'] = []
    prior, prior_point = None, None
    for name, advertised in segments:
        check(re.fullmatch(r'segment-[0-9]{5}\.m4s', name), 'audio_segment_name')
        status, fragment, _ = api.http(base + name)
        check(status == 200 and fragment, 'audio_segment_status')
        data += fragment
        check(len(data) <= 16 << 20, 'audio_join_bound')
        part = directory / 'fragment.mp4'
        part.write_bytes(init + fragment)
        rows = json.loads(run(['ffprobe', '-v', 'error', '-select_streams', 'a:0',
            '-read_intervals', '%+#4096', '-show_packets', '-show_entries', 'packet=pts_time,duration_time',
            '-of', 'json', str(part)])).get('packets', [])
        check(0 < len(rows) < 4096, 'audio_packet_bound')
        gaps, order_valid = [], True
        for row in rows:
            point, duration = float(row['pts_time']), float(row['duration_time'])
            check(math.isfinite(point) and 0 < duration <= 1, 'audio_packet_clock')
            if prior is not None:
                gaps.append(point - prior)
            order_valid = order_valid and (prior_point is None or point > prior_point)
            prior_point = point
            prior = point + duration
        if any(abs(v) > 0.05 for v in gaps):
            case['failures'].append('audio_packet_discontinuity')
        if not order_valid:
            case['failures'].append('audio_packet_order')
        case['publicPackets'].append({'segment': name, 'advertisedSeconds': advertised,
            'count': len(rows), 'first': rows[0], 'last': rows[-1], 'seamsSeconds': gaps,
            'audioPacketOrderValid': order_valid})
    joined.write_bytes(data)
    public, public_facts = native_pcm(joined, RUN_DEADLINE, directory)
    reference, reference_facts = native_pcm(source, RUN_DEADLINE, directory)
    case['nativePCMQualification'] = {'public': public_facts, 'source': reference_facts}
    check(public_facts['stream']['codec_name'] == 'aac', 'public_aac_qualification')
    case['fullEOFNativeSamples'] = {'public': len(public) // 4, 'source': len(reference) // 4,
        'publicSHA256': hashlib.sha256(public).hexdigest(), 'sourceSHA256': hashlib.sha256(reference).hexdigest(),
        'decodedToEOF': True, 'trimmed': False, 'channels': 2, 'sampleRate': 48000,
        'rateConversionApplied': False, 'channelConversionApplied': False, 'format': 's16le'}
    if abs(len(public) - len(reference)) > 0.1 * 48000 * 4:
        case['failures'].append('control_audio_sample_count')
    check(api.http(base + 'init.mp4')[1] == init, 'audio_init_changed')


def journey(name, original, metadata, offset=0, pacing=None):
    case = {'name': name, 'result': 'failed', 'failures': [], 'resumeOffsetSeconds': offset}
    receipt['cases'].append(case)
    remaining = RUN_DEADLINE - time.monotonic()
    check(remaining > 30, 'bounded_run_deadline')
    directory = RUN / name
    media = directory / 'media'
    media.mkdir(parents=True)
    source = media / ('Fixture' + original.suffix)
    shutil.copy2(original, source)
    before = source_snapshot(source)
    case['fixture'] = dict(metadata, **before)
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        port = listener.getsockname()[1]
    api = PublicServer(f'http://localhost:{port}')
    env = dict(os.environ, KINOSAIL_LISTEN=f'127.0.0.1:{port}', KINOSAIL_AUTH_URL=api.url,
        KINOSAIL_TLS_ENABLED='false', KINOSAIL_DATA_DIR=str(directory / 'config'),
        KINOSAIL_MEDIA_DIR=str(media), KINOSAIL_CACHE_DIR=str(directory / 'cache'),
        KINOSAIL_BACKUP_DIR=str(directory / 'backups'), KINOSAIL_BACKUP_KEY='synthetic-remaining-key')
    if pacing is not None:
        real = shutil.which('ffmpeg')
        check(real is not None, 'ffmpeg_unavailable')
        wrapper = directory / 'paced-ffmpeg'
        invocation = directory / 'pacing-invocations.jsonl'
        wrapper.write_text('#!/usr/bin/env python3\nimport os,sys,json\na=sys.argv[1:]\n'
            "if '-hls_time' in a:\n"
            " if '-readrate' in a: a[a.index('-readrate')+1]=" + repr(str(pacing)) + '\n'
            " else: a[a.index('-i'):a.index('-i')]=['-readrate'," + repr(str(pacing)) + ']\n'
            + ' with open(' + repr(str(invocation)) + ", 'a') as f: f.write(json.dumps({'pid':os.getpid(),'parent':os.getppid(),'sourceMatched':" + repr(str(source)) + " in a,'readrate':a[a.index('-readrate')+1]})+'\\n')\n"
            + 'os.execv(' + repr(real) + ',[' + repr(real) + ']+a)\n')
        wrapper.chmod(0o700)
        env['KINOSAIL_FFMPEG'] = str(wrapper)
        case['testOnlyRealCodecPacing'] = {'readrate': pacing, 'wrapperSHA256': sha(wrapper)}
    resources = {'samples': 0, 'peakOwnedFFmpeg': 0, 'samplingErrors': 0}
    case['resources'] = resources
    log_path = directory / 'server.log'
    item, server, sampler = None, None, None
    stop = threading.Event()
    with log_path.open('w') as log:
        try:
            signal.setitimer(signal.ITIMER_REAL, min(90, remaining - 20))
            if 'sourceFramePTS' in metadata:
                check(abs(metadata['sourceTimeOriginSeconds']) <= 0.000001, 'fixture_zero_origin')
                keys = metadata['keyframesSeconds']
                check(any(abs(v - offset) <= 0.000001 for v in keys) if name.startswith('exact-key') else all(abs(v - offset) > 0.05 for v in keys), 'fixture_key_eligibility')
            server = subprocess.Popen([str(BINARY)], cwd=ROOT, env=env, stdout=log, stderr=log, start_new_session=True)
            sampler = threading.Thread(target=sample_resources, args=(server, source, stop, resources), daemon=True)
            sampler.start()
            api.authorize()
            item = next(i for i in api.call('/api/v1/library')['items'] if i['title'] == 'Fixture')
            check(re.fullmatch(r'[a-f0-9]{16}', item['id']), 'item_id_shape')
            plan = api.call('/api/v1/items/' + item['id'] + '/playback?videoCodecs=h264&audioCodecs=aac')
            hls = plan['compatible']
            if offset:
                hls = hls.replace('/index.m3u8', '-o' + str(round(offset * 1000)) + '/index.m3u8')
            check(re.fullmatch(r'/hls/[a-f0-9]{16}/p/[ra]-[a-zA-Z0-9-]+/index\.m3u8', hls), 'planned_recipe_route')
            check(plan['media']['fileVersion'] == str(before['sizeBytes']) + ':' + before['mtimeNanoseconds'], 'source_snapshot_binding')
            case['mode'] = plan['compatiblePlan']['mode']
            check(case['mode'] == ('audio-transcode' if item['kind'] == 'audio' else 'remux'), 'expected_compatibility_mode')
            case['planDurationSeconds'] = plan['duration']
            check(hls.split('/')[2] == item['id'], 'route_item_binding')
            check(abs(plan['duration'] - metadata.get('probedDurationSeconds', metadata.get('durationSeconds'))) < 0.1, 'plan_probe_duration')
            prepare = '/api/v1/items/' + item['id'] + '/playback-prepare'
            cache = directory / 'cache'
            check(encoder_count(server, source) == 0 and not list(cache.glob(item['id'] + '-plan-*')), 'planning_side_effect')
            status, _, _ = api.http(prepare, 'POST', {'source': hls}, authenticated=False)
            check(status == 401 and encoder_count(server, source) == 0 and not list(cache.glob(item['id'] + '-plan-*')), 'unauthenticated_side_effect')
            case['unauthenticatedPreparation'] = {'status': status, 'cacheUnchanged': True}
            prepare_once(api, prepare, hls, log_path, server, source, case)
            if item['kind'] == 'audio':
                check(case['preparationAttempt']['completionState'] == 'ready', 'audio_preparation_not_ready')
                case['physicalBeforeFirstGET'] = physical(cache, item['id'])
                check(case['physicalBeforeFirstGET'] is not None, 'physical_prefix_missing')
                audio_output(api, hls, directory, case, source)
                case['physicalAfterPublicDelivery'] = physical(cache, item['id'])
            else:
                full, source_rows = decode_frames(source)
                origin = metadata['sourceTimeOriginSeconds']
                expected = [n for n, point in enumerate(metadata['sourceFramePTS']) if point >= origin + offset - 0.000001]
                case['referenceClock'] = {'originSeconds': origin, 'requestedSourcePTS': origin + offset,
                    'expectedFrames': len(expected)}
                case['expectedTimelineSeconds'] = plan['duration'] - offset
                measure(api, hls, directory, source, metadata, offset, None, case)
                public_rows = case['timestampedFrameEvidence']
                index = {v: n for n, (_, v) in enumerate(source_rows)}
                mapped = [index.get(row['md5']) for row in public_rows]
                case['rawSourceCorrespondence'] = {'expectedSourceIndices': expected, 'publicSourceIndices': mapped,
                    'sourceHashesUnique': len(index) == len(source_rows), 'negativeFramesRetained': True}
                if len(index) != len(source_rows) or mapped != expected:
                    case['failures'].append('exact_requested_source_sequence')
            case['result'] = 'passed' if not case['failures'] else 'failed'
        except Exception as error:
            case['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
        finally:
            signal.setitimer(signal.ITIMER_REAL, 0)
            signal.signal(signal.SIGTERM, signal.SIG_IGN)
            try:
                if server is not None:
                    case.update(finish_processes(server, source, stop, sampler))
            except Exception as error:
                case['cleanupFailureClass'] = type(error).__name__
                case['failures'].append('owned_process_join_failed')
                case['result'] = 'failed'
            signal.signal(signal.SIGTERM, alarm)
            log.flush()
            try:
                annotate_case(case, log_path, source, before, server, resources, offset, pacing,
                    invocation if pacing is not None else None, item is not None and item['kind'] == 'audio')
            except Exception as error:
                case['diagnosticFailureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
                case['failures'].append('diagnostic_evidence_incomplete')
                case['result'] = 'failed'
            remaining_alarm()


try:
    remaining_alarm()
    subprocess.run(['go', '-C', 'apps/player', 'build', '-p=1', '-o', str(BINARY), './cmd/kinosail'], cwd=ROOT, check=True, timeout=180)
    receipt['binarySHA256'] = sha(BINARY)
    receipt['encoderVersions'] = {t: run([t, '-version']).decode().splitlines()[0] for t in ['ffmpeg', 'ffprobe']}
    cases = [(10, 0.75), (10, 0.9), (8, 0.75)] if SUITE == 'audio-timing' else [(10, None), (10, 1.25), (8, 1.25)]
    for duration, paced in cases:
        source = RUN / f'audio-{duration}-{paced}.flac'
        run(['ffmpeg', '-nostdin', '-v', 'error', '-f', 'lavfi', '-i', f'sine=frequency=440:sample_rate=48000:duration={duration}', '-c:a', 'flac', '-ac', '2', str(source)], 60)
        probe = json.loads(run(['ffprobe', '-v', 'error', '-show_entries', 'format=duration', '-of', 'json', str(source)]))
        journey(f'audio-{duration}-{paced}', source, {'probedDurationSeconds': float(probe['format']['duration'])}, pacing=paced)
    if SUITE == 'remaining':
        source, metadata = fixture(RUN, 'regular', 48, ','.join(str(v) for v in range(0, 32, 2)), frames=768)
        metadata = reprobe(source, metadata)
        journey('exact-key-control12', source, metadata, offset=12)
        journey('nonkey-mkv12.5', source, metadata, offset=12.5)
        mp4 = RUN / 'copy.mp4'
        run(['ffmpeg', '-nostdin', '-v', 'error', '-i', str(source), '-map', '0:v:0', '-map', '0:a:0', '-c', 'copy', str(mp4)], 60)
        metadata = reprobe(mp4, metadata)
        journey('nonkey-mp4-12.5', mp4, metadata, offset=12.5)
    if len(receipt['cases']) == receipt['expectedCases'] and all(c['result'] == 'passed' for c in receipt['cases']):
        receipt['result'] = 'passed'
except Exception as error:
    receipt['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
finally:
    signal.setitimer(signal.ITIMER_REAL, 0)
    encoded = json.dumps(receipt, indent=2) + '\n'
    check(len(encoded.encode()) <= 4 << 20, 'receipt_budget')
    (RUN / 'receipt.json').write_text(encoded)
    helpers = [Path(__file__), *(ROOT / 'apps/player/scripts').glob('hls_*.py')]
    check(len(helpers) <= 32, 'helper_manifest_budget')
    checksums = {'receipt.json': sha(RUN / 'receipt.json'), **{str(p.relative_to(ROOT)): sha(p) for p in helpers}}
    (RUN / 'SHA256SUMS').write_text(''.join(f'{value}  {name}\n' for name, value in checksums.items()))
    print(json.dumps({'revision': receipt['revision'], 'result': receipt['result'], 'cases': [{'name': c['name'], 'result': c['result'], 'failures': c['failures'], 'failureClass': c.get('failureClass')} for c in receipt['cases']]}))
raise SystemExit(0 if receipt['result'] == 'passed' else 1)
