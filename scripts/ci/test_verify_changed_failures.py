"""Failure receipts and base stability through the actual literal-target caller."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class VerifyChangedFailureTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        (self.root / 'scripts/ci').mkdir(parents=True)
        self.caller = self.root / 'scripts/ci/run-verify-changed.sh'
        self.caller.write_bytes((ROOT / 'scripts/ci/run-verify-changed.sh').read_bytes())
        tools = self.root / 'tools'
        tools.mkdir()
        program = '''import json,os,pathlib,sys
root=pathlib.Path(os.environ['CONTROL_ROOT']);args=sys.argv[1:];name=pathlib.Path(sys.argv[0]).name
with (root/'events').open('a') as file:file.write(json.dumps([name,args])+'\\n')
if name=='git':
 if args[0]=='fetch':sys.exit(int(os.environ.get('CONTROL_FETCH_EXIT','0')))
 if 'rev-parse' in args:
  if '--git-common-dir' in args:print(root/'.git')
  elif 'origin/main' in args:
   marker=root/'base-reads';count=int(marker.read_text())+1 if marker.exists() else 1;marker.write_text(str(count))
   print(('c' if (count>1 and os.environ.get('CONTROL_MOVE_BASE')) or (root/'ref-moved').exists() else 'b')*40)
  else:print('a'*40)
else:
 app=args[args.index('-C')+1].split('/')[-1]
 if not os.environ.get('CONTROL_MISSING_STAGES'):
  logs=root/'.git/kinosail-verify-logs'/('apps-'+app);logs.mkdir(parents=True)
  (logs/'max-loc-required.log').write_text('command peer only')
 if os.environ.get('CONTROL_INVALID_SECOND'): (logs/'z-invalid.json').write_text('not stage evidence')
 if os.environ.get('CONTROL_MOVE_DURING_MAKE'): (root/'ref-moved').touch()
 if os.environ.get('CONTROL_LARGE_LOG'):print('x'*(4194304+1))
 sys.exit(7 if os.environ.get('CONTROL_FAIL')==app else 0)
'''
        for name in ('git', 'make'):
            path = tools / name
            path.write_text('#!' + sys.executable + '\n' + program)
            path.chmod(0o755)
        self.env = os.environ | {'PATH': str(tools) + ':' + os.environ['PATH'],
                                 'CONTROL_ROOT': str(self.root)}

    def run_peer(self, arguments=None, **environment):
        return subprocess.run(['bash', str(self.caller), *(arguments or ['both'])],
                              cwd=self.root, env=self.env | environment,
                              capture_output=True, timeout=5)

    def receipt(self):
        return json.loads((self.root / '.verification/verify-changed/receipt.json').read_text())

    def makes(self):
        path = self.root / 'events'
        return [args for name, args in map(json.loads, path.read_text().splitlines()) if name == 'make'] if path.exists() else []

    def test_moving_base_rejects_before_any_target_with_receipt(self):
        result = self.run_peer(CONTROL_MOVE_BASE='1')
        self.assertEqual(result.returncode, 2)
        self.assertEqual(self.makes(), [])
        self.assertEqual(self.receipt()['phase'], 'base-stability')

    def test_make_failure_without_stage_logs_keeps_original_exit_and_second_target(self):
        result = self.run_peer(CONTROL_FAIL='player', CONTROL_MISSING_STAGES='1')
        self.assertEqual(result.returncode, 1)
        self.assertEqual(len(self.makes()), 2)
        results = self.receipt()['results']
        self.assertEqual([row['exit'] for row in results], [7, 0])
        self.assertEqual([row['stageEvidenceExit'] for row in results], [2, 2])

    def test_second_target_failure_preserves_both_attempts(self):
        result = self.run_peer(CONTROL_FAIL='subtitles')
        self.assertEqual(result.returncode, 1)
        self.assertEqual([row['exit'] for row in self.receipt()['results']], [0, 7])

    def test_fetch_failure_has_receipt_without_target_effects(self):
        result = self.run_peer(CONTROL_FETCH_EXIT='9')
        self.assertEqual(result.returncode, 9)
        self.assertEqual(self.makes(), [])
        self.assertEqual(self.receipt()['phase'], 'fetch')

    def test_gate_and_cache_early_exits_keep_failure_receipt_without_targets(self):
        (self.root / '.gates-disabled').touch()
        result = self.run_peer()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.makes(), [])
        self.assertEqual(self.receipt()['phase'], 'gates')

    def test_existing_stage_cache_cannot_be_claimed_as_fresh_execution(self):
        (self.root / '.git/kinosail-verify-cache').mkdir(parents=True)
        result = self.run_peer()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.makes(), [])
        self.assertEqual(self.receipt()['phase'], 'cache-admission')

    def test_symlinked_output_parent_rejects_before_git_or_target_effects(self):
        outside = self.root / 'outside'
        outside.mkdir()
        (self.root / '.verification').symlink_to(outside, target_is_directory=True)
        result = self.run_peer()
        self.assertEqual(result.returncode, 2)
        self.assertFalse((self.root / 'events').exists())
        self.assertEqual(list(outside.iterdir()), [])

    def test_admit_only_does_not_allow_bypass_environment(self):
        result = self.run_peer(['both', '--admit-only'], KINOSAIL_VERIFY_PLAN='1')
        self.assertEqual(result.returncode, 2)
        self.assertFalse((self.root / 'events').exists())

    def test_partial_archive_is_never_admitted_when_later_member_rejects(self):
        result = self.run_peer(CONTROL_INVALID_SECOND='1')
        self.assertEqual(result.returncode, 1)
        self.assertEqual(len(self.makes()), 2)
        rows = self.receipt()['results']
        self.assertEqual([row['stageEvidenceExit'] for row in rows], [2, 2])
        for row in rows:
            self.assertFalse(any(file['file'].endswith('.tar.gz') for file in row['files']))
        output = self.root / '.verification/verify-changed'
        self.assertEqual(list(output.glob('*.tar.gz')), [])
        self.assertEqual(list(output.glob('*.tmp')), [])

    def test_make_uses_recorded_sha_even_if_ref_moves_during_target(self):
        result = self.run_peer(CONTROL_MOVE_DURING_MAKE='1')
        self.assertEqual(result.returncode, 2)
        self.assertEqual(self.makes(), [['-C', 'apps/player', 'verify-changed', 'BASE=' + 'b' * 40]])
        receipt = self.receipt()
        self.assertEqual(receipt['base'], 'b' * 40)
        self.assertEqual(receipt['baseRef'], 'origin/main')
        self.assertEqual(receipt['results'][0]['argv'], ['make', *self.makes()[0]])
        self.assertEqual(receipt['phase'], 'base-stability')
        self.assertEqual(receipt['exit'], 2)

    def dangling_path_rejects_before_target(self, name):
        path = self.root / '.git' / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.symlink_to(self.root / 'missing', target_is_directory=True)
        result = self.run_peer()
        self.assertEqual(result.returncode, 2)
        self.assertEqual(self.makes(), [])
        self.assertTrue(path.is_symlink())
        self.assertFalse((self.root / 'missing').exists())

    def test_dangling_cache_rejects_before_target(self):
        self.dangling_path_rejects_before_target('kinosail-verify-cache')

    def test_dangling_log_parent_rejects_before_target(self):
        self.dangling_path_rejects_before_target('kinosail-verify-logs')

    def test_dangling_app_log_directory_rejects_before_target(self):
        self.dangling_path_rejects_before_target('kinosail-verify-logs/apps-player')

    def test_dangling_output_rejects_before_git_or_targets(self):
        parent = self.root / '.verification'
        parent.mkdir()
        target = parent / 'verify-changed'
        target.symlink_to(self.root / 'missing', target_is_directory=True)
        result = self.run_peer()
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((self.root / 'events').exists())
        self.assertTrue(target.is_symlink())
        self.assertFalse((self.root / 'missing').exists())

    def test_oversized_app_log_fails_bounded_receipt_and_excludes_raw_artifact(self):
        result = self.run_peer(CONTROL_LARGE_LOG='1')
        self.assertEqual(result.returncode, 2)
        self.assertTrue(self.receipt()['oversizedAppLog'])
        self.assertEqual(self.receipt()['exit'], 2)
        self.assertFalse((self.root / '.verification/verify-changed/player.log').exists())
        self.assertFalse((self.root / '.verification/verify-changed/subtitles.log').exists())


if __name__ == '__main__':
    unittest.main()
