"""Regression checks for public CI and release trust boundaries."""
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[2]
WORKFLOWS = ROOT / '.github/workflows'


class WorkflowSecurityTests(unittest.TestCase):
    def test_ci_cannot_silently_bypass_checks(self):
        self.assertFalse((ROOT / '.gates-disabled').exists())
        for name in ('quality', 'security', 'player-hygiene', 'subtitles-hygiene', 'dashboard-hygiene'):
            with self.subTest(workflow=name):
                source = (WORKFLOWS / f'{name}.yml').read_text()
                self.assertNotIn('    paths:', source)
                self.assertNotIn('continue-on-error:', source)
                self.assertNotIn('enabled=false', source)
                self.assertIn('    if: always()', source)
                self.assertIn('python3 scripts/ci/required.py ', source)
                self.assertIn('uses: ./.github/workflows/changes.yml', source)
                self.assertIn('needs: [changes,', source)

    def test_invalid_quality_scope_has_no_side_effects(self):
        import os
        import subprocess
        import tempfile
        for script in ('check-coverage.sh', 'check-crap.sh'):
            for args in ([''], ['unknown'], ['../packages'], ['player', 'packages'], ['x' * 10000]):
                with self.subTest(script=script, args=args[:1]), tempfile.TemporaryDirectory() as directory:
                    marker = Path(directory) / 'side-effect'
                    binary = Path(directory) / 'go'
                    binary.write_text('#!/bin/sh\ntouch "' + str(marker) + '"\n')
                    binary.chmod(0o755)
                    result = subprocess.run(['bash', str(ROOT / 'scripts/quality' / script), *args],
                                            cwd=directory, env=os.environ | {'PATH': directory + ':' + os.environ['PATH']},
                                            capture_output=True)
                    self.assertEqual(result.returncode, 2)
                    self.assertFalse(marker.exists())
                    self.assertFalse((Path(directory) / '.verification').exists())

    def test_actions_are_immutable_and_untrusted_prs_cannot_write(self):
        for path in WORKFLOWS.glob('*.yml'):
            with self.subTest(workflow=path.name):
                source = path.read_text()
                self.assertNotIn('pull_request_target:', source)
                self.assertNotIn('self-hosted', source)
                for action in re.findall(r'uses: (\S+)', source):
                    if not action.startswith('./'):
                        self.assertRegex(action, r'@[0-9a-f]{40}$')
                if not path.name.endswith('-release.yml'):
                    self.assertNotIn('contents: write', source)
                    if path.name not in ('quality.yml', 'delivery.yml'):
                        self.assertNotIn('packages: write', source)
                    if path.name == 'quality.yml':
                        delivery = source.split('  delivery:\n')[1]
                        self.assertIn("github.event_name == 'push' && github.ref == 'refs/heads/main'", delivery)
                        self.assertIn('needs: [changes, required]', delivery)
                        self.assertNotIn('packages: write', source.split('  delivery:\n')[0])

    def test_release_requires_main_quality_and_security_before_promotion(self):
        for app in ('player', 'subtitles', 'dashboard'):
            with self.subTest(app=app):
                source = (WORKFLOWS / f'{app}-release.yml').read_text()
                self.assertIn('needs: quality', source)
                self.assertIn('test ! -e ../../.gates-disabled', source)
                self.assertIn('git merge-base --is-ancestor "$commit" origin/main', source)
                self.assertIn('security.yml; do', source)
                self.assertIn('needs: [quality, images]', source)
                self.assertIn('runner: ubuntu-24.04-arm', source)
                self.assertNotIn('setup-qemu-action', source)
                self.assertIn('sort == ["amd64", "arm64"]', source)
                self.assertNotIn("if: hashFiles('.gates-disabled')", source)
                candidate = source.index(f'tags: ghcr.io/kinosail/kinosail-{app}:candidate-')
                scan = source.index('name: Scan candidate')
                sign = source.index('run: cosign sign --yes')
                promote = source.index('name: Promote verified image')
                release = source.index('name: Publish ' + app.title() + ' release')
                self.assertLess(candidate, scan)
                self.assertLess(scan, sign)
                self.assertLess(sign, promote)
                self.assertLess(promote, release)


if __name__ == '__main__':
    unittest.main()
