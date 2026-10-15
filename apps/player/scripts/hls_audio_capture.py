"""Decoded-audio oracle for generated noise played by the real watch page."""
from array import array
import json
import math
import os
from pathlib import Path
import subprocess
import sys

from hls_followon_public import check
from hls_timeline_http import sha


def floats(data):
    check(0 < len(data) <= 30 << 20 and len(data) % 4 == 0, 'pcm_size')
    samples = array('f', data)
    if sys.byteorder != 'little':
        samples.byteswap()
    check(all(math.isfinite(v) for v in samples), 'pcm_nonfinite')
    return samples


def decode_source(source):
    command = ['ffmpeg', '-v', 'error', '-i', str(source), '-map', '0:a:0',
               '-ac', '1', '-ar', '48000', '-f', 'f32le', 'pipe:1']
    result = subprocess.run(command, capture_output=True, check=True, timeout=30)
    return floats(result.stdout), command


def correlation(left, right):
    energy = sum(v * v for v in left) * sum(v * v for v in right)
    return sum(a * b for a, b in zip(left, right)) / math.sqrt(energy) if energy > 1e-15 else 0


def capture_audio(root, run, api, item, label, point, span, source_pcm):
    env = dict(os.environ, QA_RESUME_ORIGIN=api.url, QA_RESUME_TOKEN=api.token,
               QA_RESUME_ITEM=item['id'], QA_AUDIO_OUTPUT=str(run / label),
               QA_AUDIO_CAPTURE_POINT=str(point), QA_AUDIO_CAPTURE_SPAN=str(span))
    command = ['node', str(root / 'apps/player/scripts/hls-audio-capture.mjs')]
    with (run / (label + '-private.log')).open('w') as log:
        result = subprocess.run(command, cwd=root, env=env, stdout=log, stderr=log, timeout=90)
    check(result.returncode == 0, 'browser_audio_capture')
    facts = json.loads((run / (label + '.json')).read_text())
    samples = floats((run / (label + '.f32')).read_bytes())
    check(facts['passed'] and facts['capture']['sampleRate'] == 48000, 'capture_sample_rate')
    check(len(samples) > (span - .6) * 48000, 'capture_incomplete')
    anchor, size = 4800, 512
    expected = round(point * 48000) + anchor
    lags = range(-2048, 2049)
    lag = max(lags, key=lambda n: correlation(samples[anchor:anchor + size], source_pcm[expected + n:expected + n + size]))
    rows = []
    end = min(len(samples) - 4096, len(source_pcm) - round(point * 48000) - 4096, round((span - .1) * 48000))
    for at in range(anchor, end, 512):
        start = round(point * 48000) + at + lag
        rows.append({'sourceSeconds': start / 48000,
                     'correlation': correlation(samples[at:at + 2048], source_pcm[start:start + 2048])})
    blocks = facts['capture']['blocks']
    gaps = [b['frame'] - a['frame'] - a['length'] for a, b in zip(blocks, blocks[1:])]
    minimum = min(row['correlation'] for row in rows)
    verdict = abs(lag) <= 48 and minimum >= .90 and not any(gaps)
    comparison = {'label': label, 'command': command, 'point': point, 'span': span,
                  'lagSamples': lag, 'minimumCorrelation': minimum, 'rows': rows,
                  'renderGapFrames': [gap for gap in gaps if gap], 'passed': verdict,
                  'pcmSHA256': sha(run / (label + '.f32')),
                  'startup': {key: facts['capture'][key] for key in ['checkpoints', 'events']},
                  'oracle': 'One fixed source alignment; all 2048-sample windows at least .90 correlation; initial offset at most 1ms; contiguous render blocks.'}
    (run / (label + '-comparison.json')).write_text(json.dumps(comparison, indent=2) + '\n')
    return {key: comparison[key] for key in ['label', 'point', 'span', 'lagSamples', 'minimumCorrelation', 'renderGapFrames', 'passed', 'pcmSHA256', 'startup']}


def packet_grid(rendition):
    initialization = (rendition / 'init.mp4').read_bytes()
    rows, previous, expected_phase, expected_rate = [], None, None, None
    for fragment in sorted(rendition.glob('segment-*.m4s')):
        command = ['ffprobe', '-v', 'error', '-show_packets', '-show_streams',
                   '-show_entries', 'packet=pts,dts,duration,codec_type:stream=codec_type,sample_rate,time_base:stream_tags=:stream_disposition=',
                   '-of', 'json', 'pipe:0']
        result = subprocess.run(command, input=initialization + fragment.read_bytes(), capture_output=True, check=True, timeout=15)
        check(len(result.stdout) <= 1 << 20, 'audio_probe_bound')
        data = json.loads(result.stdout)
        audio = next(s for s in data['streams'] if s['codec_type'] == 'audio')
        video = next(s for s in data['streams'] if s['codec_type'] == 'video')
        packets = [p for p in data['packets'] if p['codec_type'] == 'audio']
        rate = int(audio['sample_rate'])
        check(audio['time_base'] == f'1/{rate}' and packets, 'audio_probe_clock')
        numerator, denominator = map(int, video['time_base'].split('/'))
        first_video = next(p for p in data['packets'] if p['codec_type'] == 'video')
        video_start = first_video['dts'] * numerator / denominator
        start, end = packets[0]['pts'], packets[-1]['pts'] + packets[-1]['duration']
        expected_phase = start % 1024 if expected_phase is None else expected_phase
        expected_rate = rate if expected_rate is None else expected_rate
        gap = start - previous if previous is not None else 0
        valid = rate == expected_rate and abs(gap) <= 1 and all(p['pts'] % 1024 == expected_phase for p in packets)
        rows.append({'fragment': fragment.name, 'sampleRate': rate, 'start': start, 'end': end,
                     'phase': start % 1024, 'gapSamples': gap, 'videoStartDTS': video_start, 'passed': valid})
        previous = end
    for row, following in zip(rows, rows[1:]):
        boundary_gap = row['end'] - following['videoStartDTS'] * row['sampleRate']
        row['videoBoundaryGapSamples'] = boundary_gap
        row['passed'] = row['passed'] and -1 <= boundary_gap <= 1025
    return rows
