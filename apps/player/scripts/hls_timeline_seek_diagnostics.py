"""Bounded isolated codec seek diagnostics; public Server assertions remain authoritative."""
import hashlib
import json
import subprocess


def first_frames(source, offset=0):
    args = ["ffmpeg", "-nostdin", "-v", "error", "-threads", "2", "-i", str(source)]
    if offset:
        args += ["-ss", format(offset, ".9f")]
    args += ["-an", "-frames:v", "3", "-fps_mode", "passthrough", "-f", "framemd5", "pipe:1"]
    result = subprocess.run(args, capture_output=True, timeout=30)
    hashes = [row.rsplit(b",", 1)[-1].strip() for row in result.stdout.splitlines()
        if row and not row.startswith(b"#")]
    return {"exitStatus": result.returncode, "frames": len(hashes),
        "sha256": hashlib.sha256(b"\n".join(hashes)).hexdigest()}


def seek_diagnostics(directory, source, key):
    target = directory / (source.stem + "-seek-diagnostic")
    target.mkdir()
    reference = first_frames(source, max(0, key - 0.000001))
    if reference["exitStatus"] or reference["frames"] != 3:
        raise RuntimeError("seek_reference_three_frames")
    candidates = [
        ("legacy", ["-ss", format(key, ".9f")], []),
        ("copyts", ["-copyts", "-ss", format(key, ".9f")], []),
        ("padded", ["-ss", format(key + 0.14, ".9f")], []),
        ("padded-copyts", ["-copyts", "-ss", format(key + 0.14, ".9f")], []),
        ("output-ss", [], ["-ss", format(key, ".9f")]),
        ("output-ss-copyts", ["-copyts"], ["-ss", format(key, ".9f")])]
    rows = []
    for name, before, after in candidates:
        output = target / (name + ".mp4")
        args = ["ffmpeg", "-nostdin", "-v", "error", "-y"] + before + ["-i", str(source)] + after
        args += ["-map", "0:v:0", "-map", "0:a:0?", "-c", "copy", "-t", "2",
            "-avoid_negative_ts", "disabled", "-movflags", "+frag_keyframe+empty_moov", str(output)]
        result = subprocess.run(args, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=30)
        row = {"candidate": name, "exitStatus": result.returncode}
        if result.returncode == 0:
            probe = subprocess.run(["ffprobe", "-v", "error", "-select_streams", "v:0",
                "-read_intervals", "%+#64", "-show_packets", "-show_entries", "packet=pts_time,dts_time,flags",
                "-of", "json", str(output)], capture_output=True, timeout=15)
            if probe.returncode == 0 and len(probe.stdout) <= 65536:
                packets = json.loads(probe.stdout).get("packets", [])
                if packets:
                    row["firstPTS"] = float(packets[0]["pts_time"])
                    row["firstDTS"] = float(packets[0]["dts_time"])
                    row["firstKeyframe"] = "K" in packets[0]["flags"]
                actual = first_frames(output)
                row["firstThreeSourceFramesMatch"] = actual["exitStatus"] == 0 and actual["frames"] == 3 and actual == reference
                row["firstThreeFrames"] = actual
        rows.append(row)
    return {"sourceSHA256": hashlib.sha256(source.read_bytes()).hexdigest(),
        "sourceKeySeconds": key, "referenceOffsetSeconds": max(0, key - 0.000001), "reference": reference, "candidates": rows}
