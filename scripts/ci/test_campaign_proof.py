"""Guard manual proof dispatch without executing local app/tool runtimes."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
ROUTER = ROOT / 'scripts/ci/run-campaign-proof.sh'
LAYOUT = ROOT / '.github/workflows/layout-stability.yml'


class CampaignProofTests(unittest.TestCase):
    def call(self, value, args=()):
        with tempfile.TemporaryDirectory() as temporary:
            binary = Path(temporary) / 'python3'
            binary.write_text('#!/bin/sh\nprintf "%s\\n" "$@"\n')
            binary.chmod(0o700)
            return subprocess.run(['bash', str(ROUTER), value, *args],
                                  cwd=ROOT, env=os.environ | {'PATH': temporary + ':' + os.environ['PATH']},
                                  capture_output=True, text=True, timeout=3)

    def test_only_fixed_owned_drivers_dispatch(self):
        for value, path in [('R06', 'apps/subtitles/scripts/campaign-r06-public.py'),
                            ('Q14', 'apps/player/scripts/campaign-q14-public.py'),
                            ('Q09', 'apps/player/scripts/campaign-q09-public.py')]:
            with self.subTest(value=value):
                actual = self.call(value)
                self.assertEqual(actual.returncode, 0, actual.stderr)
                self.assertEqual(actual.stdout.splitlines(), [path])

    def test_unknown_or_extra_arguments_never_execute(self):
        for value in ['', 'none', '../R06', 'R06;echo bad', 'Q09\nQ14', 'Q01']:
            with self.subTest(value=value):
                actual = self.call(value)
                self.assertEqual(actual.returncode, 2)
                self.assertEqual(actual.stdout, '')
        self.assertEqual(self.call('R06', ['unexpected']).returncode, 2)

    def test_focused_route_does_not_replace_normal_ci(self):
        source = LAYOUT.read_text()
        self.assertIn('campaign_proof:', source)
        self.assertIn('options: [none, R06, Q14, Q09]', source)
        self.assertIn("if: github.event_name != 'workflow_dispatch' || inputs.campaign_proof == 'none'", source)
        self.assertIn("if: github.event_name == 'workflow_dispatch' && inputs.campaign_proof != 'none'", source)
        self.assertIn('name: Bounded campaign proof', source)
        self.assertIn('timeout-minutes: 12', source)
        self.assertIn('bash scripts/ci/run-campaign-proof.sh "$CAMPAIGN_PROOF"', source)
        self.assertNotIn('campaign_proof', (ROOT / '.github/workflows/ci.yml').read_text())
        self.assertNotIn('campaign_proof', (ROOT / '.github/workflows/app.yml').read_text())

    def test_artifact_paths_are_exact_safe_json_only(self):
        source = LAYOUT.read_text().split('  campaign-proof:\n')[1]
        for value in ('R06', 'Q14', 'Q09'):
            for name in ('receipt.json', 'results.json', 'source-manifest.json', 'artifact-manifest.json'):
                self.assertIn(f'            .verification/campaign-proof/{value}/{name}\n', source)
        artifact = source.split('name: Keep safe campaign proof')[1]
        self.assertNotIn('**', artifact)
        self.assertNotIn('*.json', artifact)
        self.assertNotIn('command.log', artifact)
        self.assertIn('retention-days: 3', artifact)
        self.assertIn('if-no-files-found: error', artifact)

    def test_restore_source_formatter_is_fixed_and_rejects_foreign_selection(self):
        from unittest import mock
        with mock.patch.dict(os.environ, {'CAMPAIGN_R06_SUITE': 'restore-source-format'}):
            actual = self.call('R06')
            self.assertEqual(actual.returncode, 0, actual.stderr)
            self.assertEqual(actual.stdout.splitlines(), ['apps/subtitles/scripts/campaign_r06_restore_format.py'])
            for value in ('Q14', 'Q09'):
                self.assertEqual(self.call(value).returncode, 2)
        with mock.patch.dict(os.environ, {'CAMPAIGN_R06_SUITE': 'restore-source-format;echo unsafe'}):
            self.assertEqual(self.call('R06').returncode, 2)

    def test_restore_source_format_requires_formatter_and_excludes_other_phases(self):
        source = LAYOUT.read_text().split('  campaign-proof:\n')[1]
        self.assertIn("env.CAMPAIGN_R06_SUITE == 'source-format' || env.CAMPAIGN_R06_SUITE == 'restore-source-format'", source)
        self.assertIn("env.CAMPAIGN_R06_SUITE != 'source-format' && env.CAMPAIGN_R06_SUITE != 'restore-source-format'", source)
        self.assertIn("python3 -B -m unittest discover -s apps/subtitles/scripts -p test_campaign_r06_restore_format.py", source)
        self.assertIn("if: always() && inputs.architecture_metadata && inputs.campaign_r06_suite != 'restore-source-format'", source)
        self.assertIn('if [ "$CAMPAIGN_R06_SUITE" == restore-source-format ] && [ "${{ inputs.architecture_metadata }}" == true ]; then exit 2; fi', source)



if __name__ == '__main__':
    unittest.main()
