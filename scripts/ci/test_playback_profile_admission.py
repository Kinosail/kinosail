"""Exact playback collection and explicit CDP boundaries never certify skips as passes."""
import copy
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch
from test_library_profile_admission import ROOT, load, report
import test_library_fixture_owner as fixture_peers


def playback_report(cases, project, completed, skips=()):
    value = report(cases, project, completed)
    files = {}
    for row in value['suites']:
        spec = row['specs'][0]
        title = spec['title']; parts = title.split(' › ')
        container = files.setdefault(spec['file'], {'title': spec['file'], 'specs': [], 'suites': []})
        for context in parts[:-1]:
            child = next((s for s in container['suites'] if s['title'] == context), None)
            if child is None:
                child = {'title': context, 'specs': [], 'suites': []}; container['suites'].append(child)
            container = child
        spec['title'] = parts[-1]; container['specs'].append(spec)
        test = spec['tests'][0]
        test['annotations'] = []
        if completed and (spec['file'], title) in skips:
            reason = [{'type': 'skip', 'description': 'network emulation uses Chromium DevTools'}]
            test.update(expectedStatus='skipped', status='skipped', annotations=reason)
            test['results'][0].update(status='skipped', annotations=copy.deepcopy(reason))
    value['suites'] = list(files.values())
    if completed: value['stats'].update(expected=len(cases)-len(skips), skipped=len(skips))
    return value


class PlaybackProfileTests(unittest.TestCase):
    def setUp(self):
        directory = str(ROOT / 'apps/player/scripts'); sys.path.insert(0, directory)
        self.addCleanup(lambda: sys.path.remove(directory))
        self.module = load()

    def cases(self, project):
        return self.module.profile_cases('playback-start', project)

    def skips(self, cases, project):
        return {row for row in cases if row[0] == 'playback-bandwidth.spec.ts' and project != 'chromium'}

    def admit(self, value, project, completed=True):
        return self.module.admit(json.dumps(value).encode(), 'playback-start', project, 'fresh', completed)

    def test_collection_exact27_or25_keeps_declared_nested_context_and_all_five_cdp_rows(self):
        for project, count in (('chromium', 27), ('firefox', 25), ('webkit', 25)):
            cases = self.cases(project)
            self.assertEqual(len(cases), count)
            self.assertEqual(len(set(cases)), count)
            self.assertEqual(sum(file == 'playback-bandwidth.spec.ts' for file, _ in cases), 5)
            self.assertEqual(sum(file == 'test-instance-direct-retry.spec.ts' for file, _ in cases), 2 if project == 'chromium' else 0)
            value = playback_report(cases, project, False)
            with patch('subprocess.run', side_effect=AssertionError('process effect')):
                self.assertEqual(len(self.admit(value, project, False)), count)
            argv = self.module.playwright_arguments(project, True, 'playback-start')
            grep = argv[argv.index('--grep')+1]
            import re
            for file, title in cases:
                candidate = project + ' ' + file + ' ' + title.replace(' › ', ' ')
                self.assertTrue(re.search(grep, candidate) or re.search(grep, candidate + ' @smoke'))
            self.assertIn('--workers=1', argv); self.assertIn('--retries=0', argv); self.assertIn('--repeat-each=1', argv)

    def test_declared_startup_smoke_tags_are_selected_without_changing_identity(self):
        import re
        for project in ('chromium', 'firefox', 'webkit'):
            argv = self.module.playwright_arguments(project, True, 'playback-start')
            grep = argv[argv.index('--grep')+1]
            for source in ('compatible', 'automatic'):
                title = f'blocked autoplay leaves one Play control that starts {source} video from saved progress'
                self.assertTrue(re.search(grep, project + ' playback-startup.spec.ts ' + title + ' @smoke'))
                self.assertIn(('playback-startup.spec.ts', title), self.cases(project))

    def test_invalid_profile_project_state_reject_before_effects(self):
        with patch('subprocess.run', side_effect=AssertionError('process effect')), patch('os.mkdir', side_effect=AssertionError('output effect')):
            for index, values in enumerate(((None, '', 'unknown', [], 'x'*1025), (None, '', 'safari', {}, 'x'*1025), (None, '', 'reused', [], 'x'*1025))):
                for value in values:
                    arguments = ['playback-start', 'chromium', 'fresh']; arguments[index] = value
                    with self.assertRaises(ValueError): self.module.selection(arguments)
            for project in (None, 'unknown', {}, 'x'*1025):
                with self.assertRaises(ValueError): self.cases(project)

    def test_completed_proof_separates_passes_from_exact_nonchromium_cdp_skips(self):
        for project, count in (('chromium', 27), ('firefox', 25), ('webkit', 25)):
            cases = self.cases(project); skips = self.skips(cases, project)
            value = playback_report(cases, project, True, skips)
            with patch('subprocess.run', side_effect=AssertionError('process effect')):
                self.assertEqual(len(self.admit(value, project)), count)
            self.assertEqual(value['stats']['expected'], 27 if project == 'chromium' else 20)
            self.assertEqual(value['stats']['skipped'], 0 if project == 'chromium' else 5)

    def test_retained_skip_schema_and_optional_result_annotations_remain_closed(self):
        cases = self.cases('webkit'); skips = self.skips(cases, 'webkit')
        good = playback_report(cases, 'webkit', True, skips)
        for spec in good['suites'][0]['specs']:
            test = spec['tests'][0]
            test['annotations'][0]['location'] = {'file': '/owned/e2e/playback-bandwidth.spec.ts', 'line': 69, 'column': 2}
            test['results'][0].pop('annotations')
        self.assertEqual(len(self.admit(good, 'webkit')), 25)
        for invalid in ([], None, [{'type': 'skip', 'description': 'unknown'}],
                        [{'type': 'skip', 'description': 'network emulation uses Chromium DevTools'}]*2):
            value = copy.deepcopy(good)
            value['suites'][0]['specs'][0]['tests'][0]['annotations'] = invalid
            with self.assertRaises(ValueError): self.admit(value, 'webkit')
        value = copy.deepcopy(good)
        value['suites'][0]['specs'][0]['tests'][0]['annotations'][0]['location']['line'] = False
        with self.assertRaises(ValueError): self.admit(value, 'webkit')

    def test_missing_extra_changed_context_or_skip_reason_and_retry_reject_without_effects(self):
        cases = self.cases('firefox'); skips = self.skips(cases, 'firefox')
        good = playback_report(cases, 'firefox', True, skips)
        changes = [lambda v: v['suites'].pop(), lambda v: v['suites'].append(copy.deepcopy(v['suites'][0])),
                   lambda v: v['stats'].update(expected=25, skipped=0),
                   lambda v: v['config']['projects'][0].update(retries=1),
                   lambda v: v['suites'][0]['specs'][0]['tests'][0]['results'][0].update(retry=1),
                   lambda v: v['suites'][0]['specs'][0]['tests'][0].update(projectName='chromium'),
                   lambda v: v['suites'][0]['specs'][0]['tests'][0]['annotations'][0].update(description='unknown'),
                   lambda v: v['suites'][0]['specs'][0]['tests'][0]['results'][0]['annotations'][0].update(description='unknown')]
        with patch('subprocess.run', side_effect=AssertionError('process effect')), patch('os.mkdir', side_effect=AssertionError('output effect')):
            for change in changes:
                value = copy.deepcopy(good); change(value)
                with self.assertRaises(ValueError): self.admit(value, 'firefox')
            for raw in (b'', b'{', b'x'*2097153):
                with self.assertRaises(ValueError): self.module.admit(raw, 'playback-start', 'firefox', 'fresh', True)
            value = playback_report(self.cases('chromium'), 'chromium', True)
            nested = next(s for s in value['suites'] if s['title'] == 'test-instance-direct-retry.spec.ts')
            nested['suites'][0]['title'] += ' unknown'
            with self.assertRaises(ValueError): self.admit(value, 'chromium')

    def test_actual_caller_rejects_incomplete_selection_before_owner_and_accepts_only_full_discovery(self):
        cases = self.cases('chromium')
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); proof=root/'discovery.json'; output=root/'output'
            command=[sys.executable,str(ROOT/'scripts/ci/run-populated-settings.py'),'--url','http://localhost:38127',
                     '--output',str(output),'--profile','playback-start','--project','chromium','--state','fresh',
                     '--discovery',str(proof),'--admit-only']
            env={key:value for key,value in os.environ.items() if not key.startswith('KINOSAIL_') and key!='PLAYWRIGHT_CHANNEL'}
            env['PYTHONDONTWRITEBYTECODE']='1'
            for valid in (True,False):
                value=playback_report(cases,'chromium',False)
                if not valid: value['suites'].pop()
                proof.write_text(json.dumps(value))
                result=subprocess.run(command,capture_output=True,timeout=3,env=env)
                self.assertEqual(result.returncode,0 if valid else 2,result.stderr)
                self.assertFalse(output.exists()); self.assertEqual(set(root.iterdir()),{proof})

    def test_wrapper_has_one_fresh_owner_and_joined_owned_cleanup(self):
        peer=fixture_peers.LibraryFixtureOwnerTests();peer.setUp();self.addCleanup(peer.temporary.cleanup)
        peer.discovery.write_text(json.dumps(playback_report(self.cases('chromium'),'chromium',False)))
        result=peer.run_caller(['chromium',str(peer.discovery),str(peer.output),'playback-start'])
        self.assertEqual(result.returncode,0,result.stderr)
        rows=peer.rows();self.assertEqual(sum(kind=='owner' for kind,_ in rows),1);peer.assert_cleanup(rows)

if __name__ == '__main__': unittest.main()
