#!/usr/bin/env python3
"""Test first: real authenticated prepared copied-video HLS timeline/continuation.

A 15s first GOP exceeds the 8s warm target but cannot establish the configured
2s segment cadence. Stop/join preparation before any HLS GET can adopt it.
Protect full timeline, every future URL, exact decoded source frames, unchanged source,
same init, admission bounds, and unauthenticated rejection. No playlist mocks.
"""
import hashlib
import itertools
import json
import os
from pathlib import Path
import re
import shutil
import socket
import subprocess
import threading
import time
from hls_timeline_http import PublicServer, sha, source_state
from hls_timeline_packets import fragment_packets, decoded_identity, safe_seek_phases, manifest_facts
from hls_timeline_fixture import fixture
from hls_timeline_preparation import prepare_scene
from hls_timeline_seek_diagnostics import seek_diagnostics

ROOT = Path(__file__).resolve().parents[3]
RUN = ROOT / ".verification/hls-prepared-timeline" / time.strftime("%Y%m%dT%H%M%SZ", time.gmtime())
RUN.mkdir(parents=True)
revision = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
binary = RUN / "player"
receipt = {"revision": revision, "result": "failed", "cases": [],
    "command": "python3 apps/player/scripts/test-hls-prepared-timeline.py",
    "boundary": "Real public Server/FFmpeg synthetic remux proof; Safari/iOS/audio acceptance separate.",
    "environment": "Disposable hosted runner; supported authenticated loopback HTTP; serial bounded encoding",
    "productionMediaOrCacheModified": False, "sourceMutationPolicy": "Application requests preserve synthetic source bytes/size/mtime."}
build = ["go", "-C", "apps/player", "build", "-p=1", "-o", str(binary), "./cmd/kinosail"]


def check(condition, failure):
    if not condition:
        raise RuntimeError(failure)


def bounded_bytes(path, limit, failure):
    with path.open("rb") as file:
        data = file.read(limit + 1)
    check(0 < len(data) <= limit, failure)
    return data


def encoder_count(server, source):
    rows = subprocess.check_output(["ps", "-eo", "ppid=,args="], text=True, timeout=5).splitlines()
    return sum(1 for row in rows if row.strip() and row.strip().split(maxsplit=1)[0] == str(server.pid)
        and "ffmpeg" in row and "-hls_time" in row and str(source) in row)


def sample_resources(server, source, stop, resources):
    while not stop.is_set():
        try:
            count = encoder_count(server, source)
            resources["samples"] += 1
            resources["peakOwnedFFmpeg"] = max(resources["peakOwnedFFmpeg"], count)
        except (OSError, subprocess.SubprocessError):
            resources["samplingErrors"] += 1
        stop.wait(0.05)


def item_hls_roots(cache, item_id):
    if not cache.exists():
        return []
    with os.scandir(cache) as stream:
        entries = list(itertools.islice(stream, 4097))
    check(len(entries) <= 4096, "cache_inventory_bound")
    return sorted(p.name for p in entries if p.name == item_id or p.name.startswith(item_id + "-"))


def journey(name, original, metadata, offset=0, cold=False):
    case = {"name": name, "result": "failed", "fixture": metadata, "resumeOffsetSeconds": offset, "failures": []}
    receipt["cases"].append(case)
    directory, media = RUN / name, RUN / name / "media"
    media.mkdir(parents=True)
    source = media / ("Fixture" + original.suffix)
    shutil.copy2(original, source)
    before = source_state(source)
    case["sourceBefore"] = before
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        port = listener.getsockname()[1]
    url = f"http://localhost:{port}"
    env = dict(os.environ, KINOSAIL_LISTEN=f"127.0.0.1:{port}", KINOSAIL_AUTH_URL=url,
        KINOSAIL_TLS_ENABLED="false", KINOSAIL_DATA_DIR=str(directory / "config"),
        KINOSAIL_MEDIA_DIR=str(media), KINOSAIL_CACHE_DIR=str(directory / "cache"),
        KINOSAIL_BACKUP_DIR=str(directory / "backups"), KINOSAIL_BACKUP_KEY="synthetic-timeline-key")
    api = PublicServer(url)
    resources = {"samples": 0, "peakOwnedFFmpeg": 0, "samplingErrors": 0}
    case["resources"] = resources
    with (directory / "server.log").open("w") as log:
        server = subprocess.Popen([str(binary)], cwd=ROOT, env=env, stdout=log, stderr=log)
        stop = threading.Event()
        sampler = threading.Thread(target=sample_resources, args=(server, source, stop, resources), daemon=True)
        sampler.start()
        try:
            api.authorize()
            items = api.call("/api/v1/library")["items"]
            item_id = next(item["id"] for item in items if item["title"] == "Fixture")
            plan = api.call("/api/v1/items/" + item_id + "/playback?videoCodecs=h264")
            case["mode"] = plan["compatiblePlan"]["mode"]
            check(case["mode"] == "remux", "fixture_must_remux")
            case["logicalDurationSeconds"] = plan["duration"]
            file_version = plan.get("media", {}).get("fileVersion", "")
            case["selectedSourceSnapshotMatches"] = file_version == str(before["sizeBytes"]) + ":" + before["mtimeNanoseconds"]
            check(abs(plan["duration"] - metadata["durationSeconds"]) < 0.1, "plan_duration")
            hls = plan["compatible"]
            if offset:
                hls = hls.replace("/index.m3u8", "-o" + str(offset * 1000) + "/index.m3u8")
            logical_duration = plan["duration"] - offset
            case["expectedTimelineSeconds"] = logical_duration
            check(re.fullmatch(r"/hls/[a-f0-9]{16}/p/r-[a-zA-Z0-9-]+/index\.m3u8", hls) is not None, "planned_remux_route")
            cache = directory / "cache"
            prepare = "/api/v1/items/" + item_id + "/playback-prepare"
            baseline_roots = item_hls_roots(cache, item_id)
            check(not baseline_roots and encoder_count(server, source) == 0, "planning_started_target_hls")
            status, _, _ = api.http(prepare, "POST", {"source": hls}, authenticated=False)
            check(status == 401, "unauthenticated_preparation_denial")
            denied_roots = item_hls_roots(cache, item_id)
            check(denied_roots == baseline_roots and encoder_count(server, source) == 0, "unauthenticated_cache_effect")
            case["unauthenticatedPreparation"] = {"status": status, "cacheUnchanged": True,
                "targetHLSRootsBefore": len(baseline_roots), "targetHLSRootsAfter": len(denied_roots), "targetOwnedFFmpeg": 0}
            root, prepared_init = prepare_scene(api, prepare, hls, cache, item_id, server, source, case,
                encoder_count, check, bounded_bytes, cold)
            status, master, _ = api.http(hls)
            check(status == 200, "master_http_" + str(status))
            renditions = re.findall(r"^[1-9][0-9]{2,3}p/index\.m3u8$", master.decode(), re.M)
            check(len(renditions) == 1, "single_remux_rendition")
            base = hls.removesuffix("index.m3u8") + renditions[0].removesuffix("index.m3u8")
            status, variant, _ = api.http(base + "index.m3u8")
            check(status == 200, "variant_http_" + str(status))
            facts, segments = manifest_facts(variant)
            case["publicVariant"] = facts
            if facts["playlistType"] != "VOD" or not facts["endlist"] or abs(facts["durationSeconds"] - logical_duration) > 0.1:
                case["failures"].append("public_full_timeline")
            elapsed, selected = 0.0, []
            for filename, duration in segments:
                selected.append(filename)
                elapsed += duration

            case["advertisedContinuationSeconds"] = elapsed
            if elapsed < logical_duration - 0.1:
                case["failures"].append("future_uri_missing")
            fragments, previous_end = [], None
            case["publicFragments"] = []
            advertised_lengths = dict(segments)
            for filename in ["init.mp4"] + selected:
                status, data, _ = api.http(base + filename)
                if status != 200 or not data:
                    case["failures"].append("public_fragment_http_" + str(status))
                    case["unavailableSegment"] = filename
                    break
                fragments.append(data)
                if filename != "init.mp4":
                    fragment = directory / "fragment-probe.mp4"
                    fragment.write_bytes(fragments[0] + data)
                    packet_facts = fragment_packets(fragment)
                    packet_facts["segment"] = filename
                    packet_facts["advertisedSeconds"] = advertised_lengths[filename]
                    case["publicFragments"].append(packet_facts)
                    if abs(packet_facts["videoSpanSeconds"] - advertised_lengths[filename]) > 0.15:
                        case["failures"].append("fragment_video_duration")
                    independent = decoded_identity(fragment)
                    packet_facts["independentDecodedFrames"] = independent["frames"]
                    if not packet_facts["keyframePackets"] or independent["frames"] != packet_facts["videoPackets"]:
                        case["failures"].append("fragment_independent_decode")
                    if abs(packet_facts["videoPackets"] / metadata["frameRate"] - advertised_lengths[filename]) > 0.15:
                        case["failures"].append("fragment_video_density")
                    if packet_facts["videoPackets"]:
                        if previous_end is not None and abs(packet_facts["firstVideoTime"] - previous_end) > 0.15:
                            case["failures"].append("fragment_video_discontinuity")
                        previous_end = packet_facts["lastVideoEnd"]
            check(fragments[0] == prepared_init, "prepared_initialization_preserved")
            decoded = directory / "public-fragments.mp4"
            decoded.write_bytes(b"".join(fragments))
            result = subprocess.run(["ffmpeg", "-nostdin", "-v", "error", "-xerror", "-threads", "2",
                "-i", str(decoded), "-progress", "pipe:1", "-fps_mode", "passthrough", "-f", "null", "-"],
                capture_output=True, timeout=40)
            frames = [int(v) for v in re.findall(rb"^frame=([0-9]+)$", result.stdout, re.M)]
            case["decodedFrames"] = max(frames, default=0)
            case["decodeExitStatus"] = result.returncode
            case["decodedContinuationSeconds"] = case["decodedFrames"] / metadata["frameRate"]
            if abs(case["decodedContinuationSeconds"] - elapsed) > 0.15:
                case["failures"].append("advertised_continuation_timing")
            expected_frames = metadata["videoFrames"] - round(offset * metadata["frameRate"])
            if result.returncode or case["decodedFrames"] != expected_frames:
                case["failures"].append("decoded_continuation")
            reference, actual = decoded_identity(source, offset), decoded_identity(decoded)
            case["referenceDecodedFrames"] = reference["frames"]
            case["decodedSourceMatches"] = actual == reference
            case["referenceFrameSHA256"] = reference["sha256"]
            case["publicFrameSHA256"] = actual["sha256"]
            if not case["decodedSourceMatches"]:
                case["failures"].append("resume_source_frames")
            status, init, _ = api.http(base + "init.mp4")
            check(status == 200 and init == fragments[0], "same_initialization")
            case["originalInitSHA256"] = hashlib.sha256(init).hexdigest()
            case["workerBound"] = resources["samples"] > 0 and resources["peakOwnedFFmpeg"] == 1 and resources["samplingErrors"] == 0
            check(case["workerBound"], "owned_encoder_bound")
            case["result"] = "passed" if not case["failures"] else "failed"
        except Exception as error:
            case["failureClass"] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
        finally:
            stop.set()
            sampler.join(timeout=5)
            server.terminate()
            try:
                server.wait(timeout=10)
            except subprocess.TimeoutExpired:
                server.kill()
                server.wait()
            log.flush()
            private_log = bounded_bytes(directory / "server.log", 2 * 1024 * 1024, "private_log_bound").decode("utf-8")
            case.update(safe_seek_phases(private_log))
            after = source_state(source)
            case["sourceAfter"] = after
            case["sourceUnchanged"] = before == after
            if before != after:
                case["failures"].append("source_mutated")
                case["result"] = "failed"


try:
    subprocess.run(build, cwd=ROOT, check=True, timeout=180)
    receipt["binarySHA256"] = sha(binary)
    receipt["encoderVersions"] = {tool: subprocess.check_output([tool, "-version"], text=True).splitlines()[0]
        for tool in ["ffmpeg", "ffprobe"]}
    controls = ",".join(str(v) for v in range(0, 96, 2))
    fixtures = [("control2s", 48, controls), ("irregular15s", 2400, "0,15,36,60,84"),
        ("cluster15s", 2400, "0,15,16,18,36,60,84")]
    for name, gop, keys in fixtures:
        source, metadata = fixture(RUN, name, gop, keys)
        if name == "control2s":
            journey("coldControl2s", source, metadata, cold=True)
            receipt["codecSeekDiagnostics"] = [seek_diagnostics(RUN, source, metadata["keyframesSeconds"][4])]
        if name == "cluster15s":
            receipt["codecSeekDiagnostics"].append(seek_diagnostics(RUN, source, metadata["keyframesSeconds"][1]))
        journey(name, source, metadata)
        if name == "cluster15s":
            journey(name + "resume15s", source, metadata, 15)
            journey(name + "resume60s", source, metadata, 60)
    source, metadata = fixture(RUN, "fractional15s", 3000, "0,15.0483,16.0493,18.0513,36.0693,60.0933,84.1173",
        "30000/1001", 2880, ".mp4")
    receipt["codecSeekDiagnostics"].append(seek_diagnostics(RUN, source, metadata["keyframesSeconds"][1]))
    journey("fractional15s", source, metadata)
    if len(receipt["cases"]) == 7 and all(case["result"] == "passed" for case in receipt["cases"]):
        receipt["result"] = "passed"
except Exception as error:
    receipt["failureClass"] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
finally:
    target = RUN / "receipt.json"
    target.write_text(json.dumps(receipt, indent=2, allow_nan=False) + "\n")
    files = [Path(__file__), Path(__file__).with_name("hls_timeline_http.py"),
        Path(__file__).with_name("hls_timeline_packets.py"), Path(__file__).with_name("hls_timeline_fixture.py"),
        Path(__file__).with_name("hls_timeline_preparation.py"), Path(__file__).with_name("hls_timeline_seek_diagnostics.py")]
    checksums = {str(p.relative_to(ROOT)): sha(p) for p in files}
    checksums["receipt.json"] = sha(target)
    (RUN / "SHA256SUMS").write_text("".join(f"{v}  {k}\n" for k, v in checksums.items()))
    print(json.dumps({"result": receipt["result"], "receiptSHA256": checksums["receipt.json"],
        "cases": [{k: c.get(k) for k in ["name", "result", "failureClass", "failures", "mode",
        "publicVariant", "decodedFrames", "beforeFirstHLSGET", "sourceUnchanged", "resources"]} for c in receipt["cases"]]}))
raise SystemExit(0 if receipt["result"] == "passed" else 1)
