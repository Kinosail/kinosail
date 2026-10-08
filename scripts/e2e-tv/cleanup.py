#!/usr/bin/env python3
"""Cancellation fallback: admit only this workflow's recorded owned device/process."""
import json
import os
from pathlib import Path
import re
import signal
import stat
import subprocess
import sys
import time
from simulator import validate_pending, cleanup_creation, strict_json


def main():
    if len(sys.argv) != 1 or os.environ.get('GITHUB_ACTIONS') != 'true':
        raise RuntimeError('Hosted cleanup only')
    run = os.environ.get('GITHUB_RUN_ID', '') + '-' + os.environ.get('GITHUB_RUN_ATTEMPT', '')
    if not re.fullmatch(r'\d{1,20}-\d{1,5}', run):
        raise RuntimeError('Invalid cleanup owner')
    root = Path(__file__).resolve().parent / '.e2e'
    if not root.exists():
        return
    current = root.lstat()
    if not stat.S_ISDIR(current.st_mode) or current.st_uid != os.getuid():
        raise RuntimeError('Unsafe cleanup root')
    path = root / 'owned-device.json'
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    try:
        info = os.fstat(fd)
        if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1 or info.st_uid != os.getuid() or stat.S_IMODE(info.st_mode) != 0o600 or info.st_size > 4096:
            raise RuntimeError('Unsafe ownership sidecar')
        owner = strict_json(os.read(fd, 4097))
    finally:
        os.close(fd)
    if owner.get('run') != run or owner.get('rootDev') != current.st_dev or owner.get('rootIno') != current.st_ino:
        raise RuntimeError('Cleanup owner mismatch')
    if owner.get('target') != 'tv' or owner.get('profile') != ('tvos' if owner.get('platform') == 'ios' else 'androidtv'): raise RuntimeError('Invalid TV cleanup profile')
    pending = validate_pending(owner)
    def save_owner():
        path.write_text(json.dumps(owner))
        os.chmod(path, 0o600)
    # app.command owns a process group separate from the SDK runner. Pin start time before signaling.
    processes_path = root / 'process-owned.json'
    if processes_path.exists():
        fd = os.open(processes_path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
        try:
            info = os.fstat(fd)
            if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1 or stat.S_IMODE(info.st_mode) != 0o600 or info.st_uid != os.getuid() or info.st_size > 4096:
                raise RuntimeError('Unsafe owned process sidecar')
            processes = strict_json(os.read(fd, 4097))['processes']
        finally:
            os.close(fd)
        if not isinstance(processes, list) or len(processes) > 3:
            raise RuntimeError('Invalid owned process cardinality')
        for process in processes:
            pid = process.get('pid')
            if not isinstance(pid, int) or pid < 2 or not isinstance(process.get('start'), str):
                raise RuntimeError('Invalid owned process')
            current_start = subprocess.run(['ps', '-p', str(pid), '-o', 'lstart='], capture_output=True, text=True, timeout=10).stdout.strip()
            if not current_start:
                continue
            if current_start != process['start']:
                raise RuntimeError('Owned process identity changed; preserved')
            os.kill(pid, signal.SIGTERM)
        deadline = time.monotonic() + 20
        while time.monotonic() < deadline:
            active = [p for p in processes if subprocess.run(['ps', '-p', str(p['pid']), '-o', 'lstart='], capture_output=True, text=True, timeout=10).stdout.strip() == p['start']]
            if not active:
                break
            time.sleep(.2)
        else:
            raise RuntimeError('Owned process did not exit; publication withheld')
    if pending is not None:
        cleanup_creation(owner, save_owner)
    if owner.get('complete'):
        return
    device = owner.get('device')
    if owner.get('platform') == 'ios' and device:
        if not re.fullmatch(r'[A-Fa-f0-9]{8}(?:-[A-Fa-f0-9]{4}){3}-[A-Fa-f0-9]{12}', device):
            raise RuntimeError('Invalid owned simulator')
        inventory = json.loads(subprocess.check_output(['xcrun', 'simctl', 'list', 'devices', '--json'], timeout=30))
        found = [d for group in inventory['devices'].values() for d in group if d['udid'] == device]
        if found:
            if len(found) != 1 or found[0]['name'] != 'Kinosail-TV-E2E-' + run:
                raise RuntimeError('Simulator ownership changed')
            subprocess.run(['xcrun', 'simctl', 'shutdown', device], capture_output=True, timeout=60)
            subprocess.run(['xcrun', 'simctl', 'delete', device], capture_output=True, timeout=60, check=True)
    elif owner.get('platform') == 'android' and owner.get('emulatorPID'):
        pid = owner['emulatorPID']
        if not isinstance(pid, int) or pid < 2 or device != 'emulator-5554':
            raise RuntimeError('Invalid owned emulator')
        proc = Path('/proc') / str(pid)
        if proc.exists():
            start = (proc / 'stat').read_text().split(') ', 1)[1].split()[19]
            args = (proc / 'cmdline').read_bytes().split(b'\0')
            if start != owner['emulatorStart'] or ('Kinosail-TV-E2E-' + run).encode() not in args:
                raise RuntimeError('Emulator process ownership changed')
            os.killpg(pid, signal.SIGTERM)
            time.sleep(2)
            if proc.exists() and (proc / 'stat').read_text().split(') ', 1)[1].split()[19] == start:
                os.killpg(pid, signal.SIGKILL)
    state = root / 'agent-device'
    if state.exists():
        if state.is_symlink() or not state.is_dir():
            raise RuntimeError('Unsafe daemon state')
        subprocess.run(['node', 'node_modules/agent-device/bin/agent-device.mjs', 'daemon', 'stop', '--state-dir', str(state)], cwd=root.parent, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=120, check=True)
    owner['complete'] = True
    save_owner()


if __name__ == '__main__':
    try:
        main()
    except Exception:
        print('Owned TV cleanup incomplete; publication withheld.', file=sys.stderr)
        sys.exit(1)
