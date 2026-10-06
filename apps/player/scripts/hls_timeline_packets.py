"""Numeric packet evidence for real synthetic public HLS fragments."""
import json
import math
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
