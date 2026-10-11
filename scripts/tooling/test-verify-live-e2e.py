#!/usr/bin/env python3
"""A source-cache hit must not hide a changed live browser target.

This isolated CLI contract uses a stand-in browser command. Product E2E tests
do not invoke the local verifier or exercise its stage cache.
"""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class LiveBrowserVerificationTests(unittest.TestCase):
    def test_live_target_is_rechecked_while_source_only_collection_can_be_cached(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            tooling = root / 'scripts/tooling'
            tooling.mkdir(parents=True)
            for name in ('verify-changed.sh', 'gates-pause.sh', 'verify-input-digest.py'):
                source = ROOT / 'scripts/tooling' / name
                if source.exists():
                    shutil.copy2(source, tooling / name)
            cap = tooling / 'check-go-loc.sh'
            cap.write_text('#!/bin/sh\nexit 0\n')
            cap.chmod(0o755)
            e2e = root / 'apps/player/e2e'
            e2e.mkdir(parents=True)
            spec = e2e / 'live.spec.ts'
            spec.write_text('// baseline\n')

            def git(*arguments):
                return subprocess.check_output(
                    ['git', '-c', 'core.hooksPath=/dev/null', '-c', 'user.name=Fixture',
                     '-c', 'user.email=fixture@example.invalid', *arguments],
                    cwd=root, text=True, stderr=subprocess.DEVNULL).strip()

            git('init', '-q')
            git('add', '.')
            git('-c', 'commit.gpgsign=false', 'commit', '-qm', 'baseline')
            base = git('rev-parse', 'HEAD')
            spec.write_text('// selected browser journey\n')
            git('add', '.')
            git('-c', 'commit.gpgsign=false', 'commit', '-qm', 'change journey')
            commands = root / 'bin'
            commands.mkdir()
            browser = commands / 'pnpm'
            browser.write_text('#!/bin/sh\ncase " $* " in\n'
                ' *" --list "*) exit 0 ;;\n'
                ' *) test ! -e "$VERIFY_LIVE_FAILURE" ;;\nesac\n')
            browser.chmod(0o755)
            failure = root / 'live-target-fails'
            env = dict(os.environ, PATH=str(commands) + os.pathsep + os.environ['PATH'],
                       KINOSAIL_VERIFY_PLAN='', KINOSAIL_VERIFY_WORKTREE='',
                       KINOSAIL_E2E_URL='http://127.0.0.1:1', VERIFY_LIVE_FAILURE=str(failure))

            def verify():
                return subprocess.run(['bash', str(tooling / 'verify-changed.sh'), 'apps/player', base],
                                      cwd=root, env=env, text=True, capture_output=True, timeout=30)

            first = verify()
            self.assertEqual(first.returncode, 0, first.stderr)
            self.assertIn('PASS e2e-changed', first.stdout)
            failure.touch()
            second = verify()
            self.assertNotEqual(second.returncode, 0, second.stdout)
            self.assertIn('CACHED e2e-list', second.stdout)
            self.assertIn('FAIL e2e-changed', second.stderr)
            failure.unlink()
            third = verify()
            self.assertEqual(third.returncode, 0, third.stderr)
            self.assertIn('PASS e2e-changed', third.stdout)
            # Same-size content and the original mtime must invalidate collection.
            previous = spec.stat()
            spec.write_text('// revised browser journey!\n')
            os.utime(spec, ns=(previous.st_atime_ns, previous.st_mtime_ns))
            fourth = verify()
            self.assertEqual(fourth.returncode, 0, fourth.stderr)
            self.assertIn('PASS e2e-list', fourth.stdout)


if __name__ == '__main__':
    unittest.main()
