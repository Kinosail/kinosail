"""Probe timeouts and cutoff cleanup must preserve bounded owned joins."""
import signal
import subprocess
import unittest
from unittest.mock import Mock, patch
from hls_remaining_nonkey_deadline import DiagnosticDeadline


class DeadlineTest(unittest.TestCase):
    def test_every_imported_subprocess_receives_remaining_budget(self):
        original = Mock(return_value='result')
        with patch('subprocess.run', original), patch('signal.signal'), patch('signal.getsignal'), \
             patch('hls_remaining_nonkey_deadline.time.monotonic', return_value=10):
            with DiagnosticDeadline(20):
                self.assertEqual(subprocess.run(['probe'], timeout=60), 'result')
                self.assertEqual(original.call_args.kwargs['timeout'], 20)
                subprocess.run(['probe'], timeout=2)
                self.assertEqual(original.call_args.kwargs['timeout'], 2)
            self.assertIs(subprocess.run, original)

    def test_expired_deadline_preserves_explicit_failure(self):
        guard = DiagnosticDeadline(1)
        guard.end = 0
        with patch('subprocess.run') as original, patch('signal.signal'), patch('signal.getsignal'):
            with guard:
                with self.assertRaisesRegex(RuntimeError, 'bounded_diagnostic_deadline'):
                    subprocess.run(['probe'], timeout=30)
            original.assert_not_called()

    def test_term_unwinds_work_and_allows_bounded_cleanup(self):
        guard = DiagnosticDeadline(1)
        guard.end = 0
        with self.assertRaisesRegex(RuntimeError, 'bounded_diagnostic_deadline'):
            guard.terminate(signal.SIGTERM, None)
        with guard.cleanup():
            guard.terminate(signal.SIGTERM, None)
            self.assertGreater(guard.check(), 0)
            self.assertLessEqual(guard.check(), 25)
        self.assertEqual(guard.signals, 2)
        with self.assertRaisesRegex(RuntimeError, 'bounded_diagnostic_deadline'):
            guard.check()

    def test_invalid_deadline_rejected(self):
        for value in [0, -1, float('inf'), float('nan')]:
            with self.subTest(value=value), self.assertRaisesRegex(RuntimeError, 'deadline_shape'):
                DiagnosticDeadline(value)
