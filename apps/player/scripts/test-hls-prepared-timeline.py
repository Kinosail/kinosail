#!/usr/bin/env python3
"""Test first: real authenticated prepared copied-video HLS timeline/continuation.

A 15s first GOP exceeds the 8s warm target but cannot establish the configured
2s segment cadence. Stop/join preparation before any HLS GET can adopt it.
Protect full timeline, future URLs, real decoded continuation, unchanged source,
same init, admission bounds, and unauthenticated rejection. No playlist mocks.
"""
import hashlib
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


def fixture(name, gop, keys):
    path = RUN / (name + ".mkv")
    command = ["ffmpeg", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "testsrc2=s=640x360:r=24:d=96",
        "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=96",
        "-c:v", "libx264", "-threads", "2", "-preset", "veryfast", "-crf", "32", "-pix_fmt", "yuv420p",
        "-g", str(gop), "-keyint_min", "1", "-sc_threshold", "0", "-force_key_frames", keys,
        "-c:a", "aac", "-ac", "2", str(path)]
    subprocess.run(command, check=True, timeout=120, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    probe = subprocess.check_output(["ffprobe", "-v", "error", "-select_streams", "v:0", "-skip_frame", "nokey",
        "-show_entries", "frame=best_effort_timestamp_time:format=duration:stream=avg_frame_rate",
        "-of", "json", str(path)], timeout=30)
    facts = json.loads(probe)
    times = [float(f["best_effort_timestamp_time"]) for f in facts["frames"] if "best_effort_timestamp_time" in f]
    duration = float(facts["format"]["duration"])
    check(facts["streams"][0]["avg_frame_rate"] == "24/1", "fixture_frame_rate")
    expected = [float(v) for v in keys.split(",")]
    check(len(times) == len(expected) and all(abs(a-b) < 0.05 for a, b in zip(times, expected)), "fixture_keyframes")
    check(abs(duration - 96) < 0.1, "fixture_duration")
    return path, {"command": command, "sha256": sha(path), "durationSeconds": duration, "frameRate": 24, "keyframesSeconds": times}


def encoder_count(server):
    rows = subprocess.check_output(["ps", "-eo", "ppid=,args="], text=True, timeout=5).splitlines()
    return sum(1 for row in rows if row.strip() and row.strip().split(maxsplit=1)[0] == str(server.pid)
        and "ffmpeg" in row and "-hls_time" in row)


def sample_resources(server, stop, resources):
    while not stop.is_set():
        try:
            count = encoder_count(server)
            resources["samples"] += 1
            resources["peakOwnedFFmpeg"] = max(resources["peakOwnedFFmpeg"], count)
        except (OSError, subprocess.SubprocessError):
            resources["samplingErrors"] += 1
        stop.wait(0.05)


def manifest_facts(data):
    text = data.decode("utf-8")
    lengths = [float(v) for v in re.findall(r"^#EXTINF:([0-9.]+),", text, re.M)]
    segments = re.findall(r"^segment-[0-9]{5}\.m4s$", text, re.M)
    check(len(lengths) == len(segments) and bool(segments), "variant_segment_shape")
    return {"sha256": hashlib.sha256(data).hexdigest(), "playlistType": "VOD" if "#EXT-X-PLAYLIST-TYPE:VOD" in text else "EVENT",
        "endlist": "#EXT-X-ENDLIST" in text, "durationSeconds": sum(lengths),
        "segmentCount": len(segments)}, list(zip(segments, lengths))


def journey(name, original, metadata):
    case = {"name": name, "result": "failed", "fixture": metadata, "failures": []}
    receipt["cases"].append(case)
    directory, media = RUN / name, RUN / name / "media"
    media.mkdir(parents=True)
    source = media / "Fixture.mkv"
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
        sampler = threading.Thread(target=sample_resources, args=(server, stop, resources), daemon=True)
        sampler.start()
        try:
            api.authorize()
            items = api.call("/api/v1/library")["items"]
            item_id = next(item["id"] for item in items if item["title"] == "Fixture")
            plan = api.call("/api/v1/items/" + item_id + "/playback?videoCodecs=h264")
            case["mode"] = plan["compatiblePlan"]["mode"]
            check(case["mode"] == "remux", "fixture_must_remux")
            case["logicalDurationSeconds"] = plan["duration"]
            check(abs(plan["duration"] - 96) < 0.1, "plan_duration")
            hls = plan["compatible"]
            check(re.fullmatch(r"/hls/[a-f0-9]{16}/p/r-[a-zA-Z0-9-]+/index\.m3u8", hls) is not None, "planned_remux_route")
            cache = directory / "cache"
            prepare = "/api/v1/items/" + item_id + "/playback-prepare"
            status, _, _ = api.http(prepare, "POST", {"source": hls}, authenticated=False)
            check(status == 401, "unauthenticated_preparation_denial")
            check(not cache.exists() or not list(cache.iterdir()), "unauthenticated_cache_effect")
            case["unauthenticatedPreparation"] = {"status": status, "cacheUnchanged": True}
            limit = time.monotonic() + 25
            while time.monotonic() < limit:
                value = api.call(prepare, "POST", {"source": hls}, 202)
                if value.get("state") == "ready":
                    break
                time.sleep(0.05)
            check(value.get("state") == "ready", "preparation_not_ready")
            case["preparationReady"] = True
            roots = [p for p in cache.iterdir() if p.name.startswith(item_id + "-plan-")]
            check(len(roots) == 1, "one_exact_recipe_cache")
            root = roots[0]
            limit, stopped_samples = time.monotonic() + 10, 0
            while time.monotonic() < limit:
                stopped = encoder_count(server) == 0 and (root / ".seekable").exists() and (root / ".startup").exists()
                stopped_samples = stopped_samples + 1 if stopped else 0
                if stopped_samples >= 3:
                    break
                time.sleep(0.05)
            check(stopped_samples >= 3 and encoder_count(server) == 0 and (root / ".seekable").exists()
                and (root / ".startup").exists(), "prepared_worker_not_joined")
            physical = list(root.glob("*p/index.m3u8"))
            check(len(physical) == 1, "single_prepared_rendition")
            prepared_facts, _ = manifest_facts(physical[0].read_bytes())
            prepared_init = (physical[0].parent / "init.mp4").read_bytes()
            check(0 < len(prepared_init) <= 2 * 1024 * 1024, "prepared_initialization_bound")
            case["preparedInitSHA256"] = hashlib.sha256(prepared_init).hexdigest()
            case["beforeFirstHLSGET"] = {"ownedFFmpeg": 0, "seekableMarker": True, "startupMarker": True,
                "authenticatedMediaGETs": 0, "stoppedSamples": stopped_samples, "physicalVariant": prepared_facts}
            status, master, _ = api.http(hls)
            check(status == 200, "master_http_" + str(status))
            renditions = re.findall(r"^[1-9][0-9]{2,3}p/index\.m3u8$", master.decode(), re.M)
            check(len(renditions) == 1, "single_remux_rendition")
            base = hls.removesuffix("index.m3u8") + renditions[0].removesuffix("index.m3u8")
            status, variant, _ = api.http(base + "index.m3u8")
            check(status == 200, "variant_http_" + str(status))
            facts, segments = manifest_facts(variant)
            case["publicVariant"] = facts
            if facts["playlistType"] != "VOD" or not facts["endlist"] or abs(facts["durationSeconds"] - plan["duration"]) > 0.1:
                case["failures"].append("public_full_timeline")
            elapsed, selected = 0.0, []
            for filename, duration in segments:
                selected.append(filename)
                elapsed += duration
                if elapsed >= 26:
                    break
            case["advertisedContinuationSeconds"] = elapsed
            if elapsed < 26:
                case["failures"].append("future_uri_missing")
            fragments = []
            for filename in ["init.mp4"] + selected:
                status, data, _ = api.http(base + filename)
                check(status == 200 and len(data) > 0, "public_fragment_http_" + str(status))
                fragments.append(data)
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
            if result.returncode or case["decodedFrames"] < 600:
                case["failures"].append("decoded_continuation")
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
    for name, gop, keys in [("control2s", 48, controls), ("irregular15s", 2400, "0,15,36,60,84")]:
        source, metadata = fixture(name, gop, keys)
        journey(name, source, metadata)
    if len(receipt["cases"]) == 2 and all(case["result"] == "passed" for case in receipt["cases"]):
        receipt["result"] = "passed"
except Exception as error:
    receipt["failureClass"] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
finally:
    target = RUN / "receipt.json"
    target.write_text(json.dumps(receipt, indent=2, allow_nan=False) + "\n")
    files = [Path(__file__), Path(__file__).with_name("hls_timeline_http.py")]
    checksums = {str(p.relative_to(ROOT)): sha(p) for p in files}
    checksums["receipt.json"] = sha(target)
    (RUN / "SHA256SUMS").write_text("".join(f"{v}  {k}\n" for k, v in checksums.items()))
    print(json.dumps({"result": receipt["result"], "receiptSHA256": checksums["receipt.json"],
        "cases": [{k: c.get(k) for k in ["name", "result", "failureClass", "failures", "mode",
        "publicVariant", "decodedFrames", "beforeFirstHLSGET", "sourceUnchanged", "resources"]} for c in receipt["cases"]]}))
raise SystemExit(0 if receipt["result"] == "passed" else 1)
