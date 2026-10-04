"""Test-first safe source graph admission; never launches Go or app runtimes."""
import copy
import importlib.util
import json
from pathlib import Path
import re
import tempfile
import unittest
from unittest.mock import patch, Mock

SCRIPT = Path(__file__).with_name('campaign-architecture.py')


class ArchitectureAdmissionTests(unittest.TestCase):
    def helper(self):
        self.assertTrue(SCRIPT.is_file(), 'test-first metadata helper is missing')
        spec = importlib.util.spec_from_file_location('campaign_architecture', SCRIPT)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return module

    def snapshot(self):
        module = 'github.com/MikeO7/kinosail-player'
        value = {'commit': 'main', 'commitShort': 'main', 'module': module,
                'sharedModule': 'github.com/MikeO7/kinosail/packages',
                'packages': [{'id': module + '/internal/example', 'name': 'internal/example',
                              'label': 'example', 'kind': 'package',
                              'responsibility': 'First-party Kinosail package',
                              'files': [{'path': 'apps/player/internal/example/example.go',
                                         'name': 'example.go', 'loc': 3, 'types': ['Example'],
                                         'functions': [], 'symbols': [{'name': 'Example', 'kind': 'type', 'line': 2}]}],
                              'fileCount': 1, 'testFileCount': 0, 'loc': 3, 'types': ['Example'],
                              'functions': [], 'imports': [], 'externalImports': ['fmt'],
                              'fanIn': 0, 'fanOut': 0}], 'edges': [],
                'summary': {'packageCount': 1, 'fileCount': 1, 'loc': 3, 'symbolCount': 1, 'dependencyCount': 0}}
        shared = copy.deepcopy(value['packages'][0])
        shared.update(id=value['sharedModule'] + '/example', name='packages/example')
        shared['files'][0]['path'] = 'packages/example/example.go'
        value['packages'].append(shared)
        value['summary'].update(packageCount=2, fileCount=2, loc=6, symbolCount=2)
        return value

    def test_only_typed_source_metadata_is_admitted(self):
        self.helper().validate_snapshot(self.snapshot(), 'player')

    def test_private_or_unexpected_fields_are_rejected(self):
        for location in ('root', 'package', 'file', 'symbol'):
            with self.subTest(location=location):
                value = self.snapshot()
                target = {'root': value, 'package': value['packages'][0],
                          'file': value['packages'][0]['files'][0],
                          'symbol': value['packages'][0]['files'][0]['symbols'][0]}[location]
                target['rawBody'] = 'fictional private value'
                with self.assertRaises(ValueError):
                    self.helper().validate_snapshot(value, 'player')

    def test_untrusted_paths_imports_symbols_or_counters_are_rejected(self):
        mutations = [lambda x: x['packages'][0]['files'][0].update(path='../private.go'),
                     lambda x: x['packages'][0]['files'][0].update(path='/private/example.go'),
                     lambda x: x['packages'][0].update(externalImports=['https://example.invalid/?token=fictional']),
                     lambda x: x['packages'][0].update(types=['bad\nname']),
                     lambda x: x['packages'][0].update(loc=True),
                     lambda x: x['packages'][0].update(id='github.com/MikeO7/kinosail-player-foreign/private')]
        for mutate in mutations:
            value = self.snapshot(); mutate(value)
            with self.subTest(value=value), self.assertRaises(ValueError):
                self.helper().validate_snapshot(value, 'player')

    def test_duplicate_packages_and_invalid_edges_are_rejected(self):
        for mutate in [lambda x: x['packages'].append(copy.deepcopy(x['packages'][0])),
                       lambda x: x['edges'].append({'source': 'unknown', 'target': x['packages'][0]['id']})]:
            value = self.snapshot(); mutate(value)
            with self.assertRaises(ValueError):
                self.helper().validate_snapshot(value, 'player')

    def test_inconsistent_counts_and_boolean_lines_are_rejected(self):
        for mutate in [lambda x: x['summary'].update(loc=4),
                       lambda x: x['packages'][0].update(fanOut=1),
                       lambda x: x['packages'][0]['files'][0]['symbols'][0].update(line=True)]:
            value = self.snapshot(); mutate(value)
            with self.assertRaises(ValueError):
                self.helper().validate_snapshot(value, 'player')

    def test_remote_html_mismatch_never_partially_replaces_docs(self):
        helper = self.helper()
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for name in helper.FILES:
                path = root / name; path.parent.mkdir(parents=True, exist_ok=True); path.write_text('fictional input')
            for app in ('player', 'subtitles'):
                path = root / 'apps' / app / 'docs/architecture-explorer/index.html'
                path.parent.mkdir(parents=True); path.write_text('preserved document')
            graph = {'schemaVersion': 1, 'revision': 'fictional-revision', 'trackedTree': {'files': 1},
                     'sourceFiles': {name: helper.pin(root / name) for name in helper.FILES},
                     'apps': {app: {'snapshot': {}, 'renderedHTML': {'bytes': 1, 'sha256': 'incorrect'}}
                              for app in ('player', 'subtitles')}}
            manifest = root / 'source-manifest.json'; manifest.write_text(json.dumps({'architectureMetadata': graph}))
            with patch.object(helper, 'ROOT', root), patch.object(helper, 'revision', return_value='fictional-revision'), \
                    patch.object(helper, 'tree_pin', return_value={'files': 1}), patch.object(helper, 'render', return_value=b'x'):
                with self.assertRaises(ValueError): helper.reproduce(manifest)
            for app in ('player', 'subtitles'):
                self.assertEqual((root / 'apps' / app / 'docs/architecture-explorer/index.html').read_text(), 'preserved document')

    def test_empty_and_incomplete_graphs_are_rejected(self):
        empty = self.snapshot(); empty['packages'] = []
        empty['summary'] = dict.fromkeys(empty['summary'], 0)
        incomplete = self.snapshot(); incomplete['packages'].pop()
        incomplete['summary'].update(packageCount=1, fileCount=1, loc=3, symbolCount=1)
        for value in (empty, incomplete):
            with self.assertRaises(ValueError): self.helper().validate_snapshot(value, 'player')

    def test_official_player_graph_edge_order_is_preserved(self):
        html = SCRIPT.parents[2].joinpath('apps/player/docs/architecture-explorer/index.html').read_text()
        match = re.search(r'^\s*const snapshot = (.+);$', html, re.M)
        self.assertIsNotNone(match)
        self.helper().validate_snapshot(json.loads(match.group(1)), 'player')

    def test_empty_successful_fake_go_output_is_rejected(self):
        helper = self.helper()
        with tempfile.TemporaryDirectory() as temporary:
            go = Path(temporary) / 'fake-go'; go.write_text('#!/bin/sh\nexit 0\n'); go.chmod(0o700)
            with self.assertRaises(ValueError):
                helper.bounded_go_list(str(go), ['go', 'list', '-json', './...'], cwd=helper.ROOT / 'apps/player')

    def test_timeout_settles_descendants_after_leader_exit(self):
        helper = self.helper(); process = Mock(pid=456, returncode=0)
        # The leader exits after TERM; a survivor still owns the group.
        with patch.object(helper.os, 'killpg', side_effect=[None, None, None, ProcessLookupError()]) as signals:
            helper.settle_owned_group(process, True)
        self.assertEqual([call.args[1] for call in signals.call_args_list],
                         [helper.signal.SIGTERM, 0, helper.signal.SIGKILL, 0])

    def test_r06_manifest_shape_is_preserved(self):
        original = {'artifacts': [{'path': name, 'present': True, 'bytes': 0, 'sha256': 'old'}
                                  for name in ('receipt.json', 'results.json', 'source-manifest.json')]}
        pins = {row['path']: {'bytes': 7, 'sha256': 'new'} for row in original['artifacts']}
        expected = {'artifacts': [{'path': row['path'], 'present': True, **pins[row['path']]}
                                  for row in original['artifacts']]}
        self.assertEqual(self.helper().refresh_manifest(original, pins), expected)

    def test_route_is_default_off_and_retains_four_json_allowlist(self):
        source = SCRIPT.parents[2].joinpath('.github/workflows/layout-stability.yml').read_text()
        self.assertIn('architecture_metadata:', source)
        self.assertIn('description: Collect bounded source metadata for canonical architecture regeneration', source)
        section = source.split('architecture_metadata:', 1)[1].split('permissions:', 1)[0]
        self.assertIn('default: false', section)
        self.assertIn('type: boolean', section)
        self.assertIn('timeout 120s python3 scripts/ci/campaign-architecture.py "$CAMPAIGN_PROOF"', source)
        artifact = source.split('name: Keep safe campaign proof')[1]
        self.assertNotIn('*.json', artifact)
        self.assertNotIn('index.html', artifact)
        self.assertNotIn('command.log', artifact)


if __name__ == '__main__':
    unittest.main()
