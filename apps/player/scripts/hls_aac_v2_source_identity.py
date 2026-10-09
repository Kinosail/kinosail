"""Typed private captured-input identity; never authorizes a Server worker."""
import os
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
