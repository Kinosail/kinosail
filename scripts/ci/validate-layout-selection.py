#!/usr/bin/env python3
"""Reject ignored HLS dispatch flags before selected workflow effects."""
import sys


def selection(arguments):
    if len(arguments) != 10 or any(len(value) > 32 for value in arguments):
        raise ValueError
    event, campaign, remaining, timing, installation, metadata, q14, r06, q47, origin = arguments
    if event not in ('pull_request', 'workflow_dispatch'):
        raise ValueError
    if campaign not in ('none', 'R06', 'Q14', 'Q09', 'Q47', 'HLS', 'HLS-navigation', 'Library'):
        raise ValueError
    if any(value not in ('true', 'false') for value in (remaining, timing, installation, metadata, origin)):
        raise ValueError
    if q14 not in ('primary', 'navigation', 'cold', 'bfcache', 'htmx', 'shows', 'search', 'safety', 'home'):
        raise ValueError
    if r06 not in ('protocol', 'save-controls', 'save-headers', 'save-body', 'source-format',
                   'restore-source-format', 'restore-controls', 'restore-headers', 'restore-inspect-body'):
        raise ValueError
    if q47 not in ('source-format', 'primary', 'recovery', 'supersession', 'contracts'):
        raise ValueError
    if any(campaign != owner and value != default for owner, value, default in (
            ('Q14', q14, 'primary'), ('R06', r06, 'protocol'), ('Q47', q47, 'source-format'))):
        raise ValueError
    if metadata == 'true' and (campaign not in ('R06', 'Q14', 'Q09') or r06 in (
            'restore-source-format', 'restore-controls', 'restore-headers', 'restore-inspect-body')):
        raise ValueError
    requested = 'true' in (remaining, timing, installation, origin)
    if event == 'pull_request' and (campaign != 'none' or requested or metadata == 'true'):
        raise ValueError
    if requested and campaign != 'HLS':
        raise ValueError
    if 'true' in (timing, installation, origin) and remaining != 'true':
        raise ValueError
    if (timing, installation, origin).count('true') > 1:
        raise ValueError
    if origin == 'true':
        return 'audio-origin'
    if installation == 'true':
        return 'audio-installation'
    if timing == 'true':
        return 'audio-timing'
    return 'remaining' if remaining == 'true' else campaign


if __name__ == '__main__':
    try:
        print('validated layout selection: ' + selection(sys.argv[1:]))
    except ValueError:
        print('unsupported layout selection', file=sys.stderr)
        raise SystemExit(2)
