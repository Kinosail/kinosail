"""Private actual codec invocation capture and closed production origin-refill recipe."""
from pathlib import Path
import fcntl
import json
import os
import re
import shutil
import stat
from hls_followon_public import check
from hls_timeline_http import sha


def capture_codec(directory, source, pacing, installation, env, case):
    check(pacing is None or type(pacing) in [int, float] and pacing in [0.75, 0.9, 1.25], 'refill_capture_pacing')
    real = shutil.which('ffmpeg')
    check(real is not None, 'ffmpeg_unavailable')
    wrapper = directory / 'paced-ffmpeg'
    invocation = directory / 'pacing-invocations.jsonl'
    wrapper.write_text('#!/usr/bin/env python3\nimport os,sys\nsys.dont_write_bytecode=True\n'
        + 'sys.path.insert(0,' + repr(str(Path(__file__).parent)) + ')\n'
        + 'from hls_remaining_capture import captured_arguments\n'
        + 'a=captured_arguments(sys.argv[1:],' + repr(str(source)) + ',' + repr(str(directory)) + ','
        + repr(pacing) + ',' + repr(str(invocation)) + ',' + repr(str(directory/'refill-recipe-private.json')) + ')\n'
        + ("if '-start_number' in a and a[a.index('-start_number')+1]=='4':\n from hls_remaining_installation import installed_refill\n"
            + " a=installed_refill(a," + repr(str(source)) + ',' + repr(str(directory))
            + ",os.environ.get('KINOSAIL_INSTALLATION_RECEIPT_DIR'))\n" if installation else '')
        + 'os.execv(' + repr(real) + ',[' + repr(real) + ']+a)\n')
    wrapper.chmod(0o700)
    env['KINOSAIL_FFMPEG'] = str(wrapper)
    case['actualCodecInvocation'] = {'wrapperSHA256': sha(wrapper), 'executableSHA256': sha(Path(real))}
    if pacing is not None:
        case['testOnlyRealCodecPacing'] = dict(case['actualCodecInvocation'], readrate=pacing)
    return real, invocation


def capture_log_bytes(fd, pacing):
    info = os.fstat(fd)
    check(stat.S_ISREG(info.st_mode) and info.st_uid == os.getuid() and info.st_nlink == 1
        and stat.S_IMODE(info.st_mode) == 0o600 and info.st_size <= 4096, 'refill_capture_log_file')
    os.lseek(fd, 0, os.SEEK_SET)
    raw = os.read(fd, 4097)
    check(len(raw) == info.st_size and (not raw or raw.endswith(b'\n')) and len(raw.splitlines()) <= 32,
        'refill_capture_log_bound')
    def unique(pairs):
        result = {}
        for key, value in pairs:
            check(key not in result, 'refill_capture_log_duplicate')
            result[key] = value
        return result
    for line in raw.splitlines():
        row = json.loads(line, object_pairs_hook=unique)
        check(type(row) is dict and set(row) == {'pid', 'parent', 'sourceMatched', 'readrate'}
            and all(type(row[k]) is int and 0 < row[k] < 1 << 31 for k in ['pid', 'parent'])
            and type(row['sourceMatched']) is bool and row['sourceMatched']
            and row['readrate'] == str(pacing), 'refill_capture_log_row')
    return raw


def captured_arguments(arguments, source, directory, pacing, invocation, capture):
    a = list(arguments)
    check(0 < len(a) <= 96 and all(type(v) is str and len(v.encode()) <= 4096 for v in a)
        and len(json.dumps(a).encode()) <= 8192, 'refill_capture_bound')
    if '-hls_time' not in a:
        return a
    check(a.count('-i') == 1 and a.index('-i')+1 < len(a) and a[a.index('-i')+1] == source,
        'refill_capture_source')
    check(a.count('-start_number') == 1 and a.index('-start_number')+1 < len(a)
        and re.fullmatch(r'0|[1-9][0-9]{0,5}', a[a.index('-start_number')+1]) is not None,
        'refill_capture_start')
    fresh = a[a.index('-start_number')+1] == '4'
    if fresh:
        check(a.count('-map') == 1 and a.index('-map')+1 < len(a) and a[a.index('-map')+1] == '0:a:0'
            and a.count('-hls_segment_filename') == 1 and a.index('-hls_segment_filename')+1 < len(a),
            'refill_capture_track')
        root = Path(a[-1]).parent.parent.parent
        check(root.parent == Path(directory)/'cache' and re.fullmatch(r'[a-f0-9]{16}-plan-a-[a-zA-Z0-9-]+', root.name)
            and a[-1] == str(root/'.seek-4/audio/index.m3u8')
            and a[a.index('-hls_segment_filename')+1] == str(root/'audio/segment-%05d.m4s'),
            'refill_capture_generation')
        check(not os.path.lexists(capture), 'refill_capture_existing')
    if pacing is not None:
        check(a.count('-readrate') <= 1 and ('-readrate' not in a or a.index('-readrate')+1 < len(a)), 'refill_capture_readrate')
        if '-readrate' in a:
            a[a.index('-readrate')+1] = str(pacing)
        else:
            a[a.index('-i'):a.index('-i')] = ['-readrate', str(pacing)]
    value = json.dumps(a).encode()
    check(len(a) <= 96 and len(value) <= 8192, 'refill_capture_bound')
    if pacing is None:
        if fresh:
            with os.fdopen(os.open(capture, os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW, 0o600), 'wb') as output:
                output.write(value)
        return a
    row = (json.dumps({'pid': os.getpid(), 'parent': os.getppid(), 'sourceMatched': True, 'readrate': str(pacing)})+'\n').encode()
    created = not os.path.lexists(invocation)
    fd = os.open(invocation, os.O_RDWR|os.O_NOFOLLOW|(os.O_CREAT|os.O_EXCL if created else 0), 0o600)
    try:
        fcntl.flock(fd, fcntl.LOCK_EX)
        raw = capture_log_bytes(fd, pacing)
        check(len(raw)+len(row) <= 4096 and len(raw.splitlines()) < 32, 'refill_capture_log_full')
        if fresh:
            with os.fdopen(os.open(capture, os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW, 0o600), 'wb') as output:
                output.write(value)
        os.lseek(fd, 0, os.SEEK_END)
        check(os.write(fd, row) == len(row), 'refill_capture_log_write')
    except Exception:
        if created and os.fstat(fd).st_size == 0 and os.path.samestat(os.fstat(fd), os.lstat(invocation)):
            os.unlink(invocation)
        raise
    finally:
        os.close(fd)
    return a


def origin_refill_template(source, root, case):
    selected = case.get('originSelectedAudio', {})
    check(selected == {'codec': 'flac', 'sampleRate': 48000, 'channels': 2, 'layout': 'stereo', 'index': 0,
        'sourceIndex': 0} and all(type(selected.get(k)) is int for k in ['sampleRate', 'channels', 'index', 'sourceIndex'])
        and type(case.get('planDurationSeconds')) in [int, float] and case['planDurationSeconds'] == 10,
        'refill_recipe_origin_eligibility')
    pacing = case.get('testOnlyRealCodecPacing')
    check(pacing is None or isinstance(pacing, dict) and type(pacing.get('readrate')) in [int, float] and
        pacing['readrate'] in [0.75, 0.9, 1.25], 'refill_recipe_pacing')
    expected = ['-hide_banner', '-loglevel', 'error', '-y', '-avoid_negative_ts', 'disabled', '-max_delay', '5000000',
        '-ss', '0.000', *(['-readrate', str(pacing['readrate'])] if pacing is not None else []), '-i', str(source),
        '-map', '0:a:0', '-vn', '-sn', '-dn', '-c:a', 'aac', '-ac', '2', '-b:a', '192000',
        '-output_ts_offset', str(8 - 382976 / 48000), '-bsf:a', 'noise=amount=0:drop=lt(pts\\,382976)',
        '-f', 'hls', '-hls_time', '2', '-hls_playlist_type', 'event', '-hls_segment_type', 'fmp4',
        '-hls_segment_options', 'movflags=+frag_discont+skip_sidx', '-hls_flags', 'temp_file',
        '-hls_fmp4_init_filename', 'init.mp4', '-start_number', '4', '-hls_segment_filename',
        str(root / 'audio/segment-%05d.m4s'), str(root / '.seek-4/audio/index.m3u8')]
    return expected
