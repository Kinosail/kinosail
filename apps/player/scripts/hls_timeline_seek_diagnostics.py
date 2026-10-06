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
    probe = subprocess.run(["ffprobe", "-v", "error", "-select_streams", "v:0", "-read_intervals", "%+#4096",
        "-show_packets", "-show_entries", "packet=pts_time,dts_time,flags", "-of", "json", str(source)],
        capture_output=True, timeout=30)
    if probe.returncode or len(probe.stdout) > 2 * 1024 * 1024:
        raise RuntimeError("seek_source_packet_bound")
    packets = json.loads(probe.stdout).get("packets", [])
    selected = next(p for p in packets if "K" in p.get("flags", "") and abs(float(p["pts_time"]) - key) < 0.000002)
    dts = float(selected["dts_time"])
    cutoff = format(dts - 0.000001, ".6f")
    candidates = [
        ("legacy", ["-ss", format(key, ".9f")], []),
        ("legacy-prior0", ["-ss", format(key, ".9f")], ["-copypriorss:v", "0"]),
        ("legacy-prior0-all", ["-ss", format(key, ".9f")], ["-copypriorss", "0"]),
        ("output-dts", ["-copyts"], ["-ss", cutoff, "-copypriorss:v", "0"]),
        ("padded-dts", ["-copyts", "-ss", format(key + 0.14, ".9f")], ["-ss", cutoff, "-copypriorss:v", "0"]),
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
    return {"boundaryCertificate": boundary_certificate(source),
        "sourceSHA256": hashlib.sha256(source.read_bytes()).hexdigest(),
        "sourceKeySeconds": key, "sourceKeyDTSSeconds": dts, "referenceOffsetSeconds": max(0, key - 0.000001), "reference": reference, "candidates": rows}

def boundary_certificate(source):
    probe = subprocess.run(["ffprobe", "-v", "error", "-select_streams", "v:0",
        "-show_packets", "-show_entries", "packet=pts_time,flags", "-of", "json", str(source)],
        capture_output=True, timeout=30)
    if probe.returncode or len(probe.stdout) > 2 * 1024 * 1024:
        raise RuntimeError("boundary_probe_bound")
    keys = [float(p["pts_time"]) for p in json.loads(probe.stdout).get("packets", [])
        if "K" in p.get("flags", "")]
    result = subprocess.run(["ffmpeg", "-nostdin", "-v", "error", "-threads", "1", "-copyts",
        "-i", str(source), "-map", "0:v:0", "-an", "-sn", "-dn", "-c:v", "copy", "-copytb", "1",
        "-bsf:v", "filter_units=pass_types=5", "-f", "framehash", "pipe:1"],
        capture_output=True, timeout=30)
    if len(result.stdout) > 1024 * 1024:
        raise RuntimeError("boundary_certificate_bound")
    base, pts, empty = 0, [], 0
    for line in result.stdout.decode().splitlines():
        if line.startswith("#tb 0: "):
            numerator, denominator = line.removeprefix("#tb 0: ").split("/")
            base = int(numerator) / int(denominator)
        if not line or line.startswith("#"):
            continue
        fields = line.split(",")
        if len(fields) != 6 or not base:
            raise RuntimeError("boundary_certificate_shape")
        if int(fields[4]) == 0:
            empty += 1
        else:
            pts.append(int(fields[2]) * base)
    return {"exitStatus": result.returncode, "sourceKeyCount": len(keys),
        "certifiedIDRCount": len(pts), "emptyPackets": empty,
        "allSourceKeysCertified": result.returncode == 0 and len(keys) > 0 and len(keys) == len(pts) and all(abs(a-b) <= 0.001 for a, b in zip(keys, pts))}
