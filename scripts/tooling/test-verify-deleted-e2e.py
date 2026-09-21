#!/usr/bin/env python3
"""Deleted browser specs must not schedule nonexistent or unfiltered test runs."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class DeletedBrowserSpecTests(unittest.TestCase):
    def test_deleted_specs_are_excluded_and_retained_changes_are_selected(self):
        for keep_changed in (False, True):
            with self.subTest(keep_changed=keep_changed), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                tooling = root / 'scripts/tooling'
                tooling.mkdir(parents=True)
                for name in ('verify-changed.sh', 'gates-pause.sh'):
                    shutil.copy2(ROOT / 'scripts/tooling' / name, tooling / name)
                e2e = root / 'apps/player/e2e'
                e2e.mkdir(parents=True)
                retired = e2e / 'retired.spec.ts'
                retained = e2e / 'retained.spec.ts'
                retired.write_text('// retired client\n')
                retained.write_text('// current browser client\n')

                def git(*arguments):
                    return subprocess.run(
                        ['git', '-c', 'core.hooksPath=/dev/null', '-c', 'user.name=Fixture',
                         '-c', 'user.email=fixture@example.invalid', *arguments],
                        cwd=root, text=True, capture_output=True, check=True,
                    ).stdout.strip()

                git('init', '-q')
                git('add', '.')
                git('-c', 'commit.gpgsign=false', 'commit', '-qm', 'fixture')
                base = git('rev-parse', 'HEAD')
                retired.unlink()
                if keep_changed:
                    retained.write_text('// updated browser client\n')
                git('add', '-u')
                git('-c', 'commit.gpgsign=false', 'commit', '-qm', 'remove retired client')
                commands = root / 'bin'
                commands.mkdir()
                hasher = commands / 'shasum'
                hasher.write_text('#!/bin/sh\necho planning must not hash inputs >&2\nexit 99\n')
                hasher.chmod(0o755)
                env = dict(os.environ, KINOSAIL_VERIFY_PLAN='1', KINOSAIL_VERIFY_WORKTREE='',
                           PATH=str(commands) + os.pathsep + os.environ['PATH'],
                           KINOSAIL_E2E_URL='http://127.0.0.1:1')
                result = subprocess.run(
                    ['bash', str(tooling / 'verify-changed.sh'), 'apps/player', base],
                    cwd=root, env=env, text=True, capture_output=True, timeout=30,
                )
                self.assertEqual(result.returncode, 0, result.stderr)
                browser_stages = [line for line in result.stdout.splitlines()
                                  if line.startswith('PLAN e2e-')]
                if keep_changed:
                    self.assertEqual(len(browser_stages), 2)
                    for stage in browser_stages:
                        self.assertIn('retained.spec.ts', stage)
                        self.assertNotIn('retired.spec.ts', stage)
                else:
                    self.assertEqual(browser_stages, [])


if __name__ == '__main__':
    unittest.main()
