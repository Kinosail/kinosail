"""Written-first deterministic budget controls for the live-empty E2E gap."""
from contextlib import ExitStack
import os
from pathlib import Path
from types import SimpleNamespace
import tempfile
import time
import unittest
from unittest.mock import patch
import hls_aac_v2_live_observer as live
from hls_aac_v2_source_identity import regular_identity


class EmptyBudgetControls(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix='kinosail-empty-budget-')
        self.addCleanup(self.directory.cleanup)
        self.source = Path(self.directory.name) / 'Fixture.mp4'
        self.source.write_bytes(b'fixed-empty-budget-source')
        self.server = SimpleNamespace(pid=os.getpid(),
            _copiedSourceWitness=regular_identity(self.source))

    def rejected(self, clock, sleep):
        calls = 0

        def empty(pid):
            nonlocal calls
            self.assertEqual(pid, self.server.pid + 1)
            calls += 1
            self.assertLessEqual(calls, 33)
            error = RuntimeError(live.FAILURE)
            error.observer_argument_shape = 'empty'
            error.observer_argument_bytes = 0
            raise error

        with ExitStack() as stack:
            stack.enter_context(patch.object(live, 'owned_children',
                return_value=[self.server.pid + 1]))
            stack.enter_context(patch.object(live, 'process_identity', return_value=('R', 7)))
            stack.enter_context(patch.object(live, 'actual_arguments', side_effect=empty))
            stack.enter_context(patch.object(time, 'monotonic', side_effect=clock))
            sleeping = stack.enter_context(patch.object(time, 'sleep', side_effect=sleep))
            with self.assertRaisesRegex(RuntimeError, '^' + live.FAILURE + '$') as caught:
                live.owned_hls_count(self.server, self.source)
        self.assertEqual(caught.exception.observer_stage, 'arguments_before')
        self.assertEqual(caught.exception.observer_exception_class, 'RuntimeError')
        self.assertEqual(getattr(caught.exception, 'observer_argument_attempts', None), calls)
        return calls, sleeping.call_count

    def test_empty_budget_caps_total_reads_without_reset(self):
        calls, _ = self.rejected(lambda: 0.0, lambda delay: None)
        self.assertEqual(calls, 33)

    def test_expired_empty_budget_stops_before_additional_read(self):
        calls = 0

        def clock():
            nonlocal calls
            calls += 1
            return 0.0 if calls == 1 else 0.021

        attempts, sleeps = self.rejected(clock, lambda delay: self.fail('expired_sleep'))
        self.assertEqual((attempts, sleeps), (1, 0))

    def test_empty_budget_rechecks_deadline_after_sleep(self):
        now = 0.0

        def sleep(delay):
            nonlocal now
            self.assertGreater(delay, 0)
            self.assertLessEqual(delay, 0.001)
            now = 0.021

        attempts, sleeps = self.rejected(lambda: now, sleep)
        self.assertEqual((attempts, sleeps), (2, 1))

    def test_last_budget_read_still_requires_original_input_qualification(self):
        other = self.source.with_name('Other.mp4')
        other.write_bytes(self.source.read_bytes())
        calls = 0

        def arguments(pid):
            nonlocal calls
            self.assertEqual(pid, self.server.pid + 1)
            calls += 1
            self.assertLessEqual(calls, 33)
            if calls == 33:
                return ['-i', str(other), '-hls_time', '0.1']
            error = RuntimeError(live.FAILURE)
            error.observer_argument_shape = 'empty'
            error.observer_argument_bytes = 0
            raise error

        with ExitStack() as stack:
            stack.enter_context(patch.object(live, 'owned_children',
                return_value=[self.server.pid + 1]))
            stack.enter_context(patch.object(live, 'process_identity', return_value=('R', 7)))
            stack.enter_context(patch.object(live, 'actual_arguments', side_effect=arguments))
            stack.enter_context(patch.object(time, 'monotonic', return_value=0.0))
            stack.enter_context(patch.object(time, 'sleep', return_value=None))
            classify = stack.enter_context(patch.object(live, 'input_classification',
                wraps=live.input_classification))
            with self.assertRaisesRegex(RuntimeError, '^' + live.FAILURE + '$') as caught:
                live.owned_hls_count(self.server, self.source)
        self.assertEqual(calls, 33)
        self.assertEqual(classify.call_count, 1)
        self.assertEqual(caught.exception.observer_stage, 'input_before')

    def fresh_read_rejection(self, shape, late):
        args = ['-i', str(self.source), '-hls_time', '0.1']
        calls, now = 0, 0.0

        def arguments(pid):
            nonlocal calls, now
            self.assertEqual(pid, self.server.pid + 1)
            calls += 1
            self.assertLessEqual(calls, 3)
            if calls == 1:
                error = RuntimeError(live.FAILURE)
                error.observer_argument_shape = shape
                error.observer_argument_bytes = 0
                raise error
            if late:
                now = 0.021
            return args

        with ExitStack() as stack:
            stack.enter_context(patch.object(live, 'owned_children',
                return_value=[self.server.pid + 1]))
            stack.enter_context(patch.object(live, 'process_identity', return_value=('R', 7)))
            stack.enter_context(patch.object(time, 'monotonic', side_effect=lambda: now))
            stack.enter_context(patch.object(time, 'sleep', return_value=None))
            with patch.object(live, 'actual_arguments', return_value=args):
                self.assertEqual(live.owned_hls_count(self.server, self.source), 1)
            classify = stack.enter_context(patch.object(live, 'input_classification',
                wraps=live.input_classification))
            stack.enter_context(patch.object(live, 'actual_arguments', side_effect=arguments))
            with self.assertRaisesRegex(RuntimeError, '^' + live.FAILURE + '$') as caught:
                live.owned_hls_count(self.server, self.source)
        self.assertEqual(calls, 2 if late else 1)
        self.assertEqual(classify.call_count, 0)
        self.assertEqual(caught.exception.observer_stage, 'arguments_before')
        if late:
            self.assertEqual(caught.exception.observer_argument_attempts, 2)

    def test_successful_fresh_read_after_deadline_is_rejected(self):
        self.fresh_read_rejection('empty', True)

    def test_nonexact_empty_shape_does_not_enter_retry(self):
        class EmptyShape(str):
            pass

        self.fresh_read_rejection(EmptyShape('empty'), False)

    def test_retry_attempt_details_preserve_exact_bounded_integer(self):
        for attempts in [0, 1, 33]:
            with self.subTest(attempts=attempts):
                error = RuntimeError(live.FAILURE)
                error.observer_argument_attempts = attempts
                normalized = live.qualified_failure('child',
                    live.qualified_failure('arguments_after', error))
                self.assertEqual(normalized.observer_stage, 'arguments_after')
                self.assertEqual(live.observation_details(normalized).get('argumentAttempts'), attempts)

    def test_retry_attempt_details_reject_untrusted_values(self):
        for attempts in [True, -1, 34, 'synthetic_private_target']:
            with self.subTest(valueType=type(attempts).__name__):
                error = RuntimeError(live.FAILURE)
                error.observer_argument_attempts = attempts
                self.assertIsNone(live.observation_details(error).get('argumentAttempts'))


if __name__ == '__main__':
    unittest.main()
