"""Bounded empty-record reobservation; diagnostics never change admission."""
import subprocess


class _EmptyArgumentFailure(RuntimeError):
    """Internal marker for rejection after a bounded retry episode."""


def empty_argument_record(error):
    size = getattr(error, 'observer_argument_bytes', None)
    shape = getattr(error, 'observer_argument_shape', None)
    return (type(error) is RuntimeError and type(size) is int and size == 0
        and type(shape) is str and shape == 'empty')


def reobserve_arguments(owner, pid, parent, before, error):
    attempts = 1
    deadline = owner.time.monotonic() + 0.02
    outcome, proof = 'read-limit', None
    try:
        for retry in range(32):
            if owner.time.monotonic() >= deadline:
                outcome = 'deadline-before-terminal'
                break
            outcome, proof = 'terminal-check-error', None
            ended = owner.child_ended(pid, parent, before, error)
            proof = ended if type(ended) is bool else None
            if owner.time.monotonic() >= deadline:
                outcome = 'deadline-after-terminal'
                break
            if ended:
                return None
            attempts += 1
            outcome = 'fresh-argument-error'
            try:
                args = owner.actual_arguments(pid)
                current = owner.time.monotonic()
                if current >= deadline:
                    outcome = 'deadline-after-arguments'
                owner.check(current < deadline, owner.FAILURE)
                return args
            except RuntimeError as fresh:
                if not empty_argument_record(fresh):
                    raise
                fresh.observer_after_state = getattr(error, 'observer_after_state', 'other')
                error = fresh
            outcome = 'read-limit'
            if retry < 31:
                if owner.time.monotonic() >= deadline:
                    outcome = 'deadline-before-sleep'
                    break
                owner.time.sleep(0.001)
        raise error
    except (OSError, subprocess.SubprocessError, RuntimeError, ValueError, IndexError, UnicodeError) as failure:
        if not hasattr(failure, 'observer_argument_shape'):
            owner.copy_observation_details(failure, error)
        failure.observer_after_state = getattr(error, 'observer_after_state', 'other')
        failure.observer_argument_attempts = attempts
        failure.observer_retry_outcome = outcome
        failure.observer_terminal_proof_completed = proof
        raise


def read_arguments(owner, pid, parent, before, stage):
    try:
        return owner.actual_arguments(pid)
    except RuntimeError as error:
        if not empty_argument_record(error):
            raise
        try:
            return reobserve_arguments(owner, pid, parent, before, error)
        except (OSError, subprocess.SubprocessError, RuntimeError, ValueError, IndexError, UnicodeError) as failure:
            handled = _EmptyArgumentFailure(owner.FAILURE)
            handled.__dict__.update(owner.qualified_failure(stage, failure).__dict__)
            raise handled from None
