"""Model checksum-writing warmup without running the real Go tool."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / 'scripts/ci/prepare-campaign-go.sh'
MANIFESTS = ('go.work', 'go.work.sum', 'apps/player/go.mod', 'apps/player/go.sum',
             'apps/subtitles/go.mod', 'apps/subtitles/go.sum', 'packages/go.mod', 'packages/go.sum')


class CampaignDependencyTests(unittest.TestCase):
    def run_fixture(self, app):
        self.assertTrue(SCRIPT.is_file(), 'test-first dependency warmup is missing')
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            script = root / 'scripts/ci/prepare-campaign-go.sh'
            script.parent.mkdir(parents=True)
            shutil.copyfile(SCRIPT, script)
            for name in MANIFESTS:
                target = root / name
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_text(name + '\n')
            tools = root / 'bin'; tools.mkdir()
            go = tools / 'go'
            go.write_text('#!/bin/sh\nset -eu\n'
                          'test "$1" = -C && test "$3" = mod && test "$4" = download\n'
                          'test "$GOSUMDB" = fictional-verifier\n'
                          'test "$GOWORK" != "$SOURCE/go.work"\n'
                          'for file in go.work go.work.sum apps/player/go.mod apps/player/go.sum apps/subtitles/go.mod apps/subtitles/go.sum packages/go.mod packages/go.sum; do\n'
                          '  cmp "$SOURCE/$file" "$(dirname "$GOWORK")/$file"\n'
                          'done\n'
                          'printf checksum-change >> "$(dirname "$GOWORK")/go.work.sum"\n')
            go.chmod(0o700)
            environment = os.environ | {'PATH': str(tools) + ':' + os.environ['PATH'],
                                        'RUNNER_TEMP': str(root), 'SOURCE': str(root),
                                        'GOSUMDB': 'fictional-verifier'}
            actual = subprocess.run(['bash', str(script), app], cwd=root, env=environment,
                                    capture_output=True, text=True, timeout=3)
            self.assertEqual(actual.returncode, 0, actual.stderr)
            self.assertEqual({name: (root / name).read_text() for name in MANIFESTS},
                             {name: name + '\n' for name in MANIFESTS})

    def test_player_cache_preparation_preserves_all_source_manifests(self):
        self.run_fixture('player')

    def test_subtitles_cache_preparation_preserves_all_source_manifests(self):
        self.run_fixture('subtitles')

    def test_unknown_app_is_rejected_before_preparation(self):
        self.assertTrue(SCRIPT.is_file())
        result = subprocess.run(['bash', str(SCRIPT), '../unknown'], capture_output=True, timeout=3)
        self.assertEqual(result.returncode, 2)


if __name__ == '__main__':
    unittest.main()
