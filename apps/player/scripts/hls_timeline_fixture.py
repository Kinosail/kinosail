"""Bounded synthetic fixtures for authenticated copied-video timeline proof."""
import json
import subprocess
from hls_timeline_http import sha


def check(condition, failure):
    if not condition:
        raise RuntimeError(failure)


def fixture(directory, name, gop, keys, rate="24", frames=2304, extension=".mkv", audio_marked=False):
    numerator, _, denominator = rate.partition("/")
    frame_rate = float(numerator) / float(denominator or "1")
    expected_duration = frames / frame_rate
    path = directory / (name + extension)
    audio = (f"aevalsrc=0.2*sin(2*PI*(440+110*floor(t/4))*t):s=48000:d={expected_duration}"
             if audio_marked else f"sine=frequency=440:sample_rate=48000:duration={expected_duration}")
    command = ["ffmpeg", "-nostdin", "-v", "error", "-f", "lavfi", "-i", f"testsrc2=s=640x360:r={rate}:d={expected_duration}",
        "-f", "lavfi", "-i", audio,
        "-c:v", "libx264", "-threads", "2", "-preset", "veryfast", "-crf", "32", "-pix_fmt", "yuv420p",
        "-g", str(gop), "-keyint_min", "1", "-sc_threshold", "0", "-force_key_frames", keys,
        "-frames:v", str(frames), "-c:a", "aac", "-ac", "2", str(path)]
    subprocess.run(command, check=True, timeout=120, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    probe = subprocess.check_output(["ffprobe", "-v", "error", "-select_streams", "v:0", "-skip_frame", "nokey",
        "-show_entries", "frame=best_effort_timestamp_time:format=duration:stream=avg_frame_rate",
        "-of", "json", str(path)], timeout=30)
    facts = json.loads(probe)
    times = [float(f["best_effort_timestamp_time"]) for f in facts["frames"] if "best_effort_timestamp_time" in f]
    duration = float(facts["format"]["duration"])
    actual_rate = facts["streams"][0]["avg_frame_rate"].split("/")
    check(abs(float(actual_rate[0]) / float(actual_rate[1]) - frame_rate) < 0.001, "fixture_frame_rate")
    expected = [float(v) for v in keys.split(",")]
    check(len(times) == len(expected) and all(abs(a-b) < 0.05 for a, b in zip(times, expected)), "fixture_keyframes")
    check(abs(duration - expected_duration) < 0.1, "fixture_duration")
    if name == "fractional15s":
        check(any(abs(v - round(v, 3)) > 0.00001 for v in times), "fixture_submillisecond_keyframe")
    return path, {"command": command, "sha256": sha(path), "durationSeconds": duration, "videoDurationSeconds": expected_duration, "frameRate": frame_rate,
        "videoFrames": frames, "keyframesSeconds": times, "audioTimeMarked": audio_marked}
