"""Regression checks for public CI and release trust boundaries."""
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[2]
WORKFLOWS = ROOT / '.github/workflows'


class WorkflowSecurityTests(unittest.TestCase):
    def test_ci_cannot_silently_bypass_checks(self):
        self.assertFalse((ROOT / '.gates-disabled').exists())
        ci = (WORKFLOWS / 'ci.yml').read_text()
        app = (WORKFLOWS / 'app.yml').read_text()
        self.assertEqual(ci.count('run: python3 scripts/ci/affected.py'), 1)
        for scope in ('Repository', 'Player', 'Subtitles', 'Security'):
            self.assertIn(f'name: {scope} checks', ci)
        self.assertNotIn('dashboard-required', ci)
        for source in (ci, app):
            self.assertNotIn('continue-on-error:', source)
            self.assertNotIn('enabled=false', source)
            self.assertIn('    if: always()', source)
            self.assertIn('python3 scripts/ci/required.py ', source)
        self.assertNotIn('    paths:', ci)
        self.assertIn('needs: [plan, static, tooling, packages, web, docs]', ci)
        self.assertIn('needs: [plan, secrets, codeql, supply-chain, findings]', ci)
        self.assertIn('needs: [static, race, security, tooling, client, android, system, browser]', app)
        self.assertNotIn('  validate:', app)

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
                if path.name != 'release.yml':
                    self.assertNotIn('contents: write', source)
                    if path.name not in ('ci.yml', 'publish.yml'):
                        self.assertNotIn('packages: write', source)
                    if path.name == 'ci.yml':
                        publish = source.split('  publish:\n')[1]
                        self.assertIn("github.event_name == 'push'", publish)
                        self.assertIn("github.ref == 'refs/heads/main'", publish)
                        self.assertIn('needs: [plan, repository-required, player-required, subtitles-required, security-required]', publish)
                        self.assertIn('results: ${{ toJSON(needs) }}', publish)
                        self.assertNotIn('packages: write', source.split('  publish:\n')[0])

    def test_publication_gates_ignore_skipped_unselected_jobs(self):
        source = (WORKFLOWS / 'ci.yml').read_text()
        publish = source.split('  publish:\n')[1].split('  publish-docs:\n')[0]
        publish_docs = source.split('  publish-docs:\n')[1].split('  diagnostics:\n')[0]
        for job in (publish, publish_docs):
            self.assertIn('always() &&', job)
            for required in ('plan', 'repository-required', 'player-required',
                             'subtitles-required', 'security-required'):
                self.assertIn(f"needs.{required}.result == 'success'", job)
        self.assertIn("needs.docs.result == 'success'", publish_docs)

    def test_release_requires_main_quality_and_security_before_promotion(self):
        source = (WORKFLOWS / 'release.yml').read_text()
        self.assertIn('tags: ["player-v*", "subtitles-v*"]', source)
        self.assertIn('git merge-base --is-ancestor "$commit" origin/main', source)
        self.assertIn('--workflow ci.yml --commit "$commit" --event push', source)
        self.assertIn('needs: [preflight, images]', source)
        self.assertIn('runner: ubuntu-24.04-arm', source)
        self.assertNotIn('setup-qemu-action', source)
        self.assertIn('sort == ["amd64", "arm64"]', source)
        self.assertNotIn("if: hashFiles('.gates-disabled')", source)
        self.assertLess(source.index('name: Test production image by digest'), source.index('name: Export verified digest'))
        self.assertLess(source.index('name: Sign the version image'), source.index('name: Promote exact version tag'))
        self.assertLess(source.index('name: Promote exact version tag'), source.index('name: Create GitHub release'))

    def test_player_package_description_is_published_on_both_image_indexes(self):
        description = ('index:org.opencontainers.image.description=Kinosail Player is a free media server '
                       'that runs at home and streams your own movies and shows. Official website: https://kinosail.com/')
        for workflow in ('publish.yml', 'release.yml'):
            with self.subTest(workflow=workflow):
                manifest = (WORKFLOWS / workflow).read_text().split('      - id: manifest\n')[1].split('      - uses: sigstore/')[0]
                self.assertIn('if [[ "$APP" == player ]]; then', manifest)
                self.assertIn('--annotation "' + description + '"', manifest)
                self.assertIn('docker buildx imagetools create --tag "$candidate" "${annotations[@]}" "${sources[@]}"', manifest)


if __name__ == '__main__':
    unittest.main()
