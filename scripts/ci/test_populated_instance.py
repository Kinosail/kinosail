"""Hosted fixture lifecycle contracts that real browser journeys cannot inject."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
RUNNER = ROOT / 'apps/player/scripts/test-populated-instance-hosted.sh'
TITLE = 'Connection choices stay optional and secure by default'


class PopulatedInstanceTests(unittest.TestCase):
    def test_populated_instance_admission_is_explicit_and_retains_failures(self):
        # A browser cannot prove that a workflow omitted its fixture or failed
        # to retain evidence after setup failed before the browser launched.
        source = (ROOT / '.github/workflows/layout-stability.yml').read_text()
        self.assertIn("inputs.campaign_proof != 'INSTANCE'", source)
        section = source.split('  populated-instance:\n', 1)[1].split('\n  hls-followon:', 1)[0]
        self.assertIn("if: github.event_name == 'workflow_dispatch' && inputs.campaign_proof == 'INSTANCE'", section)
        self.assertIn('runs-on: ubuntu-24.04', section)
        self.assertIn('timeout-minutes: 30', section)
        self.assertIn('persist-credentials: false', section)
        self.assertIn('--build-arg "REVISION=$revision"', section)
        self.assertIn('KINOSAIL_TEST_IMAGE: localhost/kinosail:dev', section)
        self.assertIn('sudo apt-get install -y libnss3-tools', section)
        self.assertIn('python3 ../../scripts/ci/e2e-artifact.py', section)
        self.assertIn('-- bash ./scripts/test-populated-instance-hosted.sh', section)
        self.assertIn('if: always()', section)
        self.assertIn('path: .verification/populated-instance/', section)
        self.assertIn('if-no-files-found: error', section)
        self.assertNotIn('continue-on-error', section)
        self.assertNotIn('ignoreHTTPSErrors', section)

    def run_fixture(self, failure='', hosted=True):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            app = root / 'repo/apps/player'
            scripts = app / 'scripts'
            scripts.mkdir(parents=True)
            shared = root / 'repo/scripts/ci'
            shared.mkdir(parents=True)
            (shared / 'browser-fixture-tls.sh').write_bytes((ROOT / 'scripts/ci/browser-fixture-tls.sh').read_bytes())
            certificate = root / 'ca.crt'
            subprocess.run(['openssl', 'req', '-x509', '-newkey', 'ec', '-pkeyopt',
                'ec_paramgen_curve:P-256', '-nodes', '-keyout', str(root / 'key'),
                '-out', str(certificate), '-days', '1', '-subj', '/CN=Disposable test CA',
                '-addext', 'basicConstraints=critical,CA:TRUE'], check=True, capture_output=True)
            if failure == 'private-ca':
                certificate.write_text(certificate.read_text() + '-----BEGIN PRIVATE KEY-----\n')
            output = root / 'evidence'
            output.mkdir()
            binaries = root / 'bin'
            binaries.mkdir()
            target = scripts / RUNNER.name
            target.write_bytes(RUNNER.read_bytes())
            log = root / 'commands.jsonl'
            standin = '''#!/usr/bin/env python3
import json,os,pathlib,sys
name=pathlib.Path(sys.argv[0]).name;args=sys.argv[1:]
with open(os.environ['COMMAND_LOG'],'a') as out:
 out.write(json.dumps({'name':name,'args':args,'root':os.environ.get('KINOSAIL_TEST_ROOT'),'project':os.environ.get('KINOSAIL_TEST_PROJECT')})+'\\n')
failure=os.environ['FAILURE']
if name=='uname':print('Linux')
elif name=='git':print('a'*40)
elif name=='test-instance.sh':
 if args[0]=='up':
  pathlib.Path(os.environ['KINOSAIL_TEST_ROOT'],'totp-secret').write_text('synthetic-private-secret')
 if args[0]==failure:sys.exit(7)
elif name=='docker':
 if args[:2]==['image','inspect']:print('sha256:'+'b'*64+' '+'a'*40)
 elif args[0]=='compose':print('c'*64)
 elif args[0]=='exec':
  if failure=='export':sys.exit(6)
  print(pathlib.Path(os.environ['CERT']).read_text(),end='')
 elif failure=='leftover':print('owned-resource')
elif name=='certutil':
 state=pathlib.Path(os.environ['NSS_STATE']);nickname=os.environ['KINOSAIL_TEST_PROJECT']
 if '-N' in args:pass
 elif '-A' in args:
  if failure=='import':sys.exit(6)
  state.write_text(nickname)
 elif '-D' in args:
  if failure=='remove-trust':sys.exit(6)
  state.unlink(missing_ok=True)
 elif '-L' in args:
  if '-n' in args:sys.exit(0 if state.exists() or failure=='collision' else 1)
  print((nickname+' C,,') if state.exists() or failure=='collision' else '')
elif name=='pnpm':
 if failure=='browser':sys.exit(8)
 status='skipped' if failure=='empty' else 'expected'
 result={'status':'skipped' if failure=='empty' else 'passed','retry':0}
 report={'suites':[{'specs':[{'title':os.environ['TITLE'],'tests':[{'projectName':'chromium','status':status,'results':[result]}]}]}]}
 pathlib.Path(os.environ['KINOSAIL_E2E_ARTIFACT_DIR'],'results-chromium.json').write_text(json.dumps(report))
'''
            for name in ['uname', 'git', 'docker', 'pnpm', 'mkdir', 'certutil']:
                path = binaries / name
                path.write_text(standin)
                path.chmod(0o755)
            helper = scripts / 'test-instance.sh'
            helper.write_text(standin)
            helper.chmod(0o755)
            result = subprocess.run(['bash', str(target)], cwd=app, capture_output=True, text=True, timeout=10,
                env={**os.environ, 'PATH': f'{binaries}{os.pathsep}{os.environ["PATH"]}',
                     'CI': 'true' if hosted else '', 'GITHUB_ACTIONS': 'true', 'RUNNER_OS': 'Linux',
                     'RUNNER_TEMP': directory, 'KINOSAIL_E2E_ARTIFACT_DIR': str(output),
                     'KINOSAIL_E2E_OUTPUT_DIR': str(output / 'browser-results'),
                     'COMMAND_LOG': str(log), 'FAILURE': failure, 'TITLE': TITLE,
                     'CERT': str(certificate), 'NSS_STATE': str(root / 'nss-entry')})
            commands = [json.loads(line) for line in log.read_text().splitlines()] if log.exists() else []
            receipt = output / 'fixture-lifecycle.json'
            evidence = json.loads(receipt.read_text()) if receipt.exists() else None
            private_roots = {row['root'] for row in commands if row['root']}
            self.assertTrue(all(not Path(path).exists() for path in private_roots))
            self.assertNotIn('synthetic-private-secret', ''.join(path.read_text() for path in output.glob('*.json')))
            return result, commands, evidence

    def test_rejects_non_hosted_execution_before_fixture_side_effects(self):
        result, commands, receipt = self.run_fixture(hosted=False)
        self.assertEqual(result.returncode, 2)
        self.assertFalse(any(row['name'] in ['test-instance.sh', 'docker', 'pnpm'] for row in commands))
        self.assertIsNone(receipt)

    def test_failure_still_cleans_owned_fixture_and_preserves_nonzero_result(self):
        # Actual E2E cannot deterministically make setup, teardown, or the
        # container inventory command fail without losing the hosted runner.
        for failure in ['up', 'verify', 'browser', 'down', 'leftover', 'empty']:
            with self.subTest(failure=failure):
                result, commands, receipt = self.run_fixture(failure)
                self.assertNotEqual(result.returncode, 0)
                actions = [row['args'][0] for row in commands if row['name'] == 'test-instance.sh']
                self.assertEqual(actions.count('down'), 1)
                self.assertEqual(actions[-1], 'down')
                self.assertEqual(receipt['result'], 'failed')
                self.assertEqual(receipt['level'], 'error')
                self.assertEqual(receipt['stage'], {'up': 'setup', 'verify': 'catalog-verification',
                    'browser': 'connection-settings', 'down': 'complete', 'leftover': 'complete',
                    'empty': 'browser-result'}[failure])
                self.assertNotIn('totp', json.dumps(receipt).lower())

    def test_success_requires_one_executed_journey_and_zero_owned_resources(self):
        result, commands, receipt = self.run_fixture()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(receipt['result'], 'passed')
        self.assertEqual(receipt['level'], 'info')
        self.assertEqual(receipt['stage'], 'complete')
        self.assertEqual(receipt['remainingContainers'], 0)
        self.assertEqual(receipt['remainingVolumes'], 0)
        self.assertTrue(receipt['browserTrustRemoved'])
        browser = [row for row in commands if row['name'] == 'pnpm']
        self.assertEqual(len(browser), 1)
        self.assertIn('--retries=0', browser[0]['args'])
        self.assertIn('--max-failures=1', browser[0]['args'])
        self.assertIn(TITLE, browser[0]['args'])
        owned = [row for row in commands if row['name'] == 'test-instance.sh']
        self.assertEqual(len({row['project'] for row in owned}), 1)
        self.assertTrue(owned[0]['project'].startswith('kinosail-populated-'))
        names = [(row['name'], row['args']) for row in commands]
        added = next(i for i, (name, args) in enumerate(names) if name == 'certutil' and '-A' in args)
        browser_index = next(i for i, (name, _) in enumerate(names) if name == 'pnpm')
        removed = next(i for i, (name, args) in enumerate(names) if name == 'certutil' and '-D' in args)
        self.assertLess(added, browser_index)
        self.assertLess(browser_index, removed)
        self.assertIn('C,,', names[added][1])

    def test_trust_failures_do_not_admit_the_browser_or_hide_cleanup_failure(self):
        # Browser E2E reproduces untrusted TLS. Stand-ins inject certificate
        # export, NSS collision, import and teardown failures before browser use.
        for failure in ['export', 'private-ca', 'collision', 'import', 'remove-trust']:
            with self.subTest(failure=failure):
                result, commands, receipt = self.run_fixture(failure)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(receipt['result'], 'failed')
                self.assertEqual(receipt['level'], 'error')
                browser = [row for row in commands if row['name'] == 'pnpm']
                self.assertEqual(len(browser), int(failure == 'remove-trust'))
                nss = [row['args'] for row in commands if row['name'] == 'certutil']
                if failure in ['export', 'private-ca', 'collision']:
                    self.assertFalse(any('-A' in args or '-D' in args for args in nss))
                if failure == 'remove-trust':
                    self.assertNotEqual(receipt['cleanupExitCode'], 0)
                    self.assertFalse(receipt['browserTrustRemoved'])
                self.assertEqual(receipt['stage'], 'complete' if failure == 'remove-trust' else 'browser-trust')
                self.assertTrue(receipt['privateFixtureRemoved'])


if __name__ == '__main__':
    unittest.main()
