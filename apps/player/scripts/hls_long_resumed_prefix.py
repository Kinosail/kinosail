"""Same-window long copied VOD proof; no browser can select a later offset."""
import hashlib
import json
import re
import subprocess
import time
from hls_timeline_fixture import LONG_RESUME_KEYS, check, fixture


def long_fixture(media, boundary, receipt):
    keys = [*LONG_RESUME_KEYS, 77, 81, 84, 89, 93, 96, 100, 104]
    offset, first_frame, first_cut = 60000, 1450, 4
    if boundary == 'on-key':
        keys[keys.index(58)] = 60
        first_frame, first_cut = 1500, 2
    elif boundary == 'after-key':
        keys[keys.index(58)] = 59.96
        first_frame, first_cut = 1499, 2.04
    elif boundary == 'origin':
        keys = [0, 32, *(key for key in keys if key > 32)]
        offset, first_frame, first_cut = 30000, 0, 32
    receipt.update(longBoundary=boundary, longResumeOffset=offset,
                   longExpectedFirstFrame=first_frame, longExpectedFirstCut=first_cut)
    source, evidence = fixture(media, 'Prefix Sample', 25 * (6 if boundary == 'previous' else 40),
                              ','.join(map(str, keys)), rate='25', frames=2662, extension='.mp4')
    check(all(abs(actual - expected) < .000001 for actual, expected in zip(evidence['keyframesSeconds'], keys)),
          'long_fixture_exact_key_positions')
    return source, evidence


def cuts(data):
    rows = re.findall(rb'^#EXTINF:(\d+(?:\.\d+)?),[^\n]*\n(segment-\d{5}\.m4s)$', data, re.M)
    check(0 < len(rows) <= 64, 'long_window_cut_bound')
    return [(float(duration), name.decode()) for duration, name in rows]


def decoded_frames(source, run, label):
    destination = run / (label + '-frames.private.txt')
    with destination.open('wb') as output, (run / (label + '-decode.private.log')).open('wb') as log:
        result = subprocess.run(['ffmpeg', '-nostdin', '-v', 'error', '-err_detect', 'explode',
            '-threads', '1', '-i', str(source), '-map', '0:v:0', '-an', '-sn', '-dn',
            '-enc_time_base', '1:1000000', '-fps_mode', 'passthrough', '-f', 'framemd5', '-'],
            stdout=output, stderr=log, timeout=45)
    check(result.returncode == 0 and (run / (label + '-decode.private.log')).stat().st_size == 0,
          'long_window_complete_decode')
    check(destination.stat().st_size <= 1 << 20, 'long_window_frame_record_bound')
    rows = [line.split(',') for line in destination.read_text().splitlines() if line and not line.startswith('#')]
    check(3 <= len(rows) <= 3000 and all(len(row) == 6 and re.fullmatch(r'\s*[0-9a-f]{32}\s*', row[-1]) for row in rows),
          'long_window_frame_records')
    return [row[-1].strip() for row in rows]


def frame_receipt(frames):
    return {'frames': len(frames), 'firstThree': frames[:3], 'lastThree': frames[-3:],
            'sequenceSHA256': hashlib.sha256('\n'.join(frames).encode()).hexdigest()}


def long_window(api, path, run, source, physical, receipt):
    initial = (run / 'public-prefix.m3u8').read_bytes()
    completed = (run / 'public-completed.m3u8').read_bytes()
    actual = physical.read_bytes()
    first, final, stored = cuts(initial), cuts(completed), cuts(actual)
    evidence = {'initialVOD': b'#EXT-X-PLAYLIST-TYPE:VOD' in initial and b'#EXT-X-ENDLIST' in initial,
                'actualCompleted': b'#EXT-X-ENDLIST' in actual, 'initialCuts': first,
                'finalPublicCuts': final, 'finalPhysicalCuts': stored,
                'sameWindow': f"-o{receipt['longResumeOffset']}/" in path, 'immutableCompleteMap': first == final == stored}
    receipt['longWindow'] = evidence
    base = path.removesuffix('index.m3u8')
    status, initialization, _ = api.http(base + 'init.mp4')
    check(status == 200 and len(initialization) <= 2 << 20, 'long_window_init_status')
    joined = run / 'long-window-public.mp4'
    with joined.open('wb') as output:
        output.write(initialization)
        evidence['fragmentStatuses'] = []
        for _, name in stored:
            status, data, _ = api.http(base + name)
            evidence['fragmentStatuses'].append({'segment': name, 'status': status})
            check(status == 200 and data, 'long_window_fragment_status')
            check(output.tell() + len(data) <= 32 << 20, 'long_window_media_bound')
            output.write(data)
    reference = decoded_frames(source, run, 'long-source')
    compatible = decoded_frames(joined, run, 'long-compatible')
    # The fixture independently fixes its retained key on the source frame clock.
    # Compare every decoded frame to reject a full-source or other-window response.
    expected_start = receipt['longExpectedFirstFrame']
    evidence['reference'] = frame_receipt(reference)
    evidence['compatible'] = frame_receipt(compatible)
    evidence['expectedFirstSourceFrame'] = expected_start
    evidence['firstSourceFrame'] = next((i for i in range(len(reference) - 2)
        if reference[i:i + 3] == compatible[:3]), None)
    evidence['completeSourceSuffix'] = (len(reference) == 2662
        and compatible == reference[expected_start:])
    check(receipt['prefixStatus'] == 200 and evidence['initialVOD'] and evidence['actualCompleted'] and evidence['sameWindow']
          and evidence['immutableCompleteMap'] and evidence['completeSourceSuffix']
          and receipt['heldPrefixUnchanged'] and receipt['producerHeld'] and receipt['completedMatchesCache']
          and receipt['warmStable'] and receipt['restartStable'] and receipt['noWarmEncode']
          and receipt['firstServerSettled'], 'long_copied_vod_must_preserve_complete_window')


def long_window_join(run, receipt):
    """Give admitted wrappers time to reap children before stopping their owner."""
    deadline = time.monotonic() + 65
    receipt['longCleanupJoinedBeforeServerStop'] = False
    try:
        while time.monotonic() < deadline:
            admissions = list(run.glob('*.started'))
            check(len(admissions) <= 16, 'long_window_admission_bound')
            children = [path.read_text() for path in admissions]
            if all(child.isdecimal() and (run / (child + '.joined')).is_file() for child in children):
                receipt['longCleanupJoinedBeforeServerStop'] = True
                return
            time.sleep(.025)
        receipt['longCleanupFailureClass'] = 'join_timeout'
    except Exception as error:
        receipt['longCleanupFailureClass'] = type(error).__name__


def final_evidence(api, run, receipt):
    admissions = list(run.glob('*.started'))
    child = admissions[0].read_text() if len(admissions) == 1 else ''
    joined = child.isdecimal() and (run / (child + '.joined')).is_file()
    with (run / 'server-private.log').open('rb') as source:
        data = source.read((4 << 20) + 1)
    check(len(data) <= 4 << 20, 'long_window_log_bound')
    raw = data.decode()
    events = [json.loads(line) for line in raw.splitlines() if line.startswith('{')]
    outcomes = [event for event in events if event.get('msg') in ('HLS transcode completed', 'HLS transcode failed')]
    receipt['longNoLateEncoder'] = len(admissions) == 1
    receipt['longProducerSuccessfullyJoined'] = (joined and len(outcomes) == 1
        and outcomes[0].get('msg') == 'HLS transcode completed' and outcomes[0].get('level') == 'INFO'
        and bool(outcomes[0].get('request_id')))
    receipt['longDiagnosticSecretsAbsent'] = all(secret not in raw for secret in
        (api.token, 'synthetic-timeline-password', 'synthetic-prefix-key') if secret)
    check(receipt['longNoLateEncoder'] and receipt['longProducerSuccessfullyJoined']
          and receipt['longDiagnosticSecretsAbsent'] and receipt['longCleanupJoinedBeforeServerStop'],
          'long_window_final_ownership')


def long_window_final(api, run, receipt, require_success):
    try:
        final_evidence(api, run, receipt)
    except Exception as error:
        receipt['longFinalEvidenceFailureClass'] = type(error).__name__
        if require_success:
            raise


def long_window_rejection(api, path, run, receipt):
    """An uncertain resumed index must reject before any encoder or VOD admission."""
    status, _, _ = api.http(path)
    receipt['longIndexFaultStatus'] = status
    pid_file = run / 'index-probe-pid'
    receipt['longIndexProbeReached'] = pid_file.is_file()
    pid = pid_file.read_text() if pid_file.exists() else ''
    receipt['longIndexProbeSettled'] = pid.isdecimal() and subprocess.run(
        ['ps', '-p', pid], stdout=subprocess.DEVNULL, check=False).returncode != 0
    receipt['longIndexNoEncoder'] = not list(run.glob('*.started'))
    receipt['longIndexNoPublishedMedia'] = not any((run / 'cache').rglob('*.m3u8'))
    with (run / 'server-private.log').open('rb') as log:
        data = log.read((4 << 20) + 1)
    check(len(data) <= 4 << 20, 'long_index_log_bound')
    raw = data.decode()
    events = [json.loads(line) for line in raw.splitlines() if line.startswith('{')]
    failures = [e for e in events if e.get('msg') == 'HLS transcode failed']
    receipt['longIndexRootDiagnostic'] = (len(failures) == 1 and failures[0].get('level') == 'ERROR'
        and bool(failures[0].get('request_id')) and 'copied HLS generation plan' in failures[0].get('error', ''))
    receipt['longIndexSecretsAbsent'] = all(secret not in raw for secret in
        (api.token, 'synthetic-timeline-password', 'synthetic-prefix-key') if secret)
    check(status == 503 and receipt['longIndexProbeReached'] and receipt['longIndexProbeSettled']
          and receipt['longIndexNoEncoder'] and receipt['longIndexNoPublishedMedia']
          and receipt['longIndexRootDiagnostic'] and receipt['longIndexSecretsAbsent'],
          'uncertified_long_index_must_reject_before_encoding')
