"""Closed FLAC fixture clock certification for the hosted AAC experiment only."""

import array
import hashlib
import json
import sys
import wave


class FixtureClockError(ValueError):
    """A fixed safe fixture admission failure, never codec diagnostic text."""


def certify(facts, rate):
    streams, frames = facts.get("streams", []), facts.get("frames", [])
    expected = {"codec_name": "flac", "time_base": "1/1000", "sample_rate": str(rate), "start_pts": 0}
    if len(streams) != 1 or any(streams[0].get(key) != value for key, value in expected.items()):
        raise FixtureClockError("fixture-stream-contract")
    if not 3 <= len(frames) <= 1024:
        raise FixtureClockError("fixture-frame-count")
    quantum = frames[0].get("nb_samples", 0)
    uncertainty = 2 * ((rate + 999) // 1000) + 2
    if not 2 * uncertainty < quantum <= 65536:
        raise FixtureClockError("fixture-clock-not-unique")
    samples, low, high = 0, 0, rate - 1
    for number, frame in enumerate(frames):
        count, pts = frame.get("nb_samples", 0), frame.get("pts")
        if type(pts) is not int or type(count) is not int or not 0 < count <= quantum:
            raise FixtureClockError("fixture-frame-shape")
        if number < len(frames) - 1 and count != quantum:
            raise FixtureClockError("fixture-partial-interior-block")
        if any(frame.get(key) != value for key, value in
               {"channels": 1, "channel_layout": "mono", "sample_fmt": "s16"}.items()):
            raise FixtureClockError("fixture-frame-format")
        if frame.get("side_data_list") or number == 0 and pts != 0:
            raise FixtureClockError("fixture-origin-or-trimming")
        whole, remainder = divmod(samples * 1000, rate)
        lower = (pts - whole) * rate - remainder
        low, high = max(low, lower), min(high, lower + rate - 1)
        if low > high:
            raise FixtureClockError("fixture-discontinuous-clock")
        samples += count
    if samples != rate * 18:
        raise FixtureClockError("fixture-incomplete-source")
    return {"blockSamples": quantum, "frames": len(frames), "samples": samples,
            "quantizerPhaseLow": low, "quantizerPhaseHigh": high,
            "maximumRoundingSamples": uncertainty}


def source_clock(trial, source, rate):
    raw = trial.run(["ffprobe", "-v", "error", "-select_streams", "a:0", "-show_streams", "-show_frames",
                     "-show_entries", "stream=codec_name,time_base,sample_rate,start_pts:"
                     "frame=pts,nb_samples,channels,channel_layout,sample_fmt:frame_side_data",
                     "-of", "json", str(source)])
    facts = json.loads(raw)
    clock = certify(facts, rate)
    changed = json.loads(raw)
    for frame in changed["frames"][len(changed["frames"]) // 2:]:
        frame["pts"] += 1
    try:
        certify(changed, rate)
    except ValueError as error:
        if str(error) != "fixture-discontinuous-clock":
            raise
    else:
        raise FixtureClockError("fixture-discontinuity-control-accepted")
    return {**clock, "persistentMillisecondStepRejected": True,
            "decodedFrameProjectionSHA256": hashlib.sha256(raw).hexdigest()}


def seek_text(rate, warm):
    micros = warm * 1_000_000 // rate
    return f"{micros // 1_000_000}.{micros % 1_000_000:06d}"


def seek_filter(clock, rate, warm, target):
    quantum = clock["blockSamples"]
    micros = warm * 1_000_000 // rate
    return (f"asettb=1/{rate},asetpts=round((PTS+{micros}*{rate}/1000000)/{quantum})*{quantum},"
            f"atrim=start_pts={warm},asetpts=PTS-{target}")


def input_seek(trial, source, rate, warm, corrected=False):
    arguments = ["ffmpeg", "-nostdin", "-v", "error", "-ss", seek_text(rate, warm)]
    if corrected:
        arguments += ["-noaccurate_seek"]
    arguments += ["-i", str(source), "-map", "0:a:0"]
    if corrected:
        arguments += ["-af", seek_filter(trial.clock, rate, warm, warm)]
    raw = trial.run(arguments + ["-t", "0.1", "-ac", "1", "-c:a", "pcm_s16le", "-f", "s16le", "-"])
    actual = array.array("h")
    actual.frombytes(raw)
    with wave.open(str(trial.root / "source.wav"), "rb") as original:
        lower = max(0, warm - 64)
        original.setpos(lower)
        reference = array.array("h")
        reference.frombytes(original.readframes(len(actual) + 128))
    if sys.byteorder != "little":
        actual.byteswap()
        reference.byteswap()
    offsets = [shift for shift in range(-64, 65) if warm + shift >= lower
               and reference[warm + shift - lower:warm + shift - lower + len(actual)] == actual]
    return {"requestedSample": warm, "decodedSamples": len(actual), "exactMatchingOffsets": offsets,
            "matchesRequestedContent": len(actual) == rate // 10 and offsets == [0],
            "decodedSHA256": hashlib.sha256(raw).hexdigest()}
