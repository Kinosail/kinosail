"""Exercise hosted codec ownership with fake tools, never apt or app processes."""
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


def browser_step(name):
    browser = (ROOT / '.github/workflows/app.yml').read_text().split('  browser:\n', 1)[1].split('  required:\n', 1)[0]
    return browser.split(f'      - name: {name}\n', 1)[1].split('\n      - ', 1)[0]


def step_command(step):
    return '\n'.join(line[10:] for line in step.split('        run: |\n', 1)[1].splitlines())


class PublicFlowCodecSetup(unittest.TestCase):
    def test_each_chromium_consumer_has_one_codec_owner_before_runner(self):
        startup = browser_step('Install startup fixture codecs')
        subtitles = browser_step('Install Subtitles public-flow codecs')
        runner = browser_step('Install public-flow runner')
        self.assertIn("if: inputs.app == 'player'\n", startup)
        self.assertNotIn('matrix.engine', startup)
        self.assertIn('sha256sum --check', startup)
        self.assertIn('echo /usr/lib/jellyfin-ffmpeg >> "$GITHUB_PATH"', startup)
        self.assertIn("if: inputs.app == 'subtitles' && matrix.engine == 'chromium'\n", subtitles)
        self.assertIn("if: matrix.engine == 'chromium'\n", runner)
        self.assertNotIn('apt-get', runner)
        self.assertNotIn('sudo', runner)
        source = (ROOT / '.github/workflows/app.yml').read_text()
        self.assertLess(source.index('name: Install startup fixture codecs'), source.index('name: Install public-flow runner'))
        self.assertLess(source.index('name: Install Subtitles public-flow codecs'), source.index('name: Install public-flow runner'))
        self.assertIn('timeout-minutes: 30', source.split('  browser:\n', 1)[1].split('  required:\n', 1)[0])
        self.assertIn('run: scripts/e2e/run.sh "$APP"', source)
        self.assertEqual(step_command(subtitles), 'sudo apt-get update\nsudo apt-get install -y --no-install-recommends ffmpeg')

    def run_runner(self, *, missing=None, encoders=('libx264', 'aac')):
        with tempfile.TemporaryDirectory(prefix='codec-contract-') as directory:
            root = Path(directory)
            tools = root / 'tools'
            tools.mkdir()
            events = root / 'events'
            scripts = {
                'sudo': '#!/bin/sh\nprintf "forbidden sudo\\n" >> "$CODEC_EVENTS"\nexit 99\n',
                'pnpm': '#!/bin/sh\nprintf "pnpm %s\\n" "$*" >> "$CODEC_EVENTS"\n',
                'ffprobe': '#!/bin/sh\nprintf "ffprobe %s\\n" "$*" >> "$CODEC_EVENTS"\n',
                'ffmpeg': '#!/bin/sh\nprintf "ffmpeg %s\\n" "$*" >> "$CODEC_EVENTS"\n'
                          + 'if [ "$2" = "-encoders" ]; then\n'
                          + ''.join(f'  printf " V..... {encoder} fixture\\n"\n' for encoder in encoders)
                          + 'fi\n',
            }
            for name, script in scripts.items():
                # A fail-closed stand-in avoids falling through to host codecs.
                path = tools / name
                path.write_text('#!/bin/sh\nexit 127\n' if name == missing else script)
                path.chmod(0o700)
            env = {'PATH': str(tools) + ':/usr/bin:/bin', 'RUNNER_TEMP': str(root), 'CODEC_EVENTS': str(events)}
            result = subprocess.run(['/bin/bash', '-e', '-o', 'pipefail', '-c', step_command(browser_step('Install public-flow runner'))],
                                    cwd=root, env=env, capture_output=True, text=True, timeout=3)
            return result.returncode, events.read_text().splitlines() if events.exists() else []

    def test_existing_codec_tools_reach_exact_frozen_pnpm_install_without_apt(self):
        status, events = self.run_runner()
        self.assertEqual(status, 0)
        self.assertEqual(events, ['ffmpeg -version', 'ffprobe -version', 'ffmpeg -hide_banner -encoders',
                                  'pnpm --dir scripts/e2e install --frozen-lockfile'])

    def test_missing_codec_or_fixture_encoder_stops_before_dependency_side_effects(self):
        for missing in ('ffmpeg', 'ffprobe'):
            with self.subTest(missing=missing):
                status, events = self.run_runner(missing=missing)
                self.assertNotEqual(status, 0)
                self.assertFalse(any(event.startswith('pnpm') or 'sudo' in event for event in events))
        for encoders in (('aac',), ('libx264',), ('libx264-extra', 'aac-extra')):
            with self.subTest(encoders=encoders):
                status, events = self.run_runner(encoders=encoders)
                self.assertNotEqual(status, 0)
                self.assertFalse(any(event.startswith('pnpm') or 'sudo' in event for event in events))


if __name__ == '__main__':
    unittest.main()
