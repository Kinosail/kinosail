"""Closed library proofs cannot certify missing, skipped, or ambiguous work."""
import copy
import importlib.util
import json
from pathlib import Path
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
MODULE = ROOT / 'apps/player/scripts/library_profile_admission.py'


def load():
    spec = importlib.util.spec_from_file_location('library_admission', MODULE)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def report(cases, project='webkit', completed=True):
    suites = []
    for file, title in cases:
        suites.append({'title': file, 'file': file, 'specs': [{
            'id': file + ':' + title, 'file': file, 'title': title, 'ok': completed,
            'tests': [{'projectName': project, 'expectedStatus': 'passed',
                       'status': 'expected', 'results': [{
                           'status': 'passed', 'retry': 0, 'errors': []}] if completed else []}]}]})
    return {'errors': [], 'suites': suites,
            'config': {'workers': 1, 'shard': None, 'projects': [
                {'name': project, 'retries': 0, 'repeatEach': 1}]},
            'stats': {'expected': len(cases) if completed else 0,
                      'skipped': 0, 'unexpected': 0, 'flaky': 0}}


class LibraryAdmissionTests(unittest.TestCase):
    def setUp(self):
        self.module = load()

    def admit(self, value, project='webkit', completed=True):
        return self.module.admit(json.dumps(value).encode(), 'library-owner', project, 'fresh', completed)

    def test_exact_46_first_attempt_and_discovery_proofs_accept_each_engine(self):
        self.assertEqual(len(set(self.module.CASES)), 46)
        self.assertEqual(len({file for file, _ in self.module.CASES}), 15)
        for project in ('chromium', 'firefox', 'webkit'):
            for completed in (False, True):
                with self.subTest(project=project, completed=completed):
                    self.assertEqual(self.admit(report(self.module.CASES, project, completed), project, completed),
                                     {(file, title, project) for file, title in self.module.CASES})

    def test_missing_unknown_wrong_type_oversized_selection_has_no_effects(self):
        good = ['library-owner', 'webkit', 'fresh']
        invalid = [[], good + ['extra']]
        for index, values in enumerate((
                [None, '', 'all', False, 'x' * 1025],
                [None, '', 'safari', 1, 'x' * 1025],
                [None, '', 'reused', {}, 'x' * 1025])):
            for value in values:
                arguments = good.copy()
                arguments[index] = value
                invalid.append(arguments)
        with patch('subprocess.run', side_effect=AssertionError('process effect')), \
                patch('os.mkdir', side_effect=AssertionError('output effect')):
            for arguments in invalid:
                with self.subTest(arguments=str(arguments)[:80]), self.assertRaises(ValueError):
                    self.module.selection(arguments)
            self.assertEqual(self.module.selection(good), ('library-owner', 'webkit', 'fresh'))

    def test_missing_extra_duplicate_file_title_and_project_never_certify(self):
        for kind in ('missing', 'extra', 'duplicate', 'file', 'title', 'project', 'duplicate-id'):
            value = report(self.module.CASES)
            specs = value['suites']
            if kind == 'missing':
                specs.pop()
            elif kind == 'extra':
                specs.append(copy.deepcopy(specs[0]))
                specs[-1]['specs'][0].update(id='extra', title='unexpected case')
            elif kind == 'duplicate':
                specs[-1] = copy.deepcopy(specs[0])
                specs[-1]['specs'][0]['id'] = 'different-id'
            elif kind == 'duplicate-id':
                specs[-1]['specs'][0]['id'] = specs[0]['specs'][0]['id']
            elif kind == 'project':
                specs[0]['specs'][0]['tests'][0]['projectName'] = 'chromium'
            else:
                specs[0]['specs'][0][kind] = 'unregistered'
            with self.subTest(kind=kind), self.assertRaises(ValueError):
                self.admit(value)

    def test_skips_retries_failures_errors_counts_and_runner_overrides_reject(self):
        changes = [
            lambda v: v.update(errors=[{'message': 'synthetic failure'}]),
            lambda v: v['stats'].update(expected=45),
            lambda v: v['stats'].update(expected=True),
            lambda v: v['stats'].update(skipped=1),
            lambda v: v['stats'].update(flaky=1),
            lambda v: v['config'].update(workers=2),
            lambda v: v['config'].update(shard={'current': 1, 'total': 2}),
            lambda v: v['config']['projects'][0].update(retries=1),
            lambda v: v['config']['projects'][0].update(repeatEach=2),
            lambda v: v['suites'][0]['specs'][0].update(ok=False),
            lambda v: v['suites'][0]['specs'][0]['tests'][0].update(status='skipped'),
            lambda v: v['suites'][0]['specs'][0]['tests'][0]['results'][0].update(retry=1),
            lambda v: v['suites'][0]['specs'][0]['tests'][0]['results'][0].update(status='failed'),
            lambda v: v['suites'][0]['specs'][0]['tests'][0]['results'][0].update(errors=[{}]),
        ]
        for index, change in enumerate(changes):
            value = report(self.module.CASES)
            change(value)
            with self.subTest(change=index), self.assertRaises(ValueError):
                self.admit(value)

    def test_malformed_oversized_duplicate_keys_and_deep_reports_reject(self):
        for raw in (b'', b'[]', b'{', b'x' * 2097153,
                    b'{"errors":[],"\\u0065rrors":[]}', b'{"value":NaN}'):
            with self.subTest(raw=raw[:50]), self.assertRaises(ValueError):
                self.module.admit(raw, 'library-owner', 'webkit', 'fresh', True)
        for malformed in (None, [], {}, {'errors': [], 'suites': [None]}):
            with self.subTest(value=malformed), self.assertRaises(ValueError):
                self.admit(malformed)
        value = report(self.module.CASES)
        for _ in range(9):
            value['suites'] = [{'title': 'nested', 'suites': value['suites']}]
        with self.assertRaises(ValueError):
            self.admit(value)

    def test_wrong_root_file_or_cross_file_suite_cannot_hide_a_title_context(self):
        for kind in ('unknown-root', 'cross-file'):
            value = report(self.module.CASES)
            value['suites'][0]['title'] = ('unregistered.spec.ts' if kind == 'unknown-root'
                                         else value['suites'][1]['title'])
            with self.subTest(kind=kind), self.assertRaises(ValueError):
                self.admit(value)

    def test_exponent_overflow_is_nonfinite_before_exact_proof_admission(self):
        value = report(self.module.CASES)
        value['stats']['duration'] = 0.25
        raw = json.dumps(value).encode()
        with patch('subprocess.run', side_effect=AssertionError('process effect')), \
                patch('os.mkdir', side_effect=AssertionError('output effect')):
            for exponent in (b'1e400', b'-1e400'):
                candidate = raw.replace(b'"duration": 0.25', b'"duration": ' + exponent)
                with self.subTest(exponent=exponent), self.assertRaises(ValueError):
                    self.module.admit(candidate, 'library-owner', 'webkit', 'fresh', True)
            self.assertEqual(len(self.module.admit(raw, 'library-owner', 'webkit', 'fresh', True)), 46)


if __name__ == '__main__':
    unittest.main()
