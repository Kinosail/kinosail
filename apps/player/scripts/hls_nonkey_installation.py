"""Optional disposable installation counterfactual; never a production certificate."""
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import stat
import sys
import time


def rewrite_initial_arguments(arguments, source, cache):
    unchanged = (arguments, False)
    if (not isinstance(arguments, list) or not arguments or len(arguments) > 256
            or not all(isinstance(v, str) for v in arguments)
            or sum(len(v.encode()) for v in arguments) > 65536):
        return unchanged
    output, root = Path(arguments[-1]), Path(cache)
    try:
        relative = output.relative_to(root)
    except ValueError:
        return unchanged
    if (not output.is_absolute() or '..' in output.parts or len(relative.parts) != 3
            or output.name != 'index.m3u8' or not re.fullmatch('[1-9][0-9]{2,3}p', output.parent.name)):
        return unchanged
    expected = ['-hide_banner', '-loglevel', 'error', '-y', '-ss', '12.5', '-i', source,
        '-map', '0:v:0', '-map', '0:a:0?', '-sn', '-c:v', 'copy', '-c:a', 'copy',
        '-f', 'hls', '-hls_time', '2', '-hls_playlist_type', 'event',
        '-hls_segment_type', 'fmp4', '-hls_segment_options', 'movflags=+frag_discont+skip_sidx',
        '-hls_flags', 'temp_file', '-hls_fmp4_init_filename', 'init.mp4',
        '-hls_segment_filename', str(output.parent / 'segment-%05d.m4s'), str(output)]
    if arguments != expected:
        return unchanged
    effective = expected.copy()
    effective[effective.index('-hls_segment_options') + 1] = 'movflags=+skip_sidx:use_editlist=1'
    effective[effective.index('-f'):effective.index('-f')] = ['-avoid_negative_ts', 'disabled']
    return effective, True


def bounded_file(path, limit, retain=False):
    try:
        with os.fdopen(os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK), 'rb') as file:
            before = os.fstat(file.fileno())
            if not stat.S_ISREG(before.st_mode) or not 0 < before.st_size <= limit:
                raise RuntimeError('installation_file_bound')
            digest, total, chunks = hashlib.sha256(), 0, []
            while data := file.read(min(1024 * 1024, limit + 1 - total)):
                total += len(data)
                if total > limit:
                    raise RuntimeError('installation_file_bound')
                digest.update(data)
                if retain:
                    chunks.append(data)
            after = os.fstat(file.fileno())
        current = os.stat(path, follow_symlinks=False)
        identity = lambda v: (v.st_ino, v.st_dev, v.st_size, v.st_mtime_ns, v.st_mode)
        if identity(before) != identity(after) or identity(before) != identity(current):
            raise RuntimeError('installation_file_changed')
        return ({'inode': before.st_ino, 'size': total, 'mtimeNs': str(before.st_mtime_ns),
                 'sha256': digest.hexdigest()}, b''.join(chunks) if retain else None)
    except OSError as error:
        raise RuntimeError('installation_file_identity') from error


def bounded_state(path, limit):
    return bounded_file(path, limit)[0]


def append_private(path, value):
    data = (json.dumps(value, separators=(',', ':'), allow_nan=False) + '\n').encode()
    try:
        with os.fdopen(os.open(path, os.O_WRONLY | os.O_CREAT | os.O_APPEND | os.O_NOFOLLOW | os.O_NONBLOCK, 0o600), 'wb') as file:
            current = os.fstat(file.fileno())
            if not stat.S_ISREG(current.st_mode):
                raise RuntimeError('installation_audit_identity')
            deadline = time.monotonic() + 0.25
            while True:
                try:
                    fcntl.flock(file.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
                    break
                except BlockingIOError:
                    if time.monotonic() >= deadline:
                        raise RuntimeError('installation_audit_busy')
                    time.sleep(0.005)
            current = os.fstat(file.fileno())
            if (not stat.S_ISREG(current.st_mode) or stat.S_IMODE(current.st_mode) != 0o600
                    or len(data) > 128 * 1024 or current.st_size + len(data) > 256 * 1024):
                raise RuntimeError('installation_audit_bound')
            file.write(data)
            file.flush()
    except OSError as error:
        raise RuntimeError('installation_audit_identity') from error


def wrapper(arguments, config):
    if bounded_state(config['executable'], 256 * 1024 * 1024) != config['executableState']:
        raise RuntimeError('installation_executable_changed')
    effective, applied = rewrite_initial_arguments(arguments, config['source'], config['cache'])
    if applied and bounded_state(config['source'], 8 * 1024 * 1024) != config['sourceState']:
        raise RuntimeError('installation_source_changed')
    if len(arguments) > 256 or sum(len(v.encode()) for v in arguments) > 65536:
        raise RuntimeError('installation_command_bound')
    append_private(config['audit'], {'applied': applied, 'original': arguments, 'effective': effective})
    # All snapshot and private receipt descriptors are closed before real exec.
    os.execv(config['executable'], [config['executable'], *effective])


def install_counterfactual(directory, source, metadata, offset, case):
    videos = [v for v in metadata['streamOrigins']['streams'] if v.get('codec_type') == 'video']
    if offset != 12.5 or len(videos) != 1 or videos[0].get('codec_name') != 'h264':
        raise RuntimeError('installation_fixture_scope')
    found = shutil.which('ffmpeg')
    if not found:
        raise RuntimeError('installation_executable_missing')
    executable = str(Path(found).resolve())
    config = {'executable': executable, 'executableState': bounded_state(executable, 256 * 1024 * 1024),
        'source': str(source), 'sourceState': bounded_state(source, 8 * 1024 * 1024),
        'cache': str(directory / 'cache'), 'audit': str(directory / 'installation-private.jsonl')}
    path = directory / 'counterfactual-ffmpeg'
    code = ('#!' + sys.executable + '\nimport sys\nsys.path.insert(0, ' + repr(str(Path(__file__).parent))
        + ')\nfrom hls_nonkey_installation import wrapper\nwrapper(sys.argv[1:], ' + repr(config) + ')\n')
    with os.fdopen(os.open(path, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o700), 'w') as file:
        file.write(code)
    case['counterfactualInstallation'] = {'boundary': 'Fresh disposable installation; mux counterfactual only; no production acceptance',
        'mode': 'negative-edit', 'wrapperSHA256': bounded_state(path, 16384)['sha256'],
        'executableSHA256': config['executableState']['sha256'], 'sourceSnapshot': config['sourceState']}
    return str(path), config


def finish_counterfactual(config, case):
    audit = Path(config['audit'])
    state, raw = bounded_file(audit, 256 * 1024, retain=True)
    rows = [json.loads(v) for v in raw.decode().splitlines()]
    if (not 0 < len(rows) <= 32 or not all(set(v) == {'applied', 'original', 'effective'}
            and type(v['applied']) is bool for v in rows)):
        raise RuntimeError('installation_audit_shape')
    changes = sum(v['applied'] for v in rows)
    if changes != 1 or any(rewrite_initial_arguments(v['original'], config['source'], config['cache'])
                          != (v['effective'], v['applied']) for v in rows):
        raise RuntimeError('installation_transform_count')
    if bounded_state(config['source'], 8 * 1024 * 1024) != config['sourceState']:
        raise RuntimeError('installation_source_changed')
    case['counterfactualInstallation'].update(privateInvocationSHA256=state['sha256'],
        invocations=len(rows), transformations=changes, sourceUnchanged=True,
        allUnrelatedInvocationsUnchanged=True)
