"""Regressions for real browser execution and continuous image consumption."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
WORKFLOWS = ROOT / '.github/workflows'
IDENTITY = 'https://github.com/Kinosail/kinosail/.github/workflows/publish.yml@refs/heads/main'
QUEUE = '''    concurrency:
      group: container-promotion-${{ matrix.app }}
      cancel-in-progress: false
      queue: max
'''


class RuntimeContracts(unittest.TestCase):
    def test_quick_go_checks_reject_unknown_modes_before_side_effects(self):
        app = (WORKFLOWS / 'app.yml').read_text()
        ci = (WORKFLOWS / 'ci.yml').read_text()
        self.assertIn("KINOSAIL_GO_TEST_MODE: ${{ fromJSON(inputs.plan).deep && 'deep' || 'quick' }}", app)
        self.assertIn("KINOSAIL_GO_TEST_MODE: ${{ fromJSON(needs.plan.outputs.plan).deep && 'deep' || 'quick' }}", ci)
        with tempfile.TemporaryDirectory() as directory:
            log = Path(directory) / 'go-called'
            binary = Path(directory) / 'go'
            binary.write_text('#!/bin/sh\nprintf "%s\\n" "$*" > "$GO_CALL_LOG"\n')
            binary.chmod(0o755)
            env = os.environ | {'PATH': directory + ':' + os.environ['PATH'], 'GO_CALL_LOG': str(log)}
            script = ROOT / 'scripts/ci/test-go.sh'
            for invalid in ('unknown', 'QUICK', 'x' * 10000):
                result = subprocess.run(['bash', str(script), 'player'],
                    env=env | {'KINOSAIL_GO_TEST_MODE': invalid}, capture_output=True)
                self.assertEqual(result.returncode, 2)
                self.assertFalse(log.exists())
            result = subprocess.run(['bash', str(script), 'player'],
                env=env | {'KINOSAIL_GO_TEST_MODE': 'quick'}, capture_output=True)
            self.assertEqual(result.returncode, 0)
            self.assertEqual(log.read_text(), 'test -count=1 ./...\n')

    def test_promotion_preserves_pending_jobs_and_verifies_consumer_identity(self):
        source = (WORKFLOWS / 'publish.yml').read_text()
        self.assertIn(QUEUE, source)
        self.assertEqual([line for line in source.splitlines() if line.strip().startswith('queue:')], ['      queue: max'])
        verify = source.index('cosign verify --certificate-identity ' + IDENTITY)
        self.assertLess(source.index('cosign sign --yes'), verify)
        self.assertLess(verify, source.index('name: Publish deployable commit tag'))
        self.assertIn('sort == ["amd64", "arm64"]', source)
        release = (WORKFLOWS / 'release.yml').read_text()
        self.assertNotIn('type=raw,value=latest', release)
        for app in ('player', 'subtitles'):
            installer = (ROOT / f'apps/{app}/scripts/install.sh').read_text()
            self.assertIn('cosign verify --certificate-identity ' + IDENTITY, installer)
            self.assertIn(f'/release.yml@refs/tags/{app}-v$version', installer)

    def test_browser_selection_cannot_succeed_without_running_a_project(self):
        workflow = (WORKFLOWS / 'app.yml').read_text()
        self.assertIn("fromJSON(inputs.plan).deep && 'full' || ''", workflow)
        self.assertIn("engine: ${{ fromJSON(fromJSON(inputs.plan).deep && (inputs.app == 'dashboard' && '[\"full\"]' || '[\"chromium\",\"firefox\",\"webkit\"]') || '[\"chromium\"]') }}", workflow)
        self.assertIn('KINOSAIL_BROWSER_PROJECT: ${{ matrix.engine }}', workflow)
        self.assertIn("KINOSAIL_BROWSER_SMOKE: ${{ !fromJSON(inputs.plan).deep && '1' || '' }}", workflow)
        source = (ROOT / 'apps/player/scripts/test-container.sh').read_text()
        self.assertNotIn('done < <(./scripts/browser-projects.sh)', source)
        self.assertIn('[[ "$PROJECT" == full ]] || args+=(--project=chromium --grep=@smoke)', workflow)
        self.assertIn('pnpm --dir apps/dashboard/e2e exec playwright test "${args[@]}"', workflow)
        for app in ('player', 'subtitles'):
            self.assertIn('browser_args+=(--grep=@smoke)', (ROOT / f'apps/{app}/scripts/test-container.sh').read_text())
        with tempfile.TemporaryDirectory() as directory:
            marker = Path(directory) / 'effects'
            for tool in ('docker', 'podman', 'mktemp'):
                binary = Path(directory) / tool
                binary.write_text(f'#!/bin/sh\ntouch "{marker}"\n')
                binary.chmod(0o755)
            env = os.environ | {'PATH': directory + ':' + os.environ['PATH'], 'KINOSAIL_BROWSER_TEST': '1'}
            for invalid in ('chromium', 'unknown', 'x' * 10000):
                result = subprocess.run(['bash', str(ROOT / 'apps/player/scripts/test-container.sh')],
                    env=env | {'KINOSAIL_BROWSER_MATRIX': invalid}, capture_output=True)
                self.assertEqual(result.returncode, 2)
                self.assertFalse(marker.exists())
            for app in ('player', 'subtitles'):
                result = subprocess.run(['bash', str(ROOT / f'apps/{app}/scripts/test-container.sh')],
                    env=env | {'KINOSAIL_BROWSER_MATRIX': '', 'KINOSAIL_BROWSER_SMOKE': 'unknown'},
                    capture_output=True)
                self.assertEqual(result.returncode, 2)
                self.assertFalse(marker.exists())
            for matrix, expected in (('', 'chromium\n'), ('full', 'chromium\nfirefox\nwebkit\n')):
                result = subprocess.run(['bash', str(ROOT / 'apps/player/scripts/browser-projects.sh')],
                    env=env | {'KINOSAIL_BROWSER_MATRIX': matrix, 'KINOSAIL_BROWSER_PROJECT': ''}, capture_output=True, text=True, check=True)
                self.assertEqual(result.stdout, expected)
            for project in ('chromium', 'firefox', 'webkit'):
                result = subprocess.run(['bash', str(ROOT / 'apps/player/scripts/browser-projects.sh')],
                    env=env | {'KINOSAIL_BROWSER_MATRIX': 'full', 'KINOSAIL_BROWSER_PROJECT': project},
                    capture_output=True, text=True, check=True)
                self.assertEqual(result.stdout, project + '\n')

    def test_browser_integrity_runs_even_without_go_jobs(self):
        source = (WORKFLOWS / 'ci.yml').read_text().split('  web:')[0]
        self.assertIn('python3 scripts/quality/check-dependency-integrity.py --browser-only', source)


if __name__ == '__main__':
    unittest.main()
