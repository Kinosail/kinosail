#!/usr/bin/env python3
"""Apple build argument boundaries; run only when repository gates are enabled."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


BUILD = Path(__file__).resolve().parents[2] / 'apps/player/apps/native/scripts/build-apple.sh'


class AppleBuildArgumentsTests(unittest.TestCase):
    def test_invalid_arguments_do_not_invoke_git_or_xcode(self):
        cases = [[], ['android'], ['macos'], ['watchos'], ['visionos'], ['ios', 'release'],
                 ['tvos', ''], ['ios', 'simulator', 'extra'], ['ios\n'], ['x' * 4096]]
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            log = root / 'invocations'
            for tool in ('git', 'xcodebuild'):
                executable = root / tool
                executable.write_text('#!/bin/sh\nprintf "called\\n" >> "$KINOSAIL_BUILD_TEST_LOG"\nexit 99\n')
                executable.chmod(0o755)
            env = dict(os.environ, PATH=f'{root}:/usr/bin:/bin', KINOSAIL_BUILD_TEST_LOG=str(log))
            for arguments in cases:
                with self.subTest(arguments=arguments):
                    result = subprocess.run([str(BUILD), *arguments], env=env, capture_output=True, timeout=10)
                    self.assertEqual(result.returncode, 2)
                    self.assertFalse(log.exists())


if __name__ == '__main__':
    unittest.main()
