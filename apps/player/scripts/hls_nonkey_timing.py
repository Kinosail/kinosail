"""Bounded paired playlist observations and independent marked AAC content."""
import hashlib
import math
import re
import stat
import subprocess
import time
from hls_followon_frames import audio_sequence, aac_clock_evidence
from hls_nonkey_installation import bounded_file
from hls_timeline_packets import manifest_facts


def generated_headers(lines):
    if not lines or lines[0] != '#EXTM3U':
        raise RuntimeError('timing_playlist_header')
    headers, index = {}, 0
    for index, line in enumerate(lines):
        if line.startswith('#EXTINF:'):
            break
        key = line.split(':')[0]
        if key in headers or key in ('#EXT-X-ENDLIST', '#EXT-X-DISCONTINUITY') or not line.startswith('#'):
            raise RuntimeError('timing_playlist_header')
        headers[key] = line
    required = ['#EXTM3U', '#EXT-X-VERSION', '#EXT-X-TARGETDURATION',
        '#EXT-X-MEDIA-SEQUENCE', '#EXT-X-PLAYLIST-TYPE', '#EXT-X-MAP']
    if not all(key in headers for key in required):
        raise RuntimeError('timing_playlist_header')
    return index, int(headers['#EXT-X-MEDIA-SEQUENCE'].split(':')[1])


def generated_order(lines):
    index, sequence = generated_headers(lines)
    while index < len(lines):
        line = lines[index]
        if line == '#EXT-X-ENDLIST' and index == len(lines) - 1:
            return  # Supported generated subset; RFC also permits ENDLIST elsewhere.
        if line == '#EXT-X-DISCONTINUITY':
            index += 1
            continue
        if not line.startswith('#EXTINF:') or index + 1 >= len(lines):
            raise RuntimeError('timing_playlist_cut_order')
        if lines[index + 1] != f'segment-{sequence:05d}.m4s':
            raise RuntimeError('timing_playlist_cut_order')
        index, sequence = index + 2, sequence + 1


def playlist_details(data):
    if not isinstance(data, bytes) or not 0 < len(data) <= 65536:
        raise RuntimeError('timing_playlist_bound')
    text = data.decode('ascii')
    lines = text.splitlines()
    allowed = (r'#EXTM3U|#EXT-X-VERSION:[0-9]{1,2}|#EXT-X-TARGETDURATION:[0-9]{1,2}'
        r'|#EXT-X-MEDIA-SEQUENCE:[0-9]{1,8}|#EXT-X-PLAYLIST-TYPE:(?:EVENT|VOD)'
        r'|#EXT-X-MAP:URI="init.mp4"|#EXTINF:[0-9]+(?:\.[0-9]{1,8})?,'
        r'|segment-[0-9]{5}\.m4s|#EXT-X-ENDLIST|#EXT-X-INDEPENDENT-SEGMENTS'
        r'|#EXT-X-ALLOW-CACHE:(?:YES|NO)|#EXT-X-DISCONTINUITY(?:-SEQUENCE:[0-9]{1,8})?')
    if len(lines) > 256 or not all(re.fullmatch(allowed, v) for v in lines):
        raise RuntimeError('timing_playlist_shape')
    generated_order(lines)
    targets = [int(v.split(':')[1]) for v in lines if v.startswith('#EXT-X-TARGETDURATION:')]
    if len(targets) != 1 or not 0 < targets[0] <= 60:
        raise RuntimeError('timing_target_shape')
    facts, cuts = manifest_facts(data)
    return facts | {'text': text, 'cuts': [list(v) for v in cuts],
        'targetDurationSeconds': targets[0],
        'targetDurationValid': all(math.floor(length + 0.5) <= targets[0] for _, length in cuts)}


def generation(path):
    values = []
    for parent in list(path.parents)[:3]:
        value = parent.lstat()
        if not stat.S_ISDIR(value.st_mode):
            raise RuntimeError('timing_directory_identity')
        values.append([value.st_dev, value.st_ino])
    return values


def physical_snapshot(path):
    result = {'stable': False}
    try:
        before = generation(path)
        identity, data = bounded_file(path, 65536, retain=True)
        result.update(identity=identity, generation=before, playlist=playlist_details(data))
        result['stable'] = before == generation(path)
    except (OSError, RuntimeError, UnicodeError) as error:
        result['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
    return result


def stable_bracket(before, after, first_workers, last_workers):
    return (before.get('stable') is True and after.get('stable') is True
        and before.get('generation') == after.get('generation')
        and before.get('identity') == after.get('identity') and first_workers == last_workers)


def physical_path(directory, uri):
    match = re.fullmatch(r'/hls/([a-f0-9]{16})/p/(r-[a-zA-Z0-9-]+)/([1-9][0-9]{2,3}p)/index.m3u8', uri)
    if not match:
        raise RuntimeError('timing_recipe_identity')
    item, token, quality = match.groups()
    return directory / 'cache' / (item + '-plan-' + token) / quality / 'index.m3u8'


def budget_http(api, uri, deadline=None):
    if deadline is None:
        return api.http(uri)
    remaining = min(40, deadline - time.monotonic())
    if remaining <= 0:
        raise RuntimeError('timing_deadline')
    response = api.http(uri, timeout=remaining)
    if time.monotonic() >= deadline:
        raise RuntimeError('timing_deadline')
    return response


def playlist_observation(api, uri, path, workers, deadline=None):
    first_workers = workers()
    before = physical_snapshot(path)
    status, data, _ = budget_http(api, uri, deadline)
    after = physical_snapshot(path)
    last_workers = workers()
    public = playlist_details(data) if status == 200 else None
    return ({'public': public, 'status': status, 'before': before, 'after': after,
        'workersBefore': first_workers, 'workersAfter': last_workers,
        'physicalBracketStable': stable_bracket(before, after, first_workers, last_workers)}, status, data)


def final_qualified(joined, snapshot, final):
    if joined < 3:
        return False
    before, after = final['before'], final['after']
    public = final.get('public') or {}
    raw = [value.get('playlist', {}) for value in [snapshot, before, after]]
    bound = (stable_bracket(snapshot, before, 0, final['workersBefore'])
        and stable_bracket(snapshot, after, 0, final['workersAfter']))
    valid = all(v.get('endlist') is True and v.get('targetDurationValid') is True for v in raw + [public])
    cuts = all(v.get('cuts') == public.get('cuts') for v in raw)
    return bound and valid and cuts and final['status'] == 200


def finish_playlist_proof(api, directory, server, source, case, encoder_count):
    evidence = case['playlistObservations']
    uri = evidence['variantURI']
    path = physical_path(directory, uri)
    workers = lambda: encoder_count(server, source)
    deadline, joined, previous = time.monotonic() + 90, 0, None
    join_deadline = time.monotonic() + 20
    while time.monotonic() < join_deadline:
        snapshot, count = physical_snapshot(path), workers()
        ready = (snapshot.get('stable') and snapshot.get('playlist', {}).get('endlist')
            and count == 0 and previous == snapshot)
        joined = joined + 1 if ready else 0
        previous = snapshot
        if joined >= 3:
            break
        time.sleep(0.05)
    evidence['producerJoinedSamples'] = joined
    evidence['producerJoinedSnapshot'] = previous
    if time.monotonic() >= deadline:
        raise RuntimeError('timing_deadline')
    final, _, _ = playlist_observation(api, uri, path, workers, deadline)
    evidence['final'] = final
    base = uri.removesuffix('index.m3u8')
    status, init, _ = budget_http(api, base + 'init.mp4', deadline)
    init_matches = status == 200 and hashlib.sha256(init).hexdigest() == case['initializationSHA256']
    initial = evidence['initial'].get('public') or {}
    final_public = final.get('public') or {}
    old_names = [v[0] for v in initial.get('cuts', [])]
    new_names = [v[0] for v in final_public.get('cuts', [])]
    if not 0 < len(new_names) <= 16:
        raise RuntimeError('timing_final_segment_bound')
    unchanged, expected = init_matches and old_names == new_names, {v['segment']: v['fragmentSHA256'] for v in case['publicFragments']}
    for name in new_names:
        status, data, _ = budget_http(api, base + name, deadline)
        unchanged = unchanged and status == 200 and hashlib.sha256(init + data).hexdigest() == expected.get(name)
    evidence.update(initializationUnchanged=init_matches, finalMediaHashesUnchanged=unchanged,
        finalObservationQualified=final_qualified(joined, previous, final))
    if time.monotonic() >= deadline:
        raise RuntimeError('timing_deadline')
    if not evidence['finalObservationQualified'] or not unchanged:
        case['failures'].append('unqualified_final_playlist_observation')


def finite_number(value, lower, upper):
    return type(value) in (int, float) and math.isfinite(value) and lower <= value <= upper


def valid_window(window):
    return (isinstance(window, dict) and window.get('available') is True
        and finite_number(window.get('rms'), 0.01, 1)
        and finite_number(window.get('frequencyHz'), 1, 8000))


def audio_content_matches(reference, public):
    left, right = reference.get('windows', []), public.get('windows', [])
    return (isinstance(left, list) and isinstance(right, list) and 0 < len(left) <= 80
        and len(left) == len(right) and all(valid_window(a) and valid_window(b)
        and abs(a['frequencyHz'] - b['frequencyHz']) <= 10 for a, b in zip(left, right)))


def expected_frequency(center, offset):
    start, end = center + offset - 0.25, center + offset + 0.25
    if not 0 <= start < end <= 32:
        raise RuntimeError('timing_source_window_scope')
    step = math.floor(start / 4)
    return 440 + 110 * step + 110 * max(0, end - (step + 1) * 4) / 0.5


def source_pattern_matches(source, centers, offset):
    windows = source.get('windows', [])
    return (isinstance(windows, list) and 0 < len(windows) == len(centers) <= 80
        and all(valid_window(window) and finite_number(window.get('sourceTimeSeconds'), 0, 32)
        and abs(window['sourceTimeSeconds'] - center - offset) <= 0.000001
        and abs(window['frequencyHz'] - expected_frequency(center, offset)) <= 10
        for window, center in zip(windows, centers)))


def copied_audio_tail(source, public):
    left, right = [[v[2] for v in value['packetRows']] for value in [source, public]]
    matches = [n for n in range(len(left) - len(right) + 1) if right and left[n:n + len(right)] == right]
    start = matches[0] if len(matches) == 1 else None
    return {'sourcePacketCount': len(left), 'publicPacketCount': len(right), 'sequenceMatches': len(matches),
        'sourceStartIndex': start, 'sourceEndIndex': start + len(right) - 1 if start is not None else None,
        'sourceTailComplete': start is not None and start + len(right) == len(left)}


def marked_audio_proof(source, public, metadata, offset, case):
    if not metadata.get('audioTimeMarked'):
        raise RuntimeError('timing_marked_audio_scope')
    duration = case['expectedTimelineSeconds']
    centers = sorted({0.25, math.floor(duration * 20) / 20 - 0.25} |
        {round(boundary - offset + step / 20, 6) for boundary in [16, 20, 24, 28] for step in range(-8, 9)})
    if len(centers) > 80 or not all(0.25 <= v <= duration - 0.25 for v in centers):
        raise RuntimeError('timing_audio_window_bound')
    reference = audio_sequence(source, duration, offset, centers)
    observed = audio_sequence(public, duration, centers=centers)
    source_valid = source_pattern_matches(reference, centers, offset)
    matched = source_valid and audio_content_matches(reference, observed)
    case['markedAAC'] = {'boundary': 'Independent offline AAC content/timing; browser/native audible and priming acceptance separate',
        'relativeWindowCenters': centers, 'windowWidthSeconds': 0.5, 'frequencyLimitHz': 10,
        'source': reference, 'public': observed, 'sourcePatternQualified': source_valid, 'contentMatches': matched,
        'decodedSampleDifference': observed['decodedSamples'] - reference['decodedSamples']}
    if not matched:
        case['failures'].append('copied_marked_audio_content')
    case['markedAAC']['decoderBudgetControl'] = {'phase': 'source_probe', 'complete': False}
    try:
        decoder_budget_proof(source, public, duration, centers, reference, source_valid, case)
    except (RuntimeError, OSError, ValueError, subprocess.TimeoutExpired) as error:
        safe = ('aac_clock_timeout' if isinstance(error, subprocess.TimeoutExpired) else
            str(error) if isinstance(error, RuntimeError) else type(error).__name__)
        if not re.fullmatch('[a-z_]{1,64}', safe):
            safe = 'aac_clock_error'
        case['markedAAC']['decoderBudgetControl'].update(failureClass=safe, complete=False)
        case['failures'].append('aac_clock_unqualified')


def decoder_budget_proof(source, public, duration, centers, reference, source_valid, case):
    evidence = case['markedAAC']['decoderBudgetControl']
    source_clock = aac_clock_evidence(source)
    evidence.update(sourceClock=source_clock, phase='public_probe')
    public_clock = aac_clock_evidence(public)
    evidence.update(publicClock=public_clock, phase='decode_budget')
    scope = source_clock['skipDiscardCountsInOriginalScope'] and public_clock['skipDiscardCountsInOriginalScope']
    if not scope:
        case['failures'].append('aac_clock_skip_scope')
    extra = max(0, -public_clock['formatStartSeconds'])
    control = audio_sequence(public, duration, centers=centers, output_budget_extra=extra)
    evidence.update({
        'complete': True, 'phase': 'completed',
        'boundary': 'Extra output time budget from measured negative format origin only; original failed windows remain',
        'originalSkipScopeQualified': scope, 'outputBudgetAddedSeconds': extra, 'public': control, 'sourceClock': source_clock, 'publicClock': public_clock,
        'decodedSampleDifference': control['decodedSamples'] - reference['decodedSamples'],
        'contentMatches': source_valid and audio_content_matches(reference, control),
        'copiedSourceTail': copied_audio_tail(source_clock, public_clock)})
