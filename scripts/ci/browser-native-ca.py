#!/usr/bin/env python3
"""Exact native CA additions for one disposable Linux Provider fixture."""
import hashlib
import itertools
import json
import os
from pathlib import Path
import re
import selectors
import time
import tempfile
import stat
import subprocess
import sys


def run(argv, data=None, cwd=None):
    source = tempfile.TemporaryFile()
    if data is not None: source.write(data)
    source.seek(0)
    try: process = subprocess.Popen(argv, stdin=source, stdout=subprocess.PIPE, stderr=subprocess.PIPE, cwd=cwd)
    except BaseException: source.close(); raise
    streams = selectors.DefaultSelector(); captured = [bytearray(), bytearray()]
    deadline = time.monotonic() + 10
    try:
        streams.register(process.stdout, selectors.EVENT_READ, 0)
        streams.register(process.stderr, selectors.EVENT_READ, 1)
        while streams.get_map():
            remaining = deadline - time.monotonic()
            if remaining <= 0: raise ValueError('native trust tool deadline')
            for key, _ in streams.select(min(remaining, .1)):
                block = os.read(key.fd, 4096)
                if not block: streams.unregister(key.fileobj); continue
                captured[key.data].extend(block)
                if len(captured[key.data]) > (65536 if key.data == 0 else 8192):
                    raise ValueError('native trust tool output overflow')
        if process.wait(timeout=max(.01, deadline-time.monotonic())):
            raise ValueError('native trust tool failed')
        return bytes(captured[0])
    finally:
        if process.poll() is None: process.kill()
        process.wait(); streams.close(); source.close()
        for file in (process.stdin, process.stdout, process.stderr):
            if file is not None: file.close()


def canonical(path, directory=False):
    path = Path(path)
    try: resolved = path.resolve(strict=True)
    except OSError as cause: raise ValueError('invalid native trust path') from cause
    if not path.is_absolute() or len(os.fsencode(path)) > 4096 or resolved != path:
        raise ValueError('invalid native trust path')
    info = path.lstat()
    if info.st_uid != os.getuid() or not (stat.S_ISDIR(info.st_mode) if directory else stat.S_ISREG(info.st_mode)):
        raise ValueError('invalid native trust owner or type')
    return info


def read_regular(path, maximum):
    before = canonical(path)
    fd = os.open(path, os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW)
    try:
        opened = os.fstat(fd)
        if not stat.S_ISREG(opened.st_mode) or (before.st_dev, before.st_ino) != (opened.st_dev, opened.st_ino) or not 0 < opened.st_size <= maximum:
            raise ValueError('invalid native trust file')
        result = bytearray()
        while len(result) <= maximum:
            block = os.read(fd, min(4096, maximum + 1 - len(result)))
            if not block: break
            result.extend(block)
        after = os.fstat(fd)
        current = canonical(path)
        if len(result) > maximum or (opened.st_dev, opened.st_ino, opened.st_size, opened.st_mtime_ns) != (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns) or (current.st_dev,current.st_ino) != (opened.st_dev,opened.st_ino):
            raise ValueError('native trust file changed or exceeded bounds')
        return bytes(result)
    finally: os.close(fd)


def identity(path, directory=False):
    info = canonical(path, directory)
    return [info.st_dev, info.st_ino]


def fingerprint(certificate):
    info = canonical(certificate)
    if not 0 < info.st_size <= 262144:
        raise ValueError('invalid fixture certificate size')
    data = read_regular(certificate, 262144)
    if not re.fullmatch(rb'-----BEGIN CERTIFICATE-----\n(?:[A-Za-z0-9+/=]+\n)+-----END CERTIFICATE-----\n?', data):
        raise ValueError('invalid fixture public certificate')
    constraints = run(['openssl', 'x509', '-noout', '-ext', 'basicConstraints'], data)
    if b'CA:TRUE' not in constraints:
        raise ValueError('fixture certificate is not a CA')
    return hashlib.sha256(run(['openssl', 'x509', '-outform', 'DER'], data)).hexdigest()


def arguments(project, certificate, workspace, nonce):
    if project not in ('chromium', 'firefox') or sys.platform != 'linux' or any(os.environ.get(key) != value for key, value in (('CI', 'true'), ('GITHUB_ACTIONS', 'true'), ('RUNNER_OS', 'Linux'))):
        raise ValueError('native trust requires the selected disposable Linux runner')
    if not isinstance(nonce, str) or len(nonce) > 32 or not re.fullmatch(r'[0-9]+-[0-9]+', nonce):
        raise ValueError('invalid fixture trust nonce')
    workspace, certificate = Path(workspace), Path(certificate)
    canonical(workspace, True)
    if certificate != workspace / 'browser-fixture-ca.crt':
        raise ValueError('certificate must belong to the fixture workspace')
    return certificate, workspace, workspace / ('browser-native-trust-' + nonce + '.json')


def write_state(path, value, create=False, expected=None):
    directory = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        opened = os.fstat(directory)
        if [opened.st_dev, opened.st_ino] != value['workspace']:
            raise ValueError('native trust workspace changed')
        flags = os.O_WRONLY | os.O_NONBLOCK | os.O_NOFOLLOW
        fd = os.open(path.name, flags | (os.O_CREAT | os.O_EXCL if create else 0), 0o600, dir_fd=directory)
        with os.fdopen(fd, 'w') as file:
            info = os.fstat(file.fileno()); captured = [info.st_dev, info.st_ino]
            if not stat.S_ISREG(info.st_mode) or info.st_uid != os.getuid() or not create and captured != expected:
                raise ValueError('native trust receipt changed')
            os.ftruncate(file.fileno(), 0); file.write(json.dumps(value) + '\n')
        if identity(path.parent, True) != value['workspace'] or identity(path) != captured:
            raise ValueError('native trust receipt namespace changed')
        return captured
    finally: os.close(directory)


def firefox_executable():
    root = Path(__file__).resolve().parents[2] / 'apps/player/e2e'
    result = run(['node', '-e', "process.stdout.write(require('@playwright/test').firefox.executablePath())"], cwd=root)
    # Require the default pinned Playwright cache, not an arbitrary browser/profile.
    executable = Path(result.decode('utf-8'))
    expected = Path.home() / '.cache/ms-playwright'
    if executable.parent.parent.parent != expected or not re.fullmatch(r'firefox-[0-9]+', executable.parent.parent.name) or executable.parent.name != 'firefox' or executable.name != 'firefox':
        raise ValueError('unsupported pinned Firefox cache path')
    canonical(executable)
    return executable


def nss_directory():
    home = Path.home(); canonical(home, True)
    legacy = home / '.pki/nssdb'
    directory = legacy if os.path.lexists(legacy) else home / '.local/share/pki/nssdb'
    if os.path.lexists(directory): canonical(directory, True)
    else:
        # Every existing ancestor must be canonical before creating any path.
        parent = directory.parent
        while not parent.exists(): parent = parent.parent
        canonical(parent, True)
    return directory


def database_files(directory):
    files = list(itertools.islice(directory.iterdir(), 17))
    if len(files) > 16: raise ValueError('native database cardinality exceeded')
    for file in files: canonical(file)
    return files


def valid_id(value):
    return isinstance(value, list) and len(value) == 2 and all(type(v) is int and 0 < v < 2**64 for v in value)


def listing(directory):
    return run(['certutil', '-L', '-d', 'sql:' + str(directory)]).decode('utf-8')


def contains_name(text, name):
    return any(re.match(re.escape(name) + r'\s{2,}', line) for line in text.splitlines())


def install(project, certificate, workspace, nonce):
    certificate, workspace, receipt = arguments(project, certificate, workspace, nonce)
    digest = fingerprint(certificate)
    if os.path.lexists(receipt): raise ValueError('native trust receipt already exists')
    state = {'project': project, 'nonce': nonce, 'certificate': digest, 'workspace': identity(workspace, True)}
    if project == 'chromium':
        directory = nss_directory(); created = not directory.exists()
        name = 'kinosail-fixture-' + os.urandom(16).hex()
        if not created:
            database_files(directory)
            if contains_name(listing(directory), name): raise ValueError('native CA nickname already exists')
        state.update(directory=str(directory), directoryID=None, created=created, name=name, files={})
        receipt_id = write_state(receipt, state, True)
        if created: directory.mkdir(parents=True, mode=0o700)
        state['directoryID'] = identity(directory, True); write_state(receipt, state, expected=receipt_id)
        if created:
            run(['certutil', '-N', '-d', 'sql:' + str(directory), '--empty-password'])
            state['files'] = {file.name: identity(file) for file in database_files(directory)}
            if set(state['files']) != {'cert9.db', 'key4.db', 'pkcs11.txt'}: raise ValueError('unexpected NSS database files')
            write_state(receipt, state, expected=receipt_id)
        run(['certutil', '-A', '-d', 'sql:' + str(directory), '-n', name, '-t', 'C,,', '-i', str(certificate)])
    else:
        executable = firefox_executable(); canonical(executable)
        directory = executable.parent / 'distribution'; policy = directory / 'policies.json'
        if os.path.lexists(policy): raise ValueError('foreign Firefox policy already exists')
        created = not os.path.lexists(directory)
        if not created: canonical(directory, True)
        content = (json.dumps({'policies': {'Certificates': {'Install': [str(certificate)]}}}) + '\n').encode()
        state.update(directory=str(directory), directoryID=None, created=created, policyID=None, policySHA=hashlib.sha256(content).hexdigest())
        receipt_id = write_state(receipt, state, True)
        if created: directory.mkdir(mode=0o700)
        state['directoryID'] = identity(directory, True); write_state(receipt, state, expected=receipt_id)
        parent = os.open(directory, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try:
            opened = os.fstat(parent)
            if [opened.st_dev, opened.st_ino] != state['directoryID']:
                raise ValueError('Firefox policy directory changed')
            fd = os.open('policies.json', os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=parent)
            with os.fdopen(fd, 'wb') as file:
                info = os.fstat(file.fileno()); state['policyID'] = [info.st_dev, info.st_ino]
                file.write(content)
            write_state(receipt, state, expected=receipt_id)
            if identity(directory, True) != state['directoryID'] or identity(policy) != state['policyID']:
                raise ValueError('Firefox policy namespace changed')
        finally: os.close(parent)


def unique(pairs):
    result = {}
    for key, value in pairs:
        if key in result: raise ValueError('duplicate native trust receipt key')
        result[key] = value
    return result


def remove(project, certificate, workspace, nonce):
    certificate, workspace, receipt = arguments(project, certificate, workspace, nonce)
    if not os.path.lexists(receipt): return
    info = canonical(receipt)
    if not 0 < info.st_size <= 4096: raise ValueError('invalid native trust receipt size')
    state = json.loads(read_regular(receipt, 4096).decode('utf-8'), object_pairs_hook=unique, parse_constant=lambda value: (_ for _ in ()).throw(ValueError('invalid number')))
    shared = {'project', 'nonce', 'certificate', 'workspace', 'directory', 'directoryID', 'created'}
    required = shared | ({'name', 'files'} if project == 'chromium' else {'policyID', 'policySHA'})
    if not isinstance(state, dict) or set(state) != required or state['project'] != project or state['nonce'] != nonce or state['workspace'] != identity(workspace, True) or type(state['created']) is not bool or state['certificate'] != fingerprint(certificate):
        raise ValueError('native trust receipt binding changed')
    if not valid_id(state['workspace']) or not valid_id(state['directoryID']) or not isinstance(state['directory'], str): raise ValueError('invalid native trust identity')
    directory = Path(state['directory'])
    if project == 'chromium':
        if directory not in (Path.home()/'.pki/nssdb', Path.home()/'.local/share/pki/nssdb') or not re.fullmatch(r'kinosail-fixture-[a-f0-9]{32}', state['name']): raise ValueError('invalid NSS authority')
    else:
        if directory != firefox_executable().parent/'distribution': raise ValueError('invalid Firefox policy authority')
    if state['directoryID'] != identity(directory, True): raise ValueError('native trust directory changed')
    if project == 'chromium':
        files = state['files']
        if not isinstance(files, dict) or (set(files) != {'cert9.db', 'key4.db', 'pkcs11.txt'} if state['created'] else files != {}): raise ValueError('invalid NSS file receipt')
        if any(not valid_id(expected) or identity(directory/name) != expected for name, expected in files.items()): raise ValueError('NSS file changed')
        database_files(directory)
        if contains_name(listing(directory), state['name']):
            pem = run(['certutil', '-L', '-d', 'sql:'+str(directory), '-n', state['name'], '-a'])
            if hashlib.sha256(run(['openssl', 'x509', '-outform', 'DER'], pem)).hexdigest() != state['certificate']: raise ValueError('native certificate changed')
            run(['certutil', '-D', '-d', 'sql:'+str(directory), '-n', state['name']])
        if state['created']:
            files = state['files']
            if not isinstance(files, dict) or set(files) != {'cert9.db', 'key4.db', 'pkcs11.txt'} or set(file.name for file in database_files(directory)) != set(files): raise ValueError('retain incomplete or changed NSS database')
            remaining = listing(directory).splitlines()
            if any(re.search(r'\s+[A-Za-z]*,[A-Za-z]*,[A-Za-z]*\s*$', line) for line in remaining): raise ValueError('retain NSS database with unrelated certificates')
            for name, expected in files.items():
                if identity(directory/name) != expected: raise ValueError('NSS file changed')
            for name in files: (directory/name).unlink()
            directory.rmdir()
    else:
        if not valid_id(state['policyID']) or not isinstance(state['policySHA'], str) or not re.fullmatch(r'[a-f0-9]{64}', state['policySHA']): raise ValueError('invalid Firefox policy receipt')
        parent = os.open(directory, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try:
            opened = os.fstat(parent); policy = directory/'policies.json'
            if [opened.st_dev, opened.st_ino] != state['directoryID'] or state['policyID'] != identity(policy) or hashlib.sha256(read_regular(policy, 4096)).hexdigest() != state['policySHA']:
                raise ValueError('Firefox policy changed')
            if identity(directory, True) != state['directoryID'] or identity(policy) != state['policyID']:
                raise ValueError('Firefox policy namespace changed')
            os.unlink('policies.json', dir_fd=parent)
        finally: os.close(parent)
        if state['created']: directory.rmdir()
    parent = os.open(workspace, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        opened = os.fstat(parent)
        if [opened.st_dev, opened.st_ino] != state['workspace'] or identity(receipt) != [info.st_dev, info.st_ino]:
            raise ValueError('native trust receipt namespace changed')
        os.unlink(receipt.name, dir_fd=parent)
    finally: os.close(parent)


def main():
    if len(sys.argv) != 6 or sys.argv[1] not in ('install', 'remove'): raise ValueError('invalid native trust invocation')
    operation = install if sys.argv[1] == 'install' else remove
    operation(sys.argv[2], Path(sys.argv[3]), Path(sys.argv[4]), sys.argv[5])


def cli():
    try: main()
    except (OSError, ValueError, TypeError, subprocess.SubprocessError) as cause:
        print('native fixture trust failed; retain any unverified owned additions', file=sys.stderr)
        categories = {'unsupported pinned Firefox cache path':'firefox_cache', 'foreign Firefox policy already exists':'foreign_policy',
                      'invalid native trust path':'path', 'invalid native trust owner or type':'path_owner',
                      'native trust tool failed':'tool_exit', 'native trust tool deadline':'tool_deadline', 'native trust tool output overflow':'tool_output'}
        family = 'io' if isinstance(cause,OSError) else 'validation' if isinstance(cause,ValueError) else 'type' if isinstance(cause,TypeError) else 'process'
        print(json.dumps({'event':'native_trust_failure','category':categories.get(str(cause),'unclassified'),'family':family}), file=sys.stderr)
        raise SystemExit(2)

if __name__ == '__main__': cli()
