"""Bounded pinned-codec counterfactuals, isolated diagnosis rather than public proof."""
import hashlib
import json
import math
import subprocess
from hls_followon_frames import decode_frames, stream_metadata
from hls_followon_public import check, bounded_bytes
from hls_remaining_process import source_snapshot
from hls_timeline_packets import manifest_facts


def mux_case(run, source, metadata, directory, label, offset, options, output_seek=None):
    directory.mkdir()
    command = ['ffmpeg', '-nostdin', '-v', 'error', '-y', *options.get('global', [])]
    if offset is not None:
        command += [*options.get('input', []), '-ss', str(offset)]
    command += ['-i', str(source), '-map', '0:v:0', '-map', '0:a:0?', '-sn', '-c', 'copy']
    if output_seek is not None:
        command += ['-ss', str(output_seek)]
    command += options.get('output', [])
    command += ['-f', 'hls', '-hls_time', '2', '-hls_playlist_type', 'event',
        '-hls_segment_type', 'fmp4', '-hls_segment_options', options.get('segment', 'movflags=+frag_discont+skip_sidx'),
        '-hls_flags', 'temp_file', '-hls_fmp4_init_filename', 'init.mp4',
        '-hls_segment_filename', str(directory / 'segment-%05d.m4s'), str(directory / 'index.m3u8')]
    row = {'label': label, 'inputSeekSeconds': offset, 'outputSeekSeconds': output_seek,
        'options': options, 'result': 'unqualified', 'sourceSHA256': metadata['sha256']}
    if '-avoid_negative_ts' in command:
        position = command.index('-avoid_negative_ts')
        row['muxOptionPlacementQualified'] = command.index('-i') + 1 < position < len(command) - 2
    stage = 'command-qualification'
    try:
        check(row.get('muxOptionPlacementQualified', True), 'mux_output_policy_misgrouped')
        stage = 'encode'
        run(command, 30)
        stage = 'manifest'
        manifest = bounded_bytes(directory / 'index.m3u8', 65536, 'mux_manifest_bound')
        facts, segments = manifest_facts(manifest)
        check(facts['endlist'] and 0 < len(segments) <= 32, 'mux_complete_fragment_bound')
        row['manifest'] = facts
        init = bounded_bytes(directory / 'init.mp4', 2 << 20, 'mux_init_bound')
        row.update(initializationSHA256=hashlib.sha256(init).hexdigest(), initializationEditBoxPresent=b'elst' in init)
        data = init
        for name, _ in segments:
            data += bounded_bytes(directory / name, 8 << 20, 'mux_fragment_bound')
            check(len(data) <= 32 << 20, 'mux_join_bound')
        joined = directory / 'joined.mp4'
        joined.write_bytes(data)
        stage = 'decode'
        decoded, frames = decode_frames(joined)
        row.update(presentation=decoded, frameColumns=['pts', 'md5'], timestampedFrames=frames, allRawFramesRetained=True)
        stage = 'packet-probe'
        packet_data = run(['ffprobe', '-v', 'error', '-select_streams', 'v:0', '-read_intervals', '%+#4096',
            '-show_packets', '-show_entries', 'packet=pts_time,dts_time,duration_time,flags', '-of', 'json', str(joined)])
        packets = json.loads(packet_data).get('packets', [])
        check(0 < len(packets) < 4096, 'mux_packet_bound')
        points = [float(v['pts_time']) for v in packets]
        check(all(math.isfinite(v) for v in points), 'mux_packet_clock')
        columns = ['pts_time', 'dts_time', 'duration_time', 'flags']
        check(all(set(v) == set(columns) for v in packets), 'mux_packet_columns')
        row.update(packetColumns=columns, packetRows=[[v[key] for key in columns] for v in packets], result='observed')
    except (RuntimeError, OSError, subprocess.SubprocessError, ValueError, KeyError) as error:
        if isinstance(error, RuntimeError) and str(error) in ['bounded_diagnostic_deadline', 'bounded_run_deadline']:
            raise
        row.update(failureStage=stage, failureClass=str(error) if isinstance(error, RuntimeError) else type(error).__name__)
    return row


def counterfactuals(run, source, metadata, directory, result):
    """One source, fixed options, and exact frame mapping; never hide failed candidates."""
    before = source_snapshot(source)
    full, source_rows = decode_frames(source)
    index = {digest: n for n, (_, digest) in enumerate(source_rows)}
    check(len(index) == len(source_rows) == 768, 'mux_unique_reference')
    directory.mkdir()
    result.update(boundary='Isolated installed-codec mux diagnosis; no Server, renderer or native acceptance.',
        source=before, sourceMetadata=stream_metadata(source), sourceFrames=full)
    # AVFormat output options must follow the input; earlier placement only
    # supplies an input-format option and cannot qualify an HLS mux policy.
    disabled = {'output': ['-avoid_negative_ts', 'disabled']}
    candidates = [
        ('legacy', {}, 12.5, None),
        ('disabled-shift', disabled, 12.5, None),
        ('explicit-edits', dict(disabled, segment='movflags=+frag_discont+skip_sidx:use_editlist=1'), 12.5, None),
        ('delayed-edits', dict(disabled, segment='movflags=+frag_discont+skip_sidx+delay_moov:use_editlist=1'), 12.5, None),
        ('negative-cts-edits', dict(disabled, segment='movflags=+frag_discont+skip_sidx+negative_cts_offsets:use_editlist=1'), 12.5, None),
        ('normal-initial-mux', {'segment': 'movflags=+skip_sidx'}, 12.5, None),
        ('normal-delayed-edits', dict(disabled, segment='movflags=+skip_sidx+delay_moov:use_editlist=1'), 12.5, None),
        ('no-prior-copy', {'output': ['-copypriorss', '0']}, 12.5, None),
        ('output-cut', {}, None, 12.5),
        ('key-control', {}, 12, None),
        ('zero-control', {}, 0, None)]
    for label, options, offset, output_seek in candidates:
        row = mux_case(run, source, metadata, directory / label, label, offset, options, output_seek)
        if 'timestampedFrames' in row:
            wanted = 12.5 if 'control' not in label else (12 if label == 'key-control' else 0)
            expected = [n for n, (point, _) in enumerate(source_rows) if point >= wanted - 0.000001]
            mapped = [index.get(v[1]) for v in row['timestampedFrames']]
            row['sourceIndices'] = mapped
            row['expectedSourceIndices'] = expected
            row['exactRequestedRawSequence'] = mapped == expected
            if 'packetRows' in row:
                row['firstPacketIsKey'] = 'K' in row['packetRows'][0][3]
        result['cases'].append(row)
    shifted = directory / 'shifted.mkv'
    run(['ffmpeg', '-nostdin', '-v', 'error', '-i', str(source), '-map', '0', '-c', 'copy',
         '-output_ts_offset', '5', str(shifted)], 30)
    shifted_before = source_snapshot(shifted)
    result['shiftedSource'] = dict(shifted_before, metadata=stream_metadata(shifted))
    for label, options, offset in [('origin-relative-control', {}, 12),
            ('origin-absolute-control', {'input': ['-seek_timestamp', '1']}, 17),
            ('origin-incorrect-relative', {}, 17)]:
        row = mux_case(run, shifted, dict(metadata, sha256=shifted_before['sha256']), directory / label, label, offset, options)
        if 'timestampedFrames' in row:
            row['sourceIndices'] = [index.get(v[1]) for v in row['timestampedFrames']]
        result['cases'].append(row)
    result['sourceUnchanged'] = source_snapshot(source) == before
    result['shiftedSourceUnchanged'] = source_snapshot(shifted) == shifted_before
    check(result['sourceUnchanged'] and result['shiftedSourceUnchanged'], 'mux_source_changed')
    return result
