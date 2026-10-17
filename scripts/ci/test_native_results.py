"""CLI artifact controls, separate from native playback behavior.

Before implementation: missing, malformed, oversized, duplicate-key or wrong
platform summaries must not pass; neither may empty/failed execution or audio
main-thread warnings. Diagnostics must not echo private artifact text. Native
journeys cannot safely manufacture these invalid xcresult exporter inputs.
"""
import copy
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import textwrap
import unittest

SCRIPT = Path(__file__).with_name('inspect-native-results.py')
REVISION = 'a' * 40


def summary(platform='iOS'):
    return {'result': 'Passed', 'passedTests': 4, 'failedTests': 0,
            'skippedTests': 1, 'expectedFailures': 0, 'totalTestCount': 5,
            'testFailures': [], 'runtimeWarnings': [],
            'devicesAndConfigurations': [{'device': {'platform': platform + ' Simulator'}}]}


class NativeResultsTests(unittest.TestCase):
    def run_gate(self, raw, platform='iOS', revision=REVISION):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'private-summary.json'
            if raw is not None:
                path.write_bytes(raw)
            return subprocess.run([sys.executable, str(SCRIPT), platform, str(path), revision],
                                  capture_output=True, text=True, timeout=5)

    def test_success_records_independent_counts_revision_and_input_hash(self):
        for platform in ('iOS', 'tvOS'):
            with self.subTest(platform=platform):
                raw = json.dumps(summary(platform)).encode()
                result = self.run_gate(raw, platform)
                self.assertEqual(result.returncode, 0, result.stderr)
                receipt = json.loads(result.stdout)
                self.assertEqual(receipt['result'], 'passed')
                self.assertEqual(receipt['revision'], REVISION)
                self.assertEqual(receipt['platform'], platform)
                self.assertEqual(receipt['passedTests'], 4)
                self.assertEqual(receipt['skippedTests'], 1)
                self.assertEqual(receipt['summarySHA256'], hashlib.sha256(raw).hexdigest())

    def test_fresh_backup_contract_requires_exactly_one_executed_test(self):
        for passed, skipped, wanted in [(1, 0, 0), (0, 1, 1), (2, 0, 1), (1, 1, 1), (0, 0, 1)]:
            with self.subTest(passed=passed, skipped=skipped):
                data = summary()
                data.update(passedTests=passed, skippedTests=skipped, totalTestCount=passed + skipped)
                result = self.run_gate(json.dumps(data).encode(), 'iOS-backup')
                self.assertEqual(result.returncode, wanted, result.stderr)
        data = summary('tvOS')
        data.update(passedTests=1, skippedTests=0, totalTestCount=1)
        self.assertNotEqual(self.run_gate(json.dumps(data).encode(), 'iOS-backup').returncode, 0)


    def test_unrelated_warning_is_counted_without_exposing_private_text(self):
        data = summary()
        data['runtimeWarnings'] = [{'message': 'FocusState private-fixture-name',
                                    'sourceURL': 'https://private.invalid/?token=secret'}]
        result = self.run_gate(json.dumps(data).encode())
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(json.loads(result.stdout)['otherRuntimeWarnings'], 1)
        self.assertNotIn('private', result.stdout + result.stderr)
        self.assertNotIn('secret', result.stdout + result.stderr)

    def test_audio_main_thread_warnings_fail_even_when_tests_pass(self):
        messages = [
            'SessionCore.mm:631 This method can lead to UI unresponsiveness if called on the main thread while the audio session is active.',
            'AVAudioSession_iOS.mm:978 This method can lead to UI unresponsiveness if called on the main thread.',
            'AVAudioSession.mm:1001 This method can lead to UI unresponsiveness if called on the main thread.',
        ]
        for message in messages:
            with self.subTest(message=message.split()[0]):
                data = summary()
                data['runtimeWarnings'] = [{'message': message}]
                result = self.run_gate(json.dumps(data).encode())
                self.assertEqual(result.returncode, 1)
                receipt = json.loads(result.stdout)
                self.assertEqual(receipt['result'], 'failed')
                self.assertEqual(receipt['audioMainThreadWarnings'], 1)
                self.assertNotIn(message, result.stdout)

    def test_empty_failed_or_inconsistent_execution_is_rejected(self):
        variants = [
            {'passedTests': 0, 'totalTestCount': 1},
            {'failedTests': 1, 'totalTestCount': 6, 'result': 'Failed'},
            {'expectedFailures': 1, 'totalTestCount': 6},
            {'testFailures': [{'message': 'private failure text'}]},
            {'totalTestCount': 999}, {'passedTests': True},
            {'passedTests': -1}, {'passedTests': 1.5},
            {'result': 'Unknown'}, {'runtimeWarnings': None},
            {'runtimeWarnings': [{}]}, {'devicesAndConfigurations': []},
            {'devicesAndConfigurations': [{'device': {'platform': 'tvOS Simulator'}}]},
        ]
        for variant in variants:
            with self.subTest(variant=variant):
                data = summary()
                data.update(variant)
                result = self.run_gate(json.dumps(data).encode())
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn('private failure text', result.stdout + result.stderr)

    def test_required_fields_cannot_silently_default_to_success(self):
        for key in summary():
            with self.subTest(key=key):
                data = copy.deepcopy(summary())
                del data[key]
                self.assertNotEqual(self.run_gate(json.dumps(data).encode()).returncode, 0)

    def test_invalid_input_is_bounded_and_diagnostics_are_fixed(self):
        valid = json.dumps(summary()).encode()
        for raw in (None, b'{"private-secret":', b'[]', b'\xff', b' ' * (2 * 1024 * 1024 + 1),
                    valid[:-1] + b',"passedTests":4}', b'[' * 2000):
            with self.subTest(size=len(raw) if raw is not None else None):
                result = self.run_gate(raw)
                self.assertEqual(result.returncode, 1)
                self.assertEqual(result.stdout, '')
                self.assertEqual(result.stderr, 'native result summary rejected\n')

    def test_untrusted_platform_and_revision_are_not_echoed(self):
        for platform, revision in [('private-secret', REVISION), ('iOS', 'private-secret')]:
            result = self.run_gate(json.dumps(summary()).encode(), platform, revision)
            self.assertEqual(result.returncode, 1)
            self.assertNotIn('private-secret', result.stdout + result.stderr)

    def test_workflow_checks_both_platforms_and_preserves_any_failure(self):
        root = SCRIPT.parents[2]
        source = (root / '.github/workflows/deep.yml').read_text()
        step = source.split('      - name: Inspect native contract results\n', 1)[1].split('      - name:', 1)[0]
        self.assertIn('if: always()', step)
        script = textwrap.dedent(step.split('        run: |\n', 1)[1])
        with tempfile.TemporaryDirectory() as directory:
            folder = Path(directory)
            fake = folder / 'xcrun'
            fake.write_text('''#!/usr/bin/env python3
import json, os, pathlib, sys
contract = next(pathlib.Path(arg).stem.removeprefix('native-') for arg in sys.argv if arg.endswith('.xcresult'))
platform = contract.split('-')[0]
with open(os.environ['TEST_EXPORT_LOG'], 'a') as stream: stream.write(contract + '\\n')
if os.environ.get('TEST_EXPORT_FAILURE') == contract: sys.exit(2)
data = json.loads(os.environ['TEST_SUMMARY'])
data['devicesAndConfigurations'][0]['device']['platform'] = platform + ' Simulator'
if contract == 'iOS-backup': data.update(passedTests=1, skippedTests=0, totalTestCount=1)
if os.environ.get('TEST_WARNING') == contract:
    data['runtimeWarnings'] = [{'message': 'AVAudioSession.mm:123 UI unresponsiveness on the main thread'}]
print(json.dumps(data))
''')
            fake.chmod(0o755)
            for export_failure, warning in [('', ''), ('iOS-backup', ''), ('iOS', ''), ('tvOS', ''), ('', 'iOS-backup'), ('', 'iOS'), ('', 'tvOS')]:
                with self.subTest(export_failure=export_failure, warning=warning):
                    log = folder / 'exports.log'
                    log.write_text('')
                    result = subprocess.run(['bash', '-ec', script], cwd=root, capture_output=True,
                        text=True, timeout=10, env={**os.environ,
                            'PATH': f'{folder}{os.pathsep}{os.environ["PATH"]}', 'RUNNER_TEMP': directory,
                            'GITHUB_SHA': REVISION, 'TEST_EXPORT_LOG': str(log),
                            'TEST_SUMMARY': json.dumps(summary()), 'TEST_EXPORT_FAILURE': export_failure,
                            'TEST_WARNING': warning})
                    self.assertEqual(log.read_text().splitlines(), ['iOS-backup', 'iOS', 'tvOS'])
                    self.assertEqual(result.returncode, 1 if export_failure or warning else 0, result.stderr)


if __name__ == '__main__':
    unittest.main()
