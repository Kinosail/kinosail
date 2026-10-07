"""Regression checks for public CI and release trust boundaries."""
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[2]
WORKFLOWS = ROOT / '.github/workflows'


class WorkflowSecurityTests(unittest.TestCase):
    def test_deep_coverage_failure_retains_exact_profiles_without_bypass(self):
        app = (WORKFLOWS / 'app.yml').read_text()
        for name in ('Summarize exact deep coverage', 'Keep exact deep coverage profiles'):
            step = app.split(f'      - name: {name}\n', 1)[1].split('\n      - ', 1)[0].split('\n  security:', 1)[0]
            self.assertIn('if: always() && fromJSON(inputs.plan).deep\n', step)
            self.assertNotIn('inputs.coverage_diagnostic', step)
            self.assertNotIn('continue-on-error', step)
        self.assertIn('apps/${{ inputs.app }}/.verification/coverage.out', step)
        self.assertIn('apps/${{ inputs.app }}/.verification/coverage-functions.txt', step)
        self.assertIn('if-no-files-found: error', step)
        script = (ROOT / 'scripts/ci/test-go.sh').read_text()
        self.assertIn('player|subtitles) directory="apps/$1"; minimum=89', script)
        self.assertIn('coverage + 0 < minimum', script)

    def test_system_scan_finishes_before_exact_revision_evidence_starts(self):
        # Trivy creates/removes files in the checkout. Overlap changes Git
        # status during the E2E receipt and invalidates exact revision proof.
        source = (WORKFLOWS / 'app.yml').read_text().split('  system:\n', 1)[1].split('  browser:\n', 1)[0]
        scan = re.search(r'^      - uses: aquasecurity/trivy-action@[a-f0-9]{40}', source, re.M)
        evidence = re.search(r'^      - name: Test production paths\n', source, re.M)
        self.assertIsNotNone(scan, 'checkout-mutating scan must be a foreground step')
        self.assertIsNotNone(evidence, 'exact revision evidence must be a foreground step')
        self.assertLess(scan.start(), evidence.start())
        self.assertNotIn('background:', source)

    def test_parallel_image_checks_finish_before_digest_export(self):
        # Browser E2E cannot prove that image publication still waits for both
        # runtime verification and the vulnerability scan before export.
        for workflow, test_name, export_name in (
                ('publish.yml', 'Test the production image by digest', 'Export tested and scanned digest'),
                ('release.yml', 'Test production image by digest', 'Export verified digest')):
            with self.subTest(workflow=workflow):
                source = (WORKFLOWS / workflow).read_text()
                group = re.search(r'^      - parallel:\n((?: {8}.+\n|\n)+)', source, re.M)
                self.assertIsNotNone(group, 'image verification must use a native parallel group')
                checks = group.group(1)
                self.assertIn(f'name: {test_name}', checks)
                self.assertIn('run: ./scripts/ci/test-image.sh "$APP" "$IMAGE"', checks)
                self.assertRegex(checks, r'uses: aquasecurity/trivy-action@[a-f0-9]{40}')
                self.assertIn('exit-code: "1"', checks)
                self.assertNotIn('continue-on-error:', checks)
                self.assertNotRegex(checks, re.compile(r'^\s+if:', re.M))
                export_start = source.index(f'      - name: {export_name}')
                self.assertLessEqual(group.end(), export_start)
                export = source[export_start:].split('\n      - ', 1)[0]
                self.assertNotRegex(export, re.compile(r'^\s+if:', re.M))
                self.assertNotIn('continue-on-error:', export)
                self.assertNotIn('digests/', checks)

    def test_player_browser_engines_receive_verified_fixture_codecs(self):
        # Actual HLS navigation fixtures spawn FFmpeg in every selected engine.
        # Container codecs cannot satisfy a host fixture's executable dependency.
        app = (WORKFLOWS / 'app.yml').read_text()
        step = app.split('      - name: Install startup fixture codecs\n', 1)[1].split('      - name:', 1)[0]
        self.assertIn("if: inputs.app == 'player'\n", step)
        self.assertNotIn('matrix.engine', step)
        self.assertIn('sha256sum --check', step)
        self.assertIn('echo /usr/lib/jellyfin-ffmpeg >> "$GITHUB_PATH"', step)

    def test_subtitles_full_browser_selection_covers_progress_restoration(self):
        # Explicit fixture selection otherwise leaves the new regression local-only.
        script = (ROOT / 'apps/subtitles/scripts/test-container.sh').read_text()
        selection = re.search(r'^  browser_args=\(([^)]+)\)', script, re.M)
        self.assertIsNotNone(selection)
        self.assertIn('player-unplayed-progress.spec.ts', selection.group(1).split())
    def test_go_media_http_journeys_install_real_codec_tools(self):
        race = (WORKFLOWS / 'app.yml').read_text().split('  race:')[1].split('  security:')[0]
        install = 'sudo apt-get install -y --no-install-recommends libarchive-tools ffmpeg'
        self.assertIn(install, race)
        self.assertLess(race.index(install), race.index('./scripts/ci/test-go.sh'))
        self.assertIn('ffmpeg -version', race)
        self.assertIn('ffprobe -version', race)

    def test_navigation_diagnostics_contracts_run_in_policy_and_tooling(self):
        command = 'node --test scripts/testing/navigation-diagnostics.test.mjs'
        static = (WORKFLOWS / 'ci.yml').read_text().split('  static:')[1].split('  tooling:')[0]
        self.assertIn('      - run: ' + command, static)
        tooling = (ROOT / 'Makefile').read_text().split('tooling-check:')[1].split('\n\n')[0]
        self.assertIn('\t@' + command, tooling)

    def test_login_failure_observer_contracts_run_after_browser_dependencies(self):
        browser = (WORKFLOWS / 'app.yml').read_text().split('  browser:')[1].split('  required:')[0]
        command = 'node --test scripts/testing/subtitle-login-navigation.test.mjs'
        self.assertFalse((ROOT / 'apps/subtitles/e2e/test-instance-navigation.test.mjs').exists(),
                         'Node-only contracts must not be discovered as Playwright journeys')
        step = browser.split('      - name: Verify login navigation failure diagnostics')[1].split('      - ')[0]
        self.assertIn("if: inputs.app == 'subtitles'", step)
        self.assertIn('run: ' + command, step)
        self.assertLess(browser.index('pnpm --dir "apps/$APP/e2e" install --frozen-lockfile'), browser.index(command))
        self.assertLess(browser.index(command), browser.index('name: Test populated browsers'))

    def test_setup_failure_observer_contracts_run_after_player_dependencies(self):
        browser = (WORKFLOWS / 'app.yml').read_text().split('  browser:')[1].split('  required:')[0]
        command = 'node --test scripts/testing/player-setup-navigation.test.mjs'
        self.assertTrue((ROOT / 'scripts/testing/player-setup-navigation.test.mjs').is_file())
        self.assertFalse((ROOT / 'apps/player/e2e/player-setup-navigation.test.mjs').exists(),
                         'Node-only contracts must not be discovered as Playwright journeys')
        step = browser.split('      - name: Verify setup navigation failure diagnostics')[1].split('      - ')[0]
        self.assertIn("if: inputs.app == 'player'", step)
        self.assertIn('run: ' + command, step)
        self.assertLess(browser.index('pnpm --dir "apps/$APP/e2e" install --frozen-lockfile'), browser.index(command))
        self.assertLess(browser.index(command), browser.index('name: Test populated browsers'))

    def test_layout_failure_contracts_and_modules_are_reproducible(self):
        command = 'node --test scripts/testing/navigation-diagnostics.test.mjs scripts/testing/layout-stability-failure.test.mjs scripts/testing/layout-stability-subtitle-background.test.mjs scripts/testing/layout-stability-flows.test.mjs scripts/testing/layout-stability-diagnostic-snapshots.test.mjs'
        self.assertIn('      - run: ' + command, (WORKFLOWS / 'ci.yml').read_text())
        self.assertIn('\t@' + command, (ROOT / 'Makefile').read_text())
        launcher = (ROOT / 'scripts/testing/test-layout-stability-local.py').read_text()
        for name in ('layout-stability-routing.mjs', 'layout-stability-failure.mjs', 'layout-stability-flow-page.mjs', 'layout-stability-diagnostic-snapshots.mjs'):
            self.assertIn('"' + name + '"', launcher)
        self.assertIn('import "./layout-response-lifecycle-tests";',
                      (ROOT / 'apps/player/e2e/layout-stability.spec.ts').read_text())

    def test_startup_boundary_has_hosted_public_interface_evidence(self):
        app = (WORKFLOWS / 'app.yml').read_text()
        self.assertIn('name: Verify bounded startup and request boundary', app)
        self.assertIn("if: inputs.app == 'player' && matrix.engine == 'chromium'", app)
        self.assertIn('run: python3 apps/player/scripts/test-startup-local.py', app)
        self.assertIn('KINOSAIL_STARTUP_BROWSER_CHANNEL: chromium', app)
        self.assertIn('name: player-startup-boundary-evidence', app)
        self.assertIn('include-hidden-files: true', app)
        self.assertIn('.verification/startup/*/receipt.json', app)
        self.assertIn('.verification/startup/*/results/**/startup-measurements.json', app)
        self.assertNotIn('.verification/startup/**\n', app)

    def test_public_flow_runner_is_a_hosted_app_browser_gate(self):
        app = (WORKFLOWS / 'app.yml').read_text()
        step = app.split('      - name: Verify public flows with tester-army e2e\n')[1].split('      - name:', 1)[0]
        self.assertIn("if: matrix.engine == 'chromium'", step)
        self.assertIn('APP: ${{ inputs.app }}', step)
        self.assertIn('E2E_TELEMETRY_DISABLED: "1"', step)
        self.assertIn('run: scripts/e2e/run.sh "$APP"', step)
        self.assertIn('scripts/e2e/.e2e/runs/', app)
        self.assertNotIn('continue-on-error:', step)

    def test_full_affected_player_recovery_suites_are_hosted(self):
        app = (WORKFLOWS / 'app.yml').read_text()
        step = app.split('      - name: Verify complete Player recovery suites\n')[1].split('      - name:', 1)[0]
        for suite in ('passkeys.spec.ts', 'worker-message-security.spec.ts', 'player-experience.spec.ts', 'player-direct-fallback.spec.ts'):
            self.assertIn(suite, step)
        self.assertNotIn('--grep', step)
        self.assertIn('e2e-artifact.py', step)

    def test_ci_cannot_silently_bypass_checks(self):
        self.assertFalse((ROOT / '.gates-disabled').exists())
        ci = (WORKFLOWS / 'ci.yml').read_text()
        app = (WORKFLOWS / 'app.yml').read_text()
        self.assertEqual(ci.count('run: python3 scripts/ci/affected.py'), 1)
        for scope in ('Repository', 'Player', 'Subtitles', 'Security'):
            self.assertIn(f'name: {scope} checks', ci)
        self.assertEqual(len(re.findall(r'^  [a-z-]+-required:$', ci, re.M)), 4)
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
