"""Public shell runner dispatch and rejection, with external build/run boundaries."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class E2ERunnerTests(unittest.TestCase):
    def run_runner(self, arguments, exit_code=0):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            runner = root / 'scripts/e2e'
            shutil.copytree(ROOT / 'scripts/e2e', runner,
                            ignore=shutil.ignore_patterns('node_modules', '.e2e'))
            tools = root / 'tools'
            tools.mkdir()
            (tools / 'go').write_text('#!/bin/sh\nprintf "%s\\n" "$*" >> "$BUILD_LOG"\n')
            (tools / 'python3').write_text(
                '#!' + sys.executable + '\nimport json, os, sys\n'
                'from pathlib import Path\n'
                'Path(os.environ["RUN_LOG"]).write_text(json.dumps(sys.argv[1:]))\n'
                'sys.exit(int(os.environ["RUN_EXIT"]))\n')
            for tool in tools.iterdir():
                tool.chmod(0o755)
            build_log, run_log = root / 'build.log', root / 'run.json'
            result = subprocess.run(['/bin/bash', str(runner / 'run.sh'), *arguments],
                                    env=os.environ | {'PATH': str(tools) + ':' + os.environ['PATH'],
                                        'BUILD_LOG': str(build_log), 'RUN_LOG': str(run_log),
                                        'RUN_EXIT': str(exit_code)}, capture_output=True, text=True)
            return result, (build_log.read_text().splitlines() if build_log.exists() else []), (
                json.loads(run_log.read_text()) if run_log.exists() else []), list((runner / '.e2e/bin').glob('run.*'))

    def test_default_and_all_dispatch_both_apps_without_target_and_preserve_failure(self):
        for arguments in ([], ['all']):
            with self.subTest(arguments=arguments):
                result, builds, command, remaining = self.run_runner(arguments, exit_code=7)
                self.assertEqual(result.returncode, 7, result.stderr)
                self.assertEqual(len(builds), 2)
                self.assertTrue(builds[0].endswith('./apps/player/cmd/kinosail'))
                self.assertTrue(builds[1].endswith('./apps/subtitles/cmd/kinosail'))
                self.assertEqual(command[4:8], ['pnpm', 'exec', 'e2e', 'run'])
                self.assertNotIn('--target', command)
                self.assertEqual(remaining, [])

    def test_selected_app_is_built_and_forwarded(self):
        for app in ('player', 'subtitles'):
            with self.subTest(app=app):
                result, builds, command, remaining = self.run_runner([app])
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(len(builds), 1)
                self.assertTrue(builds[0].endswith(f'./apps/{app}/cmd/kinosail'))
                self.assertEqual(command[4:10], ['pnpm', 'exec', 'e2e', 'run', '--target', app])
                self.assertEqual(remaining, [])

    def test_invalid_selection_cannot_build_or_execute(self):
        for arguments in ([''], ['unknown'], ['../player'], ['player', 'subtitles'], ['x' * 10000]):
            with self.subTest(arguments=arguments[:1]):
                result, builds, command, remaining = self.run_runner(arguments)
                self.assertEqual(result.returncode, 2)
                self.assertEqual((builds, command, remaining), ([], [], []))


if __name__ == '__main__':
    unittest.main()
