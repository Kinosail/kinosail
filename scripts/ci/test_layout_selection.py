"""Dispatch flags must be admitted before any selected consumer effects."""
from itertools import product
import os
import copy
import json
import re
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / 'scripts/ci/validate-layout-selection.py'
WORKFLOW = ROOT / '.github/workflows/layout-stability.yml'


class LayoutSelectionTests(unittest.TestCase):
    def test_audio_origin_cli_admits_only_its_closed_owner_and_no_effects(self):
        valid = ["workflow_dispatch", "HLS", "true", "false", "false", "false", "primary", "protocol", "source-format", "true"]
        result = self.selection(valid)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, "validated layout selection: audio-origin\n")
        for index, value in ((0, "pull_request"), (1, "Library"), (1, "HLS-navigation"),
                             (1, "Q14"), (2, "false"), (3, "true"), (4, "true"),
                             (9, ""), (9, "TRUE"), (9, "1"), (9, "true\n"), (9, "x" * 33)):
            with self.subTest(index=index, value=value):
                arguments = valid.copy()
                arguments[index] = value
                self.assertEqual(self.selection(arguments).returncode, 2)
        for arguments in (valid[:-1], valid + ["false"]):
            self.assertEqual(self.selection(arguments).returncode, 2)

    def test_fixed_navigation_lane_preserves_only_exact_webkit_recipe(self):
        source = WORKFLOW.read_text()
        block = source.split('  hls-navigation:\n', 1)[1]
        self.assertIn("inputs.campaign_proof == 'HLS-navigation'", block)
        self.assertIn('    needs: selection-admission\n', block)
        self.assertNotRegex(block, r'setup-go|go test|go build|test-layout-stability-local|continue-on-error')
        self.assertIn('install --frozen-lockfile', block)
        self.assertIn('playwright install --with-deps webkit', block)
        self.assertIn('playwright test player-hls-navigation.spec.ts --project=webkit --workers=1 --retries=0', block)
        self.assertIn('hls-navigation-fixture.test.mjs', block)
        self.assertIn('python3 scripts/ci/check-hls-navigation-result.py', block)
        for name in ('source-receipt.json', 'results-webkit.json', '**/trace.zip'):
            self.assertIn(name, block)
        campaign = source.split('  campaign-proof:\n', 1)[1].split('\n  hls-followon:', 1)[0]
        self.assertIn("inputs.campaign_proof != 'HLS-navigation'", campaign)

    def test_fixed_discovery_is_bounded_separate_and_retained(self):
        block = WORKFLOW.read_text().split('  hls-navigation:\n', 1)[1]
        discovery = block.split('      - name: Discover exact WebKit HLS titles\n', 1)[1].split('      - name: Run exact strict HLS navigation file', 1)[0]
        self.assertIn('timeout --kill-after=2s 60s pnpm --dir apps/player/e2e exec playwright test player-hls-navigation.spec.ts --project=webkit --workers=1 --retries=0 --list --reporter=json', discovery)
        self.assertIn('head -c 2097153', discovery)
        self.assertIn('test "$(wc -c < .verification/hls-navigation/list-webkit.json)" -le 2097152', discovery)
        self.assertNotRegex(discovery, r'go test|ffmpeg|test-instance|install')
        self.assertIn('            .verification/hls-navigation/list-webkit.json\n', block)
        self.assertIn("'discoveryOnly': True", block)

    def test_fixed_result_admission_rejects_partial_skipped_retried_or_unknown_identity(self):
        def report(completed):
            return {'errors': [], 'stats': {'expected': 13, 'skipped': 0, 'unexpected': 0, 'flaky': 0},
                    'suites': [{'specs': [{'id': 'identity-' + str(i), 'file': 'player-hls-navigation.spec.ts',
                        'ok': True, 'tests': [{'projectName': 'webkit', 'expectedStatus': 'passed',
                            'status': 'expected', 'results': [{'status': 'passed', 'retry': 0, 'errors': []}] if completed else []}]}]
                        } for i in range(13)]}
        discovery, valid = report(False), report(True)
        mutations = [('valid', valid, 0)]
        for kind in ('missing', 'extra', 'duplicate', 'skipped', 'retried', 'failed', 'unknown', 'project', 'errors'):
            changed = copy.deepcopy(valid)
            first = changed['suites'][0]['specs'][0]
            if kind == 'missing': changed['suites'].pop()
            elif kind == 'extra': changed['suites'].append(copy.deepcopy(changed['suites'][0]))
            elif kind == 'duplicate': changed['suites'][1]['specs'][0]['id'] = first['id']
            elif kind == 'unknown': first['id'] = 'not-discovered'
            elif kind == 'project': first['tests'][0]['projectName'] = 'firefox'
            elif kind == 'errors': changed['errors'] = [{'message': 'private controlled error'}]
            elif kind == 'skipped': first['tests'][0]['results'][0]['status'] = 'skipped'
            elif kind == 'retried': first['tests'][0]['results'][0]['retry'] = 1
            elif kind == 'failed': first['tests'][0]['results'][0]['status'] = 'failed'
            mutations.append((kind, changed, 2))
        for name, result, expected in mutations + [('malformed', '{', 2), ('oversized', 'x' * 2097153, 2),
                ('nonobject-suite', {**valid, 'suites': [None]}, 2), ('nonobject-spec', {**valid, 'suites': [{'specs': [None]}]}, 2),
                ('duplicate-key', '{"errors":[],"errors":[]}', 2)]:
            with self.subTest(kind=name), tempfile.TemporaryDirectory(prefix='hls-result-') as directory:
                root = Path(directory)
                output = root / '.verification/hls-navigation'
                output.mkdir(parents=True)
                (output / 'list-webkit.json').write_text(json.dumps(discovery))
                (output / 'results-webkit.json').write_text(result if isinstance(result, str) else json.dumps(result))
                before = sorted(str(path.relative_to(root)) for path in root.rglob('*'))
                actual = subprocess.run([sys.executable, str(ROOT / 'scripts/ci/check-hls-navigation-result.py')],
                                        cwd=root, capture_output=True, text=True, timeout=2)
                self.assertEqual(actual.returncode, expected)
                self.assertEqual(sorted(str(path.relative_to(root)) for path in root.rglob('*')), before)
                self.assertNotIn('private controlled error', actual.stdout + actual.stderr)

    def test_navigation_rejects_every_ignored_override_before_effects(self):
        valid = ['workflow_dispatch', 'HLS-navigation', 'false', 'false', 'false', 'false', 'primary', 'protocol', 'source-format', 'false']
        for index, value in ((2, 'true'), (3, 'true'), (4, 'true'), (5, 'true'),
                             (6, 'home'), (7, 'save-controls'), (8, 'primary')):
            with self.subTest(field=index):
                result = self.selection([*valid[:index], value, *valid[index + 1:]])
                self.assertEqual(result.returncode, 2)
                self.assertEqual(result.stderr, 'unsupported layout selection\n')
        for values in (valid[:5], valid[:-1], [*valid, 'false'], [*valid[:6], 'x' * 4097, *valid[7:]]):
            result = self.selection(values)
            self.assertEqual(result.returncode, 2)

    def test_existing_subsuite_and_metadata_combinations_remain_supported(self):
        for campaign, field, options in (
                ('Q14', 6, ('primary', 'navigation', 'cold', 'bfcache', 'htmx', 'shows', 'search', 'safety', 'home')),
                ('R06', 7, ('protocol', 'save-controls', 'save-headers', 'save-body', 'source-format',
                            'restore-source-format', 'restore-controls', 'restore-headers', 'restore-inspect-body')),
                ('Q47', 8, ('source-format', 'primary', 'recovery', 'supersession', 'contracts'))):
            for option in options:
                valid = ['workflow_dispatch', campaign, 'false', 'false', 'false', 'false', 'primary', 'protocol', 'source-format', 'false']
                valid[field] = option
                with self.subTest(campaign=campaign, option=option):
                    self.assertEqual(self.selection(valid).returncode, 0)
        for campaign in ('R06', 'Q14', 'Q09'):
            valid = ['workflow_dispatch', campaign, 'false', 'false', 'false', 'true', 'primary', 'protocol', 'source-format', 'false']
            self.assertEqual(self.selection(valid).returncode, 0)

    def test_conflicting_subsuite_and_unused_metadata_reject_before_effects(self):
        for campaign, field, value in (('none', 5, 'true'), ('HLS', 5, 'true'), ('Q47', 5, 'true'),
                                       ('HLS', 6, 'home'), ('Q14', 7, 'save-controls'), ('R06', 8, 'primary')):
            valid = ['workflow_dispatch', campaign, 'false', 'false', 'false', 'false', 'primary', 'protocol', 'source-format', 'false']
            valid[field] = value
            self.assertEqual(self.selection(valid).returncode, 2)
        for suite in ('restore-source-format', 'restore-controls', 'restore-headers', 'restore-inspect-body'):
            valid = ['workflow_dispatch', 'R06', 'false', 'false', 'false', 'false', 'primary', suite, 'source-format', 'false']
            self.assertEqual(self.selection(valid).returncode, 0)
            valid[5] = 'true'
            self.assertEqual(self.selection(valid).returncode, 2)

    def test_every_consumer_depends_on_selection_before_setup(self):
        source = WORKFLOW.read_text()
        self.assertIn('  selection-admission:\n', source)
        admission = source.split('  selection-admission:\n', 1)[1].split('\n  native:', 1)[0]
        self.assertIn('python3 scripts/ci/validate-layout-selection.py', admission)
        self.assertNotRegex(admission, r'setup-go|apt-get|pnpm|go test|test-layout-stability-local')
        for job in ('native', 'campaign-proof', 'hls-followon', 'hls-navigation'):
            with self.subTest(job=job):
                block = re.split(r'\n  [a-z][a-z-]*:\n', source.split('  ' + job + ':\n', 1)[1], maxsplit=1)[0]
                self.assertIn('    needs: selection-admission\n', block)
                self.assertNotIn('always()', block.split('    steps:', 1)[0])
        for flag in ('campaign_proof', 'hls_remaining_proof', 'hls_audio_timing', 'hls_audio_installation', 'architecture_metadata', 'campaign_q14_suite', 'campaign_r06_suite', 'campaign_q47_suite', 'hls_audio_origin'):
            self.assertIn('inputs.' + flag, admission)

    def test_admission_inputs_trigger_pull_request_workflow(self):
        paths = WORKFLOW.read_text().split('  pull_request:\n', 1)[1].split('  workflow_dispatch:', 1)[0]
        for path in ('scripts/ci/validate-layout-selection.py', 'scripts/ci/test_layout_selection.py'):
            with self.subTest(path=path):
                self.assertIn('      - "' + path + '"\n', paths)

    def selection(self, arguments):
        # The real validator runs with inert tripwire commands in an empty
        # owned directory. No media generator, Go or application is launched.
        with tempfile.TemporaryDirectory(prefix='layout-selection-') as directory:
            root = Path(directory)
            tools = root / 'tools'
            tools.mkdir()
            for name in ('go', 'ffmpeg', 'pnpm'):
                command = tools / name
                command.write_text('#!/bin/sh\nprintf effect > "$SELECTION_TRIPWIRE"\nexit 99\n')
                command.chmod(0o700)
            env = dict(os.environ, PATH=str(tools), SELECTION_TRIPWIRE=str(root / 'effect'))
            result = subprocess.run([sys.executable, str(SCRIPT), *arguments], cwd=root,
                                    env=env, capture_output=True, text=True, timeout=2)
            self.assertFalse((root / 'effect').exists())
            self.assertEqual(sorted(p.name for p in root.iterdir()), ['tools'])
            return result

    def test_supported_defaults_and_fixed_hls_modes(self):
        selections = [(['pull_request', 'none', 'false', 'false', 'false', 'false', 'primary', 'protocol', 'source-format', 'false'], 'none')]
        selections += [(['workflow_dispatch', campaign, 'false', 'false', 'false', 'false', 'primary', 'protocol', 'source-format', 'false'], campaign)
                       for campaign in ('none', 'R06', 'Q14', 'Q09', 'Q47', 'HLS', 'HLS-navigation')]
        selections += [(['workflow_dispatch', 'HLS', 'true', timing, installation, 'false', 'primary', 'protocol', 'source-format', 'false'], mode)
                       for timing, installation, mode in (
                           ('false', 'false', 'remaining'), ('true', 'false', 'audio-timing'),
                           ('false', 'true', 'audio-installation'))]
        for arguments, mode in selections:
            with self.subTest(arguments=arguments):
                result = self.selection(arguments)
                self.assertEqual(result.returncode, 0, result.stderr[:200])
                self.assertEqual(result.stdout, 'validated layout selection: ' + mode + '\n')
                self.assertEqual(result.stderr, '')

    def test_ignored_and_conflicting_flags_reject_without_effects(self):
        selections = [['workflow_dispatch', campaign, *flags, 'false', 'primary', 'protocol', 'source-format', 'false']
                      for campaign in ('none', 'R06', 'Q14', 'Q09', 'Q47')
                      for flags in product(('false', 'true'), repeat=3) if 'true' in flags]
        selections += [['workflow_dispatch', 'HLS', *flags, 'false', 'primary', 'protocol', 'source-format', 'false'] for flags in (
            ('false', 'true', 'false'), ('false', 'false', 'true'),
            ('false', 'true', 'true'), ('true', 'true', 'true'))]
        selections += [['pull_request', 'HLS', 'false', 'false', 'false', 'false', 'primary', 'protocol', 'source-format', 'false'],
                       ['pull_request', 'none', 'true', 'false', 'false', 'false', 'primary', 'protocol', 'source-format', 'false']]
        for arguments in selections:
            with self.subTest(arguments=arguments):
                result = self.selection(arguments)
                self.assertEqual(result.returncode, 2)
                self.assertEqual(result.stdout, '')
                self.assertEqual(result.stderr, 'unsupported layout selection\n')

    def test_missing_extra_unknown_malformed_oversized_values_reject(self):
        valid = ['workflow_dispatch', 'HLS', 'true', 'false', 'false', 'false', 'primary', 'protocol', 'source-format', 'false']
        selections = [[], valid[:-1], [*valid, 'false']]
        for index, values in enumerate((['', 'schedule', 'workflow_dispatch\n'],
                                        ['', 'hls', 'OTHER', 'x' * 4097],
                                        ['', 'TRUE', '1', 'false\n'],
                                        ['', 'no', 'true\n'], ['', 'FALSE', 'null'], ['', 'TRUE', 'true\n'], ['', 'unknown', 'home\n'], ['', 'unknown', 'protocol\n'], ['', 'unknown', 'primary\n'], ['', 'TRUE', '1', 'false\n'])):
            for value in values:
                selections.append([*valid[:index], value, *valid[index + 1:]])
        for arguments in selections:
            with self.subTest(lengths=[len(value) for value in arguments]):
                result = self.selection(arguments)
                self.assertEqual(result.returncode, 2)
                self.assertEqual(result.stdout, '')
                self.assertEqual(result.stderr, 'unsupported layout selection\n')


if __name__ == '__main__':
    unittest.main()
