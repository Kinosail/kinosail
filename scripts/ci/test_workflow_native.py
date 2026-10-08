"""Hosted native and decoded-media workflow contracts."""
from pathlib import Path
import json
import os
import subprocess
import tempfile
import textwrap
import unittest

ROOT = Path(__file__).resolve().parents[2]
WORKFLOWS = ROOT / ".github/workflows"


class WorkflowNativeTests(unittest.TestCase):
    def test_focused_native_probe_rejects_unknown_modes_before_outputs(self):
        source = (WORKFLOWS / 'app.yml').read_text()
        step = source.split('      - name: Validate focused probe\n', 1)[1].split('      - name:', 1)[0]
        script = textwrap.dedent(step.split('        run: |\n', 1)[1])
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'output'
            for mode, selector in [('AAC', 'DownloadAACProbeTests'), ('storage', 'DownloadStorageAuthorizationTests'), ('', None), ('unknown', None), ('AAC\n', None), ('../AAC', None), ('x' * 8193, None)]:
                with self.subTest(mode=mode[:24]):
                    output.unlink(missing_ok=True)
                    result = subprocess.run(['bash', '-ec', script], capture_output=True, text=True, timeout=5,
                        env={**os.environ, 'NATIVE_PROBE': mode, 'GITHUB_OUTPUT': str(output)})
                    self.assertEqual(result.returncode == 0, selector is not None, result.stderr)
                    if selector:
                        self.assertEqual(output.read_text(), f'selector={selector}\n')
                    else:
                        self.assertFalse(output.exists())
        self.assertIn('-only-testing:Kinosail-iOSTests/$SELECTOR', source)
        self.assertIn('if: always()', source)
        self.assertIn('exit "$status"', source)
        self.assertNotIn('continue-on-error', source)
        root = (WORKFLOWS / 'ci.yml').read_text()
        self.assertIn('options: [none, AAC, storage]', root)
        self.assertIn("native_probe: ${{ github.event_name == 'workflow_dispatch' && inputs.native_probe || 'none' }}", root)
        self.assertIn("steps.probe.outcome == 'success'", source)


    def test_native_failure_still_executes_the_other_platform_and_fails_the_gate(self):
        # Failure modes: iOS hides tvOS evidence; either failed platform reports
        # success; both successful platforms incorrectly report failure.
        source = (WORKFLOWS / 'app.yml').read_text()
        step = source.split('      - name: Execute iOS and tvOS native contracts\n', 1)[1].split('      - name:', 1)[0]
        script = textwrap.dedent(step.split('        run: |\n', 1)[1])
        devices = {'devices': {
            'com.apple.CoreSimulator.SimRuntime.iOS-26-0': [{'deviceTypeIdentifier': 'com.apple.CoreSimulator.SimDeviceType.iPhone-17', 'udid': '11111111-1111-1111-1111-111111111111', 'isAvailable': True}],
            'com.apple.CoreSimulator.SimRuntime.tvOS-26-0': [{'deviceTypeIdentifier': 'com.apple.CoreSimulator.SimDeviceType.Apple-TV-4K', 'udid': '22222222-2222-2222-2222-222222222222', 'isAvailable': True}],
        }}
        with tempfile.TemporaryDirectory() as directory:
            folder = Path(directory)
            (folder / 'xcrun').write_text('#!/bin/sh\ncat <<\'JSON\'\n' + json.dumps(devices) + '\nJSON\n')
            (folder / 'xcodebuild').write_text('''#!/bin/sh
printf '%s\\n' "$*" >> "$TEST_NATIVE_LOG"
case "$*" in *Kinosail-iOS*) exit "$TEST_IOS_EXIT";; *) exit "$TEST_TV_EXIT";; esac
''')
            for name in ('xcrun', 'xcodebuild'):
                (folder / name).chmod(0o755)
            for ios, tvos in ((1, 0), (0, 1), (0, 0)):
                with self.subTest(ios=ios, tvos=tvos):
                    log = folder / 'native.log'
                    log.unlink(missing_ok=True)
                    result = subprocess.run(['bash', '-ec', script], cwd=ROOT, capture_output=True, text=True, timeout=10,
                        env={**os.environ, 'PATH': f'{folder}{os.pathsep}{os.environ["PATH"]}',
                             'RUNNER_TEMP': directory, 'TEST_NATIVE_LOG': str(log),
                             'TEST_IOS_EXIT': str(ios), 'TEST_TV_EXIT': str(tvos)})
                    commands = log.read_text().splitlines() if log.exists() else []
                    self.assertEqual(len(commands), 2, result.stderr)
                    self.assertIn('-scheme Kinosail-iOS', commands[0])
                    self.assertIn('-scheme Kinosail-tvOS', commands[1])
                    self.assertEqual(result.returncode, 1 if ios or tvos else 0, result.stderr)


    def test_deep_go_lane_installs_real_media_dependencies_before_testing(self):
        source = (WORKFLOWS / 'app.yml').read_text().split('  race:\n', 1)[1].split('  security:\n', 1)[0]
        self.assertIn('sudo apt-get update && sudo apt-get install -y --no-install-recommends libarchive-tools ffmpeg', source)
        self.assertIn('ffmpeg -version && ffprobe -version', source)
        self.assertLess(source.index('ffmpeg -version'), source.index('scripts/ci/test-go.sh'))


    def test_deep_apple_lane_executes_both_native_test_targets_and_retains_results(self):
        # Compilation cannot catch failing Swift Testing contracts or an omitted
        # platform. The manual and weekly lane must execute both existing schemes.
        source = (WORKFLOWS / 'app.yml').read_text().split('  client:\n', 1)[1].split('  android:\n', 1)[0]
        self.assertIn('if: fromJSON(inputs.plan).deep', source)
        self.assertIn('for platform in iOS tvOS', source)
        self.assertIn('xcodebuild test', source)
        self.assertIn('-scheme "Kinosail-$platform"', source)
        self.assertIn('-resultBundlePath "$RUNNER_TEMP/native-$platform.xcresult"', source)
        self.assertIn('if: always() && fromJSON(inputs.plan).deep', source)
        self.assertIn('${{ runner.temp }}/native-*.xcresult', source)
        self.assertNotIn('continue-on-error', source)


    def test_deep_apple_media_contracts_receive_bounded_decodable_fixtures(self):
        source = (WORKFLOWS / 'app.yml').read_text().split('  client:\n', 1)[1].split('  android:\n', 1)[0]
        step = source.split('      - name: Prepare decoded native media fixture\n', 1)[1].split('      - name:', 1)[0]
        self.assertIn('if: fromJSON(inputs.plan).deep', step)
        self.assertIn('ffmpeg', step)
        self.assertIn('-le 1048576', step)
        for path in ('kinosail-appletv-task7-media.mp4', 'kinosail-player-buffer-bar.mp4', 'kinosail-landscape-task10-media.mp4'):
            self.assertIn(path, step)
        self.assertLess(source.index('Prepare decoded native media fixture'), source.index('Execute iOS and tvOS native contracts'))
        self.assertIn('${{ runner.temp }}/native-media.json', source)



if __name__ == "__main__":
    unittest.main()
