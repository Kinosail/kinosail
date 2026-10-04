"""Admission controls for Q09 evidence; no actual Go/Node/browser execution."""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
DRIVER = ROOT / 'apps/player/scripts/campaign-q09-public.py'


class Q09ProofTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
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

    def test_native_fixture_explicitly_forbids_retry_and_only(self):
        source = (ROOT / 'apps/player/internal/server/download_pause_browser_test.go').read_text()
        self.assertIn('"--retries=0", "--forbid-only"', source)


if __name__ == '__main__':
    unittest.main()
