#!/usr/bin/env python3
"""Public interrupted-cold cache preservation and damaged indexed cache recovery.

The cold cases protect legacy admission, init and existing fragment identity;
they do not certify lazy continuation or sparse cold source cuts. Real FFmpeg is
rate-limited only to make public stream abandonment deterministic, never replaced by output mocks.
Only this harness damages its disposable indexed cache.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import socket
import subprocess
import time
from hls_timeline_fixture import fixture
from hls_timeline_http import PublicServer, sha, source_state
from hls_timeline_packets import manifest_facts, safe_encoder_lifecycle
from hls_timeline_seek_diagnostics import first_frames

ROOT = Path(__file__).resolve().parents[3]
RUN = ROOT / ".verification/hls-cache-reopen" / time.strftime("%Y%m%dT%H%M%SZ", time.gmtime())
RUN.mkdir(parents=True)
BINARY = RUN / "player"
receipt = {"revision": subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip(),
    "command": "python3 apps/player/scripts/test-hls-cache-reopen.py", "result": "failed", "cases": [],
    "environment": "Disposable hosted loopback Server; authenticated real FFmpeg; serial synthetic H264/HEVC AAC MKV",
    "boundary": "Cold cache/prefix preservation only; lazy continuation correctness remains separate.",
    "productionMediaOrCacheModified": False}


def check(condition, failure):
    if not condition:
        raise RuntimeError(failure)


def wait_until(condition, failure, seconds=15):
    limit = time.monotonic() + seconds
    while time.monotonic() < limit:
        if condition():
            return
        time.sleep(0.05)
    raise RuntimeError(failure)


def read(path, limit=2 * 1024 * 1024):
    with path.open("rb") as file:
        data = file.read(limit + 1)
    check(0 < len(data) <= limit, "cache_file_bound")
    return data


def snapshot(path):
    stat = path.stat()
    return {"inode": stat.st_ino, "size": stat.st_size, "mtimeNs": str(stat.st_mtime_ns), "sha256": sha(path)}


def owned_encoder(server):
    rows = subprocess.check_output(["ps", "-eo", "ppid=,args="], text=True, timeout=5).splitlines()
    return sum(row.strip().split(maxsplit=1)[0] == str(server.pid) and "-hls_time" in row for row in rows if row.strip())


def cold_scene(api, hls, item_id, cache, server, case):
    case["stage"] = "cold-master"
    check(api.http(hls + "?playbackSession=cold-reopen-proof")[0] == 200, "cold_master")
    roots = list(cache.glob(item_id + "-plan-*"))
    check(len(roots) == 1, "one_cold_cache")
    root = roots[0]
    variants = list(root.glob("*p/index.m3u8"))
    check(len(variants) == 1, "one_cold_variant")
    physical = variants[0]
    wait_until(lambda: b"segment-00000.m4s" in read(physical), "cold_prefix_missing")
    check(b"#EXT-X-ENDLIST" not in read(physical) and owned_encoder(server) == 1, "cold_not_interrupted")
    # Abandon the public stream. Its documented 45-second inactivity watchdog
    # cancels the owner without requiring an optional Jellyfin integration.
    case["interruption"] = "Public cold HLS request abandoned; 45-second owner inactivity watchdog"
    case["stage"] = "cold-idle-join"
    wait_until(lambda: owned_encoder(server) == 0 and (root / ".seekable").exists(), "cold_stop_not_joined", seconds=50)
    # Let the owner finish publishing its pause receipt and remove the joined job.
    for _ in range(3):
        time.sleep(0.05)
        check(owned_encoder(server) == 0, "cold_stop_restarted")
    check(not (root / ".copy-timeline").exists() and not (root / ".startup").exists(), "cold_was_indexed_or_prepared")
    _, segments = manifest_facts(read(physical))
    check(len(segments) < 16 and b"#EXT-X-ENDLIST" not in read(physical), "cold_stop_completed")
    paths = [physical.parent / "init.mp4", physical.parent / segments[0][0]]
    before = [snapshot(path) for path in paths]
    case["interruptedPrefixSegments"] = len(segments)
    case["unindexedSeekableCache"] = True
    return root, paths, before


def corrupt_scene(api, hls, item_id, cache, server, case, damage):
    prepare = "/api/v1/items/" + item_id + "/playback-prepare"
    def ready():
        return api.call(prepare, "POST", {"source": hls}, 202).get("state") == "ready"
    wait_until(ready, "indexed_preparation_not_ready", 25)
    roots = list(cache.glob(item_id + "-plan-*"))
    check(len(roots) == 1, "one_indexed_cache")
    root = roots[0]
    physical = list(root.glob("*p/index.m3u8"))[0]
    wait_until(lambda: owned_encoder(server) == 0 and b"#EXT-X-ENDLIST" in read(physical), "indexed_not_complete")
    check((root / ".copy-timeline").is_file(), "indexed_map_missing")
    case["completedIndexedCacheBeforeDamage"] = True
    target = root / ".copy-timeline"
    valid = read(target)
    if damage == "unknown":
        target.write_bytes(valid[:-1] + b',"Unknown":true}')
    elif damage == "duplicate":
        target.write_bytes(valid[:-1] + b',"clock":0.5}')
    else:
        target.write_bytes(b"{damaged-map")
    case["mapDamage"] = damage
    return root


def journey(name, original, metadata, corrupt=False):
    case = {"name": name, "fixture": metadata, "result": "failed"}
    receipt["cases"].append(case)
    directory, media = RUN / name, RUN / name / "media"
    media.mkdir(parents=True)
    source = media / "Fixture.mkv"
    shutil.copy2(original, source)
    before = source_state(source)
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        port = listener.getsockname()[1]
    api = PublicServer(f"http://localhost:{port}")
    env = dict(os.environ, KINOSAIL_LISTEN=f"127.0.0.1:{port}", KINOSAIL_AUTH_URL=api.url,
        KINOSAIL_TLS_ENABLED="false", KINOSAIL_DATA_DIR=str(directory / "config"),
        KINOSAIL_MEDIA_DIR=str(media), KINOSAIL_CACHE_DIR=str(directory / "cache"),
        KINOSAIL_BACKUP_DIR=str(directory / "backups"), KINOSAIL_BACKUP_KEY="synthetic-reopen-key")
    if not corrupt:
        # Installation-supported executable seam; exec preserves the Server child PID.
        real = shutil.which("ffmpeg")
        check(real is not None, "ffmpeg_unavailable")
        wrapper = directory / "paced-ffmpeg"
        wrapper.write_text("#!/usr/bin/env python3\nimport os,sys\na=sys.argv[1:]\n"
            "if '-hls_time' in a:\n i=a.index('-i'); a[i:i]=['-readrate','0.2']\n"
            + "os.execv(" + repr(real) + ", [" + repr(real) + "]+a)\n")
        wrapper.chmod(0o700)
        env["KINOSAIL_FFMPEG"] = str(wrapper)
        case["coldPacingReadrate"] = 0.2
        case["pacingWrapperSHA256"] = sha(wrapper)
    with (directory / "server.log").open("w") as log:
        server = subprocess.Popen([str(BINARY)], cwd=ROOT, env=env, stdout=log, stderr=log)
        try:
            api.authorize()
            item_id = next(item["id"] for item in api.call("/api/v1/library")["items"] if item["title"] == "Fixture")
            plan = api.call("/api/v1/items/" + item_id + "/playback?videoCodecs=h264,hevc")
            check(plan["compatiblePlan"]["mode"] == "remux", "fixture_must_remux")
            hls, cache = plan["compatible"], directory / "cache"
            if corrupt:
                root = corrupt_scene(api, hls, item_id, cache, server, case, corrupt)
            else:
                root, paths, prefix_before = cold_scene(api, hls, item_id, cache, server, case)
            case["stage"] = "reopen"
            status, master, _ = api.http(hls)
            check(status == 200, "reopen_master_http_" + str(status))
            renditions = re.findall(rb"^[1-9][0-9]{2,3}p/index\.m3u8$", master, re.M)
            check(len(renditions) == 1, "one_reopen_rendition")
            base = hls.removesuffix("index.m3u8") + renditions[0].decode().removesuffix("index.m3u8")
            status, variant, _ = api.http(base + "index.m3u8")
            check(status == 200, "reopen_variant_http_" + str(status))
            facts, segments = manifest_facts(variant)
            case["publicVariant"] = facts
            fragments = []
            for name in ["init.mp4", segments[0][0]]:
                status, data, _ = api.http(base + name)
                check(status == 200 and data, "reopen_prefix_http_" + str(status))
                fragments.append(data)
            prefix = directory / "public-prefix.mp4"
            prefix.write_bytes(b"".join(fragments))
            reference = first_frames(source)
            check(reference["exitStatus"] == 0 and reference["frames"] == 3, "reopen_reference_three_frames")
            case["firstThreeSourceFramesMatch"] = first_frames(prefix) == reference
            check(case["firstThreeSourceFramesMatch"], "reopen_prefix_source_mismatch")
            if corrupt:
                case["damagedMapRecovered"] = not (root / ".copy-timeline").exists()
                check(case["damagedMapRecovered"], "damaged_map_not_recovered")
            else:
                case["cachedPrefixPreserved"] = [snapshot(path) for path in paths] == prefix_before
                case["publicInitSHA256"] = hashlib.sha256(fragments[0]).hexdigest()
                case["publicFragmentSHA256"] = hashlib.sha256(fragments[1]).hexdigest()
                check(case["cachedPrefixPreserved"] and case["publicInitSHA256"] == prefix_before[0]["sha256"]
                    and case["publicFragmentSHA256"] == prefix_before[1]["sha256"], "cold_cache_replaced")
                check(owned_encoder(server) == 0, "cold_reopen_started_encoder")
                check(facts["segmentCount"] > case["interruptedPrefixSegments"], "legacy_cadence_projection_lost")
            case["result"] = "passed"
        except Exception as error:
            case["failureClass"] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
        finally:
            server.terminate()
            try:
                server.wait(timeout=10)
            except subprocess.TimeoutExpired:
                server.kill()
                server.wait()
            log.flush()
            lifecycle = safe_encoder_lifecycle(read(directory / "server.log").decode())
            case["encoderLifecycle"] = lifecycle
            case["sourceUnchanged"] = source_state(source) == before
            if not case["sourceUnchanged"] or not lifecycle["validSequence"] or lifecycle["peakActive"] != 1:
                case["result"] = "failed"
                case.setdefault("failureClass", "source_or_encoder_bound")
                case["verificationFailures"] = ["source_or_encoder_bound"]


try:
    subprocess.run(["go", "-C", "apps/player", "build", "-p=1", "-o", str(BINARY), "./cmd/kinosail"], cwd=ROOT, check=True, timeout=180)
    receipt["binarySHA256"] = sha(BINARY)
    receipt["encoderVersions"] = {tool: subprocess.check_output([tool, "-version"], text=True).splitlines()[0] for tool in ["ffmpeg", "ffprobe"]}
    source, metadata = fixture(RUN, "cold-h264", 48, ",".join(str(v) for v in range(0, 32, 2)), frames=768)
    journey("interruptedColdH264", source, metadata)
    hevc = RUN / "cold-hevc.mkv"
    command = ["ffmpeg", "-nostdin", "-v", "error", "-i", str(source), "-c:v", "libx265", "-threads", "2",
        "-x265-params", "pools=2:frame-threads=2:keyint=48:min-keyint=48:scenecut=0", "-preset", "ultrafast", "-crf", "35", "-c:a", "copy", str(hevc)]
    subprocess.run(command, check=True, timeout=120, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    journey("interruptedColdHEVC", hevc, metadata | {"sha256": sha(hevc), "command": command})
    source, metadata = fixture(RUN, "indexed-short", 48, "0,2,4,6", frames=192)
    journey("damagedCompletedIndexedCache", source, metadata, corrupt="syntax")
    journey("unknownCompletedIndexedMap", source, metadata, corrupt="unknown")
    journey("duplicateCompletedIndexedMap", source, metadata, corrupt="duplicate")
    if len(receipt["cases"]) == 5 and all(case["result"] == "passed" for case in receipt["cases"]):
        receipt["result"] = "passed"
except Exception as error:
    receipt["failureClass"] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
finally:
    target = RUN / "receipt.json"
    target.write_text(json.dumps(receipt, indent=2, allow_nan=False) + "\n")
    files = [Path(__file__)] + [Path(__file__).with_name(name) for name in ["hls_timeline_http.py", "hls_timeline_fixture.py", "hls_timeline_packets.py", "hls_timeline_seek_diagnostics.py"]]
    checksums = {str(path.relative_to(ROOT)): sha(path) for path in files} | {"receipt.json": sha(target)}
    (RUN / "SHA256SUMS").write_text("".join(f"{value}  {key}\n" for key, value in checksums.items()))
    print(json.dumps({"result": receipt["result"], "receiptSHA256": checksums["receipt.json"], "cases": [{key: case.get(key) for key in ["name", "result", "failureClass", "cachedPrefixPreserved", "damagedMapRecovered"]} for case in receipt["cases"]]}))
raise SystemExit(0 if receipt["result"] == "passed" else 1)
