"""Independent uninterrupted AAC reference for populated resume recovery."""
import json
import math
import subprocess

from hls_timeline_http import sha
from hls_followon_public import check


def audio_packets(initialization, fragment):
    result = subprocess.run(['ffprobe', '-v', 'error', '-select_streams', 'a:0',
        '-show_packets', '-show_data_hash', 'sha256', '-show_entries',
        'packet=pts,dts,duration,data_hash', '-of', 'json', 'pipe:0'],
        input=initialization + fragment.read_bytes(), capture_output=True,
        timeout=15, check=True)
    check(len(result.stdout) <= 1024 * 1024, 'reference_packet_bound')
    return json.loads(result.stdout)['packets']


def uninterrupted_audio_reference(source, prepared, directory):
    """Encode once from the certified source key, without any refill filters."""
    directory.mkdir()
    timeline = json.loads((prepared / '.copy-timeline').read_text())
    start = timeline['Keys'][0]['PTS'] * timeline['Numerator'] / timeline['Denominator']
    check(0 < timeline['End'] - start <= 30, 'reference_window_bound')
    command = ['ffmpeg', '-nostdin', '-v', 'error', '-seek_timestamp', '1',
        '-ss', f'{math.floor(start * 1_000_000) / 1_000_000:.6f}', '-i', str(source),
        '-map', '0:v:0', '-map', '0:a:0', '-sn', '-c:v', 'copy', '-c:a', 'aac',
        '-ac', '2', '-b:a', '128k', '-copypriorss', '0', '-f', 'hls',
        '-hls_time', '0.1', '-hls_playlist_type', 'event', '-hls_segment_type', 'fmp4',
        '-hls_segment_options', 'movflags=+frag_discont+skip_sidx', '-hls_flags', 'temp_file',
        '-hls_fmp4_init_filename', 'init.mp4', '-hls_segment_filename',
        str(directory / 'segment-%05d.m4s'), str(directory / 'index.m3u8')]
    subprocess.run(command, capture_output=True, check=True, timeout=30)
    initialization = (directory / 'init.mp4').read_bytes()
    reference = {p.name: audio_packets(initialization, p) for p in sorted(directory.glob('segment-*.m4s'))}
    check(len(reference) == len(timeline['Keys']), 'reference_fragment_count')
    facts = {'command': command, 'sourceOrigin': start, 'profile': 'copied H264; AAC stereo 128k',
             'filesSHA256': {p.name: sha(p) for p in directory.iterdir() if p.is_file()},
             'oracle': 'Exact AAC packet timestamps, duration, and payload hashes against uninterrupted encoding.'}
    return initialization, reference, facts
