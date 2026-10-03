#!/usr/bin/env python3
"""Repeatable public timeout proof with disposable authentication and no TLS bypass."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time

root = Path(__file__).resolve().parents[2]
run = root / '.verification/public-session-timeouts' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
run.mkdir(parents=True)
env = dict(os.environ, GOCACHE='/tmp/kinosail-timeout-go-cache', GOMAXPROCS='4', KINOSAIL_TIMEOUT_EVIDENCE=str(run))
commands = []
for app in ['player', 'subtitles']:
    command = ['go', '-C', f'apps/{app}', 'build', '-p=2', '-o', str(run/app), './cmd/kinosail']
    commands.append(command)
    subprocess.run(command, cwd=root, env=env, check=True)
command = ['node', 'scripts/testing/public-timeouts-local.mjs']
result = subprocess.run(command, cwd=root, env=env).returncode
receipt = json.loads((run/'receipt.json').read_text())
receipt['buildCommands'] = commands
receipt['browserCommand'] = command
receipt['runnerSHA256'] = {str(path.relative_to(root)): hashlib.sha256(path.read_bytes()).hexdigest()
    for path in [Path(__file__), root/command[1], root/'scripts/testing/public-timeouts-journeys.mjs', root/'scripts/testing/public-timeouts-fixture.py']}
(run/'receipt.json').write_text(json.dumps(receipt, indent=2)+'\n')
(run/'SHA256SUMS').write_text(''.join(f'{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}\n'
    for path in sorted(run.iterdir()) if path.is_file() and path.name!='SHA256SUMS'))
print(f'E2E artifact: {run}', flush=True)
raise SystemExit(result)
