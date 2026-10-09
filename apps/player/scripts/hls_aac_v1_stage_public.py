"""Private CLI feasibility; no Server worker or canonical publication."""
import hashlib
import math
import os
import signal
import stat
import subprocess
import time
from hls_aac_v2_public_http import option
from hls_followon_frames import decode_frames
from hls_followon_public import bounded_bytes, check
from hls_remaining_nonkey_evidence import packet_rows
from hls_remaining_nonkey_init import initialization_metadata
from hls_remaining_process import group_members
from hls_timeline_http import source_state


def identity(info):
    return (info.st_dev, info.st_ino, info.st_size, info.st_mtime_ns)


def template(rows, number, source):
    selected = [row for row in rows if int(option(row['args'], '-start_number') or '0') == number]
    check(len(selected) == 1, 'v1_stage_unique_baseline_template')
    args = list(selected[0]['args'])
    check(option(args, '-i') == str(source) and option(args, '-c:a') == 'copy'
        and option(args, '-c:v') == 'copy' and option(args, '-copypriorss') == '0'
        and option(args, '-hls_time') == '0.1' and '-copypriorss:v' not in args
        and '-copypriorss:a' not in args, 'v1_stage_exact_old_profile')
    return args


def replace(args, name, value, optional=False):
    positions = [n for n, entry in enumerate(args[:-1]) if entry == name]
    check(len(positions) <= 1 and (positions or optional), 'v1_stage_template_option')
    if positions:
        args[positions[0] + 1] = value
    else:
        args[-1:-1] = [name, value]


def arguments(rows, source, retained, directory, timeline, number):
    args = template(rows, 0 if number == 0 else 4, source)
    keys, grid = timeline['Keys'], timeline['Numerator'] / timeline['Denominator']
    start = keys[number]['PTS'] * grid
    next_key = min(number + 2, len(keys))
    end = keys[next_key]['PTS'] * grid if next_key < len(keys) else timeline['End']
    check(timeline['Clock'] == 0 and math.isfinite(end) and 0 < end - start <= 4.01,
        'v1_stage_two_gop_or_eof_bound')
    replace(args, '-i', '/proc/' + str(os.getpid()) + '/fd/' + str(retained.fileno()))
    replace(args, '-ss', format(math.floor(start * 1000000) / 1000000, '.6f'))
    replace(args, '-start_number', str(number), optional=True)
    replace(args, '-t', format(end - start, '.6f'), optional=True)
    if number > 0:
        replace(args, '-output_ts_offset', format(start - keys[0]['PTS'] * grid, '.6f'))
    replace(args, '-hls_segment_filename', str(directory / 'segment-%05d.m4s'))
    args[-1] = str(directory / 'index.m3u8')
    check(option(args, '-hls_fmp4_init_filename') in [None, 'init.mp4'],
        'v1_stage_relative_init_output')
    return args


def inventory(directory, number):
    paths = list(directory.iterdir())
    check(len(paths) <= 8, 'v1_stage_file_count')
    allowed = {'index.m3u8': 256 << 10, 'init.mp4': 2 << 20}
    allowed.update({'segment-' + format(n, '05d') + '.m4s': 64 << 20
        for n in range(number, number + 3)})
    total = 0
    for path in paths:
        name = path.name.removesuffix('.tmp')
        try:
            info = path.lstat()
        except FileNotFoundError:
            continue  # Temporary rename; final joined inventory is checked again.
        check(name in allowed and stat.S_ISREG(info.st_mode) and info.st_size <= allowed[name],
            'v1_stage_unexpected_file')
        total += info.st_size
    check(total <= 128 << 20, 'v1_stage_total_bound')
    return {'files': len(paths), 'bytes': total}


def settle(process, row):
    failures = []
    def observe():
        try:
            return group_members(process.pid)
        except (OSError, subprocess.SubprocessError):
            failures.append('v1_stage_group_observation_failed')
            return None
    try:
        if process.poll() is None or observe() != []:
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
    finally:
        process.wait(timeout=5)
    zeros, deadline = 0, time.monotonic() + 3
    while zeros < 2 and time.monotonic() < deadline:
        zeros = zeros + 1 if observe() == [] else 0
        time.sleep(0.05)
    row.update(joinedGroupZeroSamples=zeros, groupQualificationFailures=failures)
    check(zeros == 2 and not failures, 'v1_stage_owned_group_not_joined')


def execute(binary, args, directory, number, guard, row):
    end = time.monotonic() + min(30, guard.check(20))
    with (directory.parent / (directory.name + '-private.log')).open('wb') as error:
        process = subprocess.Popen([binary, *args], stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL, stderr=error, start_new_session=True)
        row['commandStarted'] = True
        try:
            while process.poll() is None:
                inventory(directory, number)
                check(time.monotonic() < end, 'v1_stage_command_deadline')
                time.sleep(0.01)
            check(process.returncode == 0, 'v1_stage_command_failed')
        finally:
            settle(process, row)
    row['joinedStageInventory'] = inventory(directory, number)

def projected(rows):
    fields = ['stream_index', 'pts', 'dts', 'duration', 'size', 'flags', 'data_hash', 'side_data_list']
    return [{name: row[name] for name in fields if name in row} for row in rows]


def media_evidence(source_frames, media, directory, timeline, number, row):
    init = bounded_bytes(media / 'init.mp4', 2 << 20, 'v1_stage_canonical_init_bound')
    generated = bounded_bytes(directory / 'init.mp4', 2 << 20, 'v1_stage_generated_init_bound')
    row.update(canonicalInitSHA256=hashlib.sha256(init).hexdigest(),
        generatedInitSHA256=hashlib.sha256(generated).hexdigest(),
        canonicalInitMetadata=initialization_metadata(init),
        generatedInitMetadata=initialization_metadata(generated),
        initByteDifferences=sum(a != b for a, b in zip(init, generated)) + abs(len(init) - len(generated)),
        exactInitSHA256=generated == init)
    name = 'segment-' + format(number, '05d') + '.m4s'
    original = bounded_bytes(media / name, 64 << 20, 'v1_stage_canonical_fragment_bound')
    fragment = bounded_bytes(directory / name, 64 << 20, 'v1_stage_requested_fragment_bound')
    baseline = directory.parent / (directory.name + '-baseline.mp4')
    candidate = directory.parent / (directory.name + '-canonical-init.mp4')
    baseline.write_bytes(init + original)
    candidate.write_bytes(init + fragment)
    expected, missing_expected = packet_rows(baseline)
    actual, missing_actual = packet_rows(candidate)
    _, frames = decode_frames(candidate)
    keys, grid = timeline['Keys'], timeline['Numerator'] / timeline['Denominator']
    start = keys[number]['PTS'] * grid
    end = keys[number + 1]['PTS'] * grid if number + 1 < len(keys) else timeline['End']
    reference = [digest for point, digest in source_frames if start - 0.000001 <= point < end - 0.000001]
    check(len(reference) == 48, 'v1_stage_independent_source_window')
    row.update(canonicalFragmentSHA256=hashlib.sha256(original).hexdigest(),
        generatedFragmentSHA256=hashlib.sha256(fragment).hexdigest(), exactFragmentSHA256=fragment == original,
        canonicalPacketRows=expected, generatedPacketRows=actual,
        canonicalMissingClockFields=missing_expected, generatedMissingClockFields=missing_actual,
        exactBaselinePackets=not missing_expected and not missing_actual and projected(expected) == projected(actual),
        decodedFrames=len(frames), referenceFrames=len(reference),
        exactSourceFrames=[digest for _, digest in frames] == reference,
        historicalAACCorrectnessAccepted=False)
    row['result'] = 'observed' if row['exactInitSHA256'] and row['exactBaselinePackets'] and row['exactSourceFrames'] else 'failed'


def prove(binary, source, media, rows, timeline, run, guard, receipt):
    _, source_frames = decode_frames(source)
    check(len(source_frames) == 768, 'v1_stage_independent_complete_source')
    before = source_state(source)
    with source.open('rb') as retained:
        witness = os.fstat(retained.fileno())
        check(stat.S_ISREG(witness.st_mode) and identity(witness) == identity(source.stat()),
            'v1_stage_retained_source_identity')
        for number in [0, 4, 9]:
            row = {'cut': number, 'result': 'failed', 'serverWorkerProof': False}
            receipt['cases'].append(row)
            directory = run / ('stage-' + str(number))
            directory.mkdir()
            try:
                args = arguments(rows, source, retained, directory, timeline, number)
                row['oldProfileTemplateCut'] = 0 if number == 0 else 4
                row['executedArgumentsSHA256'] = hashlib.sha256(repr(args).encode()).hexdigest()
                row['sourceFDInputWitnessed'] = identity(os.stat(option(args, '-i'))) == identity(witness)
                check(row['sourceFDInputWitnessed'], 'v1_stage_input_fd_mismatch')
                execute(binary, args, directory, number, guard, row)
                media_evidence(source_frames, media, directory, timeline, number, row)
            except Exception as error:
                row['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
                check(not row.get('commandStarted') or row.get('joinedGroupZeroSamples') == 2,
                    'v1_stage_unknown_owned_command')
            check(identity(os.fstat(retained.fileno())) == identity(witness)
                and identity(source.stat()) == identity(witness) and source_state(source) == before,
                'v1_stage_source_changed')
