"""Strict whole-output HEVC fixture proof and separate offline diagnostics."""
from collections import Counter
from fractions import Fraction
import json
import math
import subprocess
from hls_followon_frames import audio_sequence, decode_frames, stream_metadata
from hls_followon_public import check
from hls_timeline_packets import audio_packet_facts


def packets(path, stream, maximum=4096, complete=True):
    data = subprocess.check_output(['ffprobe', '-v', 'error', '-select_streams', stream,
        '-read_intervals', '%+#' + str(maximum), '-show_packets',
        '-show_entries', 'packet=pts_time,dts_time,duration_time,flags', '-of', 'json', str(path)], timeout=30)
    check(len(data) <= 2 * 1024 * 1024, 'hevc_diagnostic_packet_bound')
    rows = json.loads(data).get('packets', [])
    check(len(rows) <= maximum and (not complete or len(rows) < maximum), 'hevc_diagnostic_packet_count')
    return rows


def evidence(source, public, case):
    case['sourceStreamOrigins'] = stream_metadata(source)
    keys = [p for p in packets(source, 'v:0') if 'K' in p.get('flags', '')]
    check(0 < len(keys) <= 32, 'hevc_diagnostic_key_bound')
    case['sourceVideoKeys'] = keys
    actual, actual_rows = decode_frames(public)
    reference, reference_rows = decode_frames(source, 0)
    origins = case['sourceStreamOrigins']
    videos = [v for v in origins['streams'] if v['codec_type'] == 'video']
    source_contract = (len(videos) == 1 and videos[0]['codec_name'] == 'hevc'
        and Fraction(videos[0]['avg_frame_rate']) == 24
        and abs(float(videos[0]['start_time'])) <= 0.001
        and abs(float(origins['format']['start_time'])) <= 0.001
        and abs(float(origins['format']['duration']) - 10) <= 0.1
        and reference['presentedFrames'] == 240 and reference['presentationQualified']
        and reference['presentationOrderValid']
        and all(abs(point - number / 24) <= 0.001
                for number, (point, _) in enumerate(reference_rows)))
    case['independentSourceContract'] = {'qualified': source_contract,
        'expectedFrames': 240, 'expectedFrameRate': 24, 'expectedDurationSeconds': 10,
        'expectedSourceOriginSeconds': 0, 'source': origins}
    case['videoDiagnostic'] = {'boundary': 'Independent whole decode facts, not a HEVC timestamp certificate',
        'public': actual, 'source': reference, 'identicalSourceFrames': actual['identity'] == reference['identity']}
    source_hashes = [value for _, value in reference_rows]
    counts = Counter(source_hashes)
    index = {value: number for number, value in enumerate(source_hashes) if counts[value] == 1}
    mapped = [index.get(value) for _, value in actual_rows]
    qualified = all(counts[value] == 1 for value in source_hashes) and all(v is not None for v in mapped)
    missing = [number for number in range(len(reference_rows)) if number not in mapped] if qualified else None
    case['sourceFrameMapping'] = {'boundary': 'Diagnosis only; never trims or reorders delivered frames',
        'qualified': qualified, 'publicSourceIndices': mapped, 'missingSourceIndices': missing,
        'missingSourcePTS': [reference_rows[n][0] for n in missing] if missing is not None else None,
        'ambiguousSourceHashes': sum(count > 1 for count in counts.values()),
        'unknownPublicFrames': sum(value not in counts for _, value in actual_rows)}
    centers = [1, 3, 5, 7, 9]
    case['sourceAudioContent'] = audio_sequence(source, 10, centers=centers)
    case['publicAudioContent'] = audio_sequence(public, 10, centers=centers)
    clock = actual_rows[0][0] - reference_rows[0][0]
    errors = [abs(a[0] - b[0] - clock) for a, b in zip(actual_rows, reference_rows)]
    case['wholeVideoClock'] = {'offsetSeconds': clock,
        'qualified': len(actual_rows) == len(reference_rows),
        'maximumFrameErrorMs': 1000 * max(errors) if len(actual_rows) == len(reference_rows) else None,
        'boundary': 'Matched zero-offset synthetic fixture only; no general HEVC seek certificate'}


def fragment_evidence(path, audio_rows, filename, advertised):
    video = packets(path, 'v:0')
    points = [float(row['pts_time']) for row in video]
    decode = [float(row['dts_time']) for row in video]
    lengths = [float(row['duration_time']) for row in video]
    check(all(math.isfinite(v) for v in points + decode + lengths)
          and all(0 < v <= 1 for v in lengths), 'hevc_public_video_packet_timing')
    audio = audio_packet_facts(audio_rows) if audio_rows else {'audioPackets': 0}
    span = max(p + d for p, d in zip(points, lengths)) - min(points) if points else None
    return {'segment': filename, 'advertisedSeconds': advertised,
        'videoSpanSeconds': span,
        'advertisedVideoDurationErrorMs': 1000 * abs(span - advertised) if span is not None else None,
        'videoPackets': len(video),
        'firstVideoPTS': min(points) if points else None,
        'lastVideoEnd': max(p + d for p, d in zip(points, lengths)) if points else None,
        'firstVideoDTS': decode[0] if decode else None, 'lastVideoDTS': decode[-1] if decode else None,
        'videoDecodeOrderValid': all(a < b for a, b in zip(decode, decode[1:])), **audio}


def validate_public_output(case):
    facts = case['videoDiagnostic']
    clock = case['wholeVideoClock']
    fragments = case['fragmentPacketEvidence']
    source_audio, public_audio = case['sourceAudioContent']['windows'], case['publicAudioContent']['windows']
    checks = [
        (case['independentSourceContract']['qualified'], 'hevc_independent_source_contract'),
        (case['publicVariant']['endlist'] and case['publicVariant']['playlistType'] == 'VOD'
         and abs(case['publicVariant']['durationSeconds'] - 10) <= 0.1, 'hevc_public_full_duration'),
        (all(f['videoSpanSeconds'] is not None and f['advertisedVideoDurationErrorMs'] <= 150
             for f in fragments), 'hevc_advertised_fragment_duration'),
        (facts['identicalSourceFrames'], 'hevc_whole_source_frames'),
        (facts['public']['presentationQualified'] and facts['public']['presentationOrderValid'],
         'hevc_whole_presentation_order'),
        (clock['qualified'] and 0 <= clock['offsetSeconds'] <= 1
         and clock['maximumFrameErrorMs'] <= 2, 'hevc_whole_source_clock'),
        (case['initializationStable'], 'hevc_retained_initialization'),
        (all(f['videoPackets'] > 0 and f['audioPackets'] > 0 for f in fragments),
         'hevc_fragment_video_or_audio_missing'),
        (all(f['videoDecodeOrderValid'] for f in fragments), 'hevc_fragment_decode_order'),
        (len(source_audio) == len(public_audio) == 5 and all(a['available'] and b['available']
             and abs(a['frequencyHz'] - b['frequencyHz']) <= 10 and b['rms'] > 0.01
             for a, b in zip(source_audio, public_audio)), 'hevc_complete_audio_content')]
    adjacent, av, interior = [], [], []
    complete = all(f['videoPackets'] and f['audioPackets'] for f in fragments)
    for number, fragment in enumerate(fragments):
        if not fragment['audioPackets'] or not fragment['videoPackets']:
            continue
        interior += [fragment['maximumAudioGapSeconds'], fragment['maximumAudioOverlapSeconds']]
        av.append(abs(fragment['firstVideoPTS'] - fragment['firstAudioTime']))
        checks.append((fragment['audioPacketOrderValid'], 'hevc_audio_packet_order'))
        if number and fragments[number - 1]['audioPackets'] and fragments[number - 1]['videoPackets']:
            previous = fragments[number - 1]
            adjacent.append(abs(fragment['firstAudioTime'] - previous['lastAudioEnd']))
            checks.append((previous['lastVideoDTS'] < fragment['firstVideoDTS'], 'hevc_fragment_decode_seam'))
    case['packetSeamEvidence'] = {'qualified': complete,
        'maximumAACInteriorErrorMs': 1000 * max(interior, default=0),
        'maximumAACAdjacentErrorMs': 1000 * max(adjacent, default=0),
        'maximumAVStartErrorMs': 1000 * max(av, default=0)}
    checks += [(complete and max(interior, default=0) <= 0.05, 'hevc_aac_interior_continuity'),
               (complete and max(adjacent, default=0) <= 0.05, 'hevc_aac_fragment_continuity'),
               (complete and max(av, default=0) <= 0.15, 'hevc_av_fragment_alignment')]
    case['failures'] += [name for passed, name in checks if not passed]


def seek_diagnostics(source, public, directory, case):
    result = {'boundary': 'Offline codec experiment only; no Server acceptance or source mutation', 'variants': []}
    case['offlineSeekDiagnostic'] = result
    source_keys = [p for p in packets(source, 'v:0') if 'K' in p.get('flags', '')]
    key = next(p for p in source_keys if abs(float(p['pts_time']) - 8) <= 0.001)
    pts, dts = float(key['pts_time']), float(key['dts_time'])
    clock = float(packets(public, 'v:0', 16, complete=False)[0]['pts_time'])
    check(all(math.isfinite(v) for v in [pts, dts, clock]) and 0 <= clock <= 1 and 0 <= pts - dts <= 1,
          'hevc_diagnostic_clock')
    reference, _ = decode_frames(source, 8)
    result.update(sourceKey=key, generatedVideoClock=clock, sourceReference=reference['identity'])
    variants = [('key-pts-clock', pts, pts + clock, -clock),
                ('key-dts-clock', dts, dts + clock, 0),
                ('key-dts-audio-shift', dts, dts + clock, -clock)]
    for name, seek, mux, shift in variants:
        attempt = {'name': name, 'inputSeek': seek, 'outputOffset': mux, 'audioShift': shift}
        result['variants'].append(attempt)
        root = directory / ('offline-' + name)
        root.mkdir()
        command = ['ffmpeg', '-nostdin', '-v', 'error', '-y', '-avoid_negative_ts', 'disabled',
            '-max_delay', '5000000', '-ss', f'{seek:.6f}', '-i', str(source), '-map', '0:v:0', '-map', '0:a:0',
            '-sn', '-c:v', 'copy', '-c:a', 'aac', '-ac', '2', '-b:a', '192k', '-copypriorss', '0',
            '-output_ts_offset', f'{mux:.6f}']
        if shift:
            command += ['-bsf:a', f'setts=pts=PTS+({shift:.6f})/TB:dts=DTS+({shift:.6f})/TB']
        command += ['-f', 'hls', '-hls_time', '0.1', '-hls_playlist_type', 'event', '-hls_segment_type', 'fmp4',
            '-hls_segment_options', 'movflags=+frag_discont+skip_sidx', '-hls_flags', 'temp_file',
            '-hls_fmp4_init_filename', 'init.mp4', '-start_number', '4',
            '-hls_segment_filename', str(root / 'segment-%05d.m4s'), str(root / 'index.m3u8')]
        attempt['command'] = command
        try:
            subprocess.run(command, check=True, timeout=30, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            fragments = sorted(root.glob('segment-*.m4s'))
            check(0 < len(fragments) <= 4, 'hevc_diagnostic_fragment_bound')
            joined = root / 'public.mp4'
            joined.write_bytes((root / 'init.mp4').read_bytes() + b''.join(p.read_bytes() for p in fragments))
            video, _ = decode_frames(joined)
            audio = packets(joined, 'a:0')
            attempt.update(video=video, identicalReferenceFrames=video['identity'] == reference['identity'],
                audioPackets=len(audio), firstAudio=audio[0] if audio else None, lastAudio=audio[-1] if audio else None)
        except Exception as error:
            attempt['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
