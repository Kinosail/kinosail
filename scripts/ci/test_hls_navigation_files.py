"""Fixed result admission must reject unsafe filesystem inputs without blocking."""
import importlib.util
import json
import os
from pathlib import Path
import stat
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
CHECKER = ROOT / 'scripts/ci/check-hls-navigation-result.py'


def report(completed):
    return {'errors': [], 'stats': {'expected': 13, 'skipped': 0, 'unexpected': 0, 'flaky': 0},
            'suites': [{'specs': [{'id': 'case-' + str(index), 'file': 'player-hls-navigation.spec.ts',
                'ok': True, 'tests': [{'projectName': 'webkit', 'expectedStatus': 'passed',
                    'status': 'expected', 'results': [{'status': 'passed', 'retry': 0, 'errors': []}]
                    if completed else []}]}]} for index in range(13)]}


def fixture(root):
    output = root / '.verification/hls-navigation'
    output.mkdir(parents=True)
    (output / 'list-webkit.json').write_text(json.dumps(report(False)))
    (output / 'results-webkit.json').write_text(json.dumps(report(True)))
    return output


def snapshot(root):
    return [(str(p.relative_to(root)), p.lstat().st_mode, p.lstat().st_size, p.lstat().st_mtime_ns)
            for p in sorted(root.rglob('*'))]


class HLSNavigationFileTests(unittest.TestCase):
    def run_checker(self, root):
        before = snapshot(root)
        try:
            result = subprocess.run([sys.executable, str(CHECKER)], cwd=root,
                                    capture_output=True, text=True, timeout=2)
        except subprocess.TimeoutExpired:
            self.fail('fixed result admission blocked while an owned FIFO writer remained open')
        self.assertEqual(snapshot(root), before)
        return result

    def test_connected_no_data_fifo_rejects_each_report_without_waiting(self):
        for name in ['list-webkit.json', 'results-webkit.json']:
            with self.subTest(name=name), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                path = fixture(root) / name
                path.unlink()
                os.mkfifo(path, 0o600)
                writer = os.open(path, os.O_RDWR | os.O_NONBLOCK)
                try:
                    actual = self.run_checker(root)
                    self.assertEqual(actual.returncode, 2)
                    self.assertEqual(actual.stderr, 'invalid fixed HLS navigation proof\n')
                finally:
                    os.close(writer)

    def test_symlink_nonregular_empty_and_oversized_reports_reject(self):
        for name in ['list-webkit.json', 'results-webkit.json']:
            for kind in ['symlink', 'directory', 'empty', 'oversized']:
                with self.subTest(name=name, kind=kind), tempfile.TemporaryDirectory() as temporary:
                    root = Path(temporary)
                    path = fixture(root) / name
                    data = path.read_bytes()
                    path.unlink()
                    if kind == 'symlink':
                        target = root / 'outside.json'
                        target.write_bytes(data)
                        path.symlink_to(target)
                    elif kind == 'directory':
                        path.mkdir()
                    else:
                        path.write_bytes(b'' if kind == 'empty' else b'x' * 2097153)
                    actual = self.run_checker(root)
                    self.assertEqual(actual.returncode, 2)
                    self.assertEqual(actual.stderr, 'invalid fixed HLS navigation proof\n')

    def test_symlinked_input_directories_reject(self):
        for component in ['.verification', 'hls-navigation']:
            with self.subTest(component=component), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                output = fixture(root)
                original = root / '.verification' if component == '.verification' else output
                moved = root / 'outside-directory'
                original.rename(moved)
                original.symlink_to(moved, target_is_directory=True)
                self.assertEqual(self.run_checker(root).returncode, 2)

    def test_original_exact_13_first_attempt_result_still_accepts(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            fixture(root)
            actual = self.run_checker(root)
            self.assertEqual(actual.returncode, 0)
            self.assertEqual(actual.stdout, 'HLS navigation proof: 13 passed, zero skips/retries/failures\n')

    def test_descriptor_and_current_path_mutation_reject(self):
        # Isolated deterministic reader control: the owned publisher changes
        # a real file after its first descriptor stat. No concurrent CLI claim.
        spec = importlib.util.spec_from_file_location('hls_result_reader', CHECKER)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        for kind in ['descriptor', 'replacement']:
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                output = fixture(root)
                path = output / 'list-webkit.json'
                original_stat, changed = os.fstat, []

                def publish_after_stat(fd):
                    info = original_stat(fd)
                    if not changed:
                        if kind == 'descriptor':
                            path.write_bytes(path.read_bytes() + b' ')
                        else:
                            replacement = output / 'replacement'
                            replacement.write_bytes(path.read_bytes())
                            replacement.replace(path)
                        changed.append(kind)
                    return info

                previous = Path.cwd()
                try:
                    os.chdir(root)
                    with patch('os.fstat', side_effect=publish_after_stat):
                        with self.assertRaises(ValueError):
                            module.read_report('list-webkit.json')
                finally:
                    os.chdir(previous)
                self.assertEqual(changed, [kind])

    def test_canonical_directory_replacement_rejects_stable_leaf(self):
        # Owned publisher control, after the report FD opens. The leaf's
        # bytes/inode remain stable; only canonical directory ownership changes.
        spec = importlib.util.spec_from_file_location('hls_directory_reader', CHECKER)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        for component in ['.verification', 'hls-navigation', 'unrelated-artifact']:
            with self.subTest(component=component), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                output = fixture(root)
                leaf = output / 'list-webkit.json'
                leaf_before = leaf.stat()
                leaf_bytes = leaf.read_bytes()
                original_stat, published = os.fstat, []

                def publish_after_leaf_stat(fd):
                    info = original_stat(fd)
                    if stat.S_ISREG(info.st_mode) and not published:
                        if component == 'unrelated-artifact':
                            (output / 'other-artifact.txt').write_text('synthetic artifact')
                        else:
                            directory = root / '.verification' if component == '.verification' else output
                            directory.rename(root / 'retired-directory')
                            directory.mkdir()
                        published.append(snapshot(root))
                    return info

                previous = Path.cwd()
                try:
                    os.chdir(root)
                    with patch('os.fstat', side_effect=publish_after_leaf_stat):
                        if component == 'unrelated-artifact':
                            self.assertEqual(module.read_report('list-webkit.json'), report(False))
                        else:
                            with self.assertRaises(ValueError):
                                module.read_report('list-webkit.json')
                    self.assertEqual(snapshot(root), published[0])
                    current_leaf = (leaf if component == 'unrelated-artifact' else
                                    root / 'retired-directory' / ('hls-navigation' if component == '.verification' else '') / 'list-webkit.json')
                    after = current_leaf.stat()
                    self.assertEqual(current_leaf.read_bytes(), leaf_bytes)
                    self.assertEqual((after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns),
                                     (leaf_before.st_dev, leaf_before.st_ino, leaf_before.st_size, leaf_before.st_mtime_ns))
                finally:
                    os.chdir(previous)
                self.assertEqual(len(published), 1)


if __name__ == '__main__':
    unittest.main()
