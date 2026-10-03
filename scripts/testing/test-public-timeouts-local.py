#!/usr/bin/env python3
"""Repeatable public timeout proof with disposable authentication and no TLS bypass."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time
import tarfile
import io
import shutil

root = Path(__file__).resolve().parents[2]
run = root / '.verification/public-session-timeouts' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
run.mkdir(parents=True)
env = dict(os.environ, GOCACHE='/tmp/kinosail-timeout-go-cache', GOMAXPROCS='4', KINOSAIL_TIMEOUT_EVIDENCE=str(run))
commands = []
legacy_revision = 'e8eca5f762822b239cf322065a915c87482fb8b0'
legacy_source = run/'legacy-source'
legacy_source.mkdir()
archive = subprocess.run(['git','archive',legacy_revision],cwd=root,check=True,stdout=subprocess.PIPE).stdout
with tarfile.open(fileobj=io.BytesIO(archive)) as source:
    source.extractall(legacy_source,filter='data')
for app in ['player', 'subtitles']:
    command = ['go', '-C', f'apps/{app}', 'build', '-p=2', '-o', str(run/app), './cmd/kinosail']
    commands.append(command)
    subprocess.run(command, cwd=root, env=env, check=True)
for app in ['player','subtitles']:
    command = ['go','-C',str(legacy_source/f'apps/{app}'),'build','-p=2','-o',str(run/(app+'-legacy')),'./cmd/kinosail']
    commands.append(command)
    subprocess.run(command,cwd=legacy_source,env=env,check=True)
shutil.rmtree(legacy_source)
command = ['node', 'scripts/testing/public-timeouts-local.mjs']
result = subprocess.run(command, cwd=root, env=env).returncode
receipt = json.loads((run/'receipt.json').read_text()) if (run/'receipt.json').exists() else {'result':'failed','command':command,'error':'Browser runner exited before completion','exitCode':result}
receipt['legacyRevision'] = legacy_revision
receipt['buildCommands'] = commands
receipt['browserCommand'] = command
receipt['runnerSHA256'] = {str(path.relative_to(root)): hashlib.sha256(path.read_bytes()).hexdigest()
    for path in [Path(__file__), root/command[1], root/'scripts/testing/public-timeouts-journeys.mjs', root/'scripts/testing/public-timeouts-fixture.py']}
(run/'receipt.json').write_text(json.dumps(receipt, indent=2)+'\n')
(run/'SHA256SUMS').write_text(''.join(f'{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}\n'
    for path in sorted(run.iterdir()) if path.is_file() and path.name!='SHA256SUMS'))
print(f'E2E artifact: {run}', flush=True)
raise SystemExit(result)
