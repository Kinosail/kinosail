"""Owned process cleanup for the real native-reentry E2E harness."""
import json
import os
import signal
import subprocess
import time

def stop_owned_group(process):
    """Reap only a new session created by this invocation, including its children."""
    if process is None:
        return True
    group = process.pid
    def exists():
        try:
            os.killpg(group, 0)
            return True
        except ProcessLookupError:
            return False
    if exists():
        try:
            os.killpg(group, signal.SIGTERM)
        except ProcessLookupError:
            pass
    for _ in range(50):
        process.poll()
        if not exists():
            break
        time.sleep(.1)
    if exists():
        try:
            os.killpg(group, signal.SIGKILL)
        except ProcessLookupError:
            pass
    process.wait(timeout=5)
    return not exists()

def run_owned_command(command, environment, timeout, cwd=None, output=None):
    process = subprocess.Popen(command, cwd=cwd, env=environment, stdout=output, stderr=subprocess.STDOUT if output else None, start_new_session=True)
    try:
        code = process.wait(timeout=timeout)
        if code:
            raise subprocess.CalledProcessError(code, command)
    finally:
        if not stop_owned_group(process):
            raise RuntimeError('An owned build/media process group did not exit')

def stop_observed_node_children(browser, process_log, receipt):
    if browser is None:
        return True
    if process_log.stat().st_size > 131072:
        raise RuntimeError('Owned child receipt exceeded its bound')
    known, children = {browser.pid}, {}
    values = [json.loads(line) for line in process_log.read_text().splitlines()]
    for value in values:
        if set(value) != {'parentPID', 'childPID', 'groupPID', 'startIdentity'}:
            raise RuntimeError('Invalid owned child receipt')
    pending = values[:]
    while pending:
        ready = [value for value in pending if value['parentPID'] in known]
        if not ready:
            raise RuntimeError('Unknown child ancestry in owned receipt')
        for value in ready:
            pid = value['childPID']
            if not isinstance(pid, int) or not 1 < pid < 2**31:
                raise RuntimeError('Invalid owned child PID')
            known.add(pid)
            group = value['groupPID']
            if not isinstance(group, int) or not 0 <= group < 2**31:
                raise RuntimeError('Invalid observed child group')
            children[pid] = (value['startIdentity'], group)
            pending.remove(value)
    receipt['observedNodeChildren'] = len(children)
    def matches(pid, identity):
        try:
            current = subprocess.check_output(['/bin/ps', '-p', str(pid), '-o', 'lstart='], text=True).strip()
            if current and not identity:
                raise RuntimeError('A surviving observed child has no creation identity')
            return current == identity and bool(identity)
        except subprocess.CalledProcessError:
            return False
    def group_alive(pid, identity, group):
        if group != pid or not identity:
            return False
        try:
            os.killpg(group, 0)
        except ProcessLookupError:
            return False
        # A PGID remains reserved while its group survives its leader. If a
        # current leader exists, reject a changed creation identity before kill.
        try:
            current = subprocess.check_output(['/bin/ps', '-p', str(pid), '-o', 'lstart='], text=True).strip()
            if current and current != identity:
                raise RuntimeError('Observed browser group creation identity changed')
        except subprocess.CalledProcessError:
            pass
        return True
    def alive(pid, value):
        identity, group = value
        return group_alive(pid, identity, group) or matches(pid, identity)
    for sig in [signal.SIGTERM, signal.SIGKILL]:
        for pid, (identity, group) in reversed(list(children.items())):
            owned_group_alive = group_alive(pid, identity, group)
            if owned_group_alive or matches(pid, identity):
                try:
                    if owned_group_alive:
                        os.killpg(group, sig)
                    else:
                        os.kill(pid, sig)
                except ProcessLookupError:
                    pass
        for _ in range(20):
            if not any(alive(pid, value) for pid, value in children.items()):
                return True
            time.sleep(.1)
    return not any(alive(pid, value) for pid, value in children.items())
