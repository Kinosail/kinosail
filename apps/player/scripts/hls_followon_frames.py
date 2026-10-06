"""Bounded independent presentation evidence; no hash-based seek trimming."""
import hashlib
import json
import math
import re
import struct
import subprocess


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


def stream_metadata(path):
    data = subprocess.check_output(['ffprobe', '-v', 'error', '-show_entries',
        'format=start_time,duration:stream=index,codec_type,codec_name,time_base,start_time,duration,avg_frame_rate',
        '-of', 'json', str(path)], timeout=30)
    if len(data) > 65536:
        raise RuntimeError('stream_origin_bound')
    facts = json.loads(data)
    if not 0 < len(facts.get('streams', [])) <= 8:
        raise RuntimeError('stream_origin_shape')
    return facts


def decode_frames(path, offset=None):
    command = ['ffmpeg', '-nostdin', '-v', 'error', '-xerror', '-threads', '2']
    if offset is None:
        command += ['-copyts']
    command += ['-i', str(path)]
    if offset is not None and offset:
        command += ['-ss', str(offset)]  # Output-side reference seek, never input seek.
    command += ['-an', '-frames:v', '4097', '-fps_mode', 'passthrough',
                '-enc_time_base', '1:1000000', '-f', 'framemd5', 'pipe:1']
    result = subprocess.run(command, capture_output=True, timeout=60)
    if result.returncode:
        raise RuntimeError('presentation_decode_failed')
    rows = parse_frames(result.stdout)
    facts = frame_facts(rows)
    facts['containerPresentation'] = stream_metadata(path)
    return facts, rows


def audio_sequence(path, duration, offset=0):
    command = ['ffmpeg', '-nostdin', '-v', 'error', '-xerror', '-threads', '2', '-i', str(path)]
    if offset:
        command += ['-ss', str(offset)]
    command += ['-vn', '-t', str(duration + 0.2), '-ac', '1', '-ar', '16000', '-f', 'f32le', 'pipe:1']
    result = subprocess.run(command, capture_output=True, timeout=40)
    if result.returncode or not 0 < len(result.stdout) <= 3 * 1024 * 1024 or len(result.stdout) % 4:
        raise RuntimeError('audio_decode_bound')
    samples = [v[0] for v in struct.iter_unpack('<f', result.stdout)]
    if any(not math.isfinite(v) for v in samples):
        raise RuntimeError('audio_nonfinite')
    windows = []
    for second in range(2, math.floor(duration), 4):
        center = second * 16000
        values = samples[center - 4000:center + 4000]
        if len(values) != 8000:
            windows.append({'sourceTimeSeconds': second + offset, 'available': False})
            continue
        crossings = sum(a <= 0 < b for a, b in zip(values, values[1:]))
        windows.append({'sourceTimeSeconds': second + offset, 'available': True,
                        'frequencyHz': crossings * 2, 'rms': math.sqrt(sum(v*v for v in values) / len(values))})
    return {'decodedSamples': len(samples), 'windows': windows}
