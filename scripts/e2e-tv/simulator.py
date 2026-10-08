"""Recover only an exact run-owned simulator creation; never select a personal device."""
import hashlib
import json
import re
import subprocess
import tempfile

TYPE = 'com.apple.CoreSimulator.SimDeviceType.Apple-TV-4K-3rd-generation-4K'
UUID = r'[A-Fa-f0-9]{8}(?:-[A-Fa-f0-9]{4}){3}-[A-Fa-f0-9]{12}'
RUNTIME = r'com\.apple\.CoreSimulator\.SimRuntime\.tvOS-27-\d{1,2}(?:-\d{1,2})?'


def strict_json(raw):
    def pairs(entries):
        result = {}
        for key, value in entries:
            if key in result:
                raise RuntimeError('Ambiguous simulator JSON')
            result[key] = value
        return result
    return json.loads(raw, object_pairs_hook=pairs)


def inventory():
    # Bound the external CLI response before parsing. A private temporary stream avoids
    # an unbounded memory capture and is closed on timeout or malformed output.
    with tempfile.TemporaryFile() as output:
        subprocess.run(['xcrun', 'simctl', 'list', 'devices', '--json'], stdout=output,
                       stderr=subprocess.DEVNULL, timeout=30, check=True)
        output.seek(0)
        raw = output.read(1024 * 1024 + 1)
    if len(raw) > 1024 * 1024:
        raise RuntimeError('Simulator inventory too large')
    data = strict_json(raw)
    if not isinstance(data, dict) or not isinstance(data.get('devices'), dict) or len(data['devices']) > 128:
        raise RuntimeError('Invalid simulator inventory')
    entries = []
    for runtime, devices in data['devices'].items():
        if not isinstance(runtime, str) or len(runtime) > 256 or not isinstance(devices, list):
            raise RuntimeError('Invalid simulator inventory group')
        for device in devices:
            if not isinstance(device, dict) or not isinstance(device.get('name'), str) or len(device['name']) > 256 or not isinstance(device.get('udid'), str) or not re.fullmatch(UUID, device['udid']):
                raise RuntimeError('Invalid simulator inventory device')
            entries.append((runtime, device))
            if len(entries) > 4096:
                raise RuntimeError('Simulator inventory too many devices')
    return entries, hashlib.sha256(raw).hexdigest()


def validate_pending(owner):
    pending = owner.get('creationPending')
    if pending is None:
        return None
    if not isinstance(pending, dict) or set(pending) != {'name', 'runtime', 'type', 'inventoryZero', 'inventorySHA256'} or owner.get('platform') != 'ios' or owner.get('complete') is not False:
        raise RuntimeError('Invalid pending simulator ownership')
    if pending['name'] != 'Kinosail-TV-E2E-' + owner['run'] or pending['type'] != TYPE or pending['inventoryZero'] is not True or not isinstance(pending['runtime'], str) or not re.fullmatch(RUNTIME, pending['runtime']) or not isinstance(pending['inventorySHA256'], str) or not re.fullmatch(r'[a-f0-9]{64}', pending['inventorySHA256']):
        raise RuntimeError('Invalid pending simulator witness')
    return pending


def prepare_creation(owner, runtime):
    if not isinstance(runtime, str) or not re.fullmatch(RUNTIME, runtime):
        raise RuntimeError('Invalid creation runtime')
    entries, digest = inventory()
    name = 'Kinosail-TV-E2E-' + owner['run']
    if any(device['name'] == name for _, device in entries):
        raise RuntimeError('Owned simulator name collision; preserved')
    owner['complete'] = False
    owner['creationPending'] = {'name': name, 'runtime': runtime, 'type': TYPE,
                                'inventoryZero': True, 'inventorySHA256': digest}


def cleanup_creation(owner, save, command=None):
    pending = validate_pending(owner)
    if pending is None:
        return []
    entries, _ = inventory()
    found = [(runtime, device) for runtime, device in entries if device['name'] == pending['name']]
    rows = []
    if found:
        if len(found) != 1 or found[0][0] != pending['runtime'] or found[0][1].get('deviceTypeIdentifier') != pending['type'] or (owner.get('device') is not None and owner['device'] != found[0][1]['udid']):
            raise RuntimeError('Pending simulator identity ambiguous; preserved')
        device = found[0][1]['udid']
        owner['device'] = device
        save()
        for action in ('shutdown', 'delete'):
            args = ['xcrun', 'simctl', action, device]
            status = command(args) if command else subprocess.run(args, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=60).returncode
            rows.append({'resource': 'owned simulator ' + device, 'action': action, 'result': status})
            if status and action == 'delete':
                raise RuntimeError('Pending simulator deletion failed')
        entries, _ = inventory()
        if any(device['name'] == pending['name'] for _, device in entries):
            raise RuntimeError('Pending simulator remains after deletion')
    owner['creationPending'] = None
    save()
    return rows
