"""Closed responsive identities use the actual selector and admission seam."""
import copy
import json
import os
from pathlib import Path
import subprocess
import sys
import unittest
import tempfile
from unittest.mock import patch
from test_library_profile_admission import load, report, ROOT
import test_library_fixture_owner as fixture_peers

class ResponsiveAdmissionTests(unittest.TestCase):
    def setUp(self):
        directory = str(ROOT / 'apps/player/scripts')
        sys.path.insert(0, directory)
        self.addCleanup(lambda: sys.path.remove(directory))

    def module(self):
        return load()

    def proof(self, cases, project='webkit', completed=True):
        value = report(cases, project, completed)
        grouped = {}
        for suite in value['suites']:
            if suite['title'] not in grouped:
                grouped[suite['title']] = suite
            else:
                grouped[suite['title']]['specs'].extend(suite['specs'])
        value['suites'] = list(grouped.values())
        return value

    def test_exact99_discovery_and_first_attempt_accept_each_engine(self):
        module = self.module()
        module.selection(('responsive-shell', 'webkit', 'fresh'))
        cases = module.profile_cases('responsive-shell')
        self.assertEqual(len(cases), 99)
        self.assertEqual(len(set(cases)), 99)
        self.assertEqual(len({file for file, _ in cases}), 12)
        for project in module.PROJECTS:
            for complete in (False, True):
                with patch('subprocess.run', side_effect=AssertionError('process effect')):
                    actual = module.admit(json.dumps(self.proof(cases, project, complete)).encode(),
                                          'responsive-shell', project, 'fresh', complete)
                self.assertEqual(actual, {(file, title, project) for file, title in cases})

    def test_wrapper_routes_only_one_fresh_owner_and_joins_owned_cleanup(self):
        peer = fixture_peers.LibraryFixtureOwnerTests()
        peer.setUp()
        self.addCleanup(peer.temporary.cleanup)
        cases = self.module().profile_cases('responsive-shell')
        peer.discovery.write_text(json.dumps(self.proof(cases, 'chromium', False)))
        result = peer.run_caller(['chromium', str(peer.discovery), str(peer.output), 'responsive-shell'])
        self.assertEqual(result.returncode, 0, result.stderr)
        rows = peer.rows()
        self.assertEqual(rows[0][0], 'admit')
        owners = [args for kind, args in rows if kind == 'owner']
        self.assertEqual(len(owners), 1)
        self.assertEqual(owners[0][owners[0].index('--profile') + 1], 'responsive-shell')
        peer.assert_cleanup(rows)

    def test_caller_admits_complete_discovery_and_rejects_changes_before_owner(self):
        module = self.module()
        cases = module.profile_cases('responsive-shell')
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            proof = root / 'discovery.json'
            output = root / 'output'
            command = [sys.executable, str(ROOT / 'scripts/ci/run-populated-settings.py'),
                       '--url', 'https://localhost:38127', '--output', str(output),
                       '--profile', 'responsive-shell', '--project', 'webkit', '--state', 'fresh',
                       '--discovery', str(proof), '--admit-only']
            environment = {key: value for key, value in os.environ.items()
                           if not key.startswith('KINOSAIL_') and key != 'PLAYWRIGHT_CHANNEL'}
            environment['PYTHONDONTWRITEBYTECODE'] = '1'
            for accepted in (True, False):
                value = self.proof(cases, completed=False)
                if not accepted: value['suites'][0]['specs'].pop()
                proof.write_text(json.dumps(value))
                result = subprocess.run(command, capture_output=True, timeout=3, env=environment)
                self.assertEqual(result.returncode, 0 if accepted else 2, result.stderr)
                self.assertFalse(output.exists())
                self.assertEqual(set(root.iterdir()), {proof})

    def test_actual_selector_and_rejected_proof_precede_process_and_output(self):
        module = self.module()
        module.selection(('responsive-shell', 'webkit', 'fresh'))
        cases = module.profile_cases('responsive-shell')
        command = module.playwright_arguments('webkit', True, 'responsive-shell')
        self.assertEqual(set(command[3:command.index('--project=webkit')]), {file for file, _ in cases})
        for option in ('--workers=1', '--retries=0', '--repeat-each=1', '--list'):
            self.assertIn(option, command)
        changes = [lambda v: v['suites'][0]['specs'].pop(),
                   lambda v: v['suites'][0]['specs'].append(copy.deepcopy(v['suites'][0]['specs'][0])),
                   lambda v: v['suites'][0]['specs'][0].update(title='unregistered'),
                   lambda v: v['suites'][0]['specs'][0].update(file='other.spec.ts'),
                   lambda v: v['suites'][0]['specs'][0]['tests'][0].update(projectName='chromium'),
                   lambda v: v['stats'].update(skipped=1),
                   lambda v: v['stats'].update(expected=98),
                   lambda v: v['config']['projects'][0].update(retries=1),
                   lambda v: v['suites'][0]['specs'][0]['tests'][0]['results'][0].update(retry=1),
                   lambda v: v['suites'][0]['specs'][0]['tests'][0]['results'][0].update(status='failed')]
        with patch('subprocess.run', side_effect=AssertionError('process effect')), \
                patch('os.mkdir', side_effect=AssertionError('output effect')):
            for change in changes:
                value = self.proof(cases)
                change(value)
                with self.assertRaises(ValueError):
                    module.admit(json.dumps(value).encode(), 'responsive-shell', 'webkit', 'fresh', True)
            for raw in (b'', b'{', b'x' * 2097153):
                with self.assertRaises(ValueError):
                    module.admit(raw, 'responsive-shell', 'webkit', 'fresh', True)
            for profile, project, state in ((None, 'webkit', 'fresh'), ('unknown', 'webkit', 'fresh'),
                                          ('responsive-shell', 'unknown', 'fresh'), ('responsive-shell', 'webkit', 'restored')):
                with self.assertRaises(ValueError):
                    module.admit(b'{}', profile, project, state, True)

if __name__ == '__main__': unittest.main()
