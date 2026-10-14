"""Reject empty native contracts and AVAudioSession main-thread diagnostics.

Input is xcresulttool's test-results summary. Emit only a safe scalar projection;
the original xcresult and summary remain private CI artifacts.
"""
import hashlib
import json
from pathlib import Path
import re
import sys

MAX_BYTES = 2 * 1024 * 1024


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError('duplicate field')
        result[key] = value
    return result


def inspect(platform, path, revision):
    if platform not in ('iOS', 'tvOS') or not re.fullmatch(r'[a-f0-9]{40}', revision):
        raise ValueError('invalid identity')
    with Path(path).open('rb') as stream:
        raw = stream.read(MAX_BYTES + 1)
    if len(raw) > MAX_BYTES:
        raise ValueError('oversized summary')
    data = json.loads(raw, object_pairs_hook=unique_object)
    counts = {key: data[key] for key in ('passedTests', 'failedTests', 'skippedTests',
                                       'expectedFailures', 'totalTestCount')}
    if any(type(value) is not int or not 0 <= value <= 100000 for value in counts.values()):
        raise ValueError('invalid counts')
    if counts['totalTestCount'] != sum(value for key, value in counts.items() if key != 'totalTestCount'):
        raise ValueError('inconsistent counts')
    devices = data['devicesAndConfigurations']
    if not isinstance(devices, list) or not 1 <= len(devices) <= 32:
        raise ValueError('invalid destinations')
    if any(device['device']['platform'] != platform + ' Simulator' for device in devices):
        raise ValueError('wrong destination')
    warnings = data['runtimeWarnings']
    if not isinstance(warnings, list) or len(warnings) > 10000:
        raise ValueError('invalid warnings')
    audio_warnings = 0
    for warning in warnings:
        message = warning['message']
        if not isinstance(message, str) or len(message) > 8192:
            raise ValueError('invalid warning')
        audio = re.search(r'(?:SessionCore|AVAudioSession(?:_iOS)?)\.mm:\d+', message)
        if 'main thread' in message.lower() and (audio or 'audio session' in message.lower()):
            audio_warnings += 1
    passed = (data['result'] == 'Passed' and counts['passedTests'] > 0
              and counts['failedTests'] == counts['expectedFailures'] == 0
              and data['testFailures'] == [] and audio_warnings == 0)
    return {'schemaVersion': 1, 'revision': revision, 'platform': platform,
            'result': 'passed' if passed else 'failed', **counts,
            'audioMainThreadWarnings': audio_warnings,
            'otherRuntimeWarnings': len(warnings) - audio_warnings,
            'summarySHA256': hashlib.sha256(raw).hexdigest()}


if __name__ == '__main__':
    try:
        if len(sys.argv) != 4:
            raise ValueError('platform, summary and revision required')
        receipt = inspect(*sys.argv[1:])
        print(json.dumps(receipt, sort_keys=True))
        sys.exit(0 if receipt['result'] == 'passed' else 1)
    except (OSError, ValueError, KeyError, TypeError, RecursionError):
        sys.exit('native result summary rejected')
