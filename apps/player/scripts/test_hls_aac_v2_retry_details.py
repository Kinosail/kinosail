"""Written-first bounded retry diagnostics; rejection semantics stay strict."""
from contextlib import ExitStack
import time
import unittest
from unittest.mock import patch
import hls_aac_v2_live_observer as live
import test_hls_aac_v2_empty_budget as budget


class RetryDetailControls(unittest.TestCase):
    setUp = budget.EmptyBudgetControls.setUp

    def failure(self, clock, terminal, fresh=None):
        calls = 0

        def arguments(pid):
            nonlocal calls
            self.assertEqual(pid, self.server.pid + 1)
            calls += 1
            self.assertLessEqual(calls, 33)
            if calls > 1 and fresh is not None:
                return fresh()
            error = RuntimeError(live.FAILURE)
            error.observer_argument_shape = 'empty'
            error.observer_argument_bytes = 0
            raise error

        with ExitStack() as stack:
            stack.enter_context(patch.object(live, 'owned_children',
                return_value=[self.server.pid + 1]))
            stack.enter_context(patch.object(live, 'process_identity', return_value=('R', 7)))
            stack.enter_context(patch.object(live, 'actual_arguments', side_effect=arguments))
            stack.enter_context(patch.object(live, 'child_ended', side_effect=terminal))
            stack.enter_context(patch.object(time, 'monotonic', side_effect=clock))
            stack.enter_context(patch.object(time, 'sleep', return_value=None))
            with self.assertRaisesRegex(RuntimeError, '^' + live.FAILURE + '$') as caught:
                live.owned_hls_count(self.server, self.source)
        self.assertEqual(caught.exception.observer_stage, 'arguments_before')
        self.assertEqual(caught.exception.observer_argument_attempts, calls)
        return live.observation_details(caught.exception), calls

    def assert_details(self, details, outcome, proof):
        self.assertEqual(details.get('retryOutcome'), outcome)
        self.assertIs(details.get('terminalProofCompleted'), proof)

    def test_live_empty_read_limit_preserves_nonterminal_result(self):
        details, calls = self.failure(lambda: 0.0, lambda *args: False)
        self.assertEqual(calls, 33)
        self.assert_details(details, 'read-limit', False)

    def test_deadline_before_terminal_has_no_completed_proof(self):
        calls = 0

        def clock():
            nonlocal calls
            calls += 1
            return 0.0 if calls == 1 else 0.021

        details, attempts = self.failure(clock,
            lambda *args: self.fail('expired_terminal_check'))
        self.assertEqual(attempts, 1)
        self.assert_details(details, 'deadline-before-terminal', None)

    def test_completed_terminal_proof_after_deadline_remains_rejected(self):
        now, checked = 0.0, False

        def terminal(*args):
            nonlocal now, checked
            now, checked = 0.021, True
            return True

        details, attempts = self.failure(lambda: now, terminal)
        self.assertIs(checked, True)
        self.assertEqual(attempts, 1)
        self.assert_details(details, 'deadline-after-terminal', True)

    def test_terminal_check_rejection_has_no_completed_proof(self):
        checked = False

        def terminal(pid, parent, before, observation):
            nonlocal checked
            checked = True
            observation.observer_after_state = 'absent'
            raise RuntimeError(live.FAILURE)

        details, attempts = self.failure(lambda: 0.0, terminal)
        self.assertIs(checked, True)
        self.assertEqual(attempts, 1)
        self.assertEqual(details['afterState'], 'absent')
        self.assert_details(details, 'terminal-check-error', None)

    def test_late_fresh_arguments_keep_completed_nonterminal_result(self):
        now = 0.0

        def fresh():
            nonlocal now
            now = 0.021
            return ['-i', str(self.source), '-hls_time', '0.1']

        details, attempts = self.failure(lambda: now, lambda *args: False, fresh)
        self.assertEqual(attempts, 2)
        self.assert_details(details, 'deadline-after-arguments', False)

    def test_fresh_argument_error_is_distinct_from_terminal_error(self):
        def fresh():
            raise ValueError('synthetic_private_target')

        details, attempts = self.failure(lambda: 0.0, lambda *args: False, fresh)
        self.assertEqual(attempts, 2)
        self.assertEqual(details['exceptionClass'], 'ValueError')
        self.assert_details(details, 'fresh-argument-error', False)

    def test_retry_diagnostics_survive_both_normalizations(self):
        for outcome in ['deadline-before-terminal', 'deadline-after-terminal',
                'deadline-after-arguments', 'deadline-before-sleep', 'read-limit',
                'terminal-check-error', 'fresh-argument-error', 'other']:
            for proof in [True, False, None]:
                with self.subTest(outcome=outcome, proof=proof):
                    error = RuntimeError(live.FAILURE)
                    error.observer_retry_outcome = outcome
                    error.observer_terminal_proof_completed = proof
                    normalized = live.qualified_failure('child',
                        live.qualified_failure('arguments_after', error))
                    self.assert_details(live.observation_details(normalized), outcome, proof)

    def test_retry_diagnostics_reject_untrusted_values(self):
        class Outcome(str):
            pass

        for outcome in ['synthetic_private_target', Outcome('read-limit'), [], None]:
            for proof in [1, 'true', [], object()]:
                with self.subTest(outcomeType=type(outcome).__name__, proofType=type(proof).__name__):
                    error = RuntimeError('synthetic_private_target')
                    error.observer_retry_outcome = outcome
                    error.observer_terminal_proof_completed = proof
                    details = live.observation_details(live.qualified_failure('child', error))
                    self.assert_details(details, 'other', None)
                    self.assertNotIn('synthetic_private_target', str(details))


if __name__ == '__main__':
    unittest.main()
