"""Counterfactual trust faults that generated valid public HLS cannot inject."""
import os
import fcntl
from pathlib import Path
import tempfile
import subprocess
import sys
import unittest
from unittest import mock
from hls_nonkey_installation import rewrite_initial_arguments, bounded_state, append_private, finish_counterfactual


SOURCE = '/owned/media/Fixture.mkv'
CACHE = '/owned/cache'


def arguments():
    return ['-hide_banner', '-loglevel', 'error', '-y', '-ss', '12.5', '-i', SOURCE,
        '-map', '0:v:0', '-map', '0:a:0?', '-sn', '-c:v', 'copy', '-c:a', 'copy',
        '-f', 'hls', '-hls_time', '2', '-hls_playlist_type', 'event',
        '-hls_segment_type', 'fmp4', '-hls_segment_options', 'movflags=+frag_discont+skip_sidx',
        '-hls_flags', 'temp_file', '-hls_fmp4_init_filename', 'init.mp4',
        '-hls_segment_filename', CACHE + '/key/360p/segment-%05d.m4s',
        CACHE + '/key/360p/index.m3u8']


class InstallationIntegrity(unittest.TestCase):
    def assert_bounded_rejection(self, path, operation):
        code = ('import sys\nfrom hls_nonkey_installation import bounded_state,append_private\n'
            'try:\n ' + operation + '\nexcept RuntimeError:\n sys.exit(0)\nsys.exit(1)\n')
        result = subprocess.run([sys.executable, '-B', '-c', code, str(path)],
            env=os.environ | {'PYTHONPATH': str(Path(__file__).parent)}, capture_output=True, timeout=1)
        self.assertEqual(result.returncode, 0)

    def test_special_files_are_rejected_without_waiting_for_a_peer(self):
        with tempfile.TemporaryDirectory() as temporary:
            fifo = Path(temporary) / 'fifo'; os.mkfifo(fifo, 0o600)
            for operation in ["bounded_state(sys.argv[1], 16)", "append_private(sys.argv[1], {'applied':False})"]:
                self.assert_bounded_rejection(fifo, operation)

    def test_held_private_audit_lock_has_an_elapsed_bound(self):
        with tempfile.TemporaryDirectory() as temporary:
            audit = Path(temporary) / 'audit'
            with os.fdopen(os.open(audit, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600), 'wb') as file:
                fcntl.flock(file.fileno(), fcntl.LOCK_EX)
                self.assert_bounded_rejection(audit, "append_private(sys.argv[1], {'applied':False})")
                self.assertEqual(audit.stat().st_size, 0)

    def test_finish_hashes_and_parses_one_verified_descriptor_without_reopening_path(self):
        with tempfile.TemporaryDirectory() as temporary:
            source = Path(temporary) / 'source'; source.write_bytes(b'owned')
            audit = Path(temporary) / 'audit'
            original = arguments(); original[original.index('-i') + 1] = str(source)
            effective, applied = rewrite_initial_arguments(original, str(source), CACHE)
            append_private(audit, {'original': original, 'effective': effective, 'applied': applied})
            before = bounded_state(audit, 256 * 1024)
            config = {'audit': str(audit), 'source': str(source), 'cache': CACHE,
                      'sourceState': bounded_state(source, 8 * 1024 * 1024)}
            case = {'counterfactualInstallation': {}}
            with mock.patch.object(Path, 'read_text', side_effect=RuntimeError('unchecked_second_open')):
                finish_counterfactual(config, case)
            self.assertEqual(case['counterfactualInstallation']['privateInvocationSHA256'], before['sha256'])
            self.assertEqual(case['counterfactualInstallation']['transformations'], 1)

    def test_exact_initial_copy_changes_only_two_output_options(self):
        original = arguments()
        effective, applied = rewrite_initial_arguments(original, SOURCE, CACHE)
        expected = arguments()
        expected[expected.index('-hls_segment_options') + 1] = 'movflags=+skip_sidx:use_editlist=1'
        expected[expected.index('-f'):expected.index('-f')] = ['-avoid_negative_ts', 'disabled']
        self.assertTrue(applied)
        self.assertEqual(effective, expected)
        self.assertEqual(original, arguments())
        self.assertGreater(effective.index('-avoid_negative_ts'), effective.index('-i'))

    def test_near_miss_duplicate_and_refill_commands_stay_unchanged(self):
        values = []
        for option, value in [('-ss', '12.4'), ('-i', '/other/source'), ('-c:v', 'libx264'),
                              ('-c:a', 'aac'), ('-f', 'mp4'), ('-hls_time', '4'),
                              ('-hls_segment_options', 'movflags=+skip_sidx')]:
            actual = arguments(); actual[actual.index(option) + 1] = value; values.append(actual)
        for option in ['-ss', '-i', '-c:v', '-c:a', '-f', '-hls_segment_options']:
            actual = arguments(); actual[-1:-1] = [option, actual[actual.index(option) + 1]]; values.append(actual)
        for extra in [['-start_number', '1'], ['-avoid_negative_ts', 'disabled'],
                      ['-output_ts_offset', '2'], ['-copyts']]:
            actual = arguments(); actual[-1:-1] = extra; values.append(actual)
        for actual in values:
            with self.subTest(command=values.index(actual)):
                self.assertEqual(rewrite_initial_arguments(actual, SOURCE, CACHE), (actual, False))

    def test_input_seek_order_output_escape_and_option_bounds_fail_closed(self):
        actual = arguments(); actual[4:6], actual[6:8] = actual[6:8], actual[4:6]
        values = [actual, arguments() + ['-unexpected'], ['x'] * 257, ['x' * 65537]]
        for destination in ['/other/index.m3u8', CACHE + '/../other/360p/index.m3u8',
                            CACHE + '/key/.seek-1/360p/index.m3u8']:
            actual = arguments(); actual[-1] = destination; values.append(actual)
        actual = arguments(); actual[actual.index('-hls_segment_filename') + 1] = '/other/segment-%05d.m4s'; values.append(actual)
        for actual in values:
            self.assertEqual(rewrite_initial_arguments(actual, SOURCE, CACHE), (actual, False))

    def test_snapshot_changes_with_content_and_rejects_oversized_or_symlink_files(self):
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / 'source'; path.write_bytes(b'abc')
            before = bounded_state(path, 3)
            path.write_bytes(b'abd')
            self.assertNotEqual(before, bounded_state(path, 3))
            link = Path(temporary) / 'link'; link.symlink_to(path)
            for bad, bound in [(path, 2), (link, 3), (Path(temporary), 3)]:
                with self.assertRaises(RuntimeError): bounded_state(bad, bound)

    def test_private_audit_rejects_symlinks_and_overflow_without_target_mutation(self):
        with tempfile.TemporaryDirectory() as temporary:
            target = Path(temporary) / 'target'; target.write_bytes(b'untouched')
            link = Path(temporary) / 'link'; link.symlink_to(target)
            with self.assertRaises(RuntimeError): append_private(link, {'applied': True})
            self.assertEqual(target.read_bytes(), b'untouched')
            audit = Path(temporary) / 'audit'; audit.write_bytes(b'x' * (256 * 1024))
            with self.assertRaises(RuntimeError): append_private(audit, {'applied': True})
            self.assertEqual(audit.stat().st_size, 256 * 1024)
            small = Path(temporary) / 'small'; append_private(small, {'applied': False})
            self.assertEqual(small.read_text(), '{"applied":false}\n')
            self.assertEqual(os.stat(small).st_mode & 0o777, 0o600)


if __name__ == '__main__':
    unittest.main()
