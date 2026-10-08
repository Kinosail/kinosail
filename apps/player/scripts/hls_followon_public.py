"""Real authenticated public measurements shared by the bounded follow-on cases."""
import hashlib
import json
import math
import re
import subprocess
import time
from hls_timeline_packets import manifest_facts, fragment_packets, fragment_audio
from hls_followon_frames import decode_frames, audio_sequence


def check(condition, failure):
    if not condition:
        raise RuntimeError(failure)


def bounded_bytes(path, limit, failure):
    with path.open('rb') as file:
        data = file.read(limit + 1)
    check(0 < len(data) <= limit, failure)
    return data


def encoder_count(server, source):
    rows = subprocess.check_output(['ps', '-eo', 'ppid=,args='], text=True, timeout=5).splitlines()
    return sum(1 for row in rows if row.strip() and row.strip().split(maxsplit=1)[0] == str(server.pid)
               and 'ffmpeg' in row and '-hls_time' in row and str(source) in row)


def sample_resources(server, source, stop, resources):
    while not stop.is_set():
        try:
            resources['peakOwnedFFmpeg'] = max(resources['peakOwnedFFmpeg'], encoder_count(server, source))
            resources['samples'] += 1
        except (OSError, subprocess.SubprocessError):
            resources['samplingErrors'] += 1
        stop.wait(0.05)


def video_decode_order(path):
    data = subprocess.check_output(['ffprobe', '-v', 'error', '-select_streams', 'v:0',
        '-read_intervals', '%+#4096', '-show_packets', '-show_entries', 'packet=dts_time',
        '-of', 'json', str(path)], timeout=30)
    check(len(data) <= 2 * 1024 * 1024, 'video_decode_order_bound')
    packets = json.loads(data).get('packets', [])
    check(0 < len(packets) < 4096, 'video_decode_order_bound')
    times = [float(p['dts_time']) for p in packets]
    check(all(math.isfinite(v) for v in times), 'video_decode_order_clock')
    return all(a < b for a, b in zip(times, times[1:]))


def preparation_fields(pairs):
    value = dict(pairs)
    check(len(value) == len(pairs), 'one_shot_preparation_state')
    return value


def prepare_once(api, prepare, hls, log_path, server, source, case):
    prior = log_path.stat().st_size
    status, data, headers = api.http(prepare, 'POST', {'source': hls})
    check(status == 202 and len(data) <= 512 * 1024, 'one_shot_preparation_response')
    value = json.loads(data, object_pairs_hook=preparation_fields)
    check(type(value) is dict and set(value) == {'state'} and value['state'] in ['queued', 'ready'],
          'one_shot_preparation_state')
    request_id = next((v for k, v in headers.items() if k.lower() == 'x-request-id'), '')
    check(re.fullmatch(r'[a-zA-Z0-9_-]{1,80}', request_id), 'one_shot_request_correlation')
    case['preparationAttempt'] = attempt = {'posts': 1, 'publicState': value['state'], 'requestID': request_id}
    limit, joined = time.monotonic() + 20, 0
    witness = attempt['joinWitness'] = {'stage': 'playback-adoption', 'deadlineSeconds': 20,
        'getRequestIDPresent': False, 'getRequestIDMatchesPost': False, 'adoptionStatus': None,
        'matchedTerminalStates': [], 'unmatchedTerminalStates': [], 'encoderCompleted': False,
        'unmatchedEncoderCompleted': False, 'encoderFailed': False, 'encoderPaused': False,
        'samples': 0, 'ownedFFmpeg': None, 'joinedSamples': 0}
    audio_playback = case.get('itemKind') in ['audio', 'audiobook']
    terminal_states = ['ready', 'unavailable', 'cancelled', 'bounded', 'adopted']
    try:
        if audio_playback:
            # Adopt actual playback before joining successful encoder completion.
            status, _, adoption_headers = api.http(hls, timeout=limit - time.monotonic())
            witness['adoptionStatus'] = status
            ids = [v for k, v in adoption_headers.items() if k.lower() == 'x-request-id']
            present = len(ids) == 1 and type(ids[0]) is str and bool(re.fullmatch(r'[a-zA-Z0-9_-]{1,80}', ids[0]))
            witness.update(getRequestIDPresent=present, getRequestIDMatchesPost=present and ids[0] == request_id)
            check(status == 200, 'audio_playback_adoption_http_' + str(status))
        witness['stage'] = 'join-sampling'
        while time.monotonic() < limit:
            private = bounded_bytes(log_path, 2 * 1024 * 1024, 'private_log_bound')[prior:].decode()
            states, completed = [], False
            for line in private.splitlines():
                check(len(line) <= 16 * 1024, 'private_log_line_bound')
                if not line.startswith('{') or not line.endswith('}'):
                    continue  # An in-flight final line may not have finished writing yet.
                entry = json.loads(line)
                check(type(entry) is dict, 'private_log_shape')
                matches = entry.get('request_id') == request_id
                message = entry.get('msg')
                if message == 'HLS startup preparation' and entry.get('state') in terminal_states:
                    state = entry['state']
                    key = 'matchedTerminalStates' if matches else 'unmatchedTerminalStates'
                    if state not in witness[key]: witness[key].append(state)
                    if matches: states.append(state)
                if message == 'HLS transcode completed':
                    witness['encoderCompleted' if matches else 'unmatchedEncoderCompleted'] = True
                if audio_playback and matches:
                    witness['encoderFailed'] |= message == 'HLS transcode failed'
                    witness['encoderPaused'] |= message == 'HLS transcode paused after playback became inactive'
                    check(not witness['encoderFailed'] and not witness['encoderPaused'], 'audio_playback_not_completed')
                    completed = completed or message == 'HLS transcode completed'
            qualified = states and (not audio_playback or completed)
            if qualified:
                count = encoder_count(server, source)
                witness.update(samples=min(witness['samples'] + 1, 1024), ownedFFmpeg=min(count, 16))
                joined = joined + 1 if count == 0 else 0
            else:
                joined = 0
            witness['joinedSamples'] = joined
            if joined >= 3:
                attempt.update(completionState=states[-1], ownedFFmpeg=0, joinedSamples=joined)
                if audio_playback: attempt.update(playbackAdopted=True, encoderCompleted=True)
                witness['stage'] = 'joined'
                return
            time.sleep(0.05)
        raise RuntimeError('one_shot_preparation_not_joined')
    finally:
        now = time.monotonic()
        witness.update(elapsedMs=min(round(max(0, now - limit + 20) * 1000), 40000), deadlineExpired=now >= limit)


def measure(api, hls, directory, source, metadata, offset, prepared_init, case):
    failure = case['failures']
    status, master, _ = api.http(hls)
    check(status == 200, 'master_http_' + str(status))
    renditions = re.findall(r'^[1-9][0-9]{2,3}p/index\.m3u8$', master.decode(), re.M)
    check(len(renditions) == 1, 'one_copied_video_rendition')
    base = hls.removesuffix('index.m3u8') + renditions[0].removesuffix('index.m3u8')
    status, variant, _ = api.http(base + 'index.m3u8')
    check(status == 200, 'variant_http_' + str(status))
    facts, segments = manifest_facts(variant)
    duration = case['expectedTimelineSeconds']
    case['publicVariant'] = facts
    if not facts['endlist'] or facts['playlistType'] != 'VOD' or abs(facts['durationSeconds'] - duration) > 0.1:
        failure.append('public_full_timeline')
    status, init, _ = api.http(base + 'init.mp4')
    check(status == 200 and init, 'initialization_unavailable')
    if prepared_init is not None and init != prepared_init:
        failure.append('prepared_initialization_changed')
    case['initializationSHA256'] = hashlib.sha256(init).hexdigest()
    fragments, previous_video_end, previous_audio_end = [init], None, None
    case['publicFragments'] = []
    for filename, advertised in segments:
        status, data, _ = api.http(base + filename)
        if status != 200 or not data:
            failure.append('fragment_http_' + str(status))
            case['unavailableSegment'] = filename
            continue
        fragments.append(data)
        path = directory / 'fragment.mp4'
        path.write_bytes(init + data)
        packets = fragment_packets(path)
        packets['fragmentSHA256'] = hashlib.sha256(init + data).hexdigest()
        try:
            packets.update(fragment_audio(path))
        except RuntimeError as error:
            if str(error) != 'public_audio_packet_bound':
                raise
            data = subprocess.check_output(['ffprobe', '-v', 'error', '-select_streams', 'a:0',
                '-read_intervals', '%+#4096', '-show_packets', '-show_entries', 'packet=pts_time,duration_time',
                '-of', 'json', str(path)], timeout=30)
            check(len(data) <= 2 * 1024 * 1024, 'missing_audio_probe_bound')
            count = len(json.loads(data).get('packets', []))
            packets.update(segment=filename, advertisedSeconds=advertised, audioPackets=count,
                           failedAssertion='public_audio_packet_bound')
            case['publicFragments'].append(packets)
            failure.append('fragment_missing_audio' if count == 0 else 'fragment_audio_bound')
            previous_video_end = packets['lastVideoEnd']
            continue  # Preserve the failed assertion and fetch later advertised media.
        packets.update(segment=filename, advertisedSeconds=advertised)
        packets['videoDecodeOrderValid'] = video_decode_order(path)
        case['publicFragments'].append(packets)
        if not packets['videoDecodeOrderValid']:
            failure.append('video_decode_order')
        if (not packets['audioPacketOrderValid'] or packets['maximumAudioGapSeconds'] > 0.05
                or packets['maximumAudioOverlapSeconds'] > 0.05):
            failure.append('interior_audio_discontinuity')
        if previous_audio_end is not None and abs(packets['firstAudioTime'] - previous_audio_end) > 0.05:
            failure.append('fragment_audio_discontinuity')
        packets['previousVideoEnd'] = previous_video_end
        previous_video_end, previous_audio_end = packets['lastVideoEnd'], packets['lastAudioEnd']
    status, after_init, _ = api.http(base + 'init.mp4')
    check(status == 200 and after_init == init, 'initialization_changed_through_refill')
    public = directory / 'public.mp4'
    public.write_bytes(b''.join(fragments))
    actual, raw = decode_frames(public)
    reference, _ = decode_frames(source, offset)
    case['presentation'] = actual
    case['reference'] = reference
    case['presentationQualified'] = actual['presentationQualified'] and reference['presentationQualified']
    case['timestampedFrameEvidence'] = [{'pts': point, 'md5': digest} for point, digest in raw]
    if not case['presentationQualified']:
        failure.append('unqualified_preroll_mapping')
        return  # Keep raw facts; an ambiguous window is not a product-failure certificate.
    check(reference['presentedFrames'] == case['referenceClock']['expectedFrames'],
          'independent_reference_frame_count')
    for packets in case['publicFragments']:
        if 'failedAssertion' in packets:
            continue  # Already failed strictly; timing cannot qualify an empty/bounded audio stream.
        if packets['firstVideoTime'] < 0 or packets['firstAudioTime'] < 0:
            failure.append('unqualified_packet_presentation')
            continue
        if abs(packets['firstAudioTime'] - packets['firstVideoTime']) > 0.15:
            failure.append('fragment_audio_video_start')
        if (packets['previousVideoEnd'] is not None and
                abs(packets['firstVideoTime'] - packets['previousVideoEnd']) > 0.15):
            failure.append('fragment_video_discontinuity')
        if abs(packets['videoSpanSeconds'] - packets['advertisedSeconds']) > 0.15:
            failure.append('fragment_video_duration')
    if actual['identity'] != reference['identity']:
        failure.append('presented_source_frames')
        if offset:
            # Diagnose the failed window independently; never trim delivered frames
            # or use a hash match to change the requested presentation boundary.
            prior = max(k for k in metadata['keyframesSeconds']
                        if k <= metadata['sourceTimeOriginSeconds'] + offset)
            earlier, _ = decode_frames(source, prior - metadata['sourceTimeOriginSeconds'])
            case['precedingKeyDiagnostic'] = {'sourceKeyPTS': prior, 'referenceIdentity': earlier['identity'],
                                             'deliveredIdentityMatches': actual['identity'] == earlier['identity']}
    if not actual['presentationOrderValid'] or actual['firstPresentedPTS'] is None:
        failure.append('presentation_clock')
    elif abs(actual['firstPresentedPTS']) > 0.15 or abs(actual['lastPresentedPTS'] + 1/metadata['frameRate'] - duration) > 0.15:
        failure.append('presentation_duration')
    if abs(actual['presentedFrames'] / metadata['frameRate'] - facts['durationSeconds']) > 0.15:
        failure.append('advertised_continuation_timing')
    if case['mode'] == 'audio-transcode':
        facts = subprocess.check_output(['ffprobe', '-v', 'error', '-select_streams', 'a:0',
            '-show_entries', 'stream=codec_name', '-of', 'csv=p=0', str(public)], timeout=20).decode().strip()
        check(facts == 'aac', 'converted_audio_codec')
        source_audio, public_audio = audio_sequence(source, duration, offset), audio_sequence(public, duration)
        case['audioReference'], case['audioPresentation'] = source_audio, public_audio
        for expected, observed in zip(source_audio['windows'], public_audio['windows']):
            if (not expected['available'] or not observed['available'] or observed['rms'] < 0.01
                    or abs(expected['frequencyHz'] - observed.get('frequencyHz', 0)) > 10):
                failure.append('converted_audio_content')
