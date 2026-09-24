#!/usr/bin/env python3
"""Install published native changes on paired development devices; run via launchd."""
import argparse
import fcntl
import json
import os
from pathlib import Path
import re
import plistlib
import shutil
import sys
import subprocess
import tempfile

NATIVE = 'apps/player/apps/native'
BUNDLE = 'com.kinosail.player'
REPOSITORY = 'https://github.com/Kinosail/kinosail.git'
DEVICE_IDENTIFIER = re.compile(
    r'(?:[0-9A-Fa-f]{8}(?:-[0-9A-Fa-f]{4}){3}-[0-9A-Fa-f]{12}|'
    r'[0-9A-Fa-f]{8}-[0-9A-Fa-f]{16})'
)


def identifier(value):
    if not DEVICE_IDENTIFIER.fullmatch(value):
        raise argparse.ArgumentTypeError(
            'expected a paired CoreDevice UUID or Apple device identifier'
        )
    return value.upper()



def read_record(path):
    if not path.exists():
        return {}
    if path.stat().st_size > 1024:
        raise ValueError('oversized delivery record')
    record = json.loads(path.read_text())
    if not isinstance(record, dict) or set(record) != {'revision', 'tree', 'build'}:
        raise ValueError('invalid delivery record fields')
    for key, pattern in (('revision', r'[0-9a-f]{40}'), ('tree', r'[0-9a-f]{40}'), ('build', r'[1-9][0-9]{0,8}')):
        if not isinstance(record[key], str) or not re.fullmatch(pattern, record[key]):
            raise ValueError(f'invalid delivery record {key}')
    return record


def write_record(path, record):
    temporary = path.with_suffix('.tmp')
    temporary.write_text(json.dumps(record))
    temporary.replace(path)


def run(*args, cwd=None, capture=False, env=None, timeout=3600):
    result = subprocess.run(args, cwd=cwd, env=env, check=True, timeout=timeout,
                            text=True, stdout=subprocess.PIPE if capture else None)
    return result.stdout.strip() if capture else None


def implementation_state(raw):
    if not isinstance(raw, str) or len(raw.encode()) > 65536:
        raise ValueError('invalid Apple implementation metadata')
    configuration = plistlib.loads(raw.encode())
    if not isinstance(configuration, dict):
        raise ValueError('invalid Apple implementation metadata')
    state = configuration.get('KinosailImplementationState')
    if not isinstance(state, str) or state not in ('scaffold', 'implemented'):
        raise ValueError('unknown Apple implementation state')
    return state


def next_build(count, installed, built):
    if not isinstance(count, str) or not re.fullmatch(r'[1-9][0-9]{0,7}', count):
        raise ValueError('invalid revision count')
    number = max(1000 + int(count), int(installed.get('build', '0')) + 1,
                 int(built.get('build', '0')) + 1)
    if number > 99_999_999:
        raise ValueError('Apple build number limit reached')
    return str(number)


def deploy(root, mirror, revision, tree, name, device, team):
    state = root / f'{name}.json'
    installed = read_record(state)
    if installed.get('tree') == tree:
        return
    platform = 'iOS' if name == 'iphone' else 'tvOS'
    configuration = run('git', f'--git-dir={mirror}', 'show',
                        f'{revision}:{NATIVE}/Configuration/{platform}-Info.plist', capture=True)
    if implementation_state(configuration) == 'scaffold':
        print(f'{name}: Swift scaffold is build-only; installed device app is unchanged', flush=True)
        return
    source = root / name
    artifact = root / f'{name}-build/Build/Products/Release-{ "iphoneos" if name == "iphone" else "appletvos"}/KinosailPlayer.app'
    built = root / f'{name}-built.json'
    record = read_record(built)
    if record.get('tree') != tree or not artifact.exists():
        build = next_build(run('git', f'--git-dir={mirror}', 'rev-list', '--count', revision, capture=True), installed, record)
        source.mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(dir=root) as temporary:
            archive = Path(temporary) / 'source.tar'
            run('git', f'--git-dir={mirror}', 'archive', f'--output={archive}', f'{revision}:{NATIVE}')
            unpacked = Path(temporary) / 'source'
            unpacked.mkdir()
            run('tar', '-xf', str(archive), '-C', str(unpacked))
            run('rsync', '-a', '--delete', '--exclude=/.build/', '--exclude=/node_modules/',
                '--exclude=/ios/', '--exclude=/dist/', f'{unpacked}/', f'{source}/')
        run('xcodebuild', '-project', 'Kinosail.xcodeproj', '-scheme', f'Kinosail-{platform}',
            '-configuration', 'Release', '-destination', f'generic/platform={"iOS" if name == "iphone" else "tvOS"}',
            '-derivedDataPath', str(root / f'{name}-build'), '-jobs', '4',
            '-allowProvisioningUpdates', '-allowProvisioningDeviceRegistration',
            f'DEVELOPMENT_TEAM={team}', 'CODE_SIGN_STYLE=Automatic', f'CURRENT_PROJECT_VERSION={build}',
            f'KINOSAIL_SOURCE_REVISION={revision}', 'build', cwd=source)
        record = dict(revision=revision, tree=tree, build=build)
        write_record(built, record)
    run('xcrun', 'devicectl', '--timeout', '120', 'device', 'install', 'app', '--device', device, str(artifact), timeout=180)
    with tempfile.TemporaryDirectory(dir=root) as temporary:
        result = Path(temporary) / 'apps.json'
        run('xcrun', 'devicectl', '--timeout', '60', 'device', 'info', 'apps', '--device', device,
            '--json-output', str(result), timeout=90)
        apps = json.loads(result.read_text())['result']['apps']
        if not any(app.get('bundleIdentifier') == BUNDLE and app.get('bundleVersion') == record['build'] for app in apps):
            raise RuntimeError(f'{name}: installed build was not confirmed')
    write_record(state, record)
    print(f'{name}: installed and confirmed {record}', flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__, allow_abbrev=False)
    parser.add_argument('--team', required=True)
    parser.add_argument('--iphone', required=True, type=identifier)
    parser.add_argument('--tv', required=True, type=identifier)
    parser.add_argument('--install', action='store_true', help='install a launch agent polling every minute')
    args = parser.parse_args()
    if not re.fullmatch(r'[A-Z0-9]{10}', args.team) or args.iphone == args.tv:
        parser.error('team must be ten uppercase alphanumeric characters; devices must differ')
    root = Path.home() / 'Library/Caches/KinosailAppleDeploy'
    root.mkdir(parents=True, exist_ok=True)
    if args.install:
        installed = Path.home() / 'Library/Application Support/KinosailAppleDeploy/deploy-apple-devices.py'
        installed.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(__file__, installed)
        label = 'com.kinosail.deploy-apple-devices'
        plist = Path.home() / f'Library/LaunchAgents/{label}.plist'
        plist.parent.mkdir(parents=True, exist_ok=True)
        plist.write_bytes(plistlib.dumps(dict(
            Label=label, ProgramArguments=[sys.executable, str(installed), '--team', args.team,
                '--iphone', args.iphone, '--tv', args.tv],
            EnvironmentVariables=dict(PATH='/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin', LANG='en_US.UTF-8'),
            RunAtLoad=True, StartInterval=60, ProcessType='Standard',
            StandardOutPath=str(root / 'deploy.log'), StandardErrorPath=str(root / 'deploy.log'))))
        subprocess.run(['launchctl', 'bootout', f'gui/{os.getuid()}/{label}'], check=False, capture_output=True)
        run('launchctl', 'bootstrap', f'gui/{os.getuid()}', str(plist))
        print('Apple device auto-deploy installed; checking native main changes every minute')
        return
    with (root / 'lock').open('w') as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            return
        mirror = root / 'repo.git'
        if not mirror.exists():
            run('git', 'clone', '--bare', REPOSITORY, str(mirror))
        run('git', f'--git-dir={mirror}', 'fetch', 'origin', '+refs/heads/main:refs/heads/main')
        revision = run('git', f'--git-dir={mirror}', 'rev-parse', 'refs/heads/main', capture=True)
        tree = run('git', f'--git-dir={mirror}', 'rev-parse', f'{revision}:{NATIVE}', capture=True)
        failures = []
        for name in ('iphone', 'tv'):
            try:
                deploy(root, mirror, revision, tree, name, getattr(args, name), args.team)
            except (subprocess.SubprocessError, OSError, ValueError, KeyError, RuntimeError) as error:
                print(f'{name}: update pending: {error}', flush=True)
                failures.append(name)
        if failures:
            raise SystemExit(1)


if __name__ == '__main__':
    main()
