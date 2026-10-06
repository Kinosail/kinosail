"""Cancel only test-owned, unpublished HEVC preparation before its natural EOF."""
import hashlib
import json
import re
import time
from hls_followon_public import bounded_bytes, check, encoder_count
from hls_timeline_packets import manifest_facts


def interrupted_preparation(api, prepare, hls, cache, item_id, server, source, log_path, case):
    prior = log_path.stat().st_size
    status, data, headers = api.http(prepare, 'POST', {'source': hls})
    check(status == 202 and len(data) <= 512 * 1024, 'interrupted_prepare_response')
    check(json.loads(data).get('state') == 'queued', 'interrupted_prepare_state')
    request_id = next((v for k, v in headers.items() if k.lower() == 'x-request-id'), '')
    check(re.fullmatch(r'[a-zA-Z0-9_-]{1,80}', request_id), 'interrupted_request_correlation')
    limit = time.monotonic() + 8
    while time.monotonic() < limit:
        roots = [p for p in cache.iterdir() if p.name.startswith(item_id + '-plan-')]
        variants = list(roots[0].glob('*p/index.m3u8')) if len(roots) == 1 else []
        if len(variants) == 1:
            raw = bounded_bytes(variants[0], 64 * 1024, 'interrupted_manifest_bound')
            facts, advertised = manifest_facts(raw)
            if (len(advertised) >= 3 and facts['durationSeconds'] < 8
                    and not facts['endlist'] and encoder_count(server, source) == 1):
                check(all(0 < (variants[0].parent / name).stat().st_size <= 1024 * 1024
                          for name, _ in advertised), 'interrupted_committed_prefix')
                init = bounded_bytes(variants[0].with_name('init.mp4'), 1024 * 1024,
                                     'interrupted_initialization_bound')
                break
        time.sleep(0.01)
    else:
        raise RuntimeError('interrupted_positive_prefix_not_observed')
    case['preparedInitializationSHA256'] = hashlib.sha256(init).hexdigest()
    case['rawPreparedManifest'] = {'sha256': hashlib.sha256(raw).hexdigest(), 'endlist': facts['endlist']}
    case['interruptedPreparation'] = {'posts': 1, 'requestID': request_id,
        'authenticatedMediaGETsBeforeCancel': 0,
        'sourceDurationSeconds': 10, 'preCancelPrefixLimitSeconds': 8,
        'physicalVariantBeforeCancel': facts, 'positiveCommittedFragments': len(advertised),
        'ownedFFmpegBeforeCancel': 1}
    status, _, _ = api.http(prepare, 'DELETE')
    check(status == 204, 'interrupted_cancel_response')
    limit, joined = time.monotonic() + 8, 0
    while time.monotonic() < limit:
        joined = joined + 1 if encoder_count(server, source) == 0 else 0
        private = bounded_bytes(log_path, 2 * 1024 * 1024, 'interrupted_private_log_bound')[prior:].decode()
        states = []
        for line in private.splitlines():
            check(len(line) <= 16 * 1024, 'interrupted_log_line_bound')
            if not line.startswith('{') or not line.endswith('}'):
                continue
            entry = json.loads(line)
            if entry.get('msg') == 'HLS startup preparation' and entry.get('request_id') == request_id:
                states.append(entry.get('state'))
        if joined >= 3 and states:
            break
        time.sleep(0.05)
    check(joined >= 3 and states, 'interrupted_worker_not_joined')
    roots = [p for p in cache.iterdir() if p.name.startswith(item_id + '-plan-')]
    variants = list(roots[0].glob('*p/index.m3u8')) if len(roots) == 1 else []
    cache_removed = not roots
    after = manifest_facts(bounded_bytes(variants[0], 64 * 1024, 'interrupted_final_manifest_bound'))[0] if len(variants) == 1 else None
    case['interruptedPreparation'].update(cancelStatus=status, joinedSamples=joined, ownedFFmpegAfterCancel=0,
        completionState=states[-1], cacheRemoved=cache_removed, physicalVariantAfterCancel=after)
    check(states[-1] in ['cancelled', 'unavailable'] and
          (cache_removed or after is not None and not after['endlist'] and after['durationSeconds'] < 10),
          'interrupted_cancel_not_qualified')
