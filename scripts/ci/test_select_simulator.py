"""Exercise the simulator CLI boundary; app journeys cannot select CI runtimes."""
import json
import os
from pathlib import Path
import subprocess
import sys
import unittest

SCRIPT = Path(__file__).with_name('select-simulator.py')
FIRST = '11111111-1111-1111-1111-111111111111'
SECOND = '22222222-2222-2222-2222-222222222222'


class SimulatorSelectionTests(unittest.TestCase):
    def run_cli(self, data, platform='iOS'):
        return subprocess.run([sys.executable, str(SCRIPT), platform],
                              input=json.dumps(data), text=True, capture_output=True,
                              env=os.environ.copy(), check=False)

    def test_selects_newest_compatible_available_platform(self):
        data = {'devices': {
            'com.apple.CoreSimulator.SimRuntime.iOS-18-0': [{'udid': FIRST, 'isAvailable': True}],
            'com.apple.CoreSimulator.SimRuntime.iOS-26-0': [{'udid': FIRST, 'isAvailable': True}],
            'com.apple.CoreSimulator.SimRuntime.iOS-27-0': [{'udid': SECOND, 'isAvailable': True}],
            'com.apple.CoreSimulator.SimRuntime.tvOS-27-0': [{'udid': FIRST, 'isAvailable': True}],
        }}
        result = self.run_cli(data)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, SECOND + '\n')
        self.assertEqual(self.run_cli(data, 'tvOS').stdout, FIRST + '\n')

    def test_rejects_unusable_or_malformed_inventory_without_destination(self):
        runtime = 'com.apple.CoreSimulator.SimRuntime.iOS-27-0'
        for data, platform in [
            ({}, 'iOS'), ({'devices': []}, 'iOS'),
            ({'devices': {runtime: [{'udid': SECOND, 'isAvailable': False}]}}, 'iOS'),
            ({'devices': {runtime: [{'udid': 'a' * 36, 'isAvailable': True}]}}, 'iOS'),
            ({'devices': {runtime: [{'udid': SECOND, 'isAvailable': 'true'}]}}, 'iOS'),
            ({'devices': {runtime: 'not a device list'}}, 'iOS'),
            ({'devices': {runtime: []}}, 'watchOS'),
            ({'devices': {runtime: [{}] * 2049}}, 'iOS'),
        ]:
            with self.subTest(data=str(data)[:60], platform=platform):
                result = self.run_cli(data, platform)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.stdout, '')

    def test_rejects_oversized_and_invalid_json(self):
        for raw in ('x' * (1024 * 1024 + 1), '{', '{"devices":{},"devices":{}}'):
            result = subprocess.run([sys.executable, str(SCRIPT), 'iOS'],
                                    input=raw, text=True, capture_output=True, check=False)
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(result.stdout, '')


if __name__ == '__main__':
    unittest.main()
