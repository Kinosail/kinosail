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
        self.assertIn('options: [none, R06, Q14, Q09, Q47, HLS]', source)
        self.assertIn("if: github.event_name != 'workflow_dispatch' || inputs.campaign_proof == 'none'", source)
        self.assertIn("if: github.event_name == 'workflow_dispatch' && inputs.campaign_proof != 'none'", source)
        self.assertIn('name: Bounded campaign proof', source)
        self.assertIn('timeout-minutes: 12', source)
        self.assertIn('bash scripts/ci/run-campaign-proof.sh "$CAMPAIGN_PROOF"', source)
        self.assertNotIn('campaign_proof', (ROOT / '.github/workflows/ci.yml').read_text())
        self.assertNotIn('campaign_proof', (ROOT / '.github/workflows/app.yml').read_text())

    def test_hls_followon_is_manual_bounded_and_keeps_only_receipts(self):
        source = LAYOUT.read_text()
        self.assertIn("inputs.campaign_proof != 'none' && inputs.campaign_proof != 'HLS'", source)
        proof = source.split('  hls-followon:\n')[1]
        self.assertIn("if: github.event_name == 'workflow_dispatch' && inputs.campaign_proof == 'HLS'", proof)
        self.assertIn('timeout-minutes: 15', proof)
        self.assertIn('b4e72894ad26c809ed0104805f5415a97be75212b9fcf9d60b89ad25bb3d43e3', proof)
        self.assertIn('python3 apps/player/scripts/test-hls-followon.py', proof)
        self.assertIn('.verification/hls-followon/*/receipt.json', proof)
        self.assertIn('.verification/hls-followon/*/SHA256SUMS', proof)
        self.assertNotIn('server.log', proof)

    def test_artifact_paths_are_exact_safe_json_only(self):
        source = LAYOUT.read_text().split('  campaign-proof:\n')[1].split('\n  hls-followon:\n')[0]
        for value in ('R06', 'Q14', 'Q09'):
            for name in ('receipt.json', 'results.json', 'source-manifest.json', 'artifact-manifest.json'):
                self.assertIn(f'            .verification/campaign-proof/{value}/{name}\n', source)
        artifact = source.split('name: Keep safe campaign proof')[1]
        self.assertNotIn('**', artifact)
        self.assertNotIn('*.json', artifact)
        self.assertNotIn('command.log', artifact)
        self.assertIn('retention-days: 3', artifact)
        self.assertIn('if-no-files-found: error', artifact)

    def test_q47_secondary_modes_are_closed_and_foreign_modes_are_rejected(self):
        source = LAYOUT.read_text().split('  campaign-proof:\n')[1]
        router = ROUTER.read_text()
        self.assertIn('options: [source-format, primary, recovery, supersession, contracts]', LAYOUT.read_text())
        for text in (source, router):
            self.assertIn('source-format|primary|recovery|supersession|contracts)', text)
            self.assertIn('CAMPAIGN_Q47_SUITE', text)
        self.assertIn('if [ "$CAMPAIGN_PROOF" != Q47 ] && [ "$CAMPAIGN_Q47_SUITE" != source-format ]; then exit 2; fi', source)
        self.assertIn('if [[ "$1" != Q47 && "$q47_suite" != source-format ]]; then exit 2; fi', router)
        self.assertIn('if [ "$CAMPAIGN_PROOF" == Q47 ] && [ "${{ inputs.architecture_metadata }}" == true ]; then exit 2; fi', source)

    def test_q47_existing_and_fault_controls_are_bounded_before_public_proof(self):
        source = LAYOUT.read_text().split('  campaign-proof:\n')[1]
        self.assertIn("if: env.CAMPAIGN_PROOF == 'Q47' && env.CAMPAIGN_Q47_SUITE != 'source-format'", source)
        self.assertIn('timeout 180s npm ci --prefix engineering/documentation --ignore-scripts', source)
        self.assertIn('timeout --kill-after=2s 10s node --test engineering/documentation/test-install-builder.cjs engineering/documentation/test-install-builder-recovery.cjs', source)
        self.assertLess(source.index('name: Verify fixed Compose recovery controls'), source.index('name: Run exact owned public proof'))


if __name__ == '__main__':
    unittest.main()
