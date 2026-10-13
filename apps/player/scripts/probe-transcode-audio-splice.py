#!/usr/bin/env python3
"""Hosted codec experiment: can bounded real preroll preserve an AAC splice?

This isolated experiment is not production or public HTTP proof. Its closed
fixtures expose missing packets, wrong clocks, silent padding, shifted content,
and unbounded origin replay. It deliberately reads the retained target's timing;
production still needs durable generation metadata before publishing a master.
Run only on the designated hosted runner, with an output directory argument.
"""

import array
import hashlib
import json
import math
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import wave
from fractions import Fraction


def digest(data):
    return hashlib.sha256(data).hexdigest()


class Trial:
    def __init__(self, root, journal, rate):
        self.root = root
        self.journal = journal
        self.rate = rate
        self.commands = []

    def record(self, record):
        with self.journal.open("a") as output:
            output.write(json.dumps(record) + "\n")

    def run(self, arguments, data=None):
        record = {
            "rate": self.rate, "number": len(self.commands), "state": "running",
            "arguments": [str(arg).replace(str(self.root), "<fixture>") for arg in arguments],
        }
        self.commands.append(record)
        self.record(record)
        try:
            result = subprocess.run(arguments, input=data, capture_output=True, timeout=60, check=False)
        except subprocess.TimeoutExpired as error:
            record.update({"state": "timeout", "stdoutSHA256": digest(error.stdout or b""),
                           "stderrSHA256": digest(error.stderr or b"")})
            self.record(record)
            raise
        record.update({
            "state": "completed",
            "exit": result.returncode, "stdoutBytes": len(result.stdout),
            "stdoutSHA256": digest(result.stdout), "stderrBytes": len(result.stderr),
            "stderrSHA256": digest(result.stderr),
        })
        self.record(record)
        if result.returncode or result.stderr:
            raise RuntimeError("codec-command-or-diagnostic")
        return result.stdout

    def source(self, rate):
        path = self.root / "source.wav"
        samples = array.array("h")
        for number in range(rate * 18):
            time = number / rate
            # Two chirps and a varying envelope make a time shift observable.
            signal = (0.22 + 0.04 * math.sin(0.73 * time)) * math.sin(2 * math.pi * (173 * time + 37 * time * time))
            signal += 0.10 * math.sin(2 * math.pi * (613 * time + 11 * time * time))
            samples.append(round(signal * 32767))
        if sys.byteorder != "little":
            samples.byteswap()
        with wave.open(str(path), "wb") as output:
            output.setparams((1, 2, rate, 0, "NONE", "not compressed"))
            output.writeframes(samples.tobytes())
        source = self.root / "source.mkv"
        self.run(["ffmpeg", "-nostdin", "-v", "error", "-f", "lavfi", "-i",
                  "testsrc2=size=320x180:rate=25:duration=18", "-i", str(path),
                  "-c:v", "ffv1", "-threads", "2", "-c:a", "flac", str(source)])
        return source

    def encode(self, source, label, rate, number=0, warm=None, cutoff=None):
        directory = self.root / label
        directory.mkdir()
        args = ["ffmpeg", "-nostdin", "-hide_banner", "-loglevel", "error",
                "-filter_threads", "1", "-filter_complex_threads", "1", "-y"]
        if number:
            args += ["-max_delay", "5000000", "-ss", f"{number * 2:.3f}"]
        args += ["-i", str(source)]
        if warm is not None:
            micros = warm * 1_000_000 // rate
            args += ["-ss", f"{micros // 1_000_000}.{micros % 1_000_000:06d}", "-i", str(source)]
        args += ["-map", "0:v:0", "-map", "1:a:0" if warm is not None else "0:a:0"]
        if warm is not None:
            # Shift after accurate input trimming; do not discard warm-up samples.
            args += ["-af", f"asettb=1/{rate},asetpts=PTS+({warm - number * 2 * rate})"]
        args += ["-sn", "-vf", "scale=w=320:h=-2,format=yuv420p", "-c:v", "libx264",
                 "-preset", "veryfast", "-crf", "22", "-fpsmax", "60", "-threads:v", "2",
                 "-profile:v", "high", "-level:v", "4.2", "-maxrate", "500k", "-bufsize", "500k",
                 "-c:a", "aac", "-ac", "2", "-b:a", "128k", "-flags", "+cgop",
                 "-force_key_frames", "expr:gte(t,n_forced*2)", "-avoid_negative_ts", "disabled"]
        if number:
            args += ["-output_ts_offset", f"{number * 2:.3f}"]
        if cutoff is not None:
            args += ["-bsf:a", f"noise=amount=0:drop=lt(pts\\,{cutoff.numerator}/{cutoff.denominator})"]
        args += ["-f", "hls", "-hls_time", "2", "-hls_playlist_type", "event",
                 "-hls_segment_type", "fmp4", "-hls_segment_options", "movflags=+frag_discont+skip_sidx",
                 "-hls_flags", "temp_file+independent_segments", "-hls_fmp4_init_filename", "init.mp4",
                 "-start_number", str(number), "-hls_segment_filename", str(directory / "segment-%05d.m4s"),
                 str(directory / "index.m3u8")]
        self.run(args)
        return directory

    def probe(self, media, stream):
        output = self.run(["ffprobe", "-v", "error", "-select_streams", stream,
                           "-show_entries", "stream=time_base,sample_rate:packet=pts,dts,duration",
                           "-of", "json", "pipe:0"], media)
        facts = json.loads(output)
        if len(facts["streams"]) != 1 or not 0 < len(facts["packets"]) <= 1024:
            raise RuntimeError("packet-projection-outside-fixture-bounds")
        return facts

    def pcm(self, media):
        raw = self.run(["ffmpeg", "-nostdin", "-v", "error", "-i", "pipe:0", "-map", "0:a:0",
                        "-c:a", "pcm_s16le", "-f", "s16le", "-"], media)
        values = array.array("h")
        values.frombytes(raw)
        if sys.byteorder != "little":
            values.byteswap()
        return values

    def input_seek(self, source, rate, warm):
        micros = warm * 1_000_000 // rate
        raw = self.run(["ffmpeg", "-nostdin", "-v", "error", "-ss",
                        f"{micros // 1_000_000}.{micros % 1_000_000:06d}", "-i", str(source),
                        "-map", "0:a:0", "-t", "0.1", "-ac", "1", "-c:a", "pcm_s16le", "-f", "s16le", "-"])
        actual = array.array("h")
        actual.frombytes(raw)
        with wave.open(str(self.root / "source.wav"), "rb") as original:
            lower = max(0, warm - 64)
            original.setpos(lower)
            reference = array.array("h")
            reference.frombytes(original.readframes(len(actual) + 128))
        if sys.byteorder != "little":
            actual.byteswap()
            reference.byteswap()
        offsets = [shift for shift in range(-64, 65) if warm + shift >= lower
                   and reference[warm + shift - lower:warm + shift - lower + len(actual)] == actual]
        # Diagnostic only: never realign or relax the fixed splice acceptance.
        return {"requestedSample": warm, "decodedSamples": len(actual), "exactMatchingOffsets": offsets,
                "matchesRequestedContent": offsets == [0], "decodedSHA256": digest(raw)}

    def frames(self, media):
        raw = self.run(["ffmpeg", "-nostdin", "-v", "error", "-copyts", "-i", "pipe:0", "-map", "0:v:0",
                        "-enc_time_base", "demux", "-fps_mode", "passthrough", "-avoid_negative_ts", "disabled",
                        "-f", "framehash", "-hash", "sha256", "-"], media)
        lines = raw.decode().splitlines()
        bases = [Fraction(line.split(":", 1)[1].strip()) for line in lines if line.startswith("#tb 0:")]
        clocks = [tuple(int(field) for field in line.split(",")[1:4])
                  for line in lines if line and not line.startswith("#")]
        if len(bases) != 1 or len(clocks) != 150:
            raise RuntimeError("video-control-count-or-timebase")
        step = Fraction(1, 25) / bases[0]
        if step.denominator != 1 or any(clock != (clocks[0][0] + index * step, clocks[0][0] + index * step, step)
                                         for index, clock in enumerate(clocks)):
            raise RuntimeError("video-clock-not-contiguous")
        return str(bases[0]), clocks


def window_metrics(original, actual):
    if len(original) != len(actual) or not original:
        return {"equalLength": False, "accepted": False}
    energy = sum(value * value for value in original)
    other = sum(value * value for value in actual)
    correlation = sum(left * right for left, right in zip(original, actual)) / math.sqrt(max(1, energy * other))
    error = math.sqrt(sum((left - right) ** 2 for left, right in zip(original, actual)) / max(1, energy))
    return {"equalLength": True, "correlation": correlation, "normalizedRMSError": error,
            "accepted": correlation >= 0.995 and error <= 0.1}


def case(trial, source, original, rate, number):
    initialization = (original / "init.mp4").read_bytes()
    retained = [(original / f"segment-{index:05d}.m4s").read_bytes() for index in range(number - 1, number + 2)]
    target = initialization + retained[1]
    video = trial.probe(target, "v:0")
    audio = trial.probe(target, "a:0")
    if audio["streams"][0].get("sample_rate") != str(rate) or audio["streams"][0].get("time_base") != f"1/{rate}":
        raise RuntimeError("declared-audio-clock-mismatch")
    boundary = int(video["packets"][0]["dts"]) * Fraction(video["streams"][0]["time_base"]) * rate
    phase = int(audio["packets"][0]["pts"]) % 1024
    limit = max(0, (number * 2 - 3) * rate)
    warm = max(0, limit - (limit - phase) % 1024)
    input_seek = trial.input_seek(source, rate, warm)
    candidate = trial.encode(source, f"refill-{number}", rate, number, warm, boundary - number * 2 * rate)
    refill = (candidate / f"segment-{number:05d}.m4s").read_bytes()
    control = initialization + b"".join(retained)
    mixed = initialization + retained[0] + refill + retained[2]
    expected_packets = trial.probe(control, "a:0")
    actual_packets = trial.probe(mixed, "a:0")
    original_pcm, mixed_pcm = trial.pcm(control), trial.pcm(mixed)
    first = int(expected_packets["packets"][0]["pts"])
    joins = []
    for label, tick in [("entry", int(audio["packets"][0]["pts"])),
                        ("exit", int(trial.probe(initialization + retained[2], "a:0")["packets"][0]["pts"]))]:
        center, radius = (tick - first) * 2, rate // 5
        left = original_pcm[center - radius:center + radius]
        right = mixed_pcm[center - radius:center + radius]
        joins.append({"join": label, **window_metrics(left, right),
                      "silenceRejected": not window_metrics(left, [0] * len(left))["accepted"],
                      "shiftRejected": not window_metrics(left, original_pcm[center - radius + 6144:center + radius + 6144])["accepted"]})
    bounded = number * 2 - warm / rate <= 3 + 1024 / rate and (number < 2 or warm > 0)
    result = {"rate": rate, "targetSeconds": number * 2, "warmStartSamples": warm,
              "warmStartMicros": warm * 1_000_000 // rate, "boundedPreroll": bounded,
              "inputSeekDiagnostic": input_seek,
              "videoDTSBoundary": str(boundary), "phase": phase,
              "packetClocksEqual": expected_packets == actual_packets,
              "originalPacketClock": expected_packets, "mixedPacketClock": actual_packets,
              "videoFrameClocksEqual": trial.frames(control) == trial.frames(mixed),
              "pcmLengthEqual": len(original_pcm) == len(mixed_pcm),
              "originalPCMSamples": len(original_pcm), "mixedPCMSamples": len(mixed_pcm),
              "originalFirstPacket": expected_packets["packets"][0],
              "originalTargetFirstPacket": audio["packets"][0],
              "refilledTargetFirstPacket": trial.probe(initialization + refill, "a:0")["packets"][0],
              "joins": joins, "sourceSHA256": digest(source.read_bytes()),
              "retainedSHA256": [digest(data) for data in retained], "refillSHA256": digest(refill)}
    result["accepted"] = all(result[key] for key in ("boundedPreroll", "packetClocksEqual", "videoFrameClocksEqual", "pcmLengthEqual")) and all(item["accepted"] and item["silenceRejected"] and item["shiftRejected"] for item in joins)
    return result


def main():
    if os.environ.get("GITHUB_ACTIONS") != "true" or len(sys.argv) != 2:
        raise SystemExit("This closed experiment requires the hosted runner and an evidence directory")
    output = Path(sys.argv[1])
    output.mkdir(parents=True, exist_ok=False)
    revision = subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip()
    results, commands = [], []
    journal = output / "commands.jsonl"
    journal.touch()
    codecs = {name: {"resolvedPath": str(Path(shutil.which(name)).resolve()),
                     "binarySHA256": digest(Path(shutil.which(name)).read_bytes())}
              for name in ("ffmpeg", "ffprobe")}
    receipt = {"revision": revision, "complete": False, "codecs": codecs,
               "boundary": "isolated codec experiment; not public HTTP or production repair proof",
               "scriptSHA256": digest(Path(__file__).read_bytes()), "results": results, "commands": commands}

    def checkpoint():
        (output / "receipt.json").write_text(json.dumps(receipt, indent=2) + "\n")
        (output / "SHA256SUMS").write_text("".join(f"{digest((output / name).read_bytes())}  {name}\n"
                                                  for name in ("receipt.json", "commands.jsonl")))

    checkpoint()
    for rate in (48000, 44100):
        with tempfile.TemporaryDirectory(prefix="transcode-audio-trial-") as temporary:
            trial = Trial(Path(temporary), journal, rate)
            try:
                source = trial.source(rate)
                original = trial.encode(source, "original", rate)
                for number in (1, 7):
                    try:
                        results.append(case(trial, source, original, rate, number))
                    except (RuntimeError, ValueError, KeyError, OSError, subprocess.SubprocessError) as error:
                        results.append({"rate": rate, "number": number, "accepted": False, "errorClass": type(error).__name__})
                    checkpoint()
            except (RuntimeError, ValueError, KeyError, OSError, subprocess.SubprocessError) as error:
                results.append({"rate": rate, "accepted": False, "errorClass": type(error).__name__})
            finally:
                commands.extend(trial.commands)
                checkpoint()
    receipt["complete"] = True
    checkpoint()
    print(json.dumps({"revision": revision, "cases": len(results), "accepted": sum(item["accepted"] for item in results)}))
    return 0 if len(results) == 4 and all(item["accepted"] for item in results) else 1


if __name__ == "__main__":
    sys.exit(main())
