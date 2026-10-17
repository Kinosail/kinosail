"""Regression checks for public CI and release trust boundaries."""
from pathlib import Path
import json
import os
import re
import subprocess
import tempfile
import textwrap
import unittest

from proof_shard import load

ROOT = Path(__file__).resolve().parents[2]
WORKFLOWS = ROOT / '.github/workflows'


class WorkflowSecurityTests(unittest.TestCase):
    def test_wear_requires_both_independent_hardware_profiles(self):
        source = (WORKFLOWS / 'deep.yml').read_text().split('  wear:\n', 1)[1].split('\n  security:', 1)[0]
        self.assertIn('fail-fast: false', source)
        self.assertIn('profile: [small-round, large-round]', source)
        self.assertIn('--profile "${{ matrix.profile }}"', source)
        self.assertIn('name: wear-journey-evidence-${{ matrix.profile }}', source)
        self.assertNotIn('continue-on-error:', source)

    def test_layout_recovery_retains_original_precredential_evidence(self):
        source = (WORKFLOWS / 'layout-stability.yml').read_text()
        triggers = source.split('  workflow_dispatch:', 1)[0]
        for path in ('apps/player/e2e/login-readiness.spec.ts', 'apps/player/e2e/layout-audit-helpers.ts'):
            self.assertIn(f'      - "{path}"\n', triggers)
        self.assertIn('login-readiness.spec.ts layout-auth-navigation.spec.ts\n', source)
        artifact = source.split('      - name: Preserve safe measurements and screenshots\n', 1)[1].split('\n  populated-instance:', 1)[0]
        for name in ('login-original-navigation-trace.zip', 'firefox-auth-form-navigation-recovery.json'):
            self.assertIn(f'            .verification/layout/*/*/{name}\n', artifact)
            self.assertIn(f'            .verification/layout-fixtures/**/{name}\n', artifact)
        self.assertIn('            .verification/layout-fixtures/**/layout-login-rejection.json\n', artifact)
        self.assertIn('layout-auth-navigation.spec.ts\n', source)
        self.assertIn('if: always()', artifact)
        self.assertNotIn('.verification/layout/**', artifact)

    def test_backup_runs_first_without_leaking_its_flag_or_hiding_later_runs(self):
        root = ROOT
        source = (root / '.github/workflows/deep.yml').read_text()
        step = source.split('      - name: Execute iOS and tvOS native contracts\n', 1)[1].split('      - name:', 1)[0]
        script = textwrap.dedent(step.split('        run: |\n', 1)[1])
        with tempfile.TemporaryDirectory() as directory:
            folder = Path(directory)
            devices = {'devices': {f'com.apple.CoreSimulator.SimRuntime.{platform}-27-0': [
                {'isAvailable': True, 'udid': 'AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA',
                 'deviceTypeIdentifier': 'com.apple.CoreSimulator.SimDeviceType.' + kind}]
                for platform, kind in [('iOS', 'iPhone-17'), ('tvOS', 'Apple-TV-4K')]}}
            (folder / 'xcrun').write_text('#!/bin/sh\nprintf \'%s\\n\' \'' + json.dumps(devices) + '\'\n')
            (folder / 'xcodebuild').write_text('''#!/usr/bin/env python3
import json, os, sys
args = sys.argv[1:]
contract = args[args.index('-resultBundlePath') + 1].rsplit('/', 1)[-1]
with open(os.environ['TEST_COMMAND_LOG'], 'a') as out:
    out.write(json.dumps({'contract': contract, 'flag': os.environ.get('TEST_RUNNER_KINOSAIL_FRESH_BACKUP_TEST'), 'args': args}) + '\\n')
sys.exit(1 if contract == os.environ['TEST_COMMAND_FAILURE'] else 0)
''')
            for name in ('xcrun', 'xcodebuild'): (folder / name).chmod(0o755)
            for failure in ('', 'native-iOS-backup.xcresult', 'native-iOS.xcresult', 'native-tvOS.xcresult'):
                log = folder / 'commands.jsonl'; log.write_text('')
                result = subprocess.run(['bash', '-ec', script], cwd=root, capture_output=True, text=True, timeout=10,
                    env={**os.environ, 'PATH': f'{folder}{os.pathsep}{os.environ["PATH"]}', 'RUNNER_TEMP': directory,
                         'TEST_COMMAND_LOG': str(log), 'TEST_COMMAND_FAILURE': failure})
                rows = [json.loads(line) for line in log.read_text().splitlines()]
                self.assertEqual([row['contract'] for row in rows], ['native-iOS-backup.xcresult', 'native-iOS.xcresult', 'native-tvOS.xcresult'])
                self.assertEqual([row['flag'] for row in rows], ['1', None, None])
                self.assertIn('-only-testing:Kinosail-iOSTests/OfflineCatalogBackupTests', rows[0]['args'])
                self.assertTrue(all(not arg.startswith('-only-testing') for row in rows[1:] for arg in row['args']))
                self.assertEqual(result.returncode, 1 if failure else 0, result.stderr)

    def test_go_lane_installs_real_media_dependencies_before_testing(self):
        version = re.search(r'ARG JELLYFIN_FFMPEG_VERSION=(\S+)', (ROOT / 'apps/player/Containerfile').read_text()).group(1)
        deb = f'/download/v{version}/jellyfin-ffmpeg8_{version}-noble_amd64.deb'
        go = (WORKFLOWS / 'ci.yml').read_text().split('\n  go:\n', 1)[1].split('\n  e2e:\n', 1)[0]
        step = go.split('      - name: Install media fixture codecs\n', 1)[1].split('      - name: Check the module contract', 1)[0]
        self.assertIn('[[ "$MEDIA" == true ]] || exit 0', step)
        self.assertIn('MEDIA: ${{ matrix.media }}', step)
        for text in (deb, 'sha256sum --check', '188fa8b31f7e0ded9dadd1d11d1cd05482a7f03cec4e5c0af80aae21fde44ce4',
                     'echo /usr/lib/jellyfin-ffmpeg >> "$GITHUB_PATH"'):
            self.assertIn(text, step)
        self.assertLess(go.index('Install media fixture codecs'), go.index('scripts/ci/test-go.sh'))
        self.assertIn('KINOSAIL_GO_TEST_MODE: quick', go)
        fragment = go.split('      - name: Verify certified first-fragment regeneration and cold reopen\n', 1)[1].split('      - name:', 1)[0]
        self.assertIn("if: matrix.fragment == 'true'", fragment)
        from plan import MODULES
        self.assertEqual([entry['fragment'] for entry in MODULES], ['true', 'false', 'false', 'false'])
        self.assertIn('KINOSAIL_COPIED_RECOVERY_MEDIA: "1"', fragment)
        self.assertIn("-run '^TestCopiedRecoveryRealFirstFragmentRegenerationReopens$'", fragment)
        self.assertNotIn('continue-on-error', fragment)
        self.assertLess(go.index('scripts/ci/test-go.sh'), go.index('Verify certified first-fragment'))
        from plan import MODULES
        self.assertEqual([entry['media'] for entry in MODULES], ['true', 'true', 'false', 'false'])
        race = (WORKFLOWS / 'deep.yml').read_text().split('\n  race:\n', 1)[1].split('\n  browser:\n', 1)[0]
        self.assertIn('KINOSAIL_COPIED_RECOVERY_MEDIA: "1"', race)
        self.assertIn(deb, race)
        self.assertLess(race.index('Install media fixture codecs'), race.index('scripts/ci/test-go.sh'))
        action = (ROOT / '.github/actions/setup-e2e/action.yml').read_text()
        self.assertIn(deb, action)
        self.assertIn('sha256sum --check', action)
        self.assertIn('echo /usr/lib/jellyfin-ffmpeg >> "$GITHUB_PATH"', action)

    def test_deep_apple_lane_executes_both_native_test_targets_and_retains_results(self):
        source = (WORKFLOWS / 'deep.yml').read_text().split('  client:\n', 1)[1].split('  android:\n', 1)[0]
        self.assertIn('for contract in iOS-backup iOS tvOS', source)
        self.assertIn('xcodebuild test', source)
        self.assertIn('-scheme "Kinosail-$platform"', source)
        self.assertIn('-resultBundlePath "$RUNNER_TEMP/native-$contract.xcresult"', source)
        self.assertIn('        if: always()\n', source)
        self.assertIn('${{ runner.temp }}/native-*.xcresult', source)
        self.assertNotIn('continue-on-error', source)

    def test_deep_apple_media_contracts_receive_bounded_decodable_fixtures(self):
        source = (WORKFLOWS / 'deep.yml').read_text().split('  client:\n', 1)[1].split('  android:\n', 1)[0]
        step = source.split('      - name: Prepare decoded native media fixture\n', 1)[1].split('      - name:', 1)[0]
        self.assertIn('ffmpeg', step)
        self.assertIn('-le 1048576', step)
        for path in ('kinosail-appletv-task7-media.mp4', 'kinosail-player-buffer-bar.mp4', 'kinosail-landscape-task10-media.mp4'):
            self.assertIn(path, step)
        self.assertLess(source.index('Prepare decoded native media fixture'), source.index('Execute iOS and tvOS native contracts'))
        self.assertIn('${{ runner.temp }}/native-media.json', source)

    def test_system_scan_finishes_before_exact_revision_evidence_starts(self):
        ci = (WORKFLOWS / 'ci.yml').read_text()
        deep = (WORKFLOWS / 'deep.yml').read_text()
        for source, job, nxt in ((ci, 'e2e', 'proofs'), (ci, 'arm', 'tools'), (deep, 'system', 'proofs')):
            with self.subTest(job=job):
                section = source.split(f'\n  {job}:\n', 1)[1].split(f'\n  {nxt}:\n', 1)[0]
                scan = re.search(r'^      - uses: aquasecurity/trivy-action@[a-f0-9]{40}', section, re.M)
                evidence = re.search(r'^      - name: Test production paths\n', section, re.M)
                self.assertIsNotNone(scan, 'checkout-mutating scan must be a foreground step')
                self.assertIsNotNone(evidence, 'exact revision evidence must be a foreground step')
                self.assertLess(scan.start(), evidence.start())
                self.assertLess(section.index('- parallel:') if '- parallel:' in section else -1, scan.start())
                self.assertNotIn('background:', section)
                self.assertIn('e2e-artifact.py', section)
        browser = ci.split('\n  e2e:\n', 1)[1].split('\n  proofs:\n', 1)[0]
        self.assertLess(browser.index('name: Test production paths'), browser.index('name: Test populated browser journeys'))

    def test_parallel_image_checks_finish_before_digest_export(self):
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

    def test_browser_and_proof_lanes_receive_verified_fixture_codecs(self):
        ci = (WORKFLOWS / 'ci.yml').read_text()
        action = (ROOT / '.github/actions/setup-e2e/action.yml').read_text()
        codecs = action.split('    - name: Install verified fixture codecs\n', 1)[1]
        self.assertNotIn('if:', codecs)
        self.assertIn('sha256sum --check', codecs)
        self.assertIn('echo /usr/lib/jellyfin-ffmpeg >> "$GITHUB_PATH"', codecs)
        self.assertIn("case \"$APP\" in player|subtitles) ;; *) echo 'invalid app' >&2; exit 2 ;; esac", action)
        for job, nxt in (('e2e', 'proofs'), ('proofs', 'arm')):
            section = ci.split(f'\n  {job}:\n', 1)[1].split(f'\n  {nxt}:\n', 1)[0]
            self.assertIn('uses: ./.github/actions/setup-e2e', section)
            self.assertNotIn('actions/setup-go@', section)
            self.assertIn('uses: ./.github/actions/setup-go', section)
        browser = ci.split('\n  e2e:\n', 1)[1].split('\n  proofs:\n', 1)[0]
        self.assertNotIn('apps/player/scripts/', browser)

    def test_subtitles_full_browser_selection_covers_removed_routes(self):
        script = (ROOT / 'apps/subtitles/scripts/test-container.sh').read_text()
        selection = re.search(r'^  browser_args=\(([^)]+)\)', script, re.M)
        self.assertIsNotNone(selection)
        self.assertIn('removed-routes.spec.ts', selection.group(1).split())

    def test_startup_boundary_has_hosted_public_interface_evidence(self):
        startup = load()['player']['startup']
        self.assertTrue(startup['browser'])
        command = next(command for command in startup['commands'] if command['argv'][1] == 'apps/player/scripts/test-startup-local.py')
        self.assertEqual(command['env'], {'KINOSAIL_STARTUP_BROWSER_CHANNEL': 'chromium'})
        self.assertIn('.verification/startup/*/receipt.json', startup['evidence'])
        self.assertIn('.verification/startup/*/results/**/startup-measurements.json', startup['evidence'])
        self.assertNotIn('.verification/startup/**', startup['evidence'])

    def test_ci_cannot_silently_bypass_checks(self):
        self.assertFalse((ROOT / '.gates-disabled').exists())
        ci = (WORKFLOWS / 'ci.yml').read_text()
        deep = (WORKFLOWS / 'deep.yml').read_text()
        self.assertEqual(ci.count('run: python3 scripts/ci/plan.py'), 1)
        self.assertEqual(deep.count('run: python3 scripts/ci/plan.py'), 1)
        for scope in ('Repository', 'Player', 'Subtitles', 'Security'):
            self.assertIn(f'name: {scope} checks', ci)
        self.assertEqual(len(re.findall(r'^  [a-z-]+-required:$', ci, re.M)), 4)
        for source in (ci, deep):
            self.assertNotIn('continue-on-error:', source)
            self.assertNotIn('enabled=false', source)
        self.assertIn('    if: always()', ci)
        self.assertEqual(ci.count('python3 scripts/ci/required.py '), 4)
        self.assertNotIn('    paths:', ci)
        self.assertIn('needs: [plan, repository, docs]', ci)
        self.assertEqual(ci.count('needs: [plan, go, e2e, proofs, arm, tools, client, android]'), 2)
        self.assertIn('needs: [plan, security, codeql-compiled]', ci)
        from required import SCOPES
        self.assertEqual(SCOPES, {'repository': ('repository', 'docs'),
                                  'app': ('go', 'e2e', 'proofs', 'arm', 'tools', 'client', 'android'),
                                  'security': ('security', 'codeql-compiled')})
        jobs = re.findall(r'^  ([a-z][a-z0-9-]*):$', ci.split('\njobs:\n', 1)[1], re.M)
        self.assertEqual(jobs, ['plan', 'repository', 'go', 'e2e', 'proofs', 'arm', 'tools', 'client', 'android', 'docs',
                                'security', 'codeql-compiled', 'repository-required', 'player-required', 'subtitles-required',
                                'security-required', 'publish', 'publish-docs', 'notify-indexnow'])

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
        publish_docs = source.split('  publish-docs:\n')[1].split('  notify-indexnow:\n')[0]
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
