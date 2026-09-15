#!/usr/bin/env python3
"""Focused updater failure-path coverage; run when repository gates are enabled."""
import importlib.util
from pathlib import Path
import tempfile
import tarfile
import subprocess
import shutil
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('updater', Path(__file__).with_name('deploy-apple-devices.py'))
updater = importlib.util.module_from_spec(spec)
spec.loader.exec_module(updater)


class AppleDeployTests(unittest.TestCase):
    def test_invalid_implementation_metadata_has_no_device_side_effects(self):
        invalid = ['', 'invalid plist', 'x' * 65537]
        invalid += [updater.plistlib.dumps(value).decode() for value in (
            [], {}, {'KinosailImplementationState': True}, {'KinosailImplementationState': []},
            {'KinosailImplementationState': 'ready'}, {'KinosailImplementationState': 'implemented '})]
        for value in invalid:
            with self.subTest(value=value[:80]), tempfile.TemporaryDirectory() as directory, patch.object(updater, 'run', return_value=value) as run:
                root = Path(directory)
                with self.assertRaises(ValueError):
                    updater.deploy(root, root / 'repo.git', 'rev', 'a' * 40, 'iphone', 'device', 'team')
                self.assertEqual(len(run.call_args_list), 1)
                self.assertEqual(run.call_args.args[0], 'git')
                self.assertFalse((root / 'iphone').exists())
                self.assertFalse((root / 'iphone.json').exists())

    def test_scaffold_is_not_built_signed_or_installed(self):
        metadata = updater.plistlib.dumps({'KinosailImplementationState': 'scaffold'}).decode()
        with tempfile.TemporaryDirectory() as directory, patch.object(updater, 'run', return_value=metadata) as run:
            root = Path(directory)
            updater.deploy(root, root / 'repo.git', 'rev', 'a' * 40, 'iphone', 'device', 'team')
            self.assertEqual(len(run.call_args_list), 1)
            self.assertEqual(run.call_args.args[0], 'git')
            self.assertEqual(list(root.iterdir()), [])

    def test_invalid_arguments_have_no_side_effects(self):
        valid = ['deploy', '--team', 'ABCDEFGHIJ', '--iphone', '89251873-20F4-574A-80B7-1CAEAE25ABA6', '--tv', '72A5068C-97F3-54A3-B5AF-E2A725E7E344']
        cases = [[], ['--unexpected'], ['--team', ''], ['--team', 'x' * 10000],
                 ['--team', 'abcdefghij'], ['--iphone', '../device'], ['--tv', '89251873-20F4-574A-80B7-1CAEAE25ABA6']]
        for extra in cases:
            argv = ['deploy'] if not extra else valid + extra
            with self.subTest(extra=extra), patch('sys.argv', argv), patch.object(updater.Path, 'mkdir') as mkdir, patch.object(updater.subprocess, 'run') as run:
                with self.assertRaises(SystemExit):
                    updater.main()
                mkdir.assert_not_called()
                run.assert_not_called()

    def test_invalid_saved_state_stops_before_device_side_effects(self):
        for contents in ('{', '{}', '[]', 'x' * 1025,
                         '{"tree":"bad","revision":"bad","build":"-1"}'):
            with self.subTest(contents=contents[:20]), tempfile.TemporaryDirectory() as directory, patch.object(updater, 'run') as run:
                root = Path(directory)
                (root / 'iphone.json').write_text(contents)
                with self.assertRaises(ValueError):
                    updater.deploy(root, root / 'repo.git', 'rev', 'a' * 40, 'iphone', 'device', 'team')
                run.assert_not_called()

    @unittest.skipUnless(shutil.which('rsync'), 'rsync is required by the deployment tool')
    def test_sync_updates_nested_swift_sources_and_preserves_project(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            fixture = root / 'fixture'
            native_file = fixture / 'Sources/Platform/Downloads.swift'
            native_file.parent.mkdir(parents=True)
            native_file.write_text('new native implementation')
            generated = root / 'iphone/ios/Project.pbxproj'
            generated.parent.mkdir(parents=True)
            generated.write_text('generated project')
            def command(*args, **kwargs):
                if args[0] == 'git' and args[2] == 'show':
                    return updater.plistlib.dumps({'KinosailImplementationState': 'implemented'}).decode()
                elif args[0] == 'git' and args[2] == 'rev-list':
                    return '100'
                elif args[0] == 'git':
                    output = next(arg.removeprefix('--output=') for arg in args if arg.startswith('--output='))
                    with tarfile.open(output, 'w') as archive:
                        archive.add(fixture / 'modules', arcname='modules')
                elif args[0] in ('tar', 'rsync'):
                    subprocess.run(args, check=True)
                else:
                    raise OSError('stop before the native build')
            with patch.object(updater, 'run', side_effect=command), self.assertRaises(OSError):
                updater.deploy(root, root / 'repo.git', 'rev', 'a' * 40, 'iphone', 'device', 'team')
            self.assertEqual((root / 'iphone/Sources/Platform/Downloads.swift').read_text(), 'new native implementation')
            self.assertEqual(generated.read_text(), 'generated project')

    def test_confirmed_tree_is_not_reinstalled(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(updater, 'run') as run:
            root = Path(directory)
            (root / 'iphone.json').write_text(updater.json.dumps(dict(tree='a' * 40, revision='b' * 40, build='1234')))
            updater.deploy(root, root / 'repo.git', 'rev', 'a' * 40, 'iphone', 'device', 'team')
            run.assert_not_called()

    def test_failed_install_does_not_mark_success(self):
        def command(*args, **kwargs):
            if args[0] == 'git':
                return updater.plistlib.dumps({'KinosailImplementationState': 'implemented'}).decode()
            raise OSError('offline')
        with tempfile.TemporaryDirectory() as directory, patch.object(updater, 'run', side_effect=command) as run:
            root = Path(directory)
            (root / 'iphone-build/Build/Products/Release-iphoneos/KinosailPlayer.app').mkdir(parents=True)
            (root / 'iphone-built.json').write_text(updater.json.dumps(dict(tree='a' * 40, revision='b' * 40, build='1234')))
            with self.assertRaises(OSError):
                updater.deploy(root, root / 'repo.git', 'rev', 'a' * 40, 'iphone', 'device', 'team')
            self.assertFalse((root / 'iphone.json').exists())
            self.assertTrue(any(call.args[:6] == ('xcrun', 'devicectl', '--timeout', '120', 'device', 'install') for call in run.call_args_list))


if __name__ == '__main__':
    unittest.main()
