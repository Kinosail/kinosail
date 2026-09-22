"""Regressions for real browser execution and continuous image consumption."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
WORKFLOWS = ROOT / '.github/workflows'
IDENTITY = 'https://github.com/Kinosail/kinosail/.github/workflows/delivery.yml@refs/heads/main'
QUEUE = '''    concurrency:
      group: container-promotion-${{ matrix.app }}
      cancel-in-progress: false
      queue: max
'''


class RuntimeContracts(unittest.TestCase):
    def test_promotion_preserves_pending_jobs_and_verifies_consumer_identity(self):
        source = (WORKFLOWS / 'delivery.yml').read_text()
        self.assertIn(QUEUE, source)
        self.assertEqual([line for line in source.splitlines() if line.strip().startswith('queue:')], ['      queue: max'])
        verify = source.index('cosign verify --certificate-identity ' + IDENTITY)
        self.assertLess(source.index('cosign sign --yes'), verify)
        self.assertLess(verify, source.index('name: Publish deployable commit tag'))
        self.assertIn('sort == ["amd64", "arm64"]', source)
        for app in ('player', 'subtitles', 'dashboard'):
            release = (WORKFLOWS / f'{app}-release.yml').read_text()
            self.assertNotIn('type=raw,value=latest', release)
            self.assertNotIn('type=raw,value=main', release)
        for app in ('player', 'subtitles'):
            installer = (ROOT / f'apps/{app}/scripts/install.sh').read_text()
            self.assertIn('cosign verify --certificate-identity ' + IDENTITY, installer)
            self.assertIn(f'/{app}-release.yml@refs/tags/{app}-v$version', installer)

    def test_browser_selection_cannot_succeed_without_running_a_project(self):
        workflow = (WORKFLOWS / 'player-hygiene.yml').read_text()
        self.assertIn(".player_browsers && 'full' || ''", workflow)
        source = (ROOT / 'apps/player/scripts/test-container.sh').read_text()
        self.assertNotIn('done < <(./scripts/browser-projects.sh)', source)
        dashboard = (WORKFLOWS / 'dashboard-hygiene.yml').read_text()
        self.assertIn('pnpm --dir apps/dashboard/e2e exec playwright test --project="$PROJECT"', dashboard)
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
            for matrix, expected in (('', 'chromium\n'), ('full', 'chromium\nfirefox\nwebkit\n')):
                result = subprocess.run(['bash', str(ROOT / 'apps/player/scripts/browser-projects.sh')],
                    env=env | {'KINOSAIL_BROWSER_MATRIX': matrix, 'KINOSAIL_BROWSER_PROJECT': ''}, capture_output=True, text=True, check=True)
                self.assertEqual(result.stdout, expected)

    def test_browser_jobs_share_one_scanned_image_but_never_mutable_server_state(self):
        for app in ("player", "subtitles"):
            source = (WORKFLOWS / f'{app}-hygiene.yml').read_text()
            images = source.split('  images:\n', 1)[1].split('  system:\n', 1)[0]
            system = source.split('  system:\n', 1)[1].split('  required:\n', 1)[0]
            self.assertEqual(source.count('uses: docker/build-push-action@'), 1)
            self.assertIn('name: Scan the production image before merge', images)
            self.assertIn('docker save "$IMAGE"', images)
            self.assertIn('needs: images', system)
            self.assertIn('docker load --input "$RUNNER_TEMP/image.tar.gz"', system)
            self.assertIn('KINOSAIL_BROWSER_PROJECT: ${{ matrix.browser }}', system)
            self.assertIn('["chromium","firefox","webkit"]', system)
            self.assertIn('browser-failures-${{ matrix.browser }}', system)
            self.assertIn('platform: linux/arm64\n            browser: firefox', system)
            self.assertIn('platform: linux/arm64\n            browser: webkit', system)
        dashboard = (WORKFLOWS / 'dashboard-hygiene.yml').read_text()
        self.assertIn('["chromium","firefox","mobile-webkit"]', dashboard)
        self.assertIn('PROJECT: ${{ matrix.browser }}', dashboard)

    def test_browser_integrity_runs_even_without_go_jobs(self):
        source = (WORKFLOWS / 'quality.yml').read_text().split('  web:')[0]
        self.assertIn('python3 scripts/quality/check-dependency-integrity.py --browser-only', source)


if __name__ == '__main__':
    unittest.main()
