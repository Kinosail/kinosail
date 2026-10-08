"""Select an available simulator compatible with the native deployment target."""
import json
import re
import sys


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError('duplicate field')
        result[key] = value
    return result


def select(platform, raw):
    if platform not in ('iOS', 'tvOS') or len(raw) > 1024 * 1024:
        raise ValueError('invalid simulator input')
    devices = json.loads(raw, object_pairs_hook=unique_object)['devices']
    if not isinstance(devices, dict) or len(devices) > 512:
        raise ValueError('invalid runtime inventory')
    choices = []
    count = 0
    for runtime, rows in devices.items():
        if not isinstance(rows, list):
            raise ValueError('invalid device inventory')
        count += len(rows)
        if count > 2048:
            raise ValueError('too many devices')
        match = re.fullmatch(r'com\.apple\.CoreSimulator\.SimRuntime\.' + platform + r'-(\d{1,3})-(\d{1,3})(?:-(\d{1,3}))?', runtime)
        if not match:
            continue
        version = tuple(int(value or 0) for value in match.groups())
        if version < (26, 0, 0):
            continue
        for device in rows:
            if not isinstance(device, dict) or type(device.get('isAvailable')) is not bool:
                raise ValueError('invalid device availability')
            if not device['isAvailable']:
                continue
            identifier = device.get('udid')
            if not isinstance(identifier, str) or not re.fullmatch(r'[0-9A-Fa-f]{8}(?:-[0-9A-Fa-f]{4}){3}-[0-9A-Fa-f]{12}', identifier):
                raise ValueError('invalid device identifier')
            kind = device.get('deviceTypeIdentifier')
            if not isinstance(kind, str) or len(kind) > 128:
                raise ValueError('invalid device type')
            family = 'iPhone-' if platform == 'iOS' else 'Apple-TV-'
            if not kind.startswith('com.apple.CoreSimulator.SimDeviceType.' + family):
                continue
            if not re.fullmatch(r'com\.apple\.CoreSimulator\.SimDeviceType\.' + family + r'[A-Za-z0-9]+(?:-[A-Za-z0-9]+)*', kind):
                raise ValueError('invalid device type')
            choices.append((version, identifier))
    if not choices:
        raise ValueError('compatible simulator required')
    return max(choices)[1]


if __name__ == '__main__':
    try:
        if len(sys.argv) != 2:
            raise ValueError('one platform required')
        print(select(sys.argv[1], sys.stdin.read(1024 * 1024 + 1)))
    except (ValueError, KeyError, TypeError):
        sys.exit('available iOS or tvOS simulator version 26 or newer required')
