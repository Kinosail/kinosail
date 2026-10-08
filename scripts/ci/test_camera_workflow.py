"""Closed Camera dispatch precedes dependency and disposable-owner effects."""
import os
from pathlib import Path
import re
import subprocess
import sys
import unittest
from test_library_profile_admission import load

ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / '.github/workflows/layout-stability.yml'
DEFAULTS = ['false', 'false', 'false', 'false', 'primary', 'protocol', 'source-format', 'false']

class CameraWorkflowTests(unittest.TestCase):
    def selection(self, argv):
        return subprocess.run([sys.executable, str(ROOT / 'scripts/ci/validate-layout-selection.py'), *argv],
                              capture_output=True, timeout=3, env=os.environ | {'PYTHONDONTWRITEBYTECODE': '1'})

    def test_only_closed_manual_selection_has_an_effect_route(self):
        result = self.selection(['workflow_dispatch', 'Camera', *DEFAULTS])
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, b'validated layout selection: Camera\n')
        rejected = [['pull_request', 'Camera', *DEFAULTS], ['workflow_dispatch', 'camera', *DEFAULTS],
                    ['workflow_dispatch', 'Camera'], ['workflow_dispatch', 'Camera', *DEFAULTS, 'extra']]
        for index, value in enumerate(('true', 'true', 'true', 'true', 'navigation', 'save-body', 'primary', 'true')):
            values = DEFAULTS.copy(); values[index] = value
            rejected.append(['workflow_dispatch', 'Camera', *values])
        for index in range(10):
            args = ['workflow_dispatch', 'Camera', *DEFAULTS];args[index] = 'x' * 33
            rejected.append(args)
        for args in rejected:
            result = self.selection(args)
            self.assertEqual(result.returncode, 2)
            self.assertEqual(result.stdout, b'')

    def block(self):
        return re.split(r'\n  [a-z][\w-]*:\n', WORKFLOW.read_text().split('  camera-owner:\n', 1)[1])[0]

    def test_fixed_three_engine_fixture_reuses_one_closed_owner_after_admission(self):
        source = WORKFLOW.read_text(); block = self.block()
        self.assertIn('needs: selection-admission', block)
        self.assertIn("inputs.campaign_proof == 'Camera'", block)
        self.assertIn('engine: [chromium, firefox, webkit]', block)
        self.assertIn('fail-fast: false', block)
        self.assertIn('discovery camera-fake', block)
        self.assertIn('--profile camera-fake', block)
        self.assertIn('env -u KINOSAIL_CAMERA_PROFILE python3 scripts/ci/run-populated-settings.py', block)
        self.assertIn('KINOSAIL_CAMERA_PROFILE: "1"', block)
        self.assertIn('KINOSAIL_BROWSER_TEST: "1"', block)
        self.assertIn('KINOSAIL_TEST_INSTANCE: "1"', block)
        self.assertIn('KINOSAIL_E2E_URL: ${{ matrix.engine', block)
        self.assertIn('timeout --kill-after=2s 60s', block)
        self.assertIn('2097153', block); self.assertIn('2097152', block)
        self.assertEqual(block.count('run: ./apps/player/scripts/run-library-profile.sh'), 1)
        self.assertIn('"$RUNNER_TEMP/camera19-${CAMERA_PROJECT}" camera-fake', block)
        self.assertIn("inputs.campaign_proof != 'Camera'", source.split('  campaign-proof:\n', 1)[1].split('  hls-followon:', 1)[0])
        self.assertNotRegex(block, r'test-instance.sh up|--retries=[1-9]|continue-on-error|Allow access|curl .*approval')
        module = load()
        for project in module.PROJECTS:
            command = module.playwright_arguments(project, False, 'camera-fake')
            for option in ('--workers=1', '--retries=0', '--repeat-each=1'): self.assertIn(option, command)
            self.assertEqual(command[3:command.index('--project=' + project)], ['quick-connect-scan.spec.ts'])

    def test_source_and_failure_artifacts_close_guard_relay_and_recipe(self):
        block = self.block(); source = WORKFLOW.read_text()
        for path in ('apps/player/e2e/camera-profile-fixture.ts',
                     'scripts/ci/library-internal-relay.mjs', 'scripts/ci/browser-fixture-tls.sh',
                     'apps/player/scripts/run-library-profile.sh', 'apps/player/scripts/library_profile_admission.py',
                     'apps/player/scripts/generate-test-media.sh', 'apps/player/e2e/pnpm-lock.yaml',
                     'apps/player/Containerfile', 'apps/player/Containerfile.test',
                     'scripts/ci/run-populated-settings.py', 'packages/playerweb/player.js',
                     'apps/player/internal/server/server.go'):
            self.assertIn("'" + path + "'", block)
        self.assertIn('from library_profile_admission import CAMERA_CASES, playwright_arguments', block)
        self.assertIn('for file, title in CAMERA_CASES', block)
        self.assertIn("f'apps/player/e2e/{file}' for file, _ in CAMERA_CASES", block)
        self.assertIn("f'.verification/camera-{project}/source-receipt.json'", block)
        self.assertIn('fixture-startup.json', block)
        self.assertIn('if: always()', block)
        self.assertIn('if-no-files-found: error', block)
        for path in ('scripts/ci/test_camera_workflow.py', 'apps/player/e2e/camera-profile-fixture.ts'):
            self.assertIn('"' + path + '"', source)

if __name__ == '__main__': unittest.main()
