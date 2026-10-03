#!/usr/bin/env python3
"""Repeatable real-app session persistence proof with disposable authentication."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time

root = Path(__file__).resolve().parents[2]
run = root / '.verification/signin-persistence' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
run.mkdir(parents=True)
env = dict(os.environ, GOCACHE='/tmp/kinosail-signin-go-cache', GOMAXPROCS='4', KINOSAIL_SIGNIN_EVIDENCE=str(run))
commands = []
for app in ['player', 'subtitles']:
    command = ['go', '-C', f'apps/{app}', 'build', '-p=2', '-o', str(run/app), './cmd/kinosail']
    commands.append(command)
    subprocess.run(command, cwd=root, env=env, check=True)
command = ['node', 'scripts/testing/session-persistence-local.mjs']
result = subprocess.run(command, cwd=root, env=env).returncode
receipt = json.loads((run/'receipt.json').read_text())
receipt['buildCommands'] = commands
receipt['browserCommand'] = command
receipt['runnerSHA256'] = hashlib.sha256((root/command[1]).read_bytes()).hexdigest()
(run/'receipt.json').write_text(json.dumps(receipt, indent=2)+'\n')
(run/'SHA256SUMS').write_text(''.join(f'{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}\n'
    for path in sorted(run.iterdir()) if path.is_file() and path.name!='SHA256SUMS'))
print(f'E2E artifact: {run}', flush=True)
raise SystemExit(result)
