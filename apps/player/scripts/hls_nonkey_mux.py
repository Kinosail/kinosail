"""Fixed copied-packet mux experiments; offline results never admit Server repair."""
import struct
import subprocess
import time
import json
import hashlib
from hls_followon_frames import decode_frames, parse_frames, frame_facts, remaining_timeout
from hls_followon_public import check, bounded_bytes
from hls_nonkey_initialization import initialization_metadata
from hls_nonkey_fragment import fragment_metadata
from hls_timeline_packets import manifest_facts
from hls_timeline_http import sha


def initial_box(path, target=b'moov', limit=1024 * 1024):
    # Read only through the bounded initialization, before any media fragment.
    with path.open('rb') as file:
        data = file.read(limit + 1)
    position = 0
    for _ in range(64):
        check(len(data) - position >= 8, 'nonkey_movie_header')
        size, kind = struct.unpack_from('>I4s', data, position)
        header = 8
        if size == 1:
            check(len(data) - position >= 16, 'nonkey_movie_extended_header')
            size, header = struct.unpack_from('>Q', data, position + 8)[0], 16
        check(header <= size <= limit - position and position + size <= len(data),
              'nonkey_movie_extent')
        beginning = position
        position += size
        if kind == target:
            return data[:position] if target == b'moov' else data[beginning:position]
    raise RuntimeError('nonkey_movie_box_bound')


def presentation_experiment(path, reference, deadline=None):
    # Explicit output-zero decode is a measured decoder window, not hash trimming.
    # It remains offline evidence until raw edits and a public renderer qualify it.
    command = ['ffmpeg', '-nostdin', '-v', 'error', '-xerror', '-threads', '2', '-i', str(path),
        '-ss', '0', '-an', '-frames:v', '4097', '-fps_mode', 'passthrough',
        '-enc_time_base', '1:1000000', '-f', 'framemd5', 'pipe:1']
    result = subprocess.run(command, capture_output=True, timeout=remaining_timeout(deadline, 60))
    check(result.returncode == 0, 'nonkey_presentation_experiment_decode')
    rows = parse_frames(result.stdout)
    facts = frame_facts(rows)
    return {'boundary': 'Explicit decoder experiment only; does not replace strict public facts',
        'command': command, 'facts': facts, 'strictReferenceIdentityMatches': facts['identity'] == reference['identity'],
        'rowColumns': ['pts', 'md5'],
        'completeRows': [json.dumps(row, separators=(',', ':')) for row in rows]}


def fragment_evidence(data):
    tracks = fragment_metadata(data)
    return {'sha256': hashlib.sha256(data).hexdigest(),
        'boundary': 'Stored unsigned decode time and signed/unsigned sample composition, before edits',
        'tracks': [{'trackID': track['trackID'], 'rowColumns': ['dts', 'pts', 'duration', 'flags'],
            'rows': [json.dumps([row[k] for k in ['dts', 'pts', 'duration', 'flags']],
                               separators=(',', ':')) for row in track['samples']]} for track in tracks]}


def experiments(source, directory, offset, metadata, source_rows, reference, packets, mapping, deadline):
    result = []
    variants = [('hls-default', [], False, True),
        ('hls-negative', ['-avoid_negative_ts', 'disabled'], False, True),
        ('hls-negative-edit-normal', ['-avoid_negative_ts', 'disabled', '-use_editlist', '1'], False, False),
        ('hls-auto-edit-normal', ['-use_editlist', '1'], False, False),
        ('mp4-delay-edit', ['-avoid_negative_ts', 'disabled', '-use_editlist', '1'], True, False)]
    for name, options, mp4, discontinuous in variants:
        attempt = {'name': name, 'boundary': 'Offline copy only; full decode retained', 'result': 'failed'}
        result.append(attempt)
        root = directory / ('offline-' + name)
        root.mkdir()
        command = ['ffmpeg', '-nostdin', '-v', 'error', '-y']
        command += ['-ss', str(offset), '-i', str(source), '-map', '0:v:0', '-map', '0:a:0',
                    '-sn', '-c:v', 'copy', '-c:a', 'copy']
        if '-avoid_negative_ts' in options:
            command += ['-avoid_negative_ts', 'disabled']
        output = root / ('output.mp4' if mp4 else 'index.m3u8')
        if mp4:
            command += ['-use_editlist', '1', '-movflags', '+frag_keyframe+delay_moov', '-f', 'mp4', str(output)]
        else:
            segment_options = 'movflags=+frag_discont+skip_sidx' if discontinuous else 'movflags=+skip_sidx'
            if '-use_editlist' in options:
                segment_options += ':use_editlist=1'
            command += ['-f', 'hls', '-hls_time', '2', '-hls_playlist_type', 'event',
                '-hls_segment_type', 'fmp4', '-hls_segment_options', segment_options,
                '-hls_flags', 'temp_file', '-hls_fmp4_init_filename', 'init.mp4',
                '-hls_segment_filename', str(root / 'segment-%05d.m4s'), str(output)]
        attempt['command'] = command
        try:
            remaining = deadline - time.monotonic()
            check(remaining > 0, 'nonkey_offline_deadline')
            completed = subprocess.run(command, capture_output=True, timeout=min(30, remaining))
            check(len(completed.stderr) <= 65536 and len(completed.stdout) <= 65536, 'nonkey_mux_log_bound')
            private_log = root / 'mux-stderr.log'
            private_log.write_bytes(completed.stderr)
            attempt.update(exitCode=completed.returncode, privateMuxLogSHA256=sha(private_log))
            check(completed.returncode == 0, 'nonkey_offline_mux_failed')
            if mp4:
                public, init = output, initial_box(output)
                first_fragment = initial_box(output, b'moof', 2 * 1024 * 1024)
            else:
                facts, segments = manifest_facts(bounded_bytes(output, 65536, 'nonkey_manifest_bound'))
                attempt['manifest'] = facts
                init = bounded_bytes(root / 'init.mp4', 1024 * 1024, 'nonkey_init_bound')
                first_fragment = bounded_bytes(root / segments[0][0], 2 * 1024 * 1024,
                                               'nonkey_first_fragment_bound')
                parts = [init] + [bounded_bytes(root / name, 2 * 1024 * 1024,
                    'nonkey_offline_fragment_bound') for name, _ in segments]
                check(sum(map(len, parts)) <= 16 * 1024 * 1024, 'nonkey_offline_media_bound')
                public = root / 'public.mp4'
                public.write_bytes(b''.join(parts))
            check(time.monotonic() < deadline, 'nonkey_offline_deadline')
            actual, rows = decode_frames(public, deadline=deadline)
            attempt.update(publicSHA256=sha(public), initialization=initialization_metadata(init),
                completePresentation=actual, packets=packets(public, deadline=deadline),
                firstFragmentSamples=fragment_evidence(first_fragment),
                outputZeroDecoder=presentation_experiment(public, reference, deadline),
                matchesStrictReference=actual['identity'] == reference['identity'],
                frameMapping=mapping(source_rows, rows, metadata['sourceFramePTS'],
                    metadata['sourceTimeOriginSeconds'] + offset), result='measured')
        except (RuntimeError, OSError, subprocess.SubprocessError) as error:
            attempt['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
        if time.monotonic() >= deadline:
            break
    return result
