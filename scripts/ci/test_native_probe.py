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
        source = (WORKFLOWS / 'deep.yml').read_text()
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
        self.assertIn('options: [none, AAC, storage, reader]', source)
        self.assertIn("if: always() && inputs.native_probe != 'none'", source)
        self.assertIn("steps.probe.outcome == 'success'", source)

    def test_reader_probe_checks_all_parameter_repetitions_from_xcresult(self):
        from unittest.mock import patch
        source = (WORKFLOWS / 'deep.yml').read_text()
        step = source.split('      - name: Execute one iOS probe suite\n', 1)[1].split('      - name:', 1)[0]
        script = textwrap.dedent(step.split("          python3 - <<'PY'\n", 1)[1].split('\n          PY', 1)[0])
        selector = 'NativePreferencesUXTests/readerPendingMatchesLoadedColumnAndClearsAtTerminalStates(light:fontSize:)'
        cases = ('passed', 'missing-summary', 'wrong-summary', 'duplicate-summary', 'mixed-summary', 'wrong-selector', 'missing-arguments',
                 'duplicate-arguments', 'missing-repeat', 'duplicate-repeat', 'skipped-repeat', 'failed-repeat',
                 'overall-failed', 'missing-result', 'invalid-result')
        for case in cases:
            with self.subTest(case=case), tempfile.TemporaryDirectory() as directory:
                details = {'testIdentifier': selector, 'testResult': 'Passed', 'testRuns': [
                    {'name': name, 'nodeType': 'Arguments', 'result': 'Passed', 'children': [
                        {'nodeType': 'Repetition', 'nodeIdentifier': str(n), 'result': 'Passed'} for n in (1, 2, 3)]}
                    for name in ('false, 20', 'false, 32', 'true, 20', 'true, 32')]}
                if case == 'wrong-selector': details['testIdentifier'] = 'Wrong/test()'
                if case == 'missing-arguments': details['testRuns'].pop()
                if case == 'duplicate-arguments': details['testRuns'].append(details['testRuns'][0])
                if case == 'missing-repeat': details['testRuns'][0]['children'].pop()
                if case == 'duplicate-repeat': details['testRuns'][0]['children'][2]['nodeIdentifier'] = '1'
                if case == 'skipped-repeat': details['testRuns'][0]['children'][2]['result'] = 'Skipped'
                if case == 'failed-repeat': details['testRuns'][0]['children'][2]['result'] = 'Failed'
                if case == 'overall-failed': details['testResult'] = 'Failed'
                run = Path(directory)
                (run / 'command.txt').write_text('isolated command fixture')
                summary = 'Test run with 1 test in 1 suite passed'
                if case == 'missing-summary': summary = ''
                if case == 'wrong-summary': summary = 'Test run with 2 tests in 1 suite passed'
                if case == 'duplicate-summary': summary += '\n' + summary
                if case == 'mixed-summary': summary = 'Test run with 2 tests in 1 suite passed\n' + summary
                (run / 'native.log').write_text(summary)
                def command_output(command, **_):
                    if command[0] == 'xcrun' and case == 'missing-result':
                        raise subprocess.CalledProcessError(1, command)
                    if command[0] == 'xcrun' and case == 'invalid-result': return '{'
                    return json.dumps(details) if command[0] == 'xcrun' else 'isolated-revision-or-xcode'
                with patch.dict(os.environ, {'NATIVE_PROBE': 'reader', 'NATIVE_PROBE_EXIT': '0', 'SELECTOR': selector,
                    'NATIVE_PROBE_RUN': str(run), 'NATIVE_PROBE_DEVICE': 'isolated-simulator'}), \
                    patch('platform.platform', return_value='isolated-platform'), \
                    patch('subprocess.check_output', side_effect=command_output):
                    if case == 'passed':
                        exec(compile(script, 'native-probe-receipt', 'exec'), {})
                    else:
                        with self.assertRaises(SystemExit):
                            exec(compile(script, 'native-probe-receipt', 'exec'), {})
                receipt = json.loads((run / 'receipt.json').read_text())
                self.assertEqual(receipt['discoveryVerified'], case == 'passed')
                self.assertEqual(receipt['result'] == 'passed', case == 'passed')
                self.assertEqual(receipt['expectedTests'], 1)
                self.assertEqual(receipt['expectedIterations'], 3)
