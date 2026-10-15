"""Bind finalized HLS cuts to packet byte positions in one joined demux pass."""

from fractions import Fraction
import hashlib
import json


class FragmentClockError(ValueError):
    """A fixed safe timing projection failure."""


def joined(trial, initialization, fragments, rate):
    chunks, intervals, position = [initialization], [], len(initialization)
    for number, fragment in fragments.items():
        if not 0 < len(fragment) <= 8 << 20:
            raise FragmentClockError("fragment-byte-bound")
        intervals.append((number, position, position + len(fragment)))
        position += len(fragment)
        chunks.append(fragment)
    if position > 32 << 20 or not 1 <= len(intervals) <= 9:
        raise FragmentClockError("joined-fixture-bound")
    raw = trial.run(["ffprobe", "-v", "error", "-show_packets", "-show_streams", "-show_entries",
                     "stream=index,codec_type,time_base,sample_rate:packet=stream_index,pts,dts,duration,pos,size",
                     "-of", "json", "pipe:0"], b"".join(chunks))
    facts = json.loads(raw)
    streams = facts.get("streams", [])
    if len(streams) != 2 or {item.get("codec_type") for item in streams} != {"video", "audio"}:
        raise FragmentClockError("joined-stream-contract")
    clocks = {item["index"]: item for item in streams}
    audio = next(item for item in streams if item["codec_type"] == "audio")
    if audio["time_base"] != f"1/{rate}" or audio["sample_rate"] != str(rate):
        raise FragmentClockError("joined-audio-clock")
    packets = facts.get("packets", [])
    if not 1 <= len(packets) <= 2048:
        raise FragmentClockError("joined-packet-count")
    groups = {number: {"video": [], "audio": []} for number in fragments}
    for packet in packets:
        offset, size = int(packet["pos"]), int(packet["size"])
        owners = [number for number, start, end in intervals if start <= offset < end and offset + size <= end]
        if len(owners) != 1 or size <= 0 or packet["stream_index"] not in clocks:
            raise FragmentClockError("joined-packet-ownership")
        record = {field: packet[field] for field in ("pts", "dts", "duration")}
        if any(type(value) is not int for value in record.values()) or record["duration"] <= 0:
            raise FragmentClockError("joined-packet-clock-fields")
        groups[owners[0]][clocks[packet["stream_index"]]["codec_type"]].append(record)
    if any(not packets for group in groups.values() for packets in group.values()):
        raise FragmentClockError("joined-empty-fragment-track")
    return groups, clocks, hashlib.sha256(raw).hexdigest()


def presentation(trial, directory, rate):
    initialization = (directory / "init.mp4").read_bytes()
    fragments = {number: (directory / f"segment-{number:05d}.m4s").read_bytes() for number in range(9)}
    groups, clocks, projection_hash = joined(trial, initialization, fragments, rate)
    video_base = Fraction(next(item["time_base"] for item in clocks.values() if item["codec_type"] == "video"))
    cuts = []
    for number, fragment in fragments.items():
        for kind, selector in (("video", "v:0"), ("audio", "a:0")):
            isolated = trial.probe(initialization + fragment, selector)
            expected_base = video_base if kind == "video" else Fraction(1, rate)
            if Fraction(isolated["streams"][0]["time_base"]) != expected_base or isolated["packets"] != groups[number][kind]:
                raise FragmentClockError("joined-isolated-clock-disagreement")
        video, audio = groups[number]["video"], groups[number]["audio"]
        cuts.append({"number": number, "ptsSeconds": str(video[0]["pts"] * video_base),
                     "dtsSeconds": str(video[0]["dts"] * video_base), "firstAudioPTS": audio[0]["pts"],
                     "videoPackets": len(video), "audioPackets": len(audio)})
    sparse = {number: fragment for number, fragment in fragments.items() if number != 4}
    sparse_groups, sparse_clocks, sparse_hash = joined(trial, initialization, sparse, rate)
    if sparse_clocks != clocks or any(sparse_groups[number] != groups[number] for number in sparse):
        raise FragmentClockError("joined-missing-interval-retimed-neighbors")
    first = groups[0]["audio"][0]["pts"]
    if any(packet["pts"] % 1024 != first % 1024 for group in groups.values() for packet in group["audio"]):
        raise FragmentClockError("joined-audio-phase-change")
    return {"source": "finalized-fragment-byte-positions", "fragmentClocksVerified": len(cuts), "cuts": cuts,
            "audioPhase": first % 1024, "missingIntervalPreserved": True, "missingInterval": 4,
            "joinedProjectionSHA256": projection_hash, "sparseProjectionSHA256": sparse_hash}
