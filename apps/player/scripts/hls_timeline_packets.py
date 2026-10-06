"""Numeric packet evidence for real synthetic public HLS fragments."""
import hashlib
import json
import math
import re
import subprocess


def fragment_packets(path):
    command = ["ffprobe", "-v", "error", "-select_streams", "v:0", "-read_intervals", "%+#4096",
        "-show_packets", "-show_entries", "packet=pts_time,dts_time,duration_time,flags",
        "-of", "json", str(path)]
    result = subprocess.run(command, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, timeout=30)
    if result.returncode or len(result.stdout) > 2 * 1024 * 1024:
        raise RuntimeError("public_fragment_probe")
    packets = json.loads(result.stdout).get("packets", [])
    if len(packets) >= 4096:
        raise RuntimeError("public_packet_bound")
    points, ends = [], []
    keys = 0
    for packet in packets:
        if "pts_time" not in packet or "duration_time" not in packet:
            raise RuntimeError("public_packet_timing")
        point, duration = float(packet["pts_time"]), float(packet["duration_time"])
        if not math.isfinite(point) or not math.isfinite(duration) or duration <= 0:
            raise RuntimeError("public_packet_timing")
        points.append(point)
        ends.append(point + duration)
        keys += "K" in packet.get("flags", "")
    first, last = min(points, default=None), max(ends, default=None)
    return {"videoPackets": len(packets), "keyframePackets": keys,
        "firstVideoTime": first, "lastVideoEnd": last,
        "videoSpanSeconds": last - first if points else 0}



def decoded_identity(path, offset=0):
    command = ["ffmpeg", "-nostdin", "-v", "error", "-xerror", "-threads", "2", "-i", str(path)]
    if offset:
        command += ["-ss", str(offset)]
    command += ["-an", "-frames:v", "4097", "-fps_mode", "passthrough", "-f", "framemd5", "pipe:1"]
    result = subprocess.run(command, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, timeout=60)
    if result.returncode or len(result.stdout) > 2 * 1024 * 1024:
        raise RuntimeError("public_fragment_decode")
    hashes = [line.rsplit(b",", 1)[-1].strip() for line in result.stdout.splitlines()
        if line and not line.startswith(b"#")]
    if len(hashes) > 4096 or any(re.fullmatch(rb"[a-f0-9]{32}", value) is None for value in hashes):
        raise RuntimeError("decoded_fixture_bound")
    return {"frames": len(hashes), "sha256": hashlib.sha256(b"\n".join(hashes)).hexdigest()}


def safe_seek_phases(private_log):
    starts = []
    for line in private_log.splitlines():
        if 'msg="HLS transcode started"' not in line:
            continue
        values = {key: re.search(r"\b" + key + r"=(-?[0-9]+)\b", line)
            for key in ["input_seek_ms", "segment_start"]}
        mode = re.search(r"\bmode=(remux|audio-transcode|transcode)\b", line)
        work = re.search(r"\bwork_class=(background|playback)\b", line)
        if all(values.values()) and mode and work:
            starts.append({key: int(value[1]) for key, value in values.items()} | {"mode": mode[1], "workClass": work[1]})
    return {"encoderStarts": starts[:32], "encoderStartsBounded": len(starts) <= 32}


def manifest_facts(data):
    text = data.decode("utf-8")
    lengths = [float(v) for v in re.findall(r"^#EXTINF:([0-9.]+),", text, re.M)]
    segments = re.findall(r"^segment-[0-9]{5}\.m4s$", text, re.M)
    if not (0 < len(segments) <= 100 and len(lengths) == len(segments)
            and all(math.isfinite(v) and 0 < v <= 60 for v in lengths)):
        raise RuntimeError("variant_segment_shape")
    return {"sha256": hashlib.sha256(data).hexdigest(), "playlistType": "VOD" if "#EXT-X-PLAYLIST-TYPE:VOD" in text else "EVENT",
        "endlist": "#EXT-X-ENDLIST" in text, "durationSeconds": sum(lengths),
        "segmentCount": len(segments)}, list(zip(segments, lengths))
