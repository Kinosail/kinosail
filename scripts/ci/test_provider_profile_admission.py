"""Closed provider catalogue uses the existing exact-result admission seam."""
import copy
import importlib
import sys
import unittest
from unittest.mock import patch
from test_library_profile_admission import ROOT, load
from test_playback_profile_admission import playback_report


class ProviderProfileTests(unittest.TestCase):
    def setUp(self):
        directory = str(ROOT / 'apps/player/scripts')
        sys.path.insert(0, directory)
        self.addCleanup(lambda: sys.path.remove(directory))
        self.provider = importlib.import_module('provider_profile_cases')
        self.shared = load()

    def admit(self, value, project, completed=True):
        return self.shared.admit_report(value, project, completed, self.provider.selected_cases(project))

    def test_exact15_discovery_and_first_attempt_results_in_each_project(self):
        with patch('subprocess.run', side_effect=AssertionError('process effect')):
            for project in ('chromium', 'firefox', 'webkit'):
                cases = self.provider.selected_cases(project)
                self.assertEqual(len(cases), 15)
                self.assertEqual(len(set(cases)), 15)
                self.assertEqual(len(self.admit(playback_report(cases, project, False), project, False)), 15)
                self.assertEqual(len(self.admit(playback_report(cases, project, True), project)), 15)

    def test_missing_unknown_malformed_oversized_project_rejects_before_process(self):
        with patch('subprocess.run', side_effect=AssertionError('process effect')):
            for project in (None, '', 'all', 'safari', 'webkit ', 'x' * 1025, [], {}, True):
                with self.assertRaises(ValueError): self.provider.selected_cases(project)

    def test_missing_extra_duplicate_file_title_project_attempt_skip_error_or_count_rejects(self):
        cases = self.provider.selected_cases('webkit')
        good = playback_report(cases, 'webkit', True)
        for kind in ('missing', 'extra', 'duplicate', 'file', 'title', 'project', 'retry', 'skip', 'failed', 'error', 'count'):
            value = copy.deepcopy(good)
            test = value['suites'][0]['specs'][0]['tests'][0]
            if kind == 'missing': value['suites'].pop()
            elif kind in ('extra', 'duplicate'): value['suites'].append(copy.deepcopy(value['suites'][0]))
            elif kind in ('file', 'title'): value['suites'][0]['specs'][0][kind] = 'unknown'
            elif kind == 'project': test['projectName'] = 'chromium'
            elif kind == 'retry': test['results'][0]['retry'] = 1
            elif kind == 'skip': test['results'][0]['status'] = 'skipped'
            elif kind == 'failed': test['status'] = 'unexpected'
            elif kind == 'error': test['results'][0]['errors'] = [{'message': 'synthetic failure'}]
            elif kind == 'count': value['stats']['expected'] = 14
            with self.subTest(kind=kind), patch('subprocess.run', side_effect=AssertionError('process effect')):
                with self.assertRaises(ValueError): self.admit(value, 'webkit')

    def test_discovery_does_not_admit_completed_results(self):
        cases = self.provider.selected_cases('chromium')
        value = playback_report(cases, 'chromium', False)
        value['suites'][0]['specs'][0]['tests'][0]['results'] = [{'status': 'passed', 'retry': 0, 'errors': []}]
        with self.assertRaises(ValueError): self.admit(value, 'chromium', False)


if __name__ == '__main__': unittest.main()
