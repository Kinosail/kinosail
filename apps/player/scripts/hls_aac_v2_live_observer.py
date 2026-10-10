"""Read-only actual owned HLS activity; unknown live input never certifies zero."""
from pathlib import Path
import subprocess
from hls_followon_public import bounded_bytes, check
from hls_aac_v2_source_identity import captured_input_identity, matches_source_input
from hls_aac_v2_source_identity import option, regular_identity, startup_fixture, validate_identity

FAILURE = 'v2_live_input_identity_unqualified'

OBSERVATION_STAGES = (
    'parent', 'source_before', 'children', 'child', 'identity_before', 'arguments_before',
    'input_before', 'arguments_after', 'input_after', 'identity_after', 'source_after',
    'disappearance_check', 'terminal_check', 'other',
)
OBSERVATION_EXCEPTIONS = (
    'RuntimeError', 'FileNotFoundError', 'OSError', 'SubprocessError', 'TimeoutExpired',
    'CalledProcessError', 'ValueError', 'IndexError', 'UnicodeError', 'UnicodeDecodeError',
    'PermissionError', 'other',
)

PROCESS_STATES = ('R', 'S', 'D', 'Z', 'T', 't', 'X', 'x', 'K', 'W', 'I', 'P', 'absent', 'other')
ARGUMENT_SHAPES = ('empty', 'byte_bound', 'nonterminated', 'argument_decode',
    'argument_count', 'argument_length', 'other')
DETAIL_ATTRIBUTES = (
    ('observer_before_state', 'beforeState'), ('observer_after_state', 'afterState'),
    ('observer_argument_shape', 'argumentShape'), ('observer_argument_bytes', 'argumentBytes'),
    ('observer_argument_count', 'argumentCount'),
    ('observer_maximum_argument_bytes', 'maximumArgumentBytes'),
)


def observation_fields(error, stage='other'):
    stage = getattr(error, 'observer_stage', stage)
    exception_class = getattr(error, 'observer_exception_class', type(error).__name__)
    return {
        'stage': stage if type(stage) is str and stage in OBSERVATION_STAGES else 'other',
        'exceptionClass': exception_class if type(exception_class) is str
            and exception_class in OBSERVATION_EXCEPTIONS else 'other',
    }


def observation_details(error):
    fields = observation_fields(error)
    for attribute, key in DETAIL_ATTRIBUTES:
        value = getattr(error, attribute, None)
        if key in ['beforeState', 'afterState', 'argumentShape']:
            allowed = ARGUMENT_SHAPES if key == 'argumentShape' else PROCESS_STATES
            fields[key] = value if type(value) is str and value in allowed else 'other'
        else:
            fields[key] = value if type(value) is int and 0 <= value <= 65537 else None
    return fields


def copy_observation_details(target, error):
    fields = observation_details(error)
    for attribute, key in DETAIL_ATTRIBUTES:
        setattr(target, attribute, fields[key])


def qualified_failure(stage, error):
    fields = observation_fields(error, stage)
    failure = RuntimeError(FAILURE)
    failure.observer_stage = fields['stage']
    failure.observer_exception_class = fields['exceptionClass']
    copy_observation_details(failure, error)
    return failure


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
    with (Path('/proc') / str(pid) / 'cmdline').open('rb') as stream:
        data = stream.read(65537)
    shape = 'empty' if not data else 'byte_bound'
    args, maximum = None, None
    try:
        check(0 < len(data) <= 65536, FAILURE)
        shape = 'nonterminated'
        check(data.endswith(bytes([0])), FAILURE)
        shape = 'argument_decode'
        args = [value.decode() for value in data[:-1].split(bytes([0]))]
        maximum = max(len(value.encode()) for value in args)
        shape = 'argument_count'
        check(0 < len(args) <= 128, FAILURE)
        shape = 'argument_length'
        check(all(len(value) <= 2048 for value in args), FAILURE)
        return args
    except (RuntimeError, UnicodeError) as error:
        error.observer_argument_bytes = len(data)
        error.observer_argument_shape = shape
        error.observer_argument_count = len(args) if args is not None else None
        error.observer_maximum_argument_bytes = maximum
        raise


def input_classification(args, parent, source, expected):
    witness = captured_input_identity(args)
    value = option(args, '-i')
    row = {'parent': parent, 'inputWitness': witness}
    if matches_source_input(value, row, source):
        check(witness == expected, FAILURE)
        return 'source', value, witness
    check(startup_fixture(value, args), FAILURE)
    return 'startup', value, witness


def child_absent(pid, parent):
    check(pid not in owned_children(parent), FAILURE)
    try:
        process_identity(pid, parent)
    except FileNotFoundError:
        check(pid not in owned_children(parent), FAILURE)
        return True
    return False


def child_ended(pid, parent, before, observation):
    try:
        after = process_identity(pid, parent)
        observation.observer_after_state = after[0]
        check(after[1] == before[1], FAILURE)
        if after[0] not in ['Z', 'X', 'x']:
            return False
        final = process_identity(pid, parent)
        observation.observer_after_state = final[0]
        check(final[1] == before[1], FAILURE)
        return final[0] in ['Z', 'X', 'x']
    except FileNotFoundError:
        observation.observer_after_state = 'absent'
        return child_absent(pid, parent)


def observe_child(pid, parent, source, expected):
    stage = 'identity_before'
    before = None
    try:
        before = process_identity(pid, parent)
        if before[0] in ['Z', 'X', 'x']:
            return 0
        stage = 'arguments_before'
        args = actual_arguments(pid)
        if '-hls_time' not in args:
            return 0
        stage = 'input_before'
        first = input_classification(args, parent, source, expected)
        stage = 'arguments_after'
        after_args = actual_arguments(pid)
        stage = 'input_after'
        check('-hls_time' in after_args
            and input_classification(after_args, parent, source, expected) == first, FAILURE)
        stage = 'identity_after'
        after = process_identity(pid, parent)
        check(after[1] == before[1], FAILURE)
        if after[0] in ['Z', 'X', 'x']:
            return 0
        stage = 'source_after'
        check(regular_identity(source) == expected, FAILURE)
        return 1
    except FileNotFoundError:
        try:
            check(pid not in owned_children(parent), FAILURE)
            return 0
        except (OSError, subprocess.SubprocessError, RuntimeError, ValueError, IndexError, UnicodeError) as error:
            raise qualified_failure('disappearance_check', error) from None
    except (OSError, subprocess.SubprocessError, RuntimeError, ValueError, IndexError, UnicodeError) as error:
        if before is not None:
            error.observer_before_state = before[0]
        if stage == 'arguments_before' and isinstance(error, RuntimeError):
            try:
                if child_ended(pid, parent, before, error):
                    return 0
            except (OSError, subprocess.SubprocessError, RuntimeError, ValueError, IndexError, UnicodeError) as terminal_error:
                copy_observation_details(terminal_error, error)
                raise qualified_failure('terminal_check', terminal_error) from None
        raise qualified_failure(stage, error) from None


def owned_hls_count(server, source):
    stage = 'parent'
    try:
        parent = server.pid
        check(type(parent) is int and 0 < parent < (1 << 31), FAILURE)
        stage = 'source_before'
        expected = validate_identity(getattr(server, '_copiedSourceWitness', regular_identity(source)))
        check(regular_identity(source) == expected, FAILURE)
        stage = 'children'
        children = owned_children(parent)
        stage = 'child'
        count = sum(observe_child(pid, parent, source, expected) for pid in children)
        stage = 'source_after'
        check(regular_identity(source) == expected, FAILURE)
        return count
    except (OSError, subprocess.SubprocessError, RuntimeError, ValueError, IndexError, UnicodeError) as error:
        raise qualified_failure(stage, error) from None
