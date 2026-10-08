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
PHONE = 'com.apple.CoreSimulator.SimDeviceType.iPhone-18-Pro'
TV = 'com.apple.CoreSimulator.SimDeviceType.Apple-TV-4K-3rd-generation-4K'


def device(identifier, kind=PHONE, available=True):
    return {'udid': identifier, 'deviceTypeIdentifier': kind, 'isAvailable': available}


class SimulatorSelectionTests(unittest.TestCase):
    def run_cli(self, data, platform='iOS'):
        return subprocess.run([sys.executable, str(SCRIPT), platform],
                              input=json.dumps(data), text=True, capture_output=True,
                              env=os.environ.copy(), check=False)

    def test_selects_newest_compatible_available_platform(self):
        data = {'devices': {
            'com.apple.CoreSimulator.SimRuntime.iOS-18-0': [device(FIRST)],
            'com.apple.CoreSimulator.SimRuntime.iOS-26-0': [device(FIRST)],
            'com.apple.CoreSimulator.SimRuntime.iOS-27-0': [device(SECOND)],
            'com.apple.CoreSimulator.SimRuntime.tvOS-27-0': [device(FIRST, TV)],
        }}
        result = self.run_cli(data)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, SECOND + '\n')
        self.assertEqual(self.run_cli(data, 'tvOS').stdout, FIRST + '\n')

    def test_phone_contracts_cannot_select_a_tablet_with_a_higher_identifier(self):
        data = {'devices': {'com.apple.CoreSimulator.SimRuntime.iOS-27-0': [
            device(FIRST), device(SECOND, 'com.apple.CoreSimulator.SimDeviceType.iPad-Pro-13-inch-M5'),
        ]}}
        result = self.run_cli(data)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, FIRST + '\n')

    def test_wrong_device_family_or_malformed_type_has_no_destination(self):
        for kind in ('com.apple.CoreSimulator.SimDeviceType.iPad-Pro-13-inch-M5',
                     TV, 'unknown', '', None, 'x' * 129,
                     PHONE + '/foreign', PHONE + '\n', PHONE + ' space',
                     'com.apple.CoreSimulator.SimDeviceType.iPhone-'):
            with self.subTest(kind=kind):
                result = self.run_cli({'devices': {'com.apple.CoreSimulator.SimRuntime.iOS-27-0': [device(FIRST, kind)]}})
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.stdout, '')

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
