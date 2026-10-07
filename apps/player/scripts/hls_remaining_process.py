"""Owned disposable Server process-group joins and bounded full-EOF PCM diagnostics."""
import os
import re
import hashlib
import json
import math
import signal
import subprocess
import time
from hls_followon_public import check, bounded_bytes, encoder_count
from hls_timeline_http import source_state
from hls_timeline_packets import manifest_facts, safe_seek_phases, safe_encoder_lifecycle


def group_members(group):
    rows = subprocess.check_output(['ps', '-eo', 'pid=,pgid='], text=True, timeout=3).splitlines()
    return [int(v[0]) for row in rows if len(v := row.split()) == 2 and int(v[1]) == group]


def join_group(server):
    errors, members, survivors, zeros = [], [], [], 0
    def observe():
        try:
            return group_members(server.pid)
        except (OSError, subprocess.SubprocessError):
            errors.append('owned_group_observation_failed')
            return None
    members = observe()
    try:
        server.terminate()
        try:
            server.wait(timeout=5)
        except subprocess.TimeoutExpired:
            pass
    finally:
        survivors = observe()
        if survivors is None or survivors:
            try:
                os.killpg(server.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
        server.wait(timeout=5)
    deadline = time.monotonic() + 3
    remaining = None
    while zeros < 2 and time.monotonic() < deadline:
        remaining = observe()
        zeros = zeros + 1 if remaining == [] else 0
        time.sleep(0.05)
    return {'ownedSessionGroup': server.pid, 'observedOwnedPIDs': members,
        'forcedOwnedGroupStop': survivors is None or bool(survivors), 'remainingOwnedPIDs': remaining,
        'confirmedZeroSamples': zeros, 'qualificationFailures': errors}


def finish_processes(server, source, stop, sampler):
    facts = {'ownedFFmpegBeforeTeardown': None, 'cleanupFailures': []}
    try:
        deadline = time.monotonic() + 5
        while encoder_count(server, source) and time.monotonic() < deadline:
            time.sleep(0.05)
        facts['ownedFFmpegBeforeTeardown'] = encoder_count(server, source)
    except (OSError, subprocess.SubprocessError):
        facts['cleanupFailures'].append('owned_encoder_observation_failed')
    finally:
        stop.set()
        try:
            if sampler is not None and sampler.ident is not None:
                sampler.join(timeout=5)
        finally:
            facts['ownedProcessJoin'] = join_group(server)
    if sampler is not None and sampler.is_alive():
        facts['cleanupFailures'].append('sampler_join_failed')
    return facts


def native_pcm(path, run_deadline, directory):
    remaining = run_deadline - time.monotonic()
    check(remaining > 0, 'native_pcm_deadline')
    raw = subprocess.check_output(['ffprobe', '-v', 'error', '-select_streams', 'a:0',
        '-read_intervals', '%+#513', '-count_packets', '-show_frames', '-show_streams',
        '-show_entries', 'frame=nb_samples,pts_time,best_effort_timestamp_time:stream=nb_read_packets,sample_rate,channels,codec_name',
        '-of', 'json', str(path)], timeout=min(30, remaining))
    check(len(raw) <= 1 << 20, 'native_probe_output_bound')
    facts = json.loads(raw)
    streams, frames = facts.get('streams', []), facts.get('frames', [])
    check(len(streams) == 1 and streams[0]['channels'] == 2 and streams[0]['sample_rate'] == '48000',
          'native_stereo48k_qualification')
    packet_count = streams[0].get('nb_read_packets', '')
    check(re.fullmatch(r'[0-9]{1,3}', packet_count) and 0 < int(packet_count) < 513, 'native_complete_packet_bound')
    check(0 < len(frames) < 513 and all(type(v.get('nb_samples')) is int and 0 < v['nb_samples'] <= 65536 for v in frames), 'native_complete_frame_bound')
    clocks = [float(v['best_effort_timestamp_time']) for v in frames]
    check(all(math.isfinite(v) for v in clocks), 'native_complete_frame_clock')
    samples = sum(v['nb_samples'] for v in frames)
    check(0 < samples * 4 <= 2 << 20, 'native_complete_sample_bound')
    target = directory / (path.name + '.pcm')
    command = ['ffmpeg', '-nostdin', '-v', 'error', '-xerror', '-i', str(path), '-map', '0:a:0',
        '-frames:a', '513', '-f', 's16le', '-']
    with target.open('wb') as output, target.with_suffix('.pcm-private.log').open('wb') as error:
        process = subprocess.Popen(command, stdout=output, stderr=error)
        try:
            deadline = min(run_deadline, time.monotonic() + 30)
            while process.poll() is None:
                check(time.monotonic() < deadline, 'native_pcm_deadline')
                check(target.stat().st_size <= 2 << 20, 'native_pcm_output_bound')
                time.sleep(0.01)
            check(process.returncode == 0, 'native_pcm_decode_failed')
        finally:
            if process.poll() is None:
                process.kill()
            process.wait(timeout=5)
    check(target.stat().st_size == samples * 4, 'native_complete_eof_accounting')
    data = target.read_bytes()
    check(data and len(data) % 4 == 0, 'native_pcm_alignment')
    return data, {'stream': streams[0], 'frames': len(frames), 'packets': int(packet_count), 'samples': samples,
        'frameLimit': 513, 'limitReached': False, 'completeEOFAccounted': True,
        'decodedFrames': frames, 'decodedClockOrderValid': all(a < b for a, b in zip(clocks, clocks[1:]))}


def source_snapshot(path):
    result = source_state(path)
    result.update(inode=path.stat().st_ino, device=path.stat().st_dev)
    return result


def asset_snapshot(path, limit):
    before = path.stat()
    data = bounded_bytes(path, limit, 'physical_asset_bound')
    after = path.stat()
    identity = lambda v: (v.st_dev, v.st_ino, v.st_size, v.st_mtime_ns)
    check(identity(before) == identity(after), 'physical_asset_changed')
    return data, {'name': path.name, 'device': before.st_dev, 'inode': before.st_ino,
        'size': before.st_size, 'mtimeNs': str(before.st_mtime_ns), 'sha256': hashlib.sha256(data).hexdigest()}

def physical(cache, item_id):
    manifests = list(cache.glob(item_id + '-plan-*/audio/index.m3u8'))
    check(len(manifests) <= 1, 'physical_generation_count')
    if not manifests:
        return None
    path = manifests[0]
    generation = path.parent.parent.stat()
    raw, manifest_asset = asset_snapshot(path, 65536)
    facts, segments = manifest_facts(raw)
    check(0 < len(segments) <= 16, 'physical_fragment_bound')
    names = [name for name, _ in segments]
    check(all(re.fullmatch(r'segment-[0-9]{5}\.m4s', n) for n in names), 'physical_fragment_name')
    data, initialization = asset_snapshot(path.with_name('init.mp4'), 2 << 20)
    assets = [manifest_asset, initialization]
    for name in names:
        fragment, asset = asset_snapshot(path.with_name(name), 8 << 20)
        assets.append(asset)
        data += fragment
        check(len(data) <= 16 << 20, 'physical_join_bound')
    target = path.parent.parent.parent.parent / 'physical.mp4'
    target.write_bytes(data)
    packet_raw = subprocess.check_output(['ffprobe', '-v', 'error', '-select_streams', 'a:0',
        '-read_intervals', '%+#4096', '-show_packets', '-show_entries', 'packet=pts_time,duration_time',
        '-of', 'json', str(target)], timeout=30)
    check(len(packet_raw) <= 2 << 20, 'physical_probe_output_bound')
    packets = json.loads(packet_raw).get('packets', [])
    check(0 < len(packets) < 4096, 'physical_packet_bound')
    final_generation = path.parent.parent.stat()
    check((generation.st_dev, generation.st_ino) == (final_generation.st_dev, final_generation.st_ino), 'physical_generation_changed')
    for asset in assets:
        target = path.with_name(asset['name'])
        _, final = asset_snapshot(target, 8 << 20)
        check(final == asset, 'physical_epoch_changed')
    return {'manifestSHA256': hashlib.sha256(raw).hexdigest(), 'manifest': facts,
        'generationInode': generation.st_ino, 'generationDevice': generation.st_dev, 'assets': assets,
        'segments': names, 'packetCount': len(packets), 'firstPacket': packets[0], 'lastPacket': packets[-1]}

def annotate_case(case, log_path, source, before, server, resources, offset, pacing, invocation, audio):
    private = bounded_bytes(log_path, 2 << 20, 'private_log_bound').decode()
    case.update(safe_seek_phases(private))
    case['encoderLifecycle'] = safe_encoder_lifecycle(private)
    life = case['encoderLifecycle']
    case['workerBound'] = resources['samples'] > 0 and resources['peakOwnedFFmpeg'] <= 1 and not resources['samplingErrors'] and life['validSequence'] and life['peakActive'] == 1 and not case.get('ownedFFmpegBeforeTeardown')
    case['sourceUnchanged'] = before == source_snapshot(source)
    initial = [v for v in case.get('encoderStarts', []) if v['segment_start'] == 0]
    case['requestedInputSeekObserved'] = bool(initial) and all(v['input_seek_ms'] == round(offset * 1000) for v in initial)
    if pacing is not None:
        rows = [json.loads(row) for row in bounded_bytes(invocation, 4096, 'pacing_invocation_bound').splitlines()] if invocation.exists() else []
        case['pacingInvocations'] = rows
        matched = [v for v in rows if v['sourceMatched']]
        case['pacingApplied'] = bool(matched) and all(v['parent'] == server.pid and float(v['readrate']) == pacing for v in matched)
        case['pacedSourceEncoderInvocations'] = len(matched)
        if not case['pacingApplied']:
            case['failures'].append('pacing_not_applied')
            case['result'] = 'failed'
    if audio and (life['starts'] != 1 or life['ends'] != 1):
        case['failures'].append('audio_required_new_encoder')
        case['result'] = 'failed'
    if not case['workerBound'] or not case['sourceUnchanged']:
        case['failures'].append('worker_or_source_changed')
        case['result'] = 'failed'
    joined = case.get('ownedProcessJoin', {})
    if joined.get('confirmedZeroSamples') != 2 or joined.get('qualificationFailures') or case.get('cleanupFailures') or not case['requestedInputSeekObserved']:
        case['failures'].append('owned_process_or_request_certificate')
        case['result'] = 'failed'
    if case.get('failureClass') or case.get('cleanupFailureClass') or case['failures']:
        case['result'] = 'failed'
