"""Strict admission of the official generator's source-only package graph."""
import re
from pathlib import PurePosixPath

SHARED = 'github.com/MikeO7/kinosail/packages'
MODULES = {app: 'github.com/MikeO7/kinosail-' + app for app in ('player', 'subtitles')}


def exact(value, fields):
    if not isinstance(value, dict) or set(value) != set(fields.split()):
        raise ValueError('unexpected graph fields')


def integer(value):
    if type(value) is not int or not 0 <= value <= 10_000_000:
        raise ValueError('invalid graph count')


def text(value, pattern=r'[A-Za-z0-9_.~/+-]+', maximum=300):
    if not isinstance(value, str) or not 0 < len(value) <= maximum or not re.fullmatch(pattern, value):
        raise ValueError('invalid graph text')


def sequence(value, maximum=8192):
    if not isinstance(value, list) or len(value) > maximum:
        raise ValueError('invalid graph list')


def names(values):
    sequence(values)
    for value in values:
        text(value, r'[A-Za-z_]\w*', 128)
    if values != sorted(set(values)):
        raise ValueError('unordered or duplicate names')


def first_party(value, app):
    return any(value == prefix or value.startswith(prefix + '/') for prefix in (MODULES[app], SHARED))


def validate_snapshot(value, app, root=None):
    if app not in MODULES:
        raise ValueError('unknown architecture app')
    exact(value, 'commit commitShort module sharedModule packages edges summary')
    if (value['commit'], value['commitShort'], value['module'], value['sharedModule']) != ('main', 'main', MODULES[app], SHARED):
        raise ValueError('invalid graph identity')
    sequence(value['packages'], 2048); sequence(value['edges'], 32768)
    identifiers = set(); expected_edges = []; all_paths = set()
    for package in value['packages']:
        exact(package, 'id name label kind responsibility files fileCount testFileCount loc types functions imports externalImports fanIn fanOut')
        text(package['id'])
        if not first_party(package['id'], app) or package['id'] in identifiers:
            raise ValueError('invalid or duplicate package')
        identifiers.add(package['id'])
        expected_name = (package['id'].removeprefix(MODULES[app] + '/') if package['id'].startswith(MODULES[app] + '/')
                         else '.' if package['id'] == MODULES[app] else 'packages/' + package['id'].removeprefix(SHARED + '/'))
        if package['name'] != expected_name or package['label'] != (expected_name.rsplit('/', 1)[-1] if expected_name != '.' else 'kinosail'):
            raise ValueError('invalid package label')
        if package['kind'] != ('command' if expected_name.startswith('cmd/') else 'package'):
            raise ValueError('invalid package kind')
        text(package['responsibility'], r'[A-Za-z0-9 .,;:/()_-]+', 256)
        for field in ('fileCount', 'testFileCount', 'loc', 'fanIn', 'fanOut'): integer(package[field])
        for field in ('types', 'functions'): names(package[field])
        for field in ('imports', 'externalImports'):
            sequence(package[field])
            if package[field] != sorted(set(package[field])): raise ValueError('duplicate imports')
            for item in package[field]:
                text(item)
                if first_party(item, app) != (field == 'imports'): raise ValueError('incorrect import class')
        sequence(package['files']); paths = []; types = set(); functions = set(); loc = 0
        for file in package['files']:
            exact(file, 'path name loc types functions symbols')
            text(file['path']); path = PurePosixPath(file['path'])
            prefix = ('apps/' + app + '/') if package['id'].startswith(MODULES[app]) else 'packages/'
            if path.is_absolute() or '..' in path.parts or not file['path'].startswith(prefix) or path.suffix != '.go' or file['name'] != path.name or file['path'] in all_paths:
                raise ValueError('invalid graph source path')
            if root is not None:
                source = root / file['path']
                if not source.is_file() or source.is_symlink() or not source.resolve().is_relative_to(root.resolve()):
                    raise ValueError('noncanonical source file')
            paths.append(file['path']); all_paths.add(file['path']); integer(file['loc']); loc += file['loc']
            names(file['types']); names(file['functions']); types.update(file['types']); functions.update(file['functions'])
            sequence(file['symbols']); previous = 0
            for symbol in file['symbols']:
                exact(symbol, 'name kind line'); text(symbol['name'], r'[A-Za-z_]\w*', 128); integer(symbol['line'])
                if symbol['kind'] not in ('type', 'function') or symbol['line'] <= 0 or symbol['line'] < previous:
                    raise ValueError('invalid source symbol')
                previous = symbol['line']
        if paths != sorted(paths) or package['fileCount'] != len(paths) or package['loc'] != loc or package['types'] != sorted(types) or package['functions'] != sorted(functions) or package['fanOut'] != len(package['imports']):
            raise ValueError('inconsistent package totals')
    incoming = dict.fromkeys(identifiers, 0)
    for package in value['packages']:
        for target in package['imports']:
            if target in identifiers:
                expected_edges.append({'source': package['id'], 'target': target}); incoming[target] += 1
    # The official generator computes edges before sorting its package table.
    # Keep that exact edge order for rendering; verify identities and multiplicity.
    for edge in value['edges']:
        exact(edge, 'source target')
        if edge['source'] not in identifiers or edge['target'] not in identifiers:
            raise ValueError('unknown graph edge')
    edge_keys = [(e['source'], e['target']) for e in value['edges']]
    expected_keys = [(e['source'], e['target']) for e in expected_edges]
    if len(edge_keys) != len(set(edge_keys)) or set(edge_keys) != set(expected_keys) or any(p['fanIn'] != incoming[p['id']] for p in value['packages']):
        raise ValueError('inconsistent graph edges')
    if not any(p['id'].startswith(MODULES[app] + '/') or p['id'] == MODULES[app] for p in value['packages']) or not any(p['id'].startswith(SHARED + '/') for p in value['packages']):
        raise ValueError('incomplete app or shared graph')
    exact(value['summary'], 'packageCount fileCount loc symbolCount dependencyCount')
    expected = {'packageCount': len(value['packages']), 'fileCount': len(all_paths),
                'loc': sum(p['loc'] for p in value['packages']),
                'symbolCount': sum(len(p['types']) + len(p['functions']) for p in value['packages']), 'dependencyCount': len(expected_edges)}
    for count in value['summary'].values(): integer(count)
    if value['summary'] != expected: raise ValueError('inconsistent graph summary')
