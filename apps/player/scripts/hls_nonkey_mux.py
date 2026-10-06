"""Fixed copied-packet mux experiments; offline results never admit Server repair."""
import struct
import subprocess
import time
from hls_followon_frames import decode_frames
from hls_followon_public import check, bounded_bytes
from hls_nonkey_initialization import initialization_metadata
from hls_timeline_packets import manifest_facts
from hls_timeline_http import sha


def movie_prefix(path):
    # Read only through the bounded initialization, before any media fragment.
    with path.open('rb') as file:
        data = file.read(1024 * 1024 + 1)
    position = 0
    for _ in range(64):
        check(len(data) - position >= 8, 'nonkey_movie_header')
        size, kind = struct.unpack_from('>I4s', data, position)
        header = 8
        if size == 1:
            check(len(data) - position >= 16, 'nonkey_movie_extended_header')
            size, header = struct.unpack_from('>Q', data, position + 8)[0], 16
        check(header <= size <= 1024 * 1024 - position and position + size <= len(data),
              'nonkey_movie_extent')
        position += size
        if kind == b'moov':
            return data[:position]
    raise RuntimeError('nonkey_movie_box_bound')


def experiments(source, directory, offset, metadata, source_rows, reference, packets, mapping, deadline):
    result = []
    variants = [('hls-default', [], False),
        ('hls-negative', ['-avoid_negative_ts', 'disabled'], False),
        ('hls-negative-edit', ['-avoid_negative_ts', 'disabled', '-use_editlist', '1'], False),
        ('mp4-delay-edit', ['-avoid_negative_ts', 'disabled', '-use_editlist', '1'], True),
        ('hls-copyts-edit', ['-copyts', '-avoid_negative_ts', 'disabled', '-use_editlist', '1'], False)]
    for name, options, mp4 in variants:
        attempt = {'name': name, 'boundary': 'Offline copy only; full decode retained', 'result': 'failed'}
        result.append(attempt)
        root = directory / ('offline-' + name)
        root.mkdir()
        command = ['ffmpeg', '-nostdin', '-v', 'error', '-y']
        if '-copyts' in options:
            command += ['-copyts']
        command += ['-ss', str(offset), '-i', str(source), '-map', '0:v:0', '-map', '0:a:0',
                    '-sn', '-c:v', 'copy', '-c:a', 'copy']
        if options:
            command += ['-avoid_negative_ts', 'disabled']
        if '-copyts' in options:
            command += ['-output_ts_offset', str(-metadata['sourceTimeOriginSeconds'] - offset)]
        output = root / ('output.mp4' if mp4 else 'index.m3u8')
        if mp4:
            command += ['-use_editlist', '1', '-movflags', '+frag_keyframe+delay_moov', '-f', 'mp4', str(output)]
        else:
            segment_options = 'movflags=+frag_discont+skip_sidx'
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
            check(completed.returncode == 0, 'nonkey_offline_mux_failed')
            if mp4:
                public, init = output, movie_prefix(output)
            else:
                facts, segments = manifest_facts(bounded_bytes(output, 65536, 'nonkey_manifest_bound'))
                attempt['manifest'] = facts
                init = bounded_bytes(root / 'init.mp4', 1024 * 1024, 'nonkey_init_bound')
                parts = [init] + [bounded_bytes(root / name, 2 * 1024 * 1024,
                    'nonkey_offline_fragment_bound') for name, _ in segments]
                check(sum(map(len, parts)) <= 16 * 1024 * 1024, 'nonkey_offline_media_bound')
                public = root / 'public.mp4'
                public.write_bytes(b''.join(parts))
            check(time.monotonic() < deadline, 'nonkey_offline_deadline')
            actual, rows = decode_frames(public, deadline=deadline)
            attempt.update(publicSHA256=sha(public), initialization=initialization_metadata(init),
                completePresentation=actual, packets=packets(public, deadline=deadline),
                matchesStrictReference=actual['identity'] == reference['identity'],
                frameMapping=mapping(source_rows, rows, metadata['sourceFramePTS'],
                    metadata['sourceTimeOriginSeconds'] + offset), result='measured')
        except (RuntimeError, OSError, subprocess.SubprocessError) as error:
            attempt['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
        if time.monotonic() >= deadline:
            break
    return result
