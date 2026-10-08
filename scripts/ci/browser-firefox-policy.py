#!/usr/bin/env python3
"""Bind Playwright's explicit Firefox policy to this fixture's owned receipt."""
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import sys

spec = importlib.util.spec_from_file_location('browser_native_ca', Path(__file__).with_name('browser-native-ca.py'))
native = importlib.util.module_from_spec(spec)
spec.loader.exec_module(native)


def policy(certificate, workspace, nonce):
    certificate, workspace, receipt = native.arguments('firefox', certificate, workspace, nonce)
    state = json.loads(native.read_regular(receipt, 4096).decode('utf-8'), object_pairs_hook=native.unique,
                      parse_constant=lambda value: (_ for _ in ()).throw(ValueError('invalid number')))
    required = {'project','nonce','certificate','workspace','directory','directoryID','created','policyID','policySHA'}
    executable = native.firefox_executable()
    directory = executable.parent/'distribution'
    path = directory/'policies.json'
    if (not isinstance(state, dict) or set(state) != required or state['project'] != 'firefox'
            or state['nonce'] != nonce or state['certificate'] != native.fingerprint(certificate)
            or state['workspace'] != native.identity(workspace, True) or type(state['created']) is not bool
            or state['directory'] != str(directory) or not native.valid_id(state['directoryID'])
            or not native.valid_id(state['policyID']) or not isinstance(state['policySHA'], str)
            or not re.fullmatch(r'[a-f0-9]{64}', state['policySHA'])):
        raise ValueError('Firefox launch receipt binding changed')
    if (state['directoryID'] != native.identity(directory, True) or state['policyID'] != native.identity(path)
            or hashlib.sha256(native.read_regular(path, 4096)).hexdigest() != state['policySHA']
            or state['directoryID'] != native.identity(directory, True) or state['policyID'] != native.identity(path)):
        raise ValueError('Firefox launch policy changed')
    expected = {'policies': {'Certificates': {'Install': [str(certificate)]}}}
    if json.loads(native.read_regular(path,4096), object_pairs_hook=native.unique) != expected:
        raise ValueError('Firefox launch policy contents changed')
    return path, executable


if __name__ == '__main__':
    try:
        if len(sys.argv) != 4: raise ValueError('invalid Firefox policy invocation')
        path, _ = policy(Path(sys.argv[1]),Path(sys.argv[2]),sys.argv[3])
        print(path)
    except (OSError, ValueError, TypeError):
        print('Firefox launch policy ownership failed', file=sys.stderr)
        raise SystemExit(2)
