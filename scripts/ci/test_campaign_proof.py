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

    def test_accepted_audio_regressions_run_in_required_player_suite(self):
        # The repaired scope must be gated while the full manual counterevidence
        # remains strict; a workflow dispatch cannot substitute for required CI.
        source = (ROOT / '.github/workflows/app.yml').read_text()
        step = source.split('      - name: Verify copied-video audio and legacy preparation\n')[1]
        self.assertIn("if: always() && inputs.app == 'player' && matrix.engine == 'chromium'", step)
        self.assertIn('python3 apps/player/scripts/test-hls-followon.py --suite audio', step)
        self.assertIn('name: player-hls-audio-conversion-evidence', step)
        self.assertIn('.verification/hls-followon/*/receipt.json', step)
        self.assertIn('.verification/hls-followon/*/SHA256SUMS', step)
        manual = LAYOUT.read_text().split('  hls-followon:\n')[1]
        self.assertIn('run: python3 apps/player/scripts/test-hls-followon.py\n', manual)
        self.assertNotIn('--suite audio', manual)

    def test_nonkey_renderer_is_bounded_manual_and_preserves_raw_suite(self):
        proof = LAYOUT.read_text().split('  hls-followon:\n')[1]
        self.assertIn('timeout 180s pnpm --dir apps/player/e2e install --frozen-lockfile', proof)
        self.assertIn('timeout 240s pnpm --dir apps/player/e2e exec playwright install --with-deps chromium', proof)
        self.assertIn('test_hls_nonkey_renderer.py', proof)
        self.assertIn('node --test apps/player/e2e/test-hls-native-planes.mjs', proof)
        self.assertIn('KINOSAIL_HLS_RENDERER: "1"', proof)
        self.assertIn('run: python3 apps/player/scripts/test-hls-followon.py\n', proof)
        self.assertNotIn('KINOSAIL_HLS_RENDERER', (ROOT / '.github/workflows/app.yml').read_text())
        driver = (ROOT / 'apps/player/scripts/test-hls-followon.py').read_text()
        self.assertIn("if case['publicRenderer']['result'] != 'passed':", driver)
        self.assertIn("case['failures'].append('public_renderer')", driver)

    def test_hevc_preparation_adds_a_separate_required_public_gate(self):
        # Completed, stopped and adopted ownership can pass audio6 while losing
        # HEVC tail media. Require their own strict public proof and safe receipt.
        source = (ROOT / '.github/workflows/app.yml').read_text()
        step = source.split('      - name: Verify complete HEVC preparation\n')[1]
        self.assertIn("if: always() && inputs.app == 'player' && matrix.engine == 'chromium'", step)
        self.assertIn('timeout-minutes: 10', step)
        self.assertIn('python3 apps/player/scripts/test-hls-followon.py --suite hevc', step)
        self.assertIn('name: player-hls-hevc-preparation-evidence', step)
        self.assertIn('.verification/hls-hevc-startup/*/receipt.json', step)
        self.assertIn('.verification/hls-hevc-startup/*/SHA256SUMS', step)
        driver = (ROOT / 'apps/player/scripts/test-hls-followon.py').read_text()
        self.assertIn("choices=['all', 'audio', 'hevc']", driver)
        self.assertIn("controls(ROOT, RUN, binary, receipt, include_hevc=suite != 'audio', include_audio=suite != 'hevc')", driver)
        self.assertIn("'expectedCases': 12 if suite == 'all' else 4 if suite == 'hevc' else 6", driver)
        self.assertIn("if suite != 'hevc':", driver)
        manual = LAYOUT.read_text().split('  hls-followon:\n')[1]
        self.assertIn('run: python3 apps/player/scripts/test-hls-followon.py\n', manual)
        self.assertNotIn('--suite', manual)

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

    def test_restore_source_format_checkout_retains_required_base_history(self):
        source = LAYOUT.read_text()
        campaign = source.split('  campaign-proof:\n')[1]
        checkout = campaign.split('      - name: Validate explicit proof selection')[0]
        self.assertIn("fetch-depth: ${{ inputs.campaign_r06_suite == 'restore-source-format' && '0' || '1' }}", checkout)
        self.assertIn('timeout-minutes: 2', checkout)
        self.assertNotIn('fetch-depth:', source.split('  campaign-proof:\n')[0])
        self.assertIn('persist-credentials: false', checkout)


    def test_restore_rejected_source_controls_are_fixed_before_public_proof(self):
        source = LAYOUT.read_text().split('  campaign-proof:\n')[1]
        self.assertIn('name: Verify Restore rejected-source projection controls', source)
        self.assertIn('python3 -B -m unittest discover -s apps/subtitles/scripts -p test_campaign_r06_restore_projection.py', source)
        step = source.split('name: Verify Restore rejected-source projection controls')[1].split('      - name: Run exact owned public proof')[0]
        self.assertIn("if: env.CAMPAIGN_PROOF == 'R06' && env.CAMPAIGN_R06_SUITE == 'restore-source-format'", step)
        self.assertIn('timeout-minutes: 1', step)
        self.assertLess(source.index('name: Verify Restore rejected-source projection controls'), source.index('name: Run exact owned public proof'))


    def test_restore_qualified_source_controls_are_fixed_and_bounded_before_proof(self):
        source = LAYOUT.read_text().split('  campaign-proof:\n')[1]
        name = 'name: Verify Restore qualified-type controls'
        command = 'timeout --kill-after=2s 10s python3 -B -m unittest discover -s apps/subtitles/scripts -p test_campaign_r06_restore_qualified_tokens.py'
        self.assertEqual(source.count(command), 1)
        step = source.split(name)[1].split('      - name: Run exact owned public proof')[0]
        self.assertIn("if: env.CAMPAIGN_PROOF == 'R06' && env.CAMPAIGN_R06_SUITE == 'restore-source-format'", step)
        self.assertIn('timeout-minutes: 1', step)
        self.assertIn('run: ' + command, step)
        self.assertLess(source.index('name: Verify Restore rejected-source projection controls'), source.index(name))
        self.assertLess(source.index(name), source.index('name: Run exact owned public proof'))


    def test_restore_runtime_dispatch_is_fixed_and_rejects_foreign_selection(self):
        from unittest import mock
        for suite in ('restore-controls', 'restore-headers', 'restore-inspect-body'):
            with self.subTest(suite=suite), mock.patch.dict(os.environ, {'CAMPAIGN_R06_SUITE': suite}):
                actual = self.call('R06')
                self.assertEqual(actual.returncode, 0, actual.stderr)
                self.assertEqual(actual.stdout.splitlines(), ['apps/subtitles/scripts/campaign-r06-restore-browser.py'])
                for value in ('Q14', 'Q09', 'Q47'):
                    self.assertEqual(self.call(value).returncode, 2)
                    self.assertEqual(self.call(value).stdout, '')
        for suite in ('restore', 'restore-controls;echo unsafe', 'restore-headers\nrestore-inspect-body'):
            with self.subTest(suite=suite), mock.patch.dict(os.environ, {'CAMPAIGN_R06_SUITE': suite}):
                actual = self.call('R06')
                self.assertEqual(actual.returncode, 2)
                self.assertEqual(actual.stdout, '')

    def test_restore_runtime_choices_and_router_are_closed(self):
        workflow, router = LAYOUT.read_text(), ROUTER.read_text()
        choices = 'options: [protocol, save-controls, save-headers, save-body, source-format, restore-source-format, restore-controls, restore-headers, restore-inspect-body]'
        modes = 'protocol|save-controls|save-headers|save-body|source-format|restore-source-format|restore-controls|restore-headers|restore-inspect-body) ;;'
        self.assertIn(choices, workflow)
        self.assertEqual(workflow.count(modes), 1)
        self.assertEqual(router.count(modes), 1)
        branch = 'elif [[ "$r06_suite" == restore-controls || "$r06_suite" == restore-headers || "$r06_suite" == restore-inspect-body ]]; then\n      exec python3 apps/subtitles/scripts/campaign-r06-restore-browser.py'
        self.assertEqual(router.count(branch), 1)

    def test_restore_runtime_browser_dependencies_exclude_go_only_controls(self):
        source = LAYOUT.read_text().split('  campaign-proof:\n')[1]
        browser_conditions = [line for line in source.splitlines() if line.strip().startswith('if: (steps.selection.outputs.app')]
        self.assertEqual(len(browser_conditions), 3)
        for condition in browser_conditions:
            self.assertIn("env.CAMPAIGN_R06_SUITE == 'restore-headers'", condition)
            self.assertIn("env.CAMPAIGN_R06_SUITE == 'restore-inspect-body'", condition)
            self.assertNotIn("env.CAMPAIGN_R06_SUITE == 'restore-controls'", condition)
        self.assertIn("env.CAMPAIGN_R06_SUITE != 'source-format' && env.CAMPAIGN_R06_SUITE != 'restore-source-format' && env.CAMPAIGN_PROOF != 'Q47'", source)

    def test_restore_runtime_controls_are_fixed_bounded_and_precede_proof(self):
        source = LAYOUT.read_text().split('  campaign-proof:\n')[1]
        name = 'name: Verify Restore runtime admission controls'
        command = 'timeout --kill-after=2s 10s python3 -B -m unittest ' + ' '.join(
            'test_campaign_r06_restore_runtime_' + part for part in ('selection', 'sources', 'process', 'controls', 'projection', 'tools'))
        self.assertEqual(source.count('run: ' + command), 1)
        step = source.split(name)[1].split('      - name: Run exact owned public proof')[0]
        self.assertIn("if: env.CAMPAIGN_PROOF == 'R06' && (env.CAMPAIGN_R06_SUITE == 'restore-controls' || env.CAMPAIGN_R06_SUITE == 'restore-headers' || env.CAMPAIGN_R06_SUITE == 'restore-inspect-body')", step)
        self.assertIn('timeout-minutes: 1', step)
        self.assertIn('PYTHONPATH: apps/subtitles/scripts', step)
        self.assertLess(source.index(name), source.index('name: Run exact owned public proof'))

    def test_restore_runtime_rejects_metadata_before_prerequisites(self):
        source = LAYOUT.read_text().split('  campaign-proof:\n')[1]
        validation = source.split('      - uses: actions/setup-go')[0]
        guard = 'restore-controls|restore-headers|restore-inspect-body)\n              if [ "${{ inputs.architecture_metadata }}" == true ]; then exit 2; fi ;;'
        self.assertEqual(validation.count(guard), 1)
        collector = source.split('name: Collect bounded canonical source metadata')[1].split('      - name: Keep safe campaign proof')[0]
        for suite in ('restore-controls', 'restore-headers', 'restore-inspect-body'):
            self.assertIn("inputs.campaign_r06_suite != '" + suite + "'", collector)


if __name__ == '__main__':
    unittest.main()
