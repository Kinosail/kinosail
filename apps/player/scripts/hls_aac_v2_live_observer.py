"""Read-only actual owned HLS activity; unknown live input never certifies zero."""
from pathlib import Path
import subprocess
from hls_followon_public import bounded_bytes, check
from hls_aac_v2_source_identity import captured_input_identity, matches_source_input
from hls_aac_v2_source_identity import option, regular_identity, startup_fixture, validate_identity

FAILURE = 'v2_live_input_identity_unqualified'


def owned_children(parent):
    data = subprocess.check_output(['ps', '-eo', 'pid=,ppid='], timeout=3)
    check(0 < len(data) <= 1 << 20, FAILURE)
    rows = data.decode('ascii').splitlines()
    check(0 < len(rows) <= 4096, FAILURE)
    children = []
    for row in rows:
        fields = row.split()
        check(len(fields) == 2 and all(v.isdecimal() and len(v) <= 10 for v in fields), FAILURE)
        pid, ppid = map(int, fields)
        check(pid > 0 and ppid >= 0, FAILURE)
        if ppid == parent:
            children.append(pid)
    check(len(children) <= 64 and len(set(children)) == len(children), FAILURE)
    return children


def process_identity(pid, parent):
    data = bounded_bytes(Path('/proc') / str(pid) / 'stat', 8192, FAILURE).decode()
    fields = data.rsplit(') ', 1)[1].split()
    check(len(fields) >= 20 and fields[1].isdecimal() and fields[19].isdecimal(), FAILURE)
    check(int(fields[1]) == parent and int(fields[19]) > 0, FAILURE)
    return fields[0], int(fields[19])


def actual_arguments(pid):
    data = bounded_bytes(Path('/proc') / str(pid) / 'cmdline', 65536, FAILURE)
    check(data.endswith(bytes([0])), FAILURE)
    args = [v.decode() for v in data[:-1].split(bytes([0]))]
    check(0 < len(args) <= 128 and all(len(v) <= 2048 for v in args), FAILURE)
    return args


def input_classification(args, parent, source, expected):
    witness = captured_input_identity(args)
    value = option(args, '-i')
    row = {'parent': parent, 'inputWitness': witness}
    if matches_source_input(value, row, source):
        check(witness == expected, FAILURE)
        return 'source', value, witness
    check(startup_fixture(value, args), FAILURE)
    return 'startup', value, witness


def observe_child(pid, parent, source, expected):
    try:
        before = process_identity(pid, parent)
        if before[0] in ['Z', 'X', 'x']:
            return 0
        args = actual_arguments(pid)
        if '-hls_time' not in args:
            return 0
        first = input_classification(args, parent, source, expected)
        after_args = actual_arguments(pid)
        check('-hls_time' in after_args
            and input_classification(after_args, parent, source, expected) == first, FAILURE)
        after = process_identity(pid, parent)
        check(after[1] == before[1], FAILURE)
        if after[0] in ['Z', 'X', 'x']:
            return 0
        check(regular_identity(source) == expected, FAILURE)
        return 1
    except FileNotFoundError:
        check(pid not in owned_children(parent), FAILURE)
        return 0


def owned_hls_count(server, source):
    try:
        parent = server.pid
        check(type(parent) is int and 0 < parent < (1 << 31), FAILURE)
        expected = validate_identity(getattr(server, '_copiedSourceWitness', regular_identity(source)))
        check(regular_identity(source) == expected, FAILURE)
        count = sum(observe_child(pid, parent, source, expected) for pid in owned_children(parent))
        check(regular_identity(source) == expected, FAILURE)
        return count
    except (OSError, subprocess.SubprocessError, RuntimeError, ValueError, IndexError, UnicodeError):
        raise RuntimeError(FAILURE) from None
