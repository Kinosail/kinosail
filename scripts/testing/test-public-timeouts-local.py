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
import argparse
import sys

root = Path(__file__).resolve().parents[2]
run = root / '.verification/public-session-timeouts' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
run.mkdir(parents=True)
env = dict(os.environ, GOCACHE='/tmp/kinosail-timeout-go-cache', GOMAXPROCS='4', KINOSAIL_TIMEOUT_EVIDENCE=str(run))
commands = []
legacy_revision = 'e8eca5f762822b239cf322065a915c87482fb8b0'
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--reuse-binaries',type=Path,help='Reuse verified task artifacts when app/package sources are unchanged')
args = parser.parse_args()
binary_revision = subprocess.run(['git','rev-parse','HEAD'],cwd=root,check=True,capture_output=True,text=True).stdout.strip()
if args.reuse_binaries:
    previous = args.reuse_binaries.resolve()
    assert previous.parent == run.parent
    saved = json.loads((previous/'receipt.json').read_text())
    binary_revision = saved.get('binaryRevision',saved['revision'])
    changed = subprocess.run(['git','diff','--name-only',binary_revision,'--','apps','packages','go.work','go.work.sum'],cwd=root,check=True,capture_output=True,text=True).stdout
    assert not changed, 'App/package sources changed; rebuild rather than reuse'
    assert saved['legacyRevision'] == legacy_revision
    for app in ['player','subtitles','player-legacy','subtitles-legacy']:
        assert hashlib.sha256((previous/app).read_bytes()).hexdigest() == saved['binarySHA256'][app]
        os.link(previous/app,run/app)
    commands = saved['buildCommands']
else:
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
receipt['binaryRevision'] = binary_revision
receipt['command'] = ['python3',str(Path(__file__).relative_to(root)),*sys.argv[1:]]
if args.reuse_binaries: receipt['reusedBinaryDirectory'] = str(previous)
receipt['buildCommands'] = commands
receipt['browserCommand'] = command
receipt['runnerSHA256'] = {str(path.relative_to(root)): hashlib.sha256(path.read_bytes()).hexdigest()
    for path in [Path(__file__), root/command[1], root/'scripts/testing/public-timeouts-journeys.mjs', root/'scripts/testing/public-timeouts-fixture.py']}
(run/'receipt.json').write_text(json.dumps(receipt, indent=2)+'\n')
(run/'SHA256SUMS').write_text(''.join(f'{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}\n'
    for path in sorted(run.iterdir()) if path.is_file() and path.name!='SHA256SUMS'))
print(f'E2E artifact: {run}', flush=True)
raise SystemExit(result)
