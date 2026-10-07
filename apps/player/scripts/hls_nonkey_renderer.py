"""Actual native frame correspondence; raw failures and every callback stay intact."""
from collections import Counter
from decimal import Decimal
import json
import math
import re
from hls_followon_public import check, bounded_bytes
from hls_timeline_http import sha, source_state
from hls_nonkey_process import owned_command
from hls_nonkey_direct import direct_delivery_matches


def native_metadata_qualified(values):
    return (isinstance(values, list) and 0 < len(values) <= 32 and all(
        isinstance(v, dict) and v.get('format') in ['I420', 'NV12']
        and v.get('visibleRect') == [0, 0, 640, 360] and v.get('displayDimensions') == [640, 360]
        and isinstance(v.get('codedDimensions'), list) and len(v['codedDimensions']) == 2
        and all(type(n) is int for n in v['codedDimensions'])
        and 640 <= v['codedDimensions'][0] <= 1024 and 360 <= v['codedDimensions'][1] <= 512
        and v.get('rotation') == 0 and v.get('flip') is False
        and isinstance(v.get('colorSpace'), dict)
        and set(v['colorSpace']) == {'primaries', 'transfer', 'matrix', 'fullRange'}
        and all(value is None or isinstance(value, str) and re.fullmatch('[a-z0-9-]{1,32}', value)
                for key, value in v['colorSpace'].items() if key != 'fullRange')
        and (v['colorSpace']['fullRange'] is None or type(v['colorSpace']['fullRange']) is bool)
        and type(v.get('allocationBytes')) is int and 0 < v['allocationBytes'] <= 1024 * 1024
        and isinstance(v.get('layout'), list) and len(v['layout']) == (3 if v['format'] == 'I420' else 2)
        and all(isinstance(p, list) and len(p) == 2 and all(type(n) is int and 0 <= n <= 1024 * 1024 for n in p)
                for p in v['layout']) for v in values))


def capture_rows(value):
    rows = value.get('rows', [])
    check(isinstance(rows, list) and len(rows) <= 4096, 'renderer_row_bound')
    for row in rows:
        check(isinstance(row, list) and len(row) == 6 and type(row[0]) in (int, float)
            and math.isfinite(row[0]) and -1 <= row[0] <= 120
            and type(row[1]) is int and 0 < row[1] <= 8192
            and isinstance(row[2], str) and (row[2] == '' or re.fullmatch('[a-f0-9]{64}', row[2]))
            and (row[3] is None or type(row[3]) is int and -1000000 <= row[3] <= 120000000)
            and (row[4] is None or type(row[4]) is int and 0 <= row[4] < 32)
            and type(row[5]) in (int, float) and math.isfinite(row[5]) and 0 <= row[5] <= 16, 'renderer_row_shape')
    qualified = native_capture_integrity(value, rows) and value.get('videoPlaybackQuality', {}).get('total') == len(rows)
    return rows, qualified


def native_capture_integrity(value, rows):
    descriptors = value.get('nativeFrames', [])
    gesture, quality = value.get('beforeGesture', []), value.get('videoPlaybackQuality', {})
    paused = (isinstance(gesture, list) and len(gesture) == 2 and all(
        isinstance(v, list) and len(v) == 8 and v[0] is True
        and type(v[1]) in (int, float) and math.isfinite(v[1])
        and type(v[2]) is int and 0 <= v[2] <= 1 and v[3] is False
        and type(v[4]) in (int, float) and 0 < v[4] <= 1
        and type(v[5]) in (int, float) and math.isfinite(v[5]) and v[5] == 1
        and v[6] is False and v[7] is False for v in gesture)
        and abs(gesture[0][1] - gesture[1][1]) <= 0.001)
    qualified = (bool(rows) and not value.get('failureClass') and value.get('ended') is True and value.get('errorCode') == 0
        and value.get('captureErrors') == 0 and value.get('width') == 640 and value.get('height') == 360
        and rows[0][1] == 1 and all(a[0] < b[0] and b[1] == a[1] + 1 for a, b in zip(rows, rows[1:]))
        and len({row[2] for row in rows}) == len(rows) and paused
        and value.get('gesture') == 'player-keyboard-space'
        and value.get('gestureEvent') == {'trusted': True, 'hasBeenActive': True, 'isActive': True}
        and native_metadata_qualified(descriptors)
        and isinstance(quality, dict)
        and quality.get('dropped') == 0 and quality.get('corrupted') == 0
        and all(re.fullmatch('[a-f0-9]{64}', row[2]) and row[3] is not None
                and abs(row[3] / 1000000 - row[0]) <= 0.001
                and row[4] is not None and row[4] < len(descriptors) and row[5] == 1 for row in rows))
    return qualified


def renderer_facts(reference, public, source_pts, requested):
    original, reference_ok = capture_rows(reference)
    observed, public_ok = capture_rows(public)
    check(isinstance(source_pts, list) and 0 < len(source_pts) <= 4096
        and all(type(v) in (int, float) and math.isfinite(v) for v in source_pts)
        and all(a < b for a, b in zip(source_pts, source_pts[1:])), 'renderer_source_clock')
    within_clock = lambda left, right: abs(Decimal(str(left)) - Decimal(str(right))) <= Decimal('0.001')
    reference_ok = (reference_ok and len(original) == len(source_pts)
        and all(within_clock(row[0], point) for row, point in zip(original, source_pts)))
    counts = Counter(row[2] for row in original)
    index = {row[2]: n for n, row in enumerate(original) if counts[row[2]] == 1}
    mapped = [index.get(row[2]) for row in observed]
    expected = [n for n, point in enumerate(source_pts) if point >= requested - 0.000001]
    colors = lambda v: sorted({json.dumps(d.get('colorSpace'), sort_keys=True)
                              for d in v.get('nativeFrames', []) if isinstance(d, dict)})
    return {'boundary': 'Complete native YUV420 frame identity from actual watch pages; no trimming or RGB color-equivalence claim',
        'referenceQualified': reference_ok, 'publicQualified': public_ok,
        'requestedIdentityMatches': reference_ok and public_ok and mapped == expected,
        'publicSourceClockMatches': reference_ok and public_ok and bool(mapped)
            and all(n is not None and within_clock(row[0], source_pts[n])
                    for row, n in zip(observed, mapped)),
        'colorInterpretationMatches': colors(reference) == colors(public), 'independentSourcePTS': source_pts,
        'rowColumns': ['mediaTimeSeconds', 'presentedFrames', 'nativeYUV420SHA256',
                       'nativeTimestampMicroseconds', 'nativeFrameMetadataIndex', 'playbackRate'],
        'completeReferenceRows': [json.dumps(row, separators=(',', ':')) for row in original],
        'completePublicRows': [json.dumps(row, separators=(',', ':')) for row in observed],
        'publicSourceIndices': mapped, 'expectedSourceIndices': expected,
        'precedingSourceIndices': [n for n in mapped if n not in expected] if reference_ok else None,
        'missingRequestedIndices': [n for n in expected if n not in mapped] if reference_ok else None,
        'unknownPublicFrames': sum(n is None for n in mapped),
        'duplicatePublicSourceIndices': [n for n, count in Counter(mapped).items() if n is not None and count > 1]}


def timing_rows_qualified(value, rows):
    timings = value.get('frameTimings', [])
    return (isinstance(timings, list) and len(timings) == len(rows) and all(
        isinstance(t, list) and len(t) == 4 and type(t[0]) is int and t[0] == row[1]
        and all(type(v) in (int, float) and math.isfinite(v) and 0 <= v <= 240000 for v in t[1:])
        and t[3] >= t[2] for t, row in zip(timings, rows))
        and all(all(a[n] < b[n] for n in [1, 2, 3]) for a, b in zip(timings, timings[1:])))


def terminal_sample_qualified(sample, rows):
    if not isinstance(sample, dict) or not rows:
        return False
    last, quality = rows[-1], sample.get('quality', {})
    return (sample.get('callbacks') == len(rows) and sample.get('presentedFrames') == last[1]
        and sample.get('pts') == last[0] and sample.get('sha256') == last[2]
        and type(sample.get('pendingCopies')) is int and sample['pendingCopies'] == 0
        and isinstance(quality, dict) and type(quality.get('total')) is int
        and len(rows) <= quality['total'] <= 8192
        and all(type(quality.get(key)) is int and quality[key] == 0 for key in ['dropped', 'corrupted']))


def tail_qualified(value, rows):
    tail = value.get('presentationTail', {})
    if not isinstance(tail, dict) or not rows:
        return False
    before, after = tail.get('beforeFinalCopy'), tail.get('afterFinalCopy')
    final = tail.get('finalNativeFrame', [])
    flags = all(tail.get(key) is True for key in ['settled', 'visibilityStable', 'generationStable', 'ended', 'paused'])
    elapsed, quiet, frames = [tail.get(key) for key in ['elapsedMilliseconds', 'quietMilliseconds', 'animationFrames']]
    bounds = (type(elapsed) in (int, float) and math.isfinite(elapsed) and 500 <= elapsed <= 2000
        and type(quiet) in (int, float) and math.isfinite(quiet) and 250 <= quiet <= elapsed
        and type(frames) is int and 8 <= frames <= 1024)
    native = (isinstance(final, list) and len(final) == 6 and final[1] is None
        and final[0] == rows[-1][3] / 1000000 and final[2:] == rows[-1][2:])
    clocks = all(type(tail.get(key)) in (int, float) and math.isfinite(tail[key])
        and 0 < tail[key] <= 120 for key in ['nativeTime', 'nativeDuration'])
    return (flags and tail.get('timedOut') is False and type(tail.get('attachments')) is int and tail['attachments'] == 1 and bounds
        and terminal_sample_qualified(before, rows) and before == after and native and clocks
        and timing_rows_qualified(value, rows))


def compositor_facts(reference, public, legacy):
    original, _ = capture_rows(reference)
    observed, _ = capture_rows(public)
    source = legacy.get('independentSourcePTS', [])
    mapped, expected = legacy.get('publicSourceIndices', []), legacy.get('expectedSourceIndices', [])
    reference_ok = legacy.get('referenceQualified') is True and tail_qualified(reference, original)
    identity = bool(expected) and mapped == expected and len(observed) == len(expected)
    within = lambda a, b: abs(Decimal(str(a)) - Decimal(str(b))) <= Decimal('0.001')
    clock = (bool(observed) and len(mapped) == len(observed) and all(type(n) is int and 0 <= n < len(source)
        and within(row[0], source[n]) for row, n in zip(observed, mapped)))
    tail_ok = tail_qualified(public, observed)
    return {'boundary': 'Separate compositor/native-frame diagnostic; legacy equality/case failures remain; no screen scanout, surplus decode or production certification',
        'referenceWithTailQualified': reference_ok, 'publicTailQualified': tail_ok,
        'exactRequestedIdentity': identity, 'sourceClockMatches': clock,
        'legacyQualityEquality': public.get('videoPlaybackQuality', {}).get('total') == len(observed),
        'completeCompositorObservation': reference_ok and native_capture_integrity(public, observed)
            and identity and clock and tail_ok and legacy.get('colorInterpretationMatches') is True}


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


def public_renderer(api, item_id, reference_id, metadata, offset, hls, directory, case, root, reference_source,
                    direct_resume_id, direct_resume_source):
    result = {'boundary': 'Actual disposable public Server/browser; native/iOS/live acceptance separate'}
    case['publicRenderer'] = result
    check(all(re.fullmatch('[a-f0-9]{16}', v) for v in [item_id, reference_id, direct_resume_id]),
          'renderer_item_identity')
    state = api.call('/api/v1/items/' + item_id + '/progress', 'PUT', {'seconds': offset})
    check(abs(state['seconds'] - offset) <= 0.000001, 'renderer_saved_resume')
    direct_state = api.call('/api/v1/items/' + direct_resume_id + '/progress', 'PUT', {'seconds': offset})
    check(abs(direct_state['seconds'] - offset) <= 0.000001, 'renderer_direct_saved_resume')
    result['directSavedResumeSeconds'] = direct_state['seconds']
    target = directory / 'browser-private.json'
    command = ['node', str(root / 'apps/player/e2e/hls-public-renderer.mjs'), str(target)]
    private = json.dumps({'url': api.url, 'token': api.token, 'itemID': item_id,
        'referenceID': reference_id, 'directResumeID': direct_resume_id, 'hls': hls})
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
    direct_after = source_state(direct_resume_source)
    direct_unchanged = case['browserDirectResumeSource']['before'] == direct_after
    case['browserDirectResumeSource'].update(after=direct_after, sourceUnchanged=direct_unchanged)
    unchanged = unchanged and direct_unchanged
    if not unchanged:
        case['failures'].append('renderer_reference_mutated')
    result['runtime'] = {key: data.get(key) for key in ['browserVersion', 'playbackRate', 'reference', 'public', 'directResume']}
    result.update(renderer_facts(data.get('reference', {}), data.get('public', {}),
                                metadata['sourceFramePTS'], metadata['sourceTimeOriginSeconds'] + offset))
    result['compositorObservation'] = compositor_facts(data.get('reference', {}), data.get('public', {}), result)
    direct_facts = renderer_facts(data.get('reference', {}), data.get('directResume', {}),
                                 metadata['sourceFramePTS'], metadata['sourceTimeOriginSeconds'] + offset)
    direct_facts.pop('completeReferenceRows')  # Original full reference remains intact above.
    direct_facts['actualDirectMediaDelivered'] = direct_unchanged and direct_delivery_matches(
        data.get('directResume', {}).get('network', {}), bounded_bytes(direct_resume_source, 8 * 1024 * 1024,
                                                                    'direct_reference_bound'))
    direct_facts['compositorObservation'] = compositor_facts(data.get('reference', {}), data.get('directResume', {}), direct_facts)
    result['directResumeControl'] = direct_facts
    network = data.get('public', {}).get('network', {})
    result['actualPlannedRecipeDelivered'] = renderer_delivery_matches(network, case['initializationSHA256'],
        case['publicVariant']['segmentCount'], unchanged)
    result['compositorObservation']['deliveryAndProcessQualified'] = result['processQualified'] and result['actualPlannedRecipeDelivered']
    direct_facts['compositorObservation']['deliveryAndProcessQualified'] = result['processQualified'] and direct_facts['actualDirectMediaDelivered']
    for observation in [result['compositorObservation'], direct_facts['compositorObservation']]:
        observation['completeCompositorObservation'] = observation['completeCompositorObservation'] and observation['deliveryAndProcessQualified']
    # Keep complete compact rows once; do not duplicate them in runtime metadata.
    for value in result['runtime'].values():
        if isinstance(value, dict):
            value.pop('rows', None)
    result['result'] = 'passed' if (result['processQualified'] and result['requestedIdentityMatches'] and result['publicSourceClockMatches']
        and result['actualPlannedRecipeDelivered']) else 'failed'
