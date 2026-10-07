#!/usr/bin/env python3
"""Admit only the fixed 13-case, one-attempt WebKit HLS navigation proof."""
import json
from pathlib import Path
import sys


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError
        result[key] = value
    return result


def read_report(name):
    with (Path('.verification/hls-navigation') / name).open('rb') as file:
        raw = file.read(2097153)
    if len(raw) > 2097152:
        raise ValueError
    report = json.loads(raw, object_pairs_hook=unique_object)
    if not isinstance(report, dict) or report.get('errors') != []:
        raise ValueError
    return report


def identities(report, completed):
    if not isinstance(report['suites'], list) or len(report['suites']) > 64:
        raise ValueError
    pending = [(suite, 0) for suite in report['suites']]
    found = set()
    visited = 0
    while pending:
        suite, depth = pending.pop()
        visited += 1
        if not isinstance(suite, dict) or visited > 64 or depth > 8:
            raise ValueError
        children, specs = suite.get('suites', []), suite.get('specs', [])
        if not isinstance(children, list) or not isinstance(specs, list) or len(children) > 64 or len(specs) > 13:
            raise ValueError
        pending.extend((child, depth + 1) for child in children)
        for spec in specs:
            if not isinstance(spec, dict) or len(found) >= 13:
                raise ValueError
            identity = spec['id']
            if not isinstance(identity, str) or not 1 <= len(identity) <= 256 or identity in found:
                raise ValueError
            if spec['file'] != 'player-hls-navigation.spec.ts' or len(spec['tests']) != 1:
                raise ValueError
            test = spec['tests'][0]
            if not isinstance(test, dict):
                raise ValueError
            if test['projectName'] != 'webkit' or test['expectedStatus'] != 'passed':
                raise ValueError
            results = test['results']
            if not isinstance(results, list):
                raise ValueError
            if completed:
                if spec['ok'] is not True or test['status'] != 'expected' or len(results) != 1:
                    raise ValueError
                result = results[0]
                if not isinstance(result, dict):
                    raise ValueError
                if result['status'] != 'passed' or type(result['retry']) is not int or result['retry'] != 0 or result['errors'] != []:
                    raise ValueError
            elif results != []:
                raise ValueError
            found.add(identity)
    if len(found) != 13:
        raise ValueError
    return found


if __name__ == '__main__':
    try:
        if len(sys.argv) != 1:
            raise ValueError
        discovery = read_report('list-webkit.json')
        actual = read_report('results-webkit.json')
        stats = actual['stats']
        if any(type(stats[key]) is not int or stats[key] != expected for key, expected in (
                ('expected', 13), ('skipped', 0), ('unexpected', 0), ('flaky', 0))):
            raise ValueError
        if identities(discovery, False) != identities(actual, True):
            raise ValueError
        print('HLS navigation proof: 13 passed, zero skips/retries/failures')
    except (OSError, ValueError, KeyError, TypeError, RecursionError):
        print('invalid fixed HLS navigation proof', file=sys.stderr)
        raise SystemExit(2)
