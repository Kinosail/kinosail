#!/usr/bin/env python3
"""Prove the pause bypasses hooks and removing it restores enforcement."""
import json
import re
import shutil
import pathlib
import subprocess
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[2]


class GatePauseTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        global ROOT
        cls.original_root = ROOT
        cls.fixture = tempfile.TemporaryDirectory()
        target = pathlib.Path(cls.fixture.name)
        import io
        import tarfile
        archive = subprocess.check_output(['git', 'archive', 'HEAD'], cwd=ROOT)
        with tarfile.open(fileobj=io.BytesIO(archive)) as files:
            files.extractall(target, filter='data')
        # Overlay tracked working changes, including deletions.
        for name in subprocess.check_output(['git', 'ls-files'], cwd=ROOT, text=True).splitlines():
            source, destination = ROOT / name, target / name
            if source.is_file():
                destination.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(source, destination)
            else:
                destination.unlink(missing_ok=True)
        (target / '.gates-disabled').touch()
        ROOT = target

    @classmethod
    def tearDownClass(cls):
        global ROOT
        ROOT = cls.original_root
        cls.fixture.cleanup()

    def test_gate_shell_resumes_only_when_marker_removed(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            tooling = root / 'scripts' / 'tooling'
            tooling.mkdir(parents=True)
            for name in ('gate-shell.sh', 'gates-pause.sh'):
                shutil.copy2(ROOT / 'scripts' / 'tooling' / name, tooling / name)
            marker = root / '.gates-disabled'
            marker.touch()
            command = [str(tooling / 'gate-shell.sh'), '-c', 'touch ran; exit 23']
            paused = subprocess.run(command, cwd=root, capture_output=True)
            self.assertEqual(paused.returncode, 0)
            self.assertFalse((root / 'ran').exists())
            marker.unlink()
            self.assertEqual(subprocess.run(command, cwd=root).returncode, 23)
            self.assertTrue((root / 'ran').exists())

    def test_all_make_gate_targets_are_disabled(self):
        for relative in ('Makefile', 'packages/Makefile', 'apps/player/Makefile',
                         'apps/subtitles/Makefile'):
            makefile = ROOT / relative
            targets = re.search(r'^(.+): SHELL :=', makefile.read_text(), re.M)[1].split()
            with self.subTest(makefile=relative):
                result = subprocess.run(['make', '--silent', '--no-print-directory', *targets],
                                        cwd=makefile.parent, capture_output=True, text=True,
                                        timeout=30)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertIn('disabled until explicitly enabled', result.stdout)
                self.assertTrue(all('disabled until explicitly enabled' in line or
                                    'Nothing to be done' in line for line in result.stdout.splitlines()),
                                result.stdout)

    def test_direct_gate_entrypoints_are_disabled(self):
        paths = list((ROOT / 'scripts').rglob('*.sh')) + list((ROOT / 'apps').glob('*/scripts/*.sh'))
        commands = [['bash', str(p)] for p in paths
                    if '# shellcheck source=scripts/tooling/gates-pause.sh' in p.read_text()
                    and p.name != 'gate-shell.sh']
        commands += [['python3', str(p)] for p in (ROOT / 'scripts/quality').glob('check-*.py')]
        commands += [['node', str(p)] for p in (ROOT / 'scripts/quality').glob('check-*.mjs')]
        for command in commands:
            with self.subTest(command=command):
                result = subprocess.run(command, cwd=ROOT, capture_output=True, text=True, timeout=10)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertIn('disabled until explicitly enabled', result.stdout)

    def test_package_gate_commands_are_disabled(self):
        paths = list((ROOT / 'apps').glob('*/e2e/package.json'))
        for path in paths:
            for name, command in json.loads(path.read_text())['scripts'].items():
                if not name.startswith(('test', 'lint', 'typecheck', 'verify', 'format:check')):
                    continue
                with self.subTest(package=str(path), script=name):
                    result = subprocess.run(['/bin/sh', '-c', command], cwd=path.parent,
                                            capture_output=True, text=True, timeout=10)
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertIn('disabled until explicitly enabled', result.stdout)

    def test_hooks_resume_when_marker_removed(self):
        for hook in ('pre-commit.sh', 'pre-push-main.sh'):
            with self.subTest(hook=hook), tempfile.TemporaryDirectory() as directory:
                repo = pathlib.Path(directory)
                subprocess.run(['git', 'init', '-q', directory], check=True)
                tooling = repo / 'scripts' / 'tooling'
                tooling.mkdir(parents=True)
                shutil.copy2(ROOT / 'scripts/tooling/check-file-loc.py',
                             tooling / 'check-file-loc.py')
                subprocess.run(['git', '-C', directory, '-c', 'user.name=Test',
                                '-c', 'user.email=test@example.invalid',
                                'commit', '--allow-empty', '-qm', 'fixture'], check=True)
                guard = tooling / 'worktree_guard.py'
                guard.write_text('#!/bin/sh\ntouch enforcement-ran\nexit 23\n')
                guard.chmod(0o755)
                marker = repo / '.gates-disabled'
                marker.touch()
                command = ['bash', str(ROOT / 'scripts' / 'tooling' / hook)]
                paused = subprocess.run(command, cwd=repo, input='', text=True,
                                        capture_output=True)
                self.assertEqual(paused.returncode, 0, paused.stderr)
                self.assertIn('paused', paused.stdout)
                self.assertFalse((repo / 'enforcement-ran').exists())
                marker.unlink()
                enabled = subprocess.run(command, cwd=repo, input='', text=True,
                                         capture_output=True)
                self.assertEqual(enabled.returncode, 23, enabled.stderr)
                self.assertTrue((repo / 'enforcement-ran').exists())


if __name__ == '__main__':
    unittest.main()
