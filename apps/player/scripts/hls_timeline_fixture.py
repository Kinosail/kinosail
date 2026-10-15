"""Bounded synthetic fixtures for authenticated copied-video timeline proof."""
import json
import subprocess
from hls_timeline_http import sha

LONG_RESUME_KEYS = (0, 2, 4, 7, 9, 12, 14, 18, 21, 23, 27, 30, 34, 38, 42, 47, 49, 53, 58, 62, 67, 70, 73)


def check(condition, failure):
    if not condition:
        raise RuntimeError(failure)


def fixture(directory, name, gop, keys, rate="24", frames=2304, extension=".mkv", noise=False, video_options=(), size="640x360"):
    numerator, _, denominator = rate.partition("/")
    frame_rate = float(numerator) / float(denominator or "1")
    expected_duration = frames / frame_rate
    path = directory / (name + extension)
    audio = f"anoisesrc=color=white:amplitude=0.12:seed=872:sample_rate=48000:duration={expected_duration}" if noise else f"sine=frequency=440:sample_rate=48000:duration={expected_duration}"
    command = ["ffmpeg", "-nostdin", "-v", "error", "-f", "lavfi", "-i", f"testsrc2=s={size}:r={rate}:d={expected_duration}",
        "-f", "lavfi", "-i", audio,
        "-c:v", "libx264", "-threads", "2", "-preset", "veryfast", "-crf", "32", "-pix_fmt", "yuv420p",
        "-vf", "setparams=range=limited:colorspace=smpte170m:color_primaries=smpte170m:color_trc=smpte170m",
        "-g", str(gop), "-keyint_min", "1", "-sc_threshold", "0", "-force_key_frames", keys,
        "-frames:v", str(frames), *video_options, *(["-af", "lowpass=f=6000"] if noise else []), "-c:a", "aac", "-ac", "2", str(path)]
    subprocess.run(command, check=True, timeout=120, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    probe = subprocess.check_output(["ffprobe", "-v", "error", "-select_streams", "v:0", "-skip_frame", "nokey",
        "-show_entries", "frame=best_effort_timestamp_time:format=duration:stream=avg_frame_rate,color_space,color_transfer,color_primaries,color_range",
        "-of", "json", str(path)], timeout=30)
    facts = json.loads(probe)
    times = [float(f["best_effort_timestamp_time"]) for f in facts["frames"] if "best_effort_timestamp_time" in f]
    duration = float(facts["format"]["duration"])
    actual_rate = facts["streams"][0]["avg_frame_rate"].split("/")
    color = {key: facts["streams"][0].get(key) for key in ("color_space", "color_transfer", "color_primaries", "color_range")}
    check(color == {"color_space": "smpte170m", "color_transfer": "smpte170m", "color_primaries": "smpte170m", "color_range": "tv"}, "fixture_color_metadata")
    check(abs(float(actual_rate[0]) / float(actual_rate[1]) - frame_rate) < 0.001, "fixture_frame_rate")
    expected = [float(v) for v in keys.split(",")]
    check(len(times) == len(expected) and all(abs(a-b) < max(0.05, 1 / frame_rate) for a, b in zip(times, expected)), "fixture_keyframes")
    check(abs(duration - expected_duration) < 0.1, "fixture_duration")
    if name == "fractional15s":
        check(any(abs(v - round(v, 3)) > 0.00001 for v in times), "fixture_submillisecond_keyframe")
    return path, {"command": command, "sha256": sha(path), "durationSeconds": duration, "videoDurationSeconds": expected_duration, "frameRate": frame_rate,
        "videoFrames": frames, "keyframesSeconds": times, "color": color}


def browser_reference(source):
    """Rewrap generated Matroska for WebKit; certify unchanged video and clock."""
    output = source.with_name('Resume Reference.mp4')
    command = ['ffmpeg', '-nostdin', '-v', 'error', '-i', str(source), '-map', '0:v:0',
               '-map', '0:a:0?', '-c', 'copy', '-movflags', '+faststart', str(output)]
    subprocess.run(command, check=True, capture_output=True, timeout=60)
    packets = []
    for path in [source, output]:
        probe = subprocess.check_output(['ffprobe', '-v', 'error', '-select_streams', 'v:0',
            '-show_packets', '-show_data_hash', 'sha256', '-show_entries', 'packet=pts_time,data_hash',
            '-of', 'json', str(path)], timeout=30)
        check(len(probe) <= 1024 * 1024, 'reference_probe_bound')
        packets.append(json.loads(probe)['packets'])
    check(len(packets[0]) == len(packets[1]) and len(packets[0]) > 0, 'reference_packet_count')
    check(all(a['data_hash'] == b['data_hash'] and abs(float(a['pts_time']) - float(b['pts_time'])) < .000001
              for a, b in zip(*packets)), 'reference_video_or_clock_changed')
    return {'command': command, 'sha256': sha(output), 'videoPackets': len(packets[0]),
            'videoPayloadAndClockUnchanged': True, 'boundary': 'Independent stream-copy MP4 reference; audio not certified by this control'}
