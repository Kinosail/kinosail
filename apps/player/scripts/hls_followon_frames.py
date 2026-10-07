"""Bounded independent presentation evidence; no hash-based seek trimming."""
import hashlib
import json
import math
import re
import struct
import subprocess
import time


def parse_frames(data):
    if len(data) > 2 * 1024 * 1024:
        raise RuntimeError('frame_evidence_bound')
    clock = re.search(rb'^#tb 0: ([0-9]+)/([0-9]+)$', data, re.M)
    if not clock or not 0 < int(clock[1]) <= int(clock[2]):
        raise RuntimeError('frame_clock_missing')
    tick = int(clock[1]) / int(clock[2])
    rows = []
    for line in data.splitlines():
        if not line or line.startswith(b'#'):
            continue
        values = [v.strip() for v in line.split(b',')]
        if (len(values) != 6 or values[0] != b'0' or
                re.fullmatch(rb'-?[0-9]+', values[2]) is None or
                re.fullmatch(rb'[a-f0-9]{32}', values[5]) is None):
            raise RuntimeError('frame_evidence_shape')
        rows.append((int(values[2]) * tick, values[5].decode()))
    if not 0 < len(rows) <= 4096:
        raise RuntimeError('frame_evidence_bound')
    return rows


def frame_facts(rows):
    # Without an independently verified edit/preroll mapping, no negative frame
    # can be silently removed. Such a case remains explicitly unqualified.
    presented = rows
    hashes = '\n'.join(value for _, value in presented).encode()
    negatives = sum(point < 0 for point, _ in rows)
    return {'rawFrames': len(rows), 'negativeTimestampFrames': negatives,
            'presentationQualified': negatives == 0,
            'presentedFrames': len(presented),
            'identity': {'frames': len(presented), 'sha256': hashlib.sha256(hashes).hexdigest()},
            'firstPresentedPTS': presented[0][0] if presented else None,
            'lastPresentedPTS': presented[-1][0] if presented else None,
            'presentationOrderValid': all(a[0] < b[0] for a, b in zip(rows, rows[1:])),
            'timestampedFrameSHA256': hashlib.sha256(repr(rows).encode()).hexdigest()}


def remaining_timeout(deadline, limit):
    budget = limit if deadline is None else min(limit, deadline - time.monotonic())
    if budget <= 0:
        raise RuntimeError('frame_evidence_deadline')
    return budget


def stream_metadata(path, deadline=None):
    data = subprocess.check_output(['ffprobe', '-v', 'error', '-show_entries',
        'format=start_time,duration:stream=index,codec_type,codec_name,time_base,start_time,duration,avg_frame_rate',
        '-of', 'json', str(path)], timeout=remaining_timeout(deadline, 30))
    if len(data) > 65536:
        raise RuntimeError('stream_origin_bound')
    facts = json.loads(data)
    if not 0 < len(facts.get('streams', [])) <= 8:
        raise RuntimeError('stream_origin_shape')
    return facts


def decode_frames(path, offset=None, deadline=None):
    command = ['ffmpeg', '-nostdin', '-v', 'error', '-xerror', '-threads', '2']
    if offset is None:
        command += ['-copyts']
    command += ['-i', str(path)]
    if offset is not None and offset:
        command += ['-ss', str(offset)]  # Output-side reference seek, never input seek.
    command += ['-an', '-frames:v', '4097', '-fps_mode', 'passthrough',
                '-enc_time_base', '1:1000000', '-f', 'framemd5', 'pipe:1']
    result = subprocess.run(command, capture_output=True, timeout=remaining_timeout(deadline, 60))
    if result.returncode:
        raise RuntimeError('presentation_decode_failed')
    rows = parse_frames(result.stdout)
    facts = frame_facts(rows)
    facts['containerPresentation'] = stream_metadata(path, deadline)
    return facts, rows


def audio_sequence(path, duration, offset=0, centers=None, output_budget_extra=0):
    if type(output_budget_extra) not in (int, float) or not math.isfinite(output_budget_extra) or not 0 <= output_budget_extra <= 1:
        raise RuntimeError('audio_budget_scope')
    command = ['ffmpeg', '-nostdin', '-v', 'error', '-xerror', '-threads', '2', '-i', str(path)]
    if offset:
        command += ['-ss', str(offset)]
    command += ['-vn', '-t', str(duration + 0.2 + output_budget_extra), '-ac', '1', '-ar', '16000', '-f', 'f32le', 'pipe:1']
    result = subprocess.run(command, capture_output=True, timeout=40)
    if result.returncode or not 0 < len(result.stdout) <= 3 * 1024 * 1024 or len(result.stdout) % 4:
        raise RuntimeError('audio_decode_bound')
    samples = [v[0] for v in struct.iter_unpack('<f', result.stdout)]
    if any(not math.isfinite(v) for v in samples):
        raise RuntimeError('audio_nonfinite')
    windows = []
    for second in centers if centers is not None else range(2, math.floor(duration), 4):
        center = round(second * 16000)
        values = samples[center - 4000:center + 4000]
        if len(values) != 8000:
            windows.append({'sourceTimeSeconds': second + offset, 'available': False})
            continue
        crossings = sum(a <= 0 < b for a, b in zip(values, values[1:]))
        windows.append({'sourceTimeSeconds': second + offset, 'available': True,
                        'frequencyHz': crossings * 2, 'rms': math.sqrt(sum(v*v for v in values) / len(values))})
    return {'decodedSamples': len(samples), 'windows': windows}


def audio_clock(value):
    try:
        number = float(value)
    except (TypeError, ValueError, OverflowError):
        raise RuntimeError('aac_clock_shape') from None
    if type(value) is bool or not math.isfinite(number) or not -1 <= number <= 40:
        raise RuntimeError('aac_clock_shape')
    return number


def audio_skips(values, sample_rate):
    if not isinstance(values, list) or len(values) > 8:
        raise RuntimeError('aac_skip_bound')
    result = []
    for value in values:
        if not isinstance(value, dict) or value.get('side_data_type') != 'Skip Samples':
            raise RuntimeError('aac_skip_shape')
        counts = [value.get(key) for key in ['skip_samples', 'discard_padding']]
        if not all(type(n) is int and 0 <= n <= sample_rate for n in counts):
            raise RuntimeError('aac_skip_shape')
        result.append(counts)
    return result


def parse_aac_clock_probe(data):
    if not isinstance(data, bytes) or not 0 < len(data) <= 2 * 1024 * 1024:
        raise RuntimeError('aac_probe_bound')
    try:
        value = json.loads(data)
        streams, rows = value['streams'], value['packets_and_frames']
        if not isinstance(streams, list) or len(streams) != 1:
            raise RuntimeError('aac_stream_shape')
        rate = streams[0]['sample_rate']
        if not isinstance(rate, str) or not re.fullmatch('[1-9][0-9]{3,5}', rate) or not 8000 <= int(rate) <= 192000:
            raise RuntimeError('aac_rate_shape')
        origin, duration = [audio_clock(value['format'][key]) for key in ['start_time', 'duration']]
        if duration <= 0 or not isinstance(rows, list) or not 0 < len(rows) <= 8192:
            raise RuntimeError('aac_probe_rows')
        packets, frames = [], []
        for row in rows:
            point = audio_clock(row['pts_time'])
            if row['type'] == 'packet':
                length, digest = audio_clock(row['duration_time']), row['data_hash']
                if not 0 < length <= 1 or not isinstance(digest, str) or not re.fullmatch('SHA256:[a-f0-9]{64}', digest):
                    raise RuntimeError('aac_packet_shape')
                packets.append([point, length, digest[7:], audio_skips(row.get('side_data_list', []), int(rate))])
            elif row['type'] == 'frame' and type(row.get('nb_samples')) is int and 0 < row['nb_samples'] <= 8192:
                frames.append([point, row['nb_samples']])
            else:
                raise RuntimeError('aac_frame_shape')
        if not 0 < len(packets) < 4096 or not 0 < len(frames) < 4096:
            raise RuntimeError('aac_complete_probe_bound')
        return {'sampleRate': int(rate), 'formatStartSeconds': origin, 'formatDurationSeconds': duration,
            'packetRowColumns': ['pts', 'duration', 'payloadSHA256', 'skipDiscardSamples'], 'packetRows': packets,
            'frameRowColumns': ['pts', 'nbSamples'], 'frameRows': frames,
            'decodedSamplesAtSourceRate': sum(v[1] for v in frames),
            'skipDiscardCountsInOriginalScope': all(n <= 8192 for row in packets for counts in row[3] for n in counts)}
    except (KeyError, TypeError, ValueError, OverflowError):
        raise RuntimeError('aac_probe_shape') from None


def aac_clock_evidence(path):
    command = ['ffprobe', '-v', 'error', '-threads', '2', '-select_streams', 'a:0', '-read_intervals', '%+#4096',
        '-show_packets', '-show_frames', '-show_data_hash', 'sha256', '-show_entries',
        'packet=type,pts_time,duration_time,data_hash:packet_side_data=side_data_type,skip_samples,discard_padding:'
        'frame=type,pts_time,nb_samples:stream=sample_rate:format=start_time,duration', '-of', 'json', str(path)]
    result = subprocess.run(command, capture_output=True, timeout=40)
    if result.returncode:
        raise RuntimeError('aac_complete_probe')
    return parse_aac_clock_probe(result.stdout)
