#!/usr/bin/env python3
"""Collect bounded official source graphs; reproduce canonical HTML without local Go."""
import contextlib
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import tempfile
import time
from types import SimpleNamespace

from campaign_architecture_admission import validate_snapshot

ROOT = Path(__file__).resolve().parents[2]
GENERATOR = ROOT / 'scripts/tooling/generate-architecture-explorer.py'
FILES = ('scripts/tooling/generate-architecture-explorer.py',
         'scripts/tooling/architecture-explorer-template.html',
         'scripts/tooling/architecture-explorer-template.css',
         'scripts/tooling/architecture-explorer-template.js',
         'scripts/ci/campaign-architecture.py', 'scripts/ci/campaign_architecture_admission.py',
         'go.work', 'go.work.sum', 'apps/player/go.mod', 'apps/player/go.sum',
         'apps/subtitles/go.mod', 'apps/subtitles/go.sum', 'packages/go.mod', 'packages/go.sum')
SAFE_NAMES = ('receipt.json', 'results.json', 'source-manifest.json', 'artifact-manifest.json')


def pin(path):
    body = path.read_bytes()
    return {'bytes': len(body), 'sha256': hashlib.sha256(body).hexdigest()}


def tree_pin():
    paths = subprocess.check_output(['git', 'ls-files', '-z'], cwd=ROOT).decode().split('\0')[:-1]
    rows = []
    for name in sorted(paths):
        path = ROOT / name
        body = os.readlink(path).encode() if path.is_symlink() else path.read_bytes()
        rows.append(name + '\t' + hashlib.sha256(body).hexdigest())
    return {'files': len(paths), 'sha256': hashlib.sha256(('\n'.join(rows) + '\n').encode()).hexdigest()}


def revision():
    return subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT).decode().strip()


def generator(app):
    old = sys.argv; sys.argv = [str(GENERATOR), app]
    try:
        spec = importlib.util.spec_from_file_location('canonical_architecture_' + app, GENERATOR)
        module = importlib.util.module_from_spec(spec); spec.loader.exec_module(module)
        return module
    finally:
        sys.argv = old


def render(app, snapshot):
    validate_snapshot(snapshot, app, ROOT)
    module = generator(app); module.build_snapshot = lambda: snapshot
    with tempfile.TemporaryDirectory(prefix='campaign-architecture-') as temporary:
        module.PUBLISHED = Path(temporary) / 'index.html'
        with contextlib.redirect_stdout(io.StringIO()): module.main()
        body = module.PUBLISHED.read_bytes()
    return body


def settle_owned_group(process, terminate):
    def send(sig):
        try: os.killpg(process.pid, sig)
        except ProcessLookupError: return False
        return True
    if terminate: send(signal.SIGTERM)
    try: process.wait(timeout=2)
    except subprocess.TimeoutExpired:
        send(signal.SIGKILL); process.wait(timeout=2)
    if not send(0): return
    send(signal.SIGKILL)
    deadline = time.monotonic() + 2
    while send(0):
        if time.monotonic() >= deadline: raise ValueError('source graph group not settled')
        time.sleep(.02)


def bounded_go_list(go, command, *, cwd, **_ignored):
    if command != ['go', 'list', '-json', './...'] or cwd not in (ROOT / 'apps/player', ROOT / 'apps/subtitles', ROOT / 'packages'):
        raise ValueError('unexpected generator command')
    environment = {k: v for k, v in os.environ.items() if not k.startswith(('KINOSAIL_', 'GO'))}
    environment.update(GOOS='linux', GOARCH='amd64', CGO_ENABLED='0', GOMAXPROCS='2',
                       GOTOOLCHAIN='local', GOPROXY='off', GOWORK=str(ROOT / 'go.work'))
    # Keep the normal checksum verifier; no network access is needed after warmup.
    if 'GOSUMDB' in os.environ: environment['GOSUMDB'] = os.environ['GOSUMDB']
    with tempfile.TemporaryFile() as output:
        process = subprocess.Popen([go, *command[1:]], cwd=cwd, env=environment,
                                   stdout=output, stderr=subprocess.DEVNULL, start_new_session=True)
        timed_out = False
        try: process.wait(timeout=20)
        except subprocess.TimeoutExpired: timed_out = True
        finally: settle_owned_group(process, timed_out or process.returncode is None)
        if timed_out: raise ValueError('source graph command deadline')
        if process.returncode != 0: raise ValueError('source graph command failed')
        output.seek(0); body = output.read(8 * 1024 * 1024 + 1)
        if len(body) > 8 * 1024 * 1024: raise ValueError('source graph output bound')
        if not body.strip(): raise ValueError('empty source graph output')
    return SimpleNamespace(stdout=body.decode('utf-8'))


def collect():
    before = tree_pin(); head = revision()
    if os.environ.get('GITHUB_SHA') != head or subprocess.run(['git', 'diff', '--quiet', 'HEAD', '--'], cwd=ROOT).returncode:
        raise ValueError('source graph revision or cleanliness')
    go = shutil.which('go')
    if not go: raise ValueError('Go unavailable')
    graph = {'schemaVersion': 1, 'revision': head, 'trackedTree': before,
             'sourceFiles': {name: pin(ROOT / name) for name in FILES},
             'goExecutable': pin(Path(go).resolve()),
             'environment': {'GOOS': 'linux', 'GOARCH': 'amd64', 'CGO_ENABLED': '0', 'GOPROXY': 'off', 'GOTOOLCHAIN': 'local'},
             'commands': {'count': 4, 'eachSeconds': 20, 'ownedGroupsSettled': True}, 'apps': {}}
    for app in ('player', 'subtitles'):
        module = generator(app)
        module.subprocess = SimpleNamespace(run=lambda args, **kw: bounded_go_list(go, args, **kw))
        snapshot = module.build_snapshot(); body = render(app, snapshot)
        graph['apps'][app] = {'snapshot': snapshot, 'renderedHTML': {'bytes': len(body), 'sha256': hashlib.sha256(body).hexdigest()}}
    if tree_pin() != before or revision() != head: raise ValueError('source graph changed')
    return graph


def write_json(path, value):
    path.write_bytes(json_bytes(value))


def json_bytes(value):
    body = (json.dumps(value, separators=(',', ':'), sort_keys=True) + '\n').encode()
    if len(body) > 4 * 1024 * 1024: raise ValueError('safe metadata file bound')
    return body


def refresh_manifest(original, pins):
    if set(original) == {'safeFiles'}:
        if set(original['safeFiles']) != set(SAFE_NAMES[:3]): raise ValueError('unexpected Q09 artifact manifest')
        return {'safeFiles': {name: pins[name]['sha256'] for name in SAFE_NAMES[:3]}}
    if set(original) == {'artifacts'}:
        rows = original['artifacts']
        if not isinstance(rows, list) or [row['path'] for row in rows] != list(SAFE_NAMES[:3]):
            raise ValueError('unexpected R06 artifact manifest')
        return {'artifacts': [{'path': name, 'present': True, **pins[name]} for name in SAFE_NAMES[:3]]}
    if set(original) != set(SAFE_NAMES[:3]): raise ValueError('unexpected artifact manifest')
    return pins


def attach(identifier):
    if identifier not in ('R06', 'Q14', 'Q09'): raise ValueError('unknown campaign')
    output = ROOT / '.verification/campaign-proof' / identifier
    values = {name: json.loads((output / name).read_text()) for name in SAFE_NAMES}
    graph = collect(); values['source-manifest.json']['architectureMetadata'] = graph
    encoded = json.dumps(graph, sort_keys=True, separators=(',', ':')).encode()
    values['receipt.json']['architectureMetadata'] = {'accepted': True, 'revision': graph['revision'],
                                                    'bytes': len(encoded), 'sha256': hashlib.sha256(encoded).hexdigest()}
    bodies = {name: json_bytes(values[name]) for name in SAFE_NAMES[:3]}
    pins = {name: {'bytes': len(body), 'sha256': hashlib.sha256(body).hexdigest()} for name, body in bodies.items()}
    # Validate all encodings, bounds and the selected original schema before writes.
    bodies['artifact-manifest.json'] = json_bytes(refresh_manifest(values['artifact-manifest.json'], pins))
    for name in SAFE_NAMES: (output / name).write_bytes(bodies[name])


def reproduce(manifest_path):
    graph = json.loads(Path(manifest_path).read_text())['architectureMetadata']
    if graph['schemaVersion'] != 1 or graph['revision'] != revision() or graph['trackedTree'] != tree_pin():
        raise ValueError('metadata source identity differs')
    if graph['sourceFiles'] != {name: pin(ROOT / name) for name in FILES} or set(graph['apps']) != {'player', 'subtitles'}:
        raise ValueError('metadata template or app identity differs')
    outputs = {}
    for app in ('player', 'subtitles'):
        record = graph['apps'][app]; body = render(app, record['snapshot'])
        if record['renderedHTML'] != {'bytes': len(body), 'sha256': hashlib.sha256(body).hexdigest()}:
            raise ValueError('canonical rendered bytes differ')
        outputs[app] = body
    # Both bytes are validated before the first tracked document write.
    for app, body in outputs.items():
        (ROOT / 'apps' / app / 'docs/architecture-explorer/index.html').write_bytes(body)


if __name__ == '__main__':
    os.umask(0o077)
    try:
        if len(sys.argv) == 2: attach(sys.argv[1])
        elif len(sys.argv) == 3 and sys.argv[1] == '--reproduce': reproduce(sys.argv[2])
        else: raise ValueError('invalid metadata invocation')
    except (OSError, ValueError, KeyError, subprocess.SubprocessError) as error:
        print('Canonical source metadata failed: ' + type(error).__name__, file=sys.stderr)
        raise SystemExit(1) from None
