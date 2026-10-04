"""Admission controls for Q09 evidence; no actual Go/Node/browser execution."""
import importlib.util
import json
import os
from pathlib import Path
import signal
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
DRIVER = ROOT / 'apps/player/scripts/campaign-q09-public.py'


class Q09ProofTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        sys.path.insert(0, str(DRIVER.parent))
        spec = importlib.util.spec_from_file_location('q09proof', DRIVER)
        cls.driver = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(cls.driver)

    def result(self, mode='isolated'):
        return {'errors': [], 'suites': [{'specs': [
            {'title': title, 'tests': [{'projectName': 'chromium', 'status': 'expected',
                                      'results': [{'status': 'passed', 'retry': 0}]}]}
            for title in self.driver.titles(mode)]}]}

    def verify(self, data, mode='isolated'):
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / 'results.json'
            path.write_text(json.dumps(data))
            return self.driver.verify_results(path, mode)

    def test_exact_three_groups_and_safe_projection(self):
        for mode, count in [('isolated', 5), ('server', 8), ('phone', 1)]:
            data = self.result(mode)
            data['config'] = {'cookie': 'PRIVATE-FAKE'}
            data['suites'][0]['specs'][0]['tests'][0]['results'][0]['attachments'] = [{'body': 'PRIVATE-FAKE'}]
            actual = self.verify(data, mode)
            self.assertEqual(actual['count'], count)
            self.assertTrue(actual['passed'])
            self.assertNotIn('PRIVATE-FAKE', json.dumps(actual))

    def test_empty_duplicate_wrong_project_and_unknown_title_reject(self):
        for mutation in ('empty', 'duplicate', 'project', 'title'):
            data = self.result()
            specs = data['suites'][0]['specs']
            if mutation == 'empty': specs.clear()
            elif mutation == 'duplicate': specs.append(specs[0])
            elif mutation == 'project': specs[0]['tests'][0]['projectName'] = 'firefox'
            else: specs[0]['title'] = 'PRIVATE-FAKE'
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                self.verify(data)

    def test_skip_retry_flaky_global_error_and_malformed_reject(self):
        for mutation in ('skip', 'retry', 'flaky', 'global', 'malformed'):
            data = self.result()
            case = data['suites'][0]['specs'][0]['tests'][0]
            if mutation == 'skip': case['results'][0]['status'] = 'skipped'
            elif mutation == 'retry': case['results'].append(case['results'][0])
            elif mutation == 'flaky': case['status'] = 'flaky'
            elif mutation == 'global': data['errors'] = [{'message': 'PRIVATE-FAKE'}]
            else: data['suites'] = 'malformed'
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                self.verify(data)

    def test_completed_failure_has_no_error_body_or_green_claim(self):
        data = self.result()
        case = data['suites'][0]['specs'][0]['tests'][0]
        case['status'] = 'unexpected'
        case['results'][0].update(status='failed', errors=[{'message': 'PRIVATE-FAKE'}])
        actual = self.verify(data)
        self.assertFalse(actual['passed'])
        self.assertNotIn('PRIVATE-FAKE', json.dumps(actual))

    def test_missing_false_nonlist_errors_and_bad_result_shapes_reject(self):
        for errors in (None, False, {}, 'PRIVATE-FAKE'):
            data = self.result()
            if errors is None: del data['errors']
            else: data['errors'] = errors
            with self.subTest(errors=errors), self.assertRaises(ValueError):
                self.verify(data)
        for part in ('spec', 'case', 'attempt'):
            data = self.result()
            specs = data['suites'][0]['specs']
            if part == 'spec': specs[0] = False
            elif part == 'case': specs[0]['tests'][0] = False
            else: specs[0]['tests'][0]['results'][0] = False
            with self.subTest(part=part), self.assertRaises(ValueError):
                self.verify(data)

    def test_completed_leader_settles_its_owned_descendant(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            child = 'import signal,time;signal.signal(signal.SIGTERM,signal.SIG_IGN);time.sleep(60)'
            parent = ('import os,subprocess,sys;from pathlib import Path;'
                      'Path(sys.argv[1]).write_text(str(os.getpgrp()));'
                      'subprocess.Popen([sys.executable,"-c",sys.argv[2]])')
            pid = None
            try:
                result = self.driver.execute([sys.executable, '-c', parent, str(root / 'pid'), child],
                                             root, os.environ, root / 'log', 2)
                pid = int((root / 'pid').read_text())
                self.assertTrue(result.get('ownedGroupSettled'))
            finally:
                if pid is None and (root / 'pid').exists(): pid = int((root / 'pid').read_text())
                if pid:
                    try: os.killpg(pid, signal.SIGKILL)
                    except ProcessLookupError: pass

    def test_real_embed_and_executed_binary_provenance_is_prepared(self):
        source = DRIVER.read_text()
        self.assertNotIn("extensions =", source)
        self.assertIn("'compiledTestBinary'", source)
        self.assertIn("'test2json'", source)
        self.assertIn("'privateEvidenceLimit'", source)

    def test_native_fixture_explicitly_forbids_retry_and_only(self):
        source = (ROOT / 'apps/player/internal/server/download_pause_browser_test.go').read_text()
        self.assertIn('"--retries=0", "--forbid-only"', source)

    def test_real_go_launcher_preserves_package_relative_browser_directory(self):
        # The public fixture cannot reach Playwright when its test-binary cwd is
        # the app root. Model admission/commands only; execute no Go or browser.
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            e2e = root / 'apps/player/e2e'; e2e.mkdir(parents=True)
            package = root / 'apps/player/internal/server'; package.mkdir(parents=True)
            browser = root / '.cache/ms-playwright/chromium-fictional/chrome-linux/chrome'
            browser.parent.mkdir(parents=True); browser.write_bytes(b'fictional-browser')
            observed = []
            def execute(command, cwd, _environment, log, _bound):
                log.write_bytes(b'fictional-command')
                if '-c' in command:
                    Path(command[command.index('-o') + 1]).write_bytes(b'fictional-test-binary')
                elif command[0] == 'node':
                    report = log.parent / 'report/results-chromium.json'
                    report.parent.mkdir(); report.write_text(json.dumps(self.result()))
                else:
                    observed.append(cwd)
                    raise RuntimeError('fictional stop before real server execution')
                return {'exitCode': 0, 'timedOut': False, 'ownedGroupSettled': True}
            revision = '1' * 40
            def git(*args):
                return revision.encode() if args == ('rev-parse', 'HEAD') else b''
            version = type('Version', (), {'stdout': 'fictional-tool'})()
            with patch.object(self.driver, 'ROOT', root), patch.object(self.driver, 'git', git), \
                    patch.object(self.driver, 'snapshot', return_value={'fictional': {}}), \
                    patch.object(self.driver, 'execute', execute), patch.object(self.driver.Path, 'home', return_value=root), \
                    patch.object(self.driver.shutil, 'which', return_value=sys.executable), \
                    patch.object(self.driver.subprocess, 'run', return_value=version), \
                    patch.dict(os.environ, {'GITHUB_SHA': revision}):
                self.assertEqual(self.driver.main(), 1)
            self.assertEqual(observed, [package])
            self.assertEqual((observed[0] / '../../e2e').resolve(), e2e)


if __name__ == '__main__':
    unittest.main()
