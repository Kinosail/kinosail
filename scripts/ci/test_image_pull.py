"""Publication input/ordering gaps that populated browser E2E cannot observe."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
DIGEST = 'sha256:' + 'a' * 64


def image(app):
    return f'ghcr.io/kinosail/kinosail-{app}@{DIGEST}'


class ImmutableImagePullTests(unittest.TestCase):
    def run_script(self, name, arguments, pull_status=0, runtime=False):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            scripts = root / 'scripts/ci'
            scripts.mkdir(parents=True)
            for source in ('test-image.sh', 'pull-image.sh'):
                if (ROOT / 'scripts/ci' / source).is_file():
                    shutil.copyfile(ROOT / 'scripts/ci' / source, scripts / source)
                    (scripts / source).chmod(0o755)
            binary = root / 'bin'
            binary.mkdir()
            docker = binary / 'docker'
            docker.write_text('#!/bin/sh\nprintf "%s\\n" "$*" >> "$CALLS"\nexit "$PULL_STATUS"\n')
            docker.chmod(0o755)
            calls, started = root / 'calls', root / 'started'
            if runtime:
                for app in ('player', 'subtitles'):
                    fixture = root / 'apps' / app / 'scripts'
                    fixture.mkdir(parents=True)
                    runner = fixture / 'test-container.sh'
                    runner.write_text('#!/bin/sh\nprintf "%s|%s|%s\\n" "$CONTAINER_ENGINE" "$KINOSAIL_TEST_IMAGE" "$KINOSAIL_TEST_IMAGE_READY" >> "$STARTED"\n')
                    runner.chmod(0o755)
            env = {**os.environ, 'PATH': str(binary) + os.pathsep + os.environ['PATH'],
                   'CALLS': str(calls), 'STARTED': str(started), 'PULL_STATUS': str(pull_status)}
            result = subprocess.run(['bash', str(scripts / name), *arguments], env=env,
                                    capture_output=True, text=True, timeout=5)
            return result, calls.read_text().splitlines() if calls.exists() else [], started.read_text().splitlines() if started.exists() else []

    def test_foreground_pull_precedes_both_workflow_verifiers(self):
        for workflow, app in (('publish.yml', '${{ matrix.app }}'),
                              ('release.yml', '${{ needs.preflight.outputs.app }}')):
            with self.subTest(workflow=workflow):
                source = (ROOT / '.github/workflows' / workflow).read_text()
                foreground, parallel = source.split('      - parallel:\n', 1)
                self.assertIn('      - name: Pull immutable candidate image\n', foreground,
                              'validated foreground pull must finish before either verifier')
                pull = foreground.split('      - name: Pull immutable candidate image\n', 1)[1]
                self.assertIn(f'APP: {app}\n', pull)
                reference = f'ghcr.io/kinosail/kinosail-{app}@${{{{ steps.build.outputs.digest }}}}'
                self.assertIn('IMAGE: ' + reference + '\n', pull)
                self.assertIn('run: ./scripts/ci/pull-image.sh "$APP" "$IMAGE"\n', pull)
                self.assertNotIn('continue-on-error:', pull)
                self.assertNotIn('if:', pull)
                self.assertIn('image-ref: ' + reference + '\n', parallel)
                self.assertIn('severity: CRITICAL,HIGH\n', parallel)
                self.assertIn('exit-code: "1"\n', parallel)

    def test_valid_candidates_pull_the_exact_digest_for_each_app(self):
        for app in ('player', 'subtitles'):
            with self.subTest(app=app):
                result, calls, started = self.run_script('pull-image.sh', [app, image(app)])
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(calls, ['pull ' + image(app)])
                self.assertEqual(started, [])

    def test_invalid_pull_inputs_make_zero_docker_calls(self):
        invalid = [[], ['player'], ['player', image('player'), 'extra'],
                   ['unknown', image('player')], ['player', image('subtitles')],
                   ['player', 'ghcr.io/kinosail/kinosail-player:latest'],
                   ['player', image('player').replace('ghcr.io/', 'elsewhere.invalid/')],
                   ['player', image('player')[:-1]],
                   ['player', image('player').replace('a' * 64, 'A' * 64)],
                   ['player', image('player') + ' '], ['player', image('player') + '\n']]
        for arguments in invalid:
            with self.subTest(arguments=arguments):
                result, calls, started = self.run_script('pull-image.sh', arguments)
                self.assertEqual(result.returncode, 2, result.stderr)
                self.assertEqual(calls, [])
                self.assertEqual(started, [])

    def test_failed_pull_propagates_without_starting_runtime(self):
        for script in ('pull-image.sh', 'test-image.sh'):
            with self.subTest(script=script):
                result, calls, started = self.run_script(script, ['player', image('player')], 42, True)
                self.assertEqual(result.returncode, 42, result.stderr)
                self.assertEqual(calls, ['pull ' + image('player')])
                self.assertEqual(started, [])

    def test_runtime_receives_the_same_validated_digest(self):
        for app in ('player', 'subtitles'):
            with self.subTest(app=app):
                result, calls, started = self.run_script('test-image.sh', [app, image(app)], runtime=True)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(calls, ['pull ' + image(app)])
                self.assertEqual(started, ['docker|' + image(app) + '|1'])

    def test_runtime_rejects_invalid_input_before_docker_or_container(self):
        for arguments in ([], ['player', image('subtitles')], ['player', image('player'), 'extra']):
            with self.subTest(arguments=arguments):
                result, calls, started = self.run_script('test-image.sh', arguments, runtime=True)
                self.assertEqual(result.returncode, 2, result.stderr)
                self.assertEqual(calls, [])
                self.assertEqual(started, [])


if __name__ == '__main__':
    unittest.main()
