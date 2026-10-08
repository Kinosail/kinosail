"""The manual lane runs literal targets; command peers never run Go or apps."""
import os
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
CALLER = ROOT / 'scripts/ci/run-verify-changed.sh'
WORKFLOW = ROOT / '.github/workflows/verify-changed.yml'


class VerifyChangedWorkflowTests(unittest.TestCase):
    def test_invalid_selection_and_bypass_environment_reject_before_effects(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            events = root / 'effects'
            for name in ('git', 'make'):
                path = root / name
                path.write_text('#!' + sys.executable + '\nimport os,pathlib\n'
                                'pathlib.Path(os.environ["CONTROL_EVENTS"]).touch()\n')
                path.chmod(0o755)
            env = os.environ | {'PATH': str(root) + ':' + os.environ['PATH'],
                                'CONTROL_EVENTS': str(events)}
            for args in ([], ['unknown'], ['both', 'extra'], ['player' * 1000]):
                result = subprocess.run(['bash', str(CALLER), *args], env=env,
                                        capture_output=True, timeout=5)
                self.assertEqual(result.returncode, 2)
                self.assertFalse(events.exists())
            for variable in ('KINOSAIL_VERIFY_PLAN', 'KINOSAIL_VERIFY_WORKTREE',
                             'KINOSAIL_PACKAGES_VERIFIED', 'KINOSAIL_E2E_URL'):
                result = subprocess.run(['bash', str(CALLER), 'both'],
                                        env=env | {variable: '1'}, capture_output=True, timeout=5)
                self.assertEqual(result.returncode, 2)
                self.assertFalse(events.exists())

    def test_selected_literal_commands_run_serially_and_preserve_first_failure(self):
        for selection, expected in [('both', ['player', 'subtitles']),
                                    ('player', ['player']), ('subtitles', ['subtitles'])]:
            with self.subTest(selection=selection), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                (root / 'scripts/ci').mkdir(parents=True)
                caller = root / 'scripts/ci/run-verify-changed.sh'
                caller.write_bytes(CALLER.read_bytes())
                tools = root / 'tools'
                tools.mkdir()
                events = root / 'events'
                program = '''import json,os,pathlib,sys
args=sys.argv[1:];root=pathlib.Path(os.environ['CONTROL_ROOT'])
with (root/'events').open('a') as file:file.write(json.dumps([pathlib.Path(sys.argv[0]).name,args])+'\\n')
if pathlib.Path(sys.argv[0]).name=='git':
 if 'rev-parse' in args:print(str(root/'.git') if '--git-common-dir' in args else ('b' if 'origin/main' in args else 'a')*40)
else:
 app=args[args.index('-C')+1].split('/')[-1]
 logs=root/'.git/kinosail-verify-logs'/('apps-'+app);logs.mkdir(parents=True)
 (logs/'max-loc-required.log').write_text('command peer; no actual Go or container')
 sys.exit(7 if app=='player' else 0)
'''
                for name in ('git', 'make'):
                    path = tools / name
                    path.write_text('#!' + sys.executable + '\n' + program)
                    path.chmod(0o755)
                result = subprocess.run(['bash', str(caller), selection], cwd=root,
                                        env=os.environ | {'CONTROL_ROOT': str(root),
                                            'PATH': str(tools) + ':' + os.environ['PATH']},
                                        capture_output=True, text=True, timeout=5)
                self.assertEqual(result.returncode, int('player' in expected), result.stderr)
                rows = [json.loads(line) for line in events.read_text().splitlines()]
                calls = [args for kind, args in rows if kind == 'make']
                self.assertEqual(calls, [['-C', 'apps/' + app, 'verify-changed', 'BASE=origin/main']
                                        for app in expected])
                self.assertTrue(any(kind == 'git' and args == ['fetch', '--no-tags', 'origin', 'main']
                                    for kind, args in rows))
                receipt = json.loads((root / '.verification/verify-changed/receipt.json').read_text())
                self.assertEqual([row['app'] for row in receipt['results']], expected)
                self.assertEqual([row['exit'] for row in receipt['results']],
                                 [7 if app == 'player' else 0 for app in expected])
                for app in expected:
                    self.assertTrue((root / '.verification/verify-changed' / (app + '-stages.tar.gz')).is_file())

    def test_manual_only_closed_selection_and_pinned_tools_precede_literal_targets(self):
        text = WORKFLOW.read_text()
        self.assertIn('workflow_dispatch:', text)
        self.assertNotIn('pull_request:', text)
        self.assertNotIn('schedule:', text)
        self.assertIn('options: [both, player, subtitles]', text)
        self.assertIn('fetch-depth: 0', text)
        self.assertIn('ref: ${{ github.sha }}', text)
        self.assertIn('version: 11.22.0', text)
        self.assertIn('node-version: 26', text)
        self.assertIn('govulncheck@v1.7.0', text)
        self.assertIn('golangci-lint@v2.13.1', text)
        self.assertLess(text.index('run-verify-changed.sh "$APP_SELECTION" --admit-only'),
                        text.index('actions/setup-go@'))
        self.assertIn('if: always()', text)
        self.assertIn('run-verify-changed.sh "$APP_SELECTION"', text)

    def test_target_source_preserves_order_real_base_and_failure_artifacts(self):
        text = CALLER.read_text()
        self.assertIn('git fetch --no-tags origin main', text)
        self.assertIn('make -C "apps/$app" verify-changed BASE=origin/main', text)
        self.assertIn('both) apps=(player subtitles)', text)
        self.assertIn('status=1', text)
        self.assertIn('kinosail-verify-logs', text)
        self.assertIn('receipt.json', text)


if __name__ == '__main__':
    unittest.main()
