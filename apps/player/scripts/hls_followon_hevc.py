"""Bounded offline HEVC codec diagnostics; these are not public acceptance."""
import json
import math
import subprocess
from hls_followon_frames import decode_frames, stream_metadata
from hls_followon_public import check


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
    actual, _ = decode_frames(public)
    reference, _ = decode_frames(source, 0)
    case['videoDiagnostic'] = {'boundary': 'Independent whole decode facts, not a HEVC timestamp certificate',
        'public': actual, 'source': reference, 'identicalSourceFrames': actual['identity'] == reference['identity']}


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
