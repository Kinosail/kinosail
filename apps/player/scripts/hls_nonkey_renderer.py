"""Actual public watch-page pixels; raw failures and every callback remain intact."""
from collections import Counter
import json
import math
import re
from hls_followon_public import check, bounded_bytes
from hls_timeline_http import sha, source_state
from hls_nonkey_process import owned_command


def capture_rows(value):
    rows = value.get('rows', [])
    check(isinstance(rows, list) and len(rows) <= 4096, 'renderer_row_bound')
    for row in rows:
        check(isinstance(row, list) and len(row) == 3 and type(row[0]) in (int, float)
            and math.isfinite(row[0]) and -1 <= row[0] <= 120
            and type(row[1]) is int and 0 < row[1] <= 8192
            and isinstance(row[2], str) and re.fullmatch('[a-f0-9]{64}', row[2]), 'renderer_row_shape')
    qualified = (bool(rows) and not value.get('failureClass') and value.get('ended') is True and value.get('errorCode') == 0
        and value.get('captureErrors') == 0 and value.get('width') == 640 and value.get('height') == 360
        and rows[0][1] == 1 and all(a[0] < b[0] and b[1] == a[1] + 1 for a, b in zip(rows, rows[1:]))
        and len({row[2] for row in rows}) == len(rows))
    return rows, qualified


def renderer_facts(reference, public, source_pts, requested):
    original, reference_ok = capture_rows(reference)
    observed, public_ok = capture_rows(public)
    check(isinstance(source_pts, list) and 0 < len(source_pts) <= 4096
        and all(type(v) in (int, float) and math.isfinite(v) for v in source_pts)
        and all(a < b for a, b in zip(source_pts, source_pts[1:])), 'renderer_source_clock')
    reference_ok = (reference_ok and len(original) == len(source_pts)
        and all(abs(row[0] - point) <= 0.001 for row, point in zip(original, source_pts)))
    counts = Counter(row[2] for row in original)
    index = {row[2]: n for n, row in enumerate(original) if counts[row[2]] == 1}
    mapped = [index.get(row[2]) for row in observed]
    expected = [n for n, point in enumerate(source_pts) if point >= requested - 0.000001]
    return {'boundary': 'Complete native-resolution browser pixels from actual authenticated watch pages; no trimming',
        'referenceQualified': reference_ok, 'publicQualified': public_ok,
        'requestedIdentityMatches': reference_ok and public_ok and mapped == expected,
        'publicSourceClockMatches': reference_ok and public_ok and bool(mapped)
            and all(n is not None and abs(row[0] - source_pts[n]) <= 0.001
                    for row, n in zip(observed, mapped)),
        'rowColumns': ['mediaTimeSeconds', 'presentedFrames', 'rgbaSHA256'],
        'completeReferenceRows': [json.dumps(row, separators=(',', ':')) for row in original],
        'completePublicRows': [json.dumps(row, separators=(',', ':')) for row in observed],
        'publicSourceIndices': mapped, 'expectedSourceIndices': expected,
        'precedingSourceIndices': [n for n in mapped if n not in expected] if reference_ok else None,
        'missingRequestedIndices': [n for n in expected if n not in mapped] if reference_ok else None,
        'unknownPublicFrames': sum(n is None for n in mapped),
        'duplicatePublicSourceIndices': [n for n, count in Counter(mapped).items() if count > 1]}


def renderer_delivery_matches(network, init_sha, segment_count, reference_unchanged):
    values = network.get('initializationSHA256s', [])
    return (reference_unchanged and network.get('master') == 200 and network.get('variant') == 200
        and isinstance(values, list) and 0 < len(values) <= 32 and all(v == init_sha for v in values)
        and network.get('successfulFragments') == segment_count
        and network.get('unexpectedMediaRequests') == 0 and network.get('failedMediaResponses') == 0)


def renderer_process_accepted(process, data):
    return (process.get('exitCode') == 0 and process.get('timedOut') is False
        and process.get('ownedGroupJoined') is True and process.get('browserOwnershipVerified') is True
        and process.get('liveOwnedProcesses') == 0 and process.get('joinedSamples', 0) >= 2
        and not data.get('failureClass'))


def public_renderer(api, item_id, reference_id, metadata, offset, hls, directory, case, root, reference_source):
    result = {'boundary': 'Actual disposable public Server/browser; native/iOS/live acceptance separate'}
    case['publicRenderer'] = result
    check(re.fullmatch('[a-f0-9]{16}', item_id) and re.fullmatch('[a-f0-9]{16}', reference_id),
          'renderer_item_identity')
    state = api.call('/api/v1/items/' + item_id + '/progress', 'PUT', {'seconds': offset})
    check(abs(state['seconds'] - offset) <= 0.000001, 'renderer_saved_resume')
    target = directory / 'browser-private.json'
    command = ['node', str(root / 'apps/player/e2e/hls-public-renderer.mjs'), str(target)]
    private = json.dumps({'url': api.url, 'token': api.token, 'itemID': item_id,
        'referenceID': reference_id, 'hls': hls})
    process = owned_command(command, private.encode(), 260, target.with_name(target.name + '.owner'))
    check(len(process['stdout']) <= 65536 and len(process['stderr']) <= 65536, 'renderer_log_bound')
    log = directory / 'browser-stderr.log'
    log.write_bytes(process['stderr'])
    result.update(process={k: v for k, v in process.items() if k not in ['stdout', 'stderr']},
                  privateLogSHA256=sha(log))
    raw = bounded_bytes(target, 512 * 1024, 'renderer_receipt_bound')
    data = json.loads(raw)
    result['processQualified'] = renderer_process_accepted(process, data)
    result['privateReceiptSHA256'] = sha(target)
    after = source_state(reference_source)
    unchanged = case['browserReferenceSource']['before'] == after
    case['browserReferenceSource'].update(after=after, sourceUnchanged=unchanged)
    if not unchanged:
        case['failures'].append('renderer_reference_mutated')
    result['runtime'] = {key: data.get(key) for key in ['browserVersion', 'playbackRate', 'reference', 'public']}
    result.update(renderer_facts(data.get('reference', {}), data.get('public', {}),
                                metadata['sourceFramePTS'], metadata['sourceTimeOriginSeconds'] + offset))
    network = data.get('public', {}).get('network', {})
    result['actualPlannedRecipeDelivered'] = renderer_delivery_matches(network, case['initializationSHA256'],
        case['publicVariant']['segmentCount'], unchanged)
    # Keep complete compact rows once; do not duplicate them in runtime metadata.
    for value in result['runtime'].values():
        if isinstance(value, dict):
            value.pop('rows', None)
    result['result'] = 'passed' if (result['processQualified'] and result['requestedIdentityMatches'] and result['publicSourceClockMatches']
        and result['actualPlannedRecipeDelivered']) else 'failed'
