#!/usr/bin/env python3
"""Native verification must select and hash every catalog/build dependency.

This isolated CLI contract executes the real verifier, Git and content hasher.
Compiler, module and source-cap commands are stand-ins. Product E2E cannot
exercise this local command's selection and persistent stage cache.
"""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
COPY_INPUTS = (
    'Makefile', 'scripts/generate-native-copy.py',
    'internal/server/locales/active.es.json',
    'apps/android/app/src/main/java/Copy.kt',
    'apps/android/wear/src/main/java/Copy.kt',
    'apps/android/app/src/main/assets/interface-copy/es.json',
    'apps/android/wear/src/main/res/xml/locale_config.xml',
    '../../packages/localization/languages.go',
)


class NativeVerificationTests(unittest.TestCase):
    def fixture(self, directory):
        root = Path(directory)
        tooling = root / 'scripts/tooling'
        tooling.mkdir(parents=True)
        for name in ('verify-changed.sh', 'gates-pause.sh', 'verify-input-digest.py'):
            shutil.copy2(ROOT / 'scripts/tooling' / name, tooling / name)
        for name in ('check-go-loc.sh', 'with-go-module.sh'):
            path = tooling / name
            path.write_text('#!/bin/sh\nexit 0\n')
            path.chmod(0o755)
        app = root / 'apps/player'
        native = app / 'apps/native/Sources/View.swift'
        native.parent.mkdir(parents=True)
        native.write_text('// A\n')
        for name in COPY_INPUTS:
            path = app / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text('// A\n')
        (app / 'README.md').write_text('A\n')
        commands = root / 'bin'
        commands.mkdir()
        make = commands / 'make'
        make.write_text('#!/bin/sh\nprintf "%s\\n" "$*" >> "$VERIFY_NATIVE_COMMANDS"\n'
            'case "$*" in\nclient-check|native-copy-check) test ! -e "$VERIFY_NATIVE_FAILURE" ;;\n'
            '*) exit 0 ;;\nesac\n')
        make.chmod(0o755)

        def git(*arguments):
            return subprocess.check_output(
                ['git', '-c', 'core.hooksPath=/dev/null', '-c', 'user.name=Fixture',
                 '-c', 'user.email=fixture@example.invalid', '-c', 'commit.gpgsign=false', *arguments],
                cwd=root, text=True, stderr=subprocess.DEVNULL).strip()

        git('init', '-q')
        git('add', '.')
        git('commit', '-qm', 'baseline')
        base = git('rev-parse', 'HEAD')
        failure = root / 'fail-native'
        env = dict(os.environ, PATH=str(commands) + os.pathsep + os.environ['PATH'],
                   KINOSAIL_VERIFY_PLAN='', KINOSAIL_VERIFY_WORKTREE='',
                   KINOSAIL_E2E_URL='', KINOSAIL_PACKAGES_VERIFIED='1',
                   VERIFY_NATIVE_COMMANDS=str(root / 'commands.log'), VERIFY_NATIVE_FAILURE=str(failure))

        def verify():
            return subprocess.run(['bash', str(tooling / 'verify-changed.sh'), 'apps/player', base],
                                  cwd=root, env=env, text=True, capture_output=True, timeout=30)

        def change(path):
            path.write_text(path.read_text().replace('A', 'B'))
            git('add', str(path))
            git('commit', '-qm', 'change verification dependency')

        return app, native, failure, verify, change

    def test_native_client_cache_cannot_hide_changed_catalog_inputs(self):
        for name in COPY_INPUTS:
            with self.subTest(input=name), tempfile.TemporaryDirectory() as directory:
                app, native, failure, verify, change = self.fixture(directory)
                change(native)
                first = verify()
                self.assertEqual(first.returncode, 0, first.stderr)
                self.assertIn('PASS native-client', first.stdout)
                self.assertIn('CACHED native-client', verify().stdout)
                dependency = app / name
                previous = dependency.stat()
                change(dependency)
                os.utime(dependency, ns=(previous.st_atime_ns, previous.st_mtime_ns))
                failure.touch()
                result = verify()
                self.assertNotEqual(result.returncode, 0, result.stdout)
                self.assertIn('FAIL native-client', result.stderr)

    def test_copy_only_commits_are_verified(self):
        for name in COPY_INPUTS:
            with self.subTest(input=name), tempfile.TemporaryDirectory() as directory:
                app, _, failure, verify, change = self.fixture(directory)
                change(app / name)
                failure.touch()
                result = verify()
                self.assertNotEqual(result.returncode, 0, result.stdout)
                self.assertRegex(result.stderr, r'FAIL native-(client|copy)')

    def test_unrelated_documentation_does_not_schedule_native_commands(self):
        with tempfile.TemporaryDirectory() as directory:
            app, _, failure, verify, change = self.fixture(directory)
            change(app / 'README.md')
            failure.touch()
            result = verify()
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertNotIn('native-client', result.stdout)
            self.assertNotIn('native-copy', result.stdout)


if __name__ == '__main__':
    unittest.main()
