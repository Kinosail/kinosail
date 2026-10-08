"""Closed Library dispatch and argv must precede real fixture effects."""
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import unittest

from test_library_profile_admission import load

ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / '.github/workflows/layout-stability.yml'
VALIDATOR = ROOT / 'scripts/ci/validate-layout-selection.py'
MODULE = ROOT / 'apps/player/scripts/library_profile_admission.py'
DEFAULTS = ['false', 'false', 'false', 'false', 'primary', 'protocol', 'source-format']


class LibraryWorkflowTests(unittest.TestCase):
    def cli(self, script, arguments):
        return subprocess.run([sys.executable, str(script), *arguments], cwd=ROOT,
                              env=os.environ | {'PYTHONDONTWRITEBYTECODE': '1'},
                              capture_output=True, timeout=3)

    def test_actual_dispatch_admits_only_closed_manual_library_selection(self):
        result = self.cli(VALIDATOR, ['workflow_dispatch', 'Library', *DEFAULTS])
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, b'validated layout selection: Library\n')
        rejected = [['pull_request', 'Library', *DEFAULTS],
                    ['workflow_dispatch', 'library', *DEFAULTS]]
        for index, value in enumerate(('true', 'true', 'true', 'true', 'navigation',
                                       'save-body', 'primary')):
            values = DEFAULTS.copy()
            values[index] = value
            rejected.append(['workflow_dispatch', 'Library', *values])
        rejected += [['workflow_dispatch', 'Library'],
                     ['workflow_dispatch', 'Library', *DEFAULTS, 'extra']]
        for arguments in rejected:
            with self.subTest(arguments=arguments):
                result = self.cli(VALIDATOR, arguments)
                self.assertEqual(result.returncode, 2)
                self.assertEqual(result.stdout, b'')

    def test_selector_cli_and_owner_share_exact_46_cases(self):
        module = load()
        for project in module.PROJECTS:
            execution = module.playwright_arguments(project, False)
            discovery = module.playwright_arguments(project, True)
            result = self.cli(MODULE, [project, 'discovery'])
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout.decode().split('\0')[:-1], discovery)
            self.assertEqual(discovery, execution + ['--list', '--reporter=json'])
            self.assertEqual(execution[:3], ['exec', 'playwright', 'test'])
            files = execution[3:execution.index('--project=' + project)]
            self.assertEqual(set(files), {file for file, _ in module.CASES})
            self.assertEqual(len(files), 15)
            grep = execution[execution.index('--grep') + 1]
            self.assertTrue(all(re.search(grep, title + (' @smoke' if file == 'test-instance-watched-departure.spec.ts' else ''))
                                for file, title in module.CASES))
            self.assertFalse(re.search(grep, 'unknown extra title'))
            for option in ('--workers=1', '--retries=0', '--repeat-each=1'):
                self.assertIn(option, execution)
        source = (ROOT / 'scripts/ci/run-populated-settings.py').read_text()
        self.assertIn('playwright_arguments(args.project, False)', source)
        self.assertNotIn("re.escape(title) for _, title in CASES", source)

    def test_selector_keeps_declared_tag_suffix_without_admitting_changed_titles(self):
        module = load()
        argv = self.cli(MODULE, ['firefox', 'discovery']).stdout.decode().split('\0')[:-1]
        grep = argv[argv.index('--grep') + 1]
        tagged = [title for file, title in module.CASES if file == 'test-instance-watched-departure.spec.ts']
        self.assertEqual(len(tagged), 2)
        for title in tagged:
            self.assertTrue(re.search(grep, 'firefox test-instance-watched-departure.spec.ts ' + title + ' @smoke'))
            self.assertFalse(re.search(grep, title + ' @unknown'))
            self.assertFalse(re.search(grep, title + ' changed @smoke'))
        for file, title in module.CASES:
            if file != 'test-instance-watched-departure.spec.ts':
                self.assertTrue(re.search(grep, 'firefox ' + file + ' ' + title))
                self.assertFalse(re.search(grep, title + ' @smoke'))

    def test_invalid_selector_cli_rejects_without_arguments_for_fixture_effects(self):
        for arguments in ([], ['webkit'], ['webkit', 'execution', 'extra'],
                          ['all', 'discovery'], ['webkit', 'unknown'],
                          ['x' * 1025, 'discovery'], ['webkit', 'discovery\n']):
            with self.subTest(arguments=str(arguments)[:80]):
                result = self.cli(MODULE, arguments)
                self.assertEqual(result.returncode, 2)
                self.assertEqual(result.stdout, b'')

    def test_workflow_admission_routing_and_bounded_artifacts_preserve_other_lanes(self):
        source = WORKFLOW.read_text()
        block = re.split(r'\n  [a-z][\w-]*:\n', source.split('  library-owner:\n', 1)[1])[0]
        self.assertIn('needs: selection-admission', block)
        self.assertIn("inputs.campaign_proof == 'Library'", block)
        self.assertIn('fail-fast: false', block)
        self.assertIn('engine: [chromium, firefox, webkit]', block)
        self.assertIn('install --frozen-lockfile', block)
        self.assertIn('2097153', block)
        self.assertIn('2097152', block)
        self.assertIn('timeout --kill-after=2s 60s', block)
        self.assertIn('library_profile_admission.py', block)
        self.assertIn('run-library-profile.sh', block)
        self.assertIn('if: always()', block)
        self.assertIn('if-no-files-found: error', block)
        self.assertIn('include-hidden-files: true', block)
        self.assertNotRegex(block, r'continue-on-error|--retries=[1-9]|test-instance.sh up')
        for line in block.splitlines():
            if 'python3 scripts/ci/run-populated-settings.py' in line:
                self.assertIn('--admit-only', line)
        campaign = source.split('  campaign-proof:\n', 1)[1].split('\n  hls-followon:', 1)[0]
        self.assertIn("inputs.campaign_proof != 'Library'", campaign)
        for path in ('apps/player/scripts/run-library-profile.sh',
                     'apps/player/scripts/library_profile_admission.py',
                     'scripts/ci/test_library_workflow.py'):
            self.assertIn('"' + path + '"', source)


if __name__ == '__main__':
    unittest.main()
