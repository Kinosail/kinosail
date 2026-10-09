"""Typed private captured-input identity; never authorizes a Server worker."""
import os
from pathlib import Path
import re
import stat
from hls_followon_public import check


def validate_identity(value):
    check(type(value) is dict and set(value) == {'device', 'inode', 'size', 'mtimeNs', 'regular'},
        'v2_source_witness_fields')
    check(value['regular'] is True, 'v2_source_witness_regular')
    for name in ['device', 'inode', 'size', 'mtimeNs']:
        check(type(value[name]) is int and 0 <= value[name] < (1 << 63),
            'v2_source_witness_integer')
    check(value['inode'] > 0 and value['size'] > 0, 'v2_source_witness_positive')
    return value


def regular_identity(path):
    try:
        info = os.stat(path)
    except OSError:
        raise RuntimeError('v2_source_witness_observation_failed') from None
    return validate_identity({'device': info.st_dev, 'inode': info.st_ino,
        'size': info.st_size, 'mtimeNs': info.st_mtime_ns, 'regular': stat.S_ISREG(info.st_mode)})


def matches_source_input(value, row, source):
    literal = value == str(source)
    alias = re.fullmatch(r'/proc/([1-9][0-9]{0,9})/fd/([1-9][0-9]{0,4})', value)
    if not literal and alias is None:
        return False
    if not literal:
        check(alias[1] == str(row['parent']) and 3 <= int(alias[2]) <= 65535,
            'v2_source_fd_owner_or_bound')
    check('inputWitnessFailureClass' not in row, 'v2_source_witness_capture_failed')
    if not literal or 'inputWitness' in row:
        check(validate_identity(row.get('inputWitness')) == regular_identity(source),
            'v2_source_witness_identity')
    return True


def captured_input_identity(args):
    check(type(args) is list and 0 < len(args) <= 128
        and all(type(value) is str and len(value) <= 2048 for value in args),
        'v2_actual_argv_shape')
    check(args.count('-i') == 1 and args[-1] != '-i', 'v2_argv_single_input_required')
    return regular_identity(args[args.index('-i') + 1])


def option(args, name):
    matches = [args[n + 1] for n, value in enumerate(args[:-1]) if value == name]
    check(len(matches) <= 1, 'v2_argv_duplicate_option')
    return matches[0] if matches else None


def startup_fixture(value, args):
    path = Path(value)
    return (path.is_absolute() and path.parent.parent == Path('/tmp')
        and re.fullmatch(r'kinosail-transcoder-check-[a-zA-Z0-9_-]{1,64}', path.parent.name)
        and path.name in ['source.mp4', 'source.mkv']
        and args[-1] == str(path.parent / 'index.m3u8')
        and option(args, '-hls_time') == '1' and option(args, '-frames:v') == '24'
        and option(args, '-c:a') == 'aac' and '-shortest' in args)
