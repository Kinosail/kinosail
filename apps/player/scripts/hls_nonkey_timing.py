"""Bounded paired playlist observations and independent marked AAC content."""
import hashlib
import math
import re
import stat
import time
from hls_followon_frames import audio_sequence
from hls_nonkey_installation import bounded_file
from hls_timeline_packets import manifest_facts


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


def playlist_observation(api, uri, path, workers):
    first_workers = workers()
    before = physical_snapshot(path)
    status, data, _ = api.http(uri)
    after = physical_snapshot(path)
    last_workers = workers()
    public = playlist_details(data) if status == 200 else None
    return ({'public': public, 'status': status, 'before': before, 'after': after,
        'workersBefore': first_workers, 'workersAfter': last_workers,
        'physicalBracketStable': stable_bracket(before, after, first_workers, last_workers)}, status, data)


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
    if time.monotonic() >= deadline:
        raise RuntimeError('timing_deadline')
    final, _, _ = playlist_observation(api, uri, path, workers)
    evidence['final'] = final
    base = uri.removesuffix('index.m3u8')
    status, init, _ = api.http(base + 'init.mp4')
    init_matches = status == 200 and hashlib.sha256(init).hexdigest() == case['initializationSHA256']
    initial = evidence['initial'].get('public') or {}
    final_public = final.get('public') or {}
    old_names = [v[0] for v in initial.get('cuts', [])]
    new_names = [v[0] for v in final_public.get('cuts', [])]
    if not 0 < len(new_names) <= 16:
        raise RuntimeError('timing_final_segment_bound')
    unchanged, expected = init_matches and old_names == new_names, {v['segment']: v['fragmentSHA256'] for v in case['publicFragments']}
    for name in new_names:
        if time.monotonic() >= deadline:
            raise RuntimeError('timing_deadline')
        status, data, _ = api.http(base + name)
        unchanged = unchanged and status == 200 and hashlib.sha256(init + data).hexdigest() == expected.get(name)
    evidence.update(initializationUnchanged=init_matches, finalMediaHashesUnchanged=unchanged,
        finalObservationQualified=joined >= 3 and final['physicalBracketStable'] and final['workersAfter'] == 0)
    if not evidence['finalObservationQualified'] or not unchanged:
        case['failures'].append('unqualified_final_playlist_observation')


def audio_content_matches(reference, public):
    left, right = reference.get('windows', []), public.get('windows', [])
    return (bool(left) and len(left) == len(right) and all(a.get('available') is True
        and b.get('available') is True and a.get('rms', 0) >= 0.01 and b.get('rms', 0) >= 0.01
        and abs(a.get('frequencyHz', 0) - b.get('frequencyHz', 0)) <= 10 for a, b in zip(left, right)))


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
    matched = audio_content_matches(reference, observed)
    case['markedAAC'] = {'boundary': 'Independent offline AAC content/timing; browser/native audible and priming acceptance separate',
        'relativeWindowCenters': centers, 'windowWidthSeconds': 0.5, 'frequencyLimitHz': 10,
        'source': reference, 'public': observed, 'contentMatches': matched,
        'decodedSampleDifference': observed['decodedSamples'] - reference['decodedSamples']}
    if not matched:
        case['failures'].append('copied_marked_audio_content')
