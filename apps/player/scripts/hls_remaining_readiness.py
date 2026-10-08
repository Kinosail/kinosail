"""Fresh authenticated AAC delivery control without speculative preparation."""
import hashlib
import json
import os
import re
import shutil
import signal
import socket
import subprocess
import threading
import time
from hls_followon_public import bounded_bytes, check, encoder_count, sample_resources
from hls_remaining_audio import marker_clock, packet_evidence
from hls_remaining_process import annotate_case, finish_processes, native_pcm, physical, source_snapshot
from hls_timeline_http import PublicServer
from hls_timeline_packets import manifest_facts


def readiness_cold_control(root, binary, original, directory):
    """Protect the E2E gap: preparation pacing cannot serve as an uninterrupted control."""
    directory.mkdir()
    media = directory / 'media'
    media.mkdir()
    source = media / 'Fixture.flac'
    shutil.copy2(original, source)
    before = source_snapshot(source)
    case = {'name': 'uninterrupted-cold-control', 'result': 'failed', 'failures': [],
        'fixture': dict(before, markerNear8Seconds=True), 'planDurationSeconds': 10, 'preparationPosts': 0}
    deadline = time.monotonic() + 90
    def run(command, seconds=30):
        remaining = deadline - time.monotonic()
        check(remaining > 0, 'readiness_cold_deadline')
        value = subprocess.run(command, capture_output=True, timeout=min(seconds, remaining))
        check(value.returncode == 0 and len(value.stdout) <= 2 << 20, 'readiness_cold_command')
        return value.stdout
    def alarm(_signal, _frame):
        raise RuntimeError('readiness_cold_deadline')
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        port = listener.getsockname()[1]
    api = PublicServer(f'http://localhost:{port}')
    cache = directory / 'cache'
    environment = dict(os.environ, KINOSAIL_LISTEN=f'127.0.0.1:{port}', KINOSAIL_AUTH_URL=api.url,
        KINOSAIL_TLS_ENABLED='false', KINOSAIL_DATA_DIR=str(directory / 'config'),
        KINOSAIL_MEDIA_DIR=str(media), KINOSAIL_CACHE_DIR=str(cache),
        KINOSAIL_BACKUP_DIR=str(directory / 'backups'), KINOSAIL_BACKUP_KEY='synthetic-readiness-key')
    environment.pop('KINOSAIL_FFMPEG', None)
    resources = {'samples': 0, 'peakOwnedFFmpeg': 0, 'samplingErrors': 0}
    case['resources'] = resources
    stop, server, sampler = threading.Event(), None, None
    log_path = directory / 'server.log'
    prior_alarm, prior_term = signal.signal(signal.SIGALRM, alarm), signal.signal(signal.SIGTERM, alarm)
    signal.setitimer(signal.ITIMER_REAL, 90)
    try:
        with log_path.open('w') as log:
            try:
                server = subprocess.Popen([str(binary)], cwd=root, env=environment,
                    stdout=log, stderr=log, start_new_session=True)
                sampler = threading.Thread(target=sample_resources, args=(server, source, stop, resources), daemon=True)
                sampler.start()
                api.authorize()
                item = next(v for v in api.call('/api/v1/library')['items'] if v['title'] == 'Fixture')
                check(re.fullmatch(r'[a-f0-9]{16}', item['id']) and item['kind'] == 'audio', 'readiness_cold_item')
                plan = api.call('/api/v1/items/' + item['id'] + '/playback?videoCodecs=h264&audioCodecs=aac')
                hls = plan['compatible']
                check(re.fullmatch(r'/hls/[a-f0-9]{16}/p/a-[a-zA-Z0-9-]+/index\.m3u8', hls)
                    and hls.split('/')[2] == item['id']
                    and plan['compatiblePlan']['mode'] == 'audio-transcode'
                    and plan['duration'] == 10
                    and plan['media']['fileVersion'] == str(before['sizeBytes']) + ':' + before['mtimeNanoseconds'],
                    'readiness_cold_plan_binding')
                check(encoder_count(server, source) == 0 and not list(cache.glob('*-plan-*')),
                    'readiness_cold_independent_empty_cache')
                case['emptyCacheBeforeFirstGET'] = True
                readiness_uninterrupted_audio(api, hls, directory, case, source, run, deadline)
                case['physicalAfterPublicDelivery'] = physical(cache, item['id'])
            finally:
                signal.setitimer(signal.ITIMER_REAL, 0)
                signal.signal(signal.SIGTERM, signal.SIG_IGN)
                if server is not None:
                    case.update(finish_processes(server, source, stop, sampler))
        annotate_case(case, log_path, source, before, server, resources, 0, None, None, True)
        check(case['physicalAfterPublicDelivery']['manifest']['endlist'], 'readiness_cold_genuine_endlist')
        check(not case['failures'] and case['workerBound'] and case['sourceUnchanged']
            and case['encoderLifecycle']['starts'] == case['encoderLifecycle']['ends'] == 1
            and not case['cleanupFailures'] and not case['ownedProcessJoin']['forcedOwnedGroupStop']
            and case['ownedProcessJoin']['remainingOwnedPIDs'] == [], 'readiness_cold_single_joined_encoder')
        check(source_snapshot(original)['sha256'] == before['sha256'], 'readiness_cold_same_original')
        case['result'] = 'passed'
    finally:
        signal.setitimer(signal.ITIMER_REAL, 0)
        signal.signal(signal.SIGALRM, prior_alarm)
        signal.signal(signal.SIGTERM, prior_term)
    return case


def readiness_uninterrupted_audio(api, hls, directory, case, source, run, deadline):
    """Read real public media once, without a prepared-prefix assumption."""
    status, master, _ = api.http(hls)
    check(status == 200 and re.findall(r'^audio/index\.m3u8(directory, pacing):
    """Validate both paced argv witnesses without changing the legacy0.75 oracle."""
    source = directory / 'media/Fixture.flac'
    roots = list((directory / 'cache').glob('*-plan-*'))
    check(len(roots) == 1, 'readiness_observed_single_generation')
    root = roots[0]
    arguments = json.loads(bounded_bytes(directory / 'refill-recipe-private.json', 8192, 'readiness_actual_argv_bound'))
    expected = ['-hide_banner', '-loglevel', 'error', '-y', '-avoid_negative_ts', 'disabled', '-max_delay', '5000000',
        '-ss', '0.000', '-readrate', str(pacing), '-i', str(source), '-map', '0:a:0', '-vn', '-sn', '-dn',
        '-c:a', 'aac', '-ac', '2', '-b:a', '192000', '-output_ts_offset', str(8 - 382976 / 48000),
        '-bsf:a', 'noise=amount=0:drop=lt(pts\\,382976)', '-f', 'hls', '-hls_time', '2',
        '-hls_playlist_type', 'event', '-hls_segment_type', 'fmp4', '-hls_segment_options',
        'movflags=+frag_discont+skip_sidx', '-hls_flags', 'temp_file', '-hls_fmp4_init_filename', 'init.mp4',
        '-start_number', '4', '-hls_segment_filename', str(root / 'audio/segment-%05d.m4s'),
        str(root / '.seek-4/audio/index.m3u8')]
    check(arguments == expected, 'readiness_exact_refill_argv')
    return {'actualSourceSeekSeconds': 0, 'logicalRefillCutSeconds': 8,
        'testOnlyReadrate': pacing, 'productionArgumentsNotTransformed': True}
, master.decode(), re.M) == ['audio/index.m3u8'],
        'readiness_cold_master')
    base = hls.removesuffix('index.m3u8') + 'audio/'
    status, raw, _ = api.http(base + 'index.m3u8')
    facts, segments = manifest_facts(raw)
    check(status == 200 and facts['endlist'] and facts['playlistType'] == 'VOD'
        and abs(facts['durationSeconds'] - 10) < 0.1 and 0 < len(segments) <= 16,
        'readiness_cold_public_timeline')
    status, init, _ = api.http(base + 'init.mp4')
    check(status == 200 and 0 < len(init) <= 2 << 20, 'readiness_cold_init')
    case['initializationSHA256'] = hashlib.sha256(init).hexdigest()
    joined = init
    for name, _ in segments:
        check(re.fullmatch(r'segment-[0-9]{5}\.m4s', name), 'readiness_cold_segment_name')
        status, fragment, _ = api.http(base + name)
        check(status == 200 and fragment, 'readiness_cold_segment')
        joined += fragment
        check(len(joined) <= 16 << 20, 'readiness_cold_media_bound')
    check(api.http(base + 'init.mp4')[1] == init, 'readiness_cold_init_changed')
    public_path = directory / 'public.mp4'
    public_path.write_bytes(joined)
    case['joinedPublicPacketPayloads'] = packet_evidence(run, public_path)
    public, native = native_pcm(public_path, deadline, directory)
    reference, reference_native = native_pcm(source, deadline, directory)
    case['nativePCMQualification'] = {'public': native, 'source': reference_native}
    case['markerSourcePublicSampleClock'] = marker_clock(reference, public)
    case['fullEOFNativeSamples'] = {'public': len(public) // 4, 'source': len(reference) // 4,
        'publicSHA256': hashlib.sha256(public).hexdigest(), 'sourceSHA256': hashlib.sha256(reference).hexdigest(),
        'decodedToEOF': True, 'trimmed': False, 'channels': 2, 'sampleRate': 48000,
        'rateConversionApplied': False, 'channelConversionApplied': False, 'format': 's16le'}


def readiness_refill_arguments(directory, pacing):
    """Validate both paced argv witnesses without changing the legacy0.75 oracle."""
    source = directory / 'media/Fixture.flac'
    roots = list((directory / 'cache').glob('*-plan-*'))
    check(len(roots) == 1, 'readiness_observed_single_generation')
    root = roots[0]
    arguments = json.loads(bounded_bytes(directory / 'refill-recipe-private.json', 8192, 'readiness_actual_argv_bound'))
    expected = ['-hide_banner', '-loglevel', 'error', '-y', '-avoid_negative_ts', 'disabled', '-max_delay', '5000000',
        '-ss', '0.000', '-readrate', str(pacing), '-i', str(source), '-map', '0:a:0', '-vn', '-sn', '-dn',
        '-c:a', 'aac', '-ac', '2', '-b:a', '192000', '-output_ts_offset', str(8 - 382976 / 48000),
        '-bsf:a', 'noise=amount=0:drop=lt(pts\\,382976)', '-f', 'hls', '-hls_time', '2',
        '-hls_playlist_type', 'event', '-hls_segment_type', 'fmp4', '-hls_segment_options',
        'movflags=+frag_discont+skip_sidx', '-hls_flags', 'temp_file', '-hls_fmp4_init_filename', 'init.mp4',
        '-start_number', '4', '-hls_segment_filename', str(root / 'audio/segment-%05d.m4s'),
        str(root / '.seek-4/audio/index.m3u8')]
    check(arguments == expected, 'readiness_exact_refill_argv')
    return {'actualSourceSeekSeconds': 0, 'logicalRefillCutSeconds': 8,
        'testOnlyReadrate': pacing, 'productionArgumentsNotTransformed': True}
