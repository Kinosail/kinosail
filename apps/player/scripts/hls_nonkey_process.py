"""Bounded ownership of the disposable renderer and its process descendants."""
import os
import json
import re
import signal
import subprocess
import time


def group_signal(group, value):
    try:
        os.killpg(group, value)
    except ProcessLookupError:
        pass


def process_rows():
    raw = subprocess.check_output(['ps', '-eo', 'pid=,ppid=,pgid=,stat='], timeout=2)
    if len(raw) > 1024 * 1024:
        raise RuntimeError('renderer_process_evidence_bound')
    result = {}
    for row in raw.decode().splitlines():
        fields = row.split()
        if len(fields) != 4 or not all(v.isdigit() for v in fields[:3]) or not re.fullmatch('[A-Za-z+<>=?]+', fields[3]):
            raise RuntimeError('renderer_process_evidence_shape')
        result[int(fields[0])] = (int(fields[1]), int(fields[2]), fields[3])
    return result


def owned_command(command, private, timeout, owner=None):
    process = subprocess.Popen(command, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                               stderr=subprocess.PIPE, start_new_session=True)
    result = {'timedOut': False, 'ownedGroupJoined': False, 'joinedSamples': 0,
              'liveOwnedProcesses': None, 'stdout': b'', 'stderr': b'', 'browserOwnershipVerified': False}
    groups = {process.pid}
    deadline = time.monotonic() + timeout
    payload = private

    def register_browser():
        if owner is None or result['browserOwnershipVerified'] or not owner.exists():
            return
        with owner.open('rb') as file:
            raw = file.read(513)
        if len(raw) > 512:
            raise RuntimeError('renderer_browser_owner_bound')
        value = json.loads(raw)
        pid = value.get('browserPID')
        if set(value) != {'browserPID'} or type(pid) is not int or pid <= 1:
            raise RuntimeError('renderer_browser_owner_shape')
        row = process_rows().get(pid)
        if row is None or row[0] != process.pid or row[1] not in [process.pid, pid] or row[2].startswith('Z'):
            raise RuntimeError('renderer_browser_owner_identity')
        groups.add(row[1])
        result['browserOwnershipVerified'] = True
        result['browserPID'], result['browserGroup'] = pid, row[1]
        with owner.with_name(owner.name + '.ack').open('x') as file:
            file.write('verified\n')

    def signal_owned(value):
        for group in groups:
            group_signal(group, value)
    try:
        while True:
            register_browser()
            budget = deadline - time.monotonic()
            if budget <= 0:
                result['timedOut'] = True
                break
            try:
                result['stdout'], result['stderr'] = process.communicate(payload, timeout=min(0.05, budget))
                break
            except subprocess.TimeoutExpired:
                payload = None
        if result['timedOut']:
            signal_owned(signal.SIGTERM)
            try:
                result['stdout'], result['stderr'] = process.communicate(timeout=2)
            except subprocess.TimeoutExpired:
                signal_owned(signal.SIGKILL)
                result['stdout'], result['stderr'] = process.communicate(timeout=2)
    finally:
        # Also handle a normally exited leader that left descendants behind.
        signal_owned(signal.SIGTERM)
        if process.poll() is None:
            try:
                process.wait(timeout=2)
            except subprocess.TimeoutExpired:
                signal_owned(signal.SIGKILL)
                process.wait(timeout=2)
        for pipe in [process.stdin, process.stdout, process.stderr]:
            if pipe is not None:
                pipe.close()
        deadline = time.monotonic() + 2
        while time.monotonic() < deadline:
            result['liveOwnedProcesses'] = sum(group in groups and not state.startswith('Z')
                for _, group, state in process_rows().values())
            result['joinedSamples'] = result['joinedSamples'] + 1 if result['liveOwnedProcesses'] == 0 else 0
            if result['joinedSamples'] >= 2:
                result['ownedGroupJoined'] = True
                break
            signal_owned(signal.SIGKILL)
            time.sleep(0.05)
        result['exitCode'] = process.returncode
    return result
