"""Prepared versus completed cold public Server controls; no production substitutes."""
import hashlib
import re
import time
from hls_timeline_packets import manifest_facts


def prepare_scene(api, prepare, hls, cache, item_id, server, source, case, encoder_count, check, bounded_bytes, cold):
    if cold:
        check(api.http(hls)[0] == 200, "cold_master")
        limit = time.monotonic() + 25
        while time.monotonic() < limit:
            roots = [p for p in cache.iterdir() if p.name.startswith(item_id + "-plan-")]
            physical = list(roots[0].glob("*p/index.m3u8")) if len(roots) == 1 else []
            if len(physical) == 1 and b"#EXT-X-ENDLIST" in physical[0].read_bytes() and encoder_count(server, source) == 0:
                break
            time.sleep(0.05)
        check(len(physical) == 1 and b"#EXT-X-ENDLIST" in physical[0].read_bytes(), "cold_worker_incomplete")
        init = bounded_bytes(physical[0].parent / "init.mp4", 2 * 1024 * 1024, "cold_initialization_bound")
        facts, _ = manifest_facts(bounded_bytes(physical[0], 1024 * 1024, "cold_manifest_bound"))
        case["preparationReady"] = False
        case["beforeFirstHLSGET"] = {"authenticatedMediaGETs": 1, "ownedFFmpeg": 0, "physicalVariant": facts}
        return roots[0], init
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
    recipe_token = hls.split("/p/", 1)[1].split("/", 1)[0]
    case["preparedRecipeMatches"] = root.name == item_id + "-plan-" + recipe_token
    suffix = re.search(r"-o([0-9]+)$", root.name)
    case["preparedRecipeOffsetMilliseconds"] = int(suffix[1]) if suffix else 0
    check(case["preparedRecipeMatches"], "prepared_recipe_identity")
    limit, stopped_samples = time.monotonic() + 10, 0
    while time.monotonic() < limit:
        stopped = encoder_count(server, source) == 0 and (root / ".seekable").exists() and (root / ".startup").exists()
        stopped_samples = stopped_samples + 1 if stopped else 0
        if stopped_samples >= 3:
            break
        time.sleep(0.05)
    check(stopped_samples >= 3 and encoder_count(server, source) == 0 and (root / ".seekable").exists()
        and (root / ".startup").exists(), "prepared_worker_not_joined")
    physical = list(root.glob("*p/index.m3u8"))
    check(len(physical) == 1, "single_prepared_rendition")
    prepared_facts, _ = manifest_facts(bounded_bytes(physical[0], 1024 * 1024, "prepared_manifest_bound"))
    prepared_init = bounded_bytes(physical[0].parent / "init.mp4", 2 * 1024 * 1024,
        "prepared_initialization_bound")
    case["preparedInitSHA256"] = hashlib.sha256(prepared_init).hexdigest()
    case["beforeFirstHLSGET"] = {"ownedFFmpeg": 0, "seekableMarker": True, "startupMarker": True,
        "authenticatedMediaGETs": 0, "stoppedSamples": stopped_samples, "physicalVariant": prepared_facts}
    return root, prepared_init
