"""Bounded empty-record controls sharing the actual owned codec fixture."""
from unittest.mock import patch
import hls_aac_v2_live_observer as live
from hls_followon_public import check, encoder_count


class EmptyArgumentObserverControls:
    def empty_observation(self):
        error = RuntimeError(live.FAILURE)
        error.observer_argument_bytes = 0
        error.observer_argument_shape = 'empty'
        return error

    def delayed_terminal_count(self, phase, reap=False):
        process = self.spawn(self.alias(self.retained[0]))
        actual, identity = live.actual_arguments, live.process_identity
        ending = self.ending_arguments(process, reap)
        calls, delayed = 0, False

        def arguments(pid):
            nonlocal calls
            calls += 1
            if calls == (1 if phase == 'before' else 2):
                return ending(pid)
            return actual(pid)

        def intermediate(pid, parent):
            nonlocal delayed
            state, start = identity(pid, parent)
            if self.row.get('actualLiveToTerminalTransition') and not reap and not delayed:
                delayed = True
                self.row['injectedIntermediateNonterminalState'] = True
                return 'R', start
            return state, start

        with patch.object(live, 'actual_arguments', side_effect=arguments):
            with patch.object(live, 'process_identity', side_effect=intermediate):
                self.assertEqual(encoder_count(self.server, self.source), 0)
        self.qualified_terminal_transition(reap)
        if not reap:
            self.assertIs(self.row.get('injectedIntermediateNonterminalState'), True)
        self.row.update(observedOwnedHLSCount=0, result='observed')

    def test_initial_empty_record_reobserves_same_terminal_child(self):
        self.delayed_terminal_count('before')

    def test_second_empty_record_reobserves_same_terminal_child(self):
        self.delayed_terminal_count('after')

    def test_second_empty_record_reobserves_reaped_child(self):
        self.delayed_terminal_count('after', True)

    def transient_arguments(self, process):
        actual = live.actual_arguments
        calls = 0

        def arguments(pid):
            nonlocal calls
            check(pid == process.pid, 'fd_live_reobservation_exact_child')
            self.alive(process)
            calls += 1
            if calls == 1:
                self.row['injectedEmptyObservation'] = True
                raise self.empty_observation()
            args = actual(pid)
            self.row['actualArgumentsReobserved'] = True
            return args

        return arguments

    def test_empty_observation_reobserves_live_actual_source_arguments(self):
        process = self.spawn(self.alias(self.retained[0]))
        self.assertEqual(self.observe(process), 1)
        with patch.object(live, 'actual_arguments', side_effect=self.transient_arguments(process)):
            self.assertEqual(encoder_count(self.server, self.source), 1)
        self.assertIs(self.row.get('actualArgumentsReobserved'), True)
        self.assertEqual(self.observe(process), 1)
        self.row.update(actualSourceReobservationQualified=True, result='observed')

    def test_empty_observation_cannot_admit_wrong_byte_identical_fd(self):
        process = self.spawn(self.alias(self.other()))
        with patch.object(live, 'actual_arguments', side_effect=self.transient_arguments(process)):
            with self.assertRaisesRegex(RuntimeError, '^' + live.FAILURE + '$'):
                encoder_count(self.server, self.source)
        self.assertIs(self.row.get('actualArgumentsReobserved'), True)
        self.alive(process)
        self.row.update(actualWrongSourceReobservationRejected=True, result='observed')

    def test_persistent_empty_live_record_remains_bounded_rejection(self):
        process = self.spawn(self.alias(self.retained[0]))
        self.assertEqual(self.observe(process), 1)
        calls = 0

        def arguments(pid):
            nonlocal calls
            check(pid == process.pid, 'fd_live_persistent_exact_child')
            self.alive(process)
            calls += 1
            raise self.empty_observation()

        with patch.object(live, 'actual_arguments', side_effect=arguments):
            with self.assertRaisesRegex(RuntimeError, '^' + live.FAILURE + '$'):
                encoder_count(self.server, self.source)
        self.assertTrue(1 <= calls <= 33)
        self.assertEqual(self.observe(process), 1)
        self.row.update(injectedEmptyObservationAttempts=calls,
            actualPersistentLiveEmptyRejected=True, result='observed')

    def test_empty_reobservation_rejects_changed_start_before_admission(self):
        process = self.spawn(self.alias(self.retained[0]))
        identity = live.process_identity

        def changed(pid, parent):
            state, start = identity(pid, parent)
            if self.row.get('injectedEmptyObservation'):
                self.row['injectedChangedStartObserved'] = True
                return state, start + 1
            return state, start

        with patch.object(live, 'actual_arguments', side_effect=self.transient_arguments(process)):
            with patch.object(live, 'process_identity', side_effect=changed):
                with self.assertRaisesRegex(RuntimeError, '^' + live.FAILURE + '$'):
                    encoder_count(self.server, self.source)
        self.assertIs(self.row.get('injectedChangedStartObserved'), True)
        self.alive(process)
        self.row.update(changedReobservationStartRejected=True, result='observed')
