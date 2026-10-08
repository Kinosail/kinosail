"""Exercise the real closed caller with synthetic HTTP/process peers only."""
import contextlib
import copy
import io
import json
import os
from pathlib import Path
import runpy
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

from test_library_profile_admission import load, report

SOURCE = Path(__file__).with_name('run-populated-settings.py')


class LibraryOwnerCallerTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name).resolve()
        self.discovery = self.root / 'discovery.json'
        self.discovery.write_text(json.dumps(report(load().CASES, completed=False)))
        self.output = self.root / 'results'
        self.effects = []
        self.browser_status = 0
        self.setup_status = 201
        self.result = report(load().CASES)
        self.project = 'webkit'
        self.arguments = ['--url', 'https://localhost:38127', '--output', str(self.output),
                          '--profile', 'library-owner', '--project', 'webkit',
                          '--state', 'fresh', '--discovery', str(self.discovery)]

    def execute(self, arguments=None, **environment):
        test = self

        class Response:
            def __init__(self, status, body):
                self.status, self.body = status, body
                self.headers = type('Headers', (), {'get_content_type': lambda _: 'application/json'})()

            def __enter__(self):
                return self

            def __exit__(self, *_):
                pass

            def read(self):
                return json.dumps(self.body).encode()

        class Peer:
            def open(self, request, timeout):
                path = request.full_url.split('38127', 1)[1]
                test.effects.append(('http', request.method, path))
                if path == '/api/v1/setup':
                    return Response(test.setup_status, {'token': 'synthetic-token',
                                    'totp': {'secret': 'JBSWY3DPEHPK3PXP'}})
                return Response(200 if request.method == 'PUT' else 303,
                                {'enabled': True} if request.method == 'PUT' else {})

        def process(command, **options):
            self.effects.append(('process', command, options))
            if command[0] == 'bash':
                return subprocess.CompletedProcess(command, 0)
            self.assertEqual(command[:6], ['pnpm', '--dir', str(SOURCE.parents[2] / 'apps/player/e2e'),
                                          'exec', 'playwright', 'test'])
            self.assertEqual(set(command[6:21]), {file for file, _ in load().CASES})
            self.assertIn('--project=' + self.project, command)
            self.assertIn('--workers=1', command)
            self.assertIn('--retries=0', command)
            self.assertIn('--repeat-each=1', command)
            self.assertEqual(options['env']['KINOSAIL_BROWSER_PROJECT'], self.project)
            self.assertEqual(options['env']['KINOSAIL_TEST_INSTANCE'], '1')
            (self.output / ('results-' + self.project + '.json')).write_text(json.dumps(self.result))
            return subprocess.CompletedProcess(command, self.browser_status)

        with patch.object(sys, 'argv', [str(SOURCE), *(arguments or self.arguments)]), \
                patch.dict(os.environ, {'NODE_EXTRA_CA_CERTS': 'synthetic-ca', **environment}, clear=True), \
                patch('subprocess.run', side_effect=process), \
                patch('subprocess.check_output', return_value='synthetic-revision\n'), \
                patch('urllib.request.build_opener', return_value=Peer()), \
                patch('ssl.create_default_context'), \
                contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
            with self.assertRaises(SystemExit) as exit_result:
                runpy.run_path(str(SOURCE), run_name='__main__')
        return exit_result.exception.code

    def test_exact_discovery_admits_before_effects_and_one_owner_runs_fixed_46(self):
        self.assertEqual(self.execute([*self.arguments, '--admit-only']), 0)
        self.assertEqual(self.effects, [])
        self.assertFalse(self.output.exists())
        self.assertEqual(self.execute(), 0)
        self.assertEqual([row for row in self.effects if row[0] == 'http'], [
            ('http', 'POST', '/api/v1/setup'), ('http', 'PUT', '/api/v1/me/mfa'),
            ('http', 'GET', '/onboarding/finish')])
        self.assertEqual(sum(row[0] == 'process' and row[1][0] == 'pnpm' for row in self.effects), 1)
        receipt = json.loads((self.output / 'setup-and-run.json').read_text())
        self.assertEqual(receipt['journeys']['count'], 46)
        self.assertEqual(receipt['profile'], 'library-owner')

    def test_invalid_selection_and_overrides_reject_before_tls_http_output_or_browser(self):
        for flag, values in (('--profile', ('', 'all', 'x' * 1025)),
                             ('--project', ('', 'safari', 'x' * 1025)),
                             ('--state', ('', 'reused', 'x' * 1025))):
            for value in values:
                arguments = self.arguments.copy()
                arguments[arguments.index(flag) + 1] = value
                with self.subTest(flag=flag, value=value[:32]):
                    self.assertEqual(self.execute(arguments), 2)
        for suffix in (['--project', 'webkit'], ['--required-title', 'override'],
                       ['--project=webkit'], ['--admit-only', '--admit-only'],
                       ['--url', 'https://localhost:38127'], ['--output', str(self.output)],
                       ['--', 'arbitrary-command'], ['--unknown', 'value']):
            self.assertEqual(self.execute([*self.arguments, *suffix]), 2)
        for flag in ('--profile', '--project', '--state', '--discovery'):
            arguments = self.arguments.copy()
            index = arguments.index(flag)
            del arguments[index:index + 2]
            self.assertEqual(self.execute(arguments), 2)
        self.assertEqual(self.execute(KINOSAIL_BROWSER_PROJECT='chromium'), 2)
        self.assertEqual(self.execute(PLAYWRIGHT_CHANNEL='chrome'), 2)
        self.assertEqual(self.effects, [])
        self.assertFalse(self.output.exists())

    def test_output_parent_link_or_relative_path_rejects_before_effects(self):
        linked = self.root / 'linked'
        linked.symlink_to(self.root, target_is_directory=True)
        for output in (linked / 'escaped-results', Path('relative-results')):
            arguments = self.arguments.copy()
            arguments[arguments.index('--output') + 1] = str(output)
            self.assertEqual(self.execute(arguments), 2)
        self.assertEqual(self.effects, [])
        self.assertFalse((self.root / 'escaped-results').exists())

    def test_project_transport_contract_rejects_before_effects(self):
        for project, url in (('webkit', 'http://localhost:38127'),
                             ('chromium', 'https://localhost:38127'),
                             ('firefox', 'https://localhost:38127')):
            self.project = project
            self.result = report(load().CASES, project)
            self.discovery.write_text(json.dumps(report(load().CASES, project, False)))
            arguments = self.arguments.copy()
            arguments[arguments.index('--project') + 1] = project
            arguments[arguments.index('--url') + 1] = url
            self.assertEqual(self.execute(arguments), 2)
        self.assertEqual(self.effects, [])
        self.assertFalse(self.output.exists())

    def test_chromium_and_firefox_use_http_and_one_owner(self):
        for project in ('chromium', 'firefox'):
            self.project = project
            self.result = report(load().CASES, project)
            self.discovery.write_text(json.dumps(report(load().CASES, project, False)))
            arguments = self.arguments.copy()
            arguments[arguments.index('--project') + 1] = project
            arguments[arguments.index('--url') + 1] = 'http://localhost:38127'
            self.assertEqual(self.execute(arguments), 0)
            self.assertEqual(sum(row[0] == 'http' and row[1] == 'POST' for row in self.effects), 1)
            self.assertFalse(any(row[0] == 'process' and row[1][0] == 'bash' for row in self.effects))
            self.effects.clear()
            for path in self.output.iterdir():
                path.unlink()
            self.output.rmdir()

    def test_bad_discovery_and_unsafe_inputs_reject_before_all_effects(self):
        good = report(load().CASES, completed=False)
        for kind in ('missing', 'duplicate', 'extra', 'project', 'retry', 'executed', 'title'):
            value = copy.deepcopy(good)
            if kind == 'missing':
                value['suites'].pop()
            elif kind in ('duplicate', 'extra'):
                value['suites'].append(copy.deepcopy(value['suites'][0]))
            elif kind == 'project':
                value['suites'][0]['specs'][0]['tests'][0]['projectName'] = 'firefox'
            elif kind == 'retry':
                value['config']['projects'][0]['retries'] = 1
            elif kind == 'executed':
                value['suites'][0]['specs'][0]['tests'][0]['results'] = [{'status': 'passed'}]
            else:
                value['suites'][0]['specs'][0]['title'] = 'unknown'
            self.discovery.write_text(json.dumps(value))
            self.assertEqual(self.execute(), 2, kind)
        for raw in (b'', b'{', b'{}', b'x' * 2097153):
            self.discovery.write_bytes(raw)
            self.assertEqual(self.execute(), 2)
        self.discovery.unlink()
        os.mkfifo(self.discovery)
        self.assertEqual(self.execute(), 2)
        self.discovery.unlink()
        regular = self.root / 'regular.json'
        regular.write_text(json.dumps(good))
        self.discovery.symlink_to(regular)
        self.assertEqual(self.execute(), 2)
        self.assertEqual(self.effects, [])
        self.assertFalse(self.output.exists())

    def test_setup_409_fails_without_nested_owner_or_browser(self):
        self.setup_status = 409
        self.assertEqual(self.execute(), 1)
        self.assertEqual([row for row in self.effects if row[0] == 'http'],
                         [('http', 'POST', '/api/v1/setup')])
        self.assertFalse(any(row[0] == 'process' and row[1][0] == 'pnpm' for row in self.effects))

    def test_browser_failure_skip_retry_and_extra_results_do_not_admit(self):
        for kind in ('failure', 'skip', 'retry', 'extra'):
            with self.subTest(kind=kind):
                self.result = report(load().CASES)
                self.browser_status = 1 if kind == 'failure' else 0
                if kind == 'skip':
                    self.result['suites'][0]['specs'][0]['tests'][0]['results'][0]['status'] = 'skipped'
                elif kind == 'retry':
                    self.result['suites'][0]['specs'][0]['tests'][0]['results'][0]['retry'] = 1
                elif kind == 'extra':
                    self.result['suites'].append(copy.deepcopy(self.result['suites'][0]))
                self.assertEqual(self.execute(), 1)
                for path in self.output.iterdir():
                    path.unlink()
                self.output.rmdir()


if __name__ == '__main__':
    unittest.main()
