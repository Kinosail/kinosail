"""Bounded source-prefix clock experiment; full controls never choose its edit."""
from fractions import Fraction
import hashlib
import json
import math
import subprocess
import time
from hls_remaining_nonkey_aac_clock import filter_clock, frame_md5
from hls_remaining_nonkey_evidence import native_pcm
from hls_remaining_nonkey_boundary import native_clock_rows


def require(value, name):
    if not value:
        raise RuntimeError(name)


def prefix_command(arguments, deadline, result):
    remaining = deadline - time.monotonic()
    require(remaining > 0, 'aac_prefix_shared_deadline_exhausted')
    try:
        process = subprocess.run(arguments, capture_output=True, timeout=remaining)
    except subprocess.TimeoutExpired as error:
        result.update(result='failed', failureClass='aac_prefix_shared_deadline_exhausted',
            failedStage=result['currentStage'], elapsedSeconds=time.monotonic() - result['startedMonotonic'],
            partialStdoutBytes=len(error.stdout or b''), partialStderrBytes=len(error.stderr or b''),
            partialStdoutSHA256=hashlib.sha256(error.stdout or b'').hexdigest(),
            partialStderrSHA256=hashlib.sha256(error.stderr or b'').hexdigest())
        raise RuntimeError('aac_prefix_shared_deadline_exhausted') from error
    result['elapsedSeconds'] = time.monotonic() - result['startedMonotonic']
    result.setdefault('processFacts', []).append({'stage': result['currentStage'],
        'exitCode': process.returncode, 'stdoutBytes': len(process.stdout), 'stderrBytes': len(process.stderr),
        'stdoutSHA256': hashlib.sha256(process.stdout).hexdigest(),
        'stderrSHA256': hashlib.sha256(process.stderr).hexdigest()})
    require(process.returncode == 0 and len(process.stdout) <= 2 << 20 and
            len(process.stderr) <= 2 << 20, 'aac_prefix_process_or_output_bound')
    require(time.monotonic() <= deadline, 'aac_prefix_shared_deadline_exhausted')
    return process


def measure_prefix(source, result):
    started = time.monotonic()
    deadline = started + 2
    result.update(result='in-flight', startedMonotonic=started, sharedDeadlineSeconds=2,
        currentStage='probe', completedStages=[], productionAcceptance=False)
    probe = prefix_command(['ffprobe', '-v', 'error', '-threads', '1',
        '-select_streams', 'a:0', '-read_intervals', '%+22',
        '-show_packets', '-show_frames', '-show_streams', '-show_data_hash', 'sha256',
        '-show_entries', 'packet=pts,dts,duration,data_hash,side_data_list:'
        'frame=pts,nb_samples,side_data_list:'
        'stream=codec_name,profile,sample_rate,channels,time_base',
        '-of', 'json', str(source)], deadline, result)
    facts = json.loads(probe.stdout)
    mixed = facts.get('packets_and_frames', [])
    frames = [row for row in mixed if row.get('type') == 'frame']
    packets = [row for row in mixed if row.get('type') == 'packet']
    streams = facts.get('streams', [])
    require(len(streams) == 1 and streams[0]['codec_name'] == 'aac' and
            streams[0]['sample_rate'] == '48000' and streams[0]['channels'] == 2,
            'aac_prefix_source_format')
    require(0 < len(frames) <= 1060 and 0 < len(packets) <= 1061 and
            all(type(r.get('pts')) is int and 0 < r.get('nb_samples', 0) <= 1024 for r in frames),
            'aac_prefix_native_frame_bound')
    result.update(sourceStream=streams[0], nativeFrames=frames, sourcePackets=packets,
        probeSHA256=hashlib.sha256(probe.stdout).hexdigest())
    result['completedStages'].append('probe')
    result['currentStage'] = 'normalized-prefix'
    process = prefix_command(['ffmpeg', '-nostdin', '-v', 'info', '-xerror',
        '-threads', '1', '-filter_threads', '1', '-i', str(source),
        '-map', '0:a:0', '-vn', '-sn', '-dn', '-frames:a', '1024',
        '-af', 'ashowinfo', '-c:a', 'pcm_s16le', '-threads:a', '1',
        '-f', 'framemd5', 'pipe:1'], deadline, result)
    normalized = filter_clock(process.stderr.decode())
    result.update(normalizedFilter=normalized, normalizedOutputRows=[],
        outputFilterClockEquivalent=False)
    require(0 < len(normalized['completeRows']) <= 1025, 'aac_prefix_normalized_frame_bound')
    output_rows = normalized_output_rows(process.stdout.decode())
    result['normalizedOutputRows'] = output_rows
    filtered = normalized['completeRows']
    require(len(output_rows) == 1024 and len(filtered) >= len(output_rows) and
            output_rows == [(r['pts'], r['samples']) for r in filtered[:len(output_rows)]],
            'aac_prefix_output_filter_clock_equivalence')
    elapsed = time.monotonic() - started
    require(elapsed <= 2, 'aac_prefix_shared_deadline_exhausted')
    result.update({'result': 'observed', 'sharedDeadlineSeconds': 2, 'elapsedSeconds': elapsed,
        'sequentialProcesses': 2, 'configuredThreads': 1, 'outputFrameCap': 1024,
        'maximumNativePrefixFrames': 1060, 'inputPrefixSeconds': 22,
        'sourceStream': streams[0], 'nativeFrames': frames, 'sourcePackets': packets,
        'normalizedFilter': normalized, 'outputFilterClockEquivalent': True,
        'probeSHA256': hashlib.sha256(probe.stdout).hexdigest(),
        'filterLogSHA256': hashlib.sha256(process.stderr).hexdigest(),
        'frameMD5OutputSHA256': hashlib.sha256(process.stdout).hexdigest(),
        'fullSourceClockQualified': False, 'productionAcceptance': False})
    result['completedStages'].append('normalized-prefix')
    result['currentStage'] = 'complete-prefix'
    return process.stdout


def qualify_prefix(prefix, full, native_rows, source_frames, document, source):
    require(full.get('fullSourceClockQualified') is True, 'aac_prefix_full_control_unqualified')
    rows = prefix['normalizedFilter']['completeRows']
    frames = prefix['nativeFrames']
    full_rows = full['preTrimUserFilterClock']['completeRows']
    require(len(rows) <= len(frames) <= len(source_frames) and
            rows == full_rows[:len(rows)], 'aac_prefix_full_normalized_equivalence')
    native_fields = lambda row: (row['pts'], row['nb_samples'], row.get('side_data_list', []))
    require([native_fields(r) for r in frames] == [native_fields(r) for r in source_frames[:len(frames)]],
            'aac_prefix_full_native_equivalence')
    require([r['samples'] for r in rows] == [r['nb_samples'] for r in frames[:len(rows)]],
            'aac_prefix_leading_frame_and_sample_association')
    output_lines = [line for line in document.decode().splitlines() if line and not line.startswith('#')]
    require(0 < len(output_lines) <= 1024, 'aac_prefix_complete_output_frame_bound')
    samples = sum(int(line.split(',')[3]) for line in output_lines)
    source_facts, pcm = native_pcm(source)  # Fixed full control outside the production-prefix budget.
    require(source_facts['sha256'] == full['sourcePCM']['sha256'], 'aac_prefix_full_pcm_control_identity')
    output = frame_md5(document.decode(), pcm[:samples * 4])
    require(output['completeRows'] == full['outputPCMClock']['completeRows'][:len(output_lines)],
            'aac_prefix_full_output_pcm_clock_equivalence')
    independent = native_clock_rows(frames, prefix['sourceStream']['time_base'], 0, None)['completeRows'][:len(rows)]
    fields = ['pts', 'samples', 'nativeStartSample', 'timestampSampleNumerator', 'timestampSampleDenominator']
    require(all(all(a[k] == b[k] for k in fields) for a, b in zip(independent, native_rows)),
            'aac_prefix_independent_native_ordinals')
    prefix.update(prefixEquivalentToFull=True, outputPCMClock=output,
        nativeOrdinalRows=independent,
        leadingNativeFrames=frames[:3], leadingNormalizedFrames=rows[:3],
        usedFullClockToChooseEdit=False)


def derive_prefix_edit(prefix, first_packet, offset, current):
    require(prefix.get('prefixEquivalentToFull') is True, 'aac_prefix_equivalence_missing')
    require(math.isfinite(offset) and 0 < offset <= 20 and type(current) is int,
            'aac_prefix_requested_eligibility')
    requested = Fraction(str(offset)) * 48000
    require(requested.denominator == 1, 'aac_prefix_requested_sample_grid')
    native = prefix['nativeOrdinalRows']
    normalized = prefix['normalizedFilter']['completeRows']
    first_pts = first_packet['pts']
    associated = [r for r in prefix['sourcePackets'] if r.get('pts') == first_pts and
                  r.get('data_hash') == first_packet.get('data_hash')]
    first = [n for n, r in enumerate(native) if r['pts'] == first_pts]
    target = [n for n, r in enumerate(normalized) if r['pts'] <= requested < r['pts'] + r['samples']]
    require(len(associated) == len(first) == len(target) == 1,
            'aac_prefix_unique_packet_frame_target')
    selected = target[0]
    require(selected + 1 < len(native), 'aac_prefix_target_lookahead_coverage')
    wanted = native[selected]['nativeStartSample'] + int(requested) - normalized[selected]['pts']
    media_time = wanted - native[first[0]]['nativeStartSample']
    delta = media_time - current
    require(media_time >= 32 and -32 <= delta <= 32, 'aac_prefix_edit_bound')
    return {'requestedTimestampSamples': int(requested), 'desiredNativeStartSample': wanted,
        'firstCopiedNativeFrame': native[first[0]], 'targetNativeFrame': native[selected],
        'targetNormalizedFilterFrame': normalized[selected], 'originalMediaTime': current,
        'derivedMediaTime': media_time, 'deltaSamples': delta,
        'sourcePacketIdentity': associated[0], 'usedFullClockToChooseEdit': False,
        'usedReferencePCMToChooseEdit': False, 'productionAcceptance': False,
        'boundary': 'Two-process bounded-prefix producer matched complete fixed-source controls; no generic/native certificate'}



def qualify_negative_controls(prefix, first_packet, offset, current):
    import copy
    controls = []
    target = [n for n, r in enumerate(prefix['normalizedFilter']['completeRows'])
              if r['pts'] <= Fraction(str(offset)) * 48000 < r['pts'] + r['samples']]
    require(len(target) == 1, 'aac_prefix_control_target')
    changes = [
        ('missing_packet', 'aac_prefix_unique_packet_frame_target',
         lambda p, packet: p.update(sourcePackets=[])),
        ('missing_target', 'aac_prefix_unique_packet_frame_target',
         lambda p, packet: p['normalizedFilter'].update(completeRows=[])),
        ('missing_lookahead', 'aac_prefix_target_lookahead_coverage',
         lambda p, packet: p.update(nativeOrdinalRows=p['nativeOrdinalRows'][:target[0] + 1])),
        ('changed_payload', 'aac_prefix_unique_packet_frame_target',
         lambda p, packet: packet.update(data_hash='SHA256:' + '0' * 64)),
        ('missing_equivalence', 'aac_prefix_equivalence_missing',
         lambda p, packet: p.update(prefixEquivalentToFull=False))]
    for name, expected, change in changes:
        candidate, packet = copy.deepcopy(prefix), dict(first_packet)
        change(candidate, packet)
        try:
            derive_prefix_edit(candidate, packet, offset, current)
        except RuntimeError as error:
            require(str(error) == expected, 'aac_prefix_negative_control_wrong_rejection_' + name)
            controls.append({'name': name, 'result': 'rejected', 'reason': str(error)})
        else:
            raise RuntimeError('aac_prefix_negative_control_admitted_' + name)
    require(len(controls) == 5, 'aac_prefix_negative_control_count')
    return controls

def normalized_output_rows(document):
    import re
    require(len(document.encode()) <= 2 << 20, 'aac_prefix_normalized_output_bound')
    headers = [line.strip() for line in document.splitlines() if line.startswith('#')]
    require([h for h in headers if h.startswith('#tb ')] == ['#tb 0: 1/48000'] and
            [h for h in headers if h.startswith('#media_type ')] == ['#media_type 0: audio'] and
            [h for h in headers if h.startswith('#sample_rate ')] == ['#sample_rate 0: 48000'],
            'aac_prefix_normalized_output_format')
    rows = []
    for line in document.splitlines():
        if not line.strip() or line.startswith('#'):
            continue
        fields = [value.strip() for value in line.split(',')]
        require(len(fields) == 6 and all(re.fullmatch(r'-?[0-9]{1,17}', v)
                for v in fields[:5]) and re.fullmatch(r'[a-f0-9]{32}', fields[5]),
                'aac_prefix_normalized_output_shape')
        stream, dts, pts, samples, size = [int(v) for v in fields[:5]]
        require(stream == 0 and dts == pts and 0 < samples <= 1024 and size == samples * 4,
                'aac_prefix_normalized_output_extent')
        rows.append((pts, samples))
        require(len(rows) <= 1024, 'aac_prefix_normalized_output_frame_bound')
    require(rows, 'aac_prefix_normalized_output_empty')
    return rows
