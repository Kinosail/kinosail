"""Focused native probe selection and discovery trust boundaries."""
from pathlib import Path
import json
import os
import subprocess
import tempfile
import textwrap
import unittest

ROOT = Path(__file__).resolve().parents[2]
WORKFLOWS = ROOT / '.github/workflows'


class NativeProbeContractTests(unittest.TestCase):
    def test_focused_native_probe_rejects_unknown_modes_before_outputs(self):
        source = (WORKFLOWS / 'app.yml').read_text()
        step = source.split('      - name: Validate focused probe\n', 1)[1].split('      - name:', 1)[0]
        script = textwrap.dedent(step.split('        run: |\n', 1)[1])
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'output'
            for mode, selector in [('AAC', 'DownloadAACProbeTests'), ('storage', 'DownloadStorageAuthorizationTests'),
                ('reader', 'NativePreferencesUXTests/readerPendingMatchesLoadedColumnAndClearsAtTerminalStates(light:fontSize:)'),
                ('', None), ('unknown', None), ('AAC\n', None), ('reader\n', None), ('../AAC', None), ('x' * 8193, None)]:
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
        self.assertIn('options: [none, AAC, storage, reader]', root)
        self.assertIn("native_probe: ${{ github.event_name == 'workflow_dispatch' && inputs.native_probe || 'none' }}", root)
        self.assertIn("steps.probe.outcome == 'success'", source)

    def test_reader_probe_rejects_missing_or_wrong_iteration_discovery(self):
        # Native E2E cannot prove its own missing selector or truncated iteration
        # log is rejected. Require all three complete one-test repetitions.
        from unittest.mock import patch
        source = (WORKFLOWS / 'app.yml').read_text()
        step = source.split('      - name: Execute one iOS probe suite\n', 1)[1].split('      - name:', 1)[0]
        script = textwrap.dedent(step.split("          python3 - <<'PY'\n", 1)[1].split('\n          PY', 1)[0])
        for counts in ([1, 1, 1], [], [1, 1], [0, 0, 0], [1, 1, 2]):
            with self.subTest(counts=counts), tempfile.TemporaryDirectory() as directory:
                run = Path(directory)
                (run / 'command.txt').write_text('isolated command fixture')
                (run / 'native.log').write_text('\n'.join(f'Test run with {count} test in 1 suite passed' for count in counts))
                with patch.dict(os.environ, {'NATIVE_PROBE': 'reader', 'NATIVE_PROBE_EXIT': '0',
                    'NATIVE_PROBE_RUN': str(run), 'NATIVE_PROBE_DEVICE': 'isolated-simulator'}), \
                    patch('platform.platform', return_value='isolated-platform'), \
                    patch('subprocess.check_output', return_value='isolated-revision-or-xcode'):
                    if counts == [1, 1, 1]:
                        exec(compile(script, 'native-probe-receipt', 'exec'), {})
                    else:
                        with self.assertRaises(SystemExit):
                            exec(compile(script, 'native-probe-receipt', 'exec'), {})
                receipt = json.loads((run / 'receipt.json').read_text())
                self.assertEqual(receipt['discoveryVerified'], counts == [1, 1, 1])
                self.assertEqual(receipt['expectedTests'], 1)
                self.assertEqual(receipt['expectedIterations'], 3)

