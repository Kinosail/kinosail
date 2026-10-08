"""Internal probe deadline and handled cutoff; owned cleanup gets bounded grace."""
from contextlib import contextmanager
import math
import signal
import subprocess
import time


class DiagnosticDeadline:
    def __init__(self, seconds):
        if not math.isfinite(seconds) or seconds <= 0:
            raise RuntimeError('diagnostic_deadline_shape')
        self.end = time.monotonic() + seconds
        self.cleanup_end = None
        self.signals = 0

    def check(self, reserve=0):
        remaining = (self.cleanup_end if self.cleanup_end is not None else self.end) - time.monotonic()
        if remaining <= reserve:
            raise RuntimeError('bounded_diagnostic_deadline')
        return remaining

    def terminate(self, _signum, _frame):
        self.signals += 1
        self.end = 0
        if self.cleanup_end is None:
            raise RuntimeError('bounded_diagnostic_deadline')

    def __enter__(self):
        self.previous_run = subprocess.run
        self.previous_signal = signal.getsignal(signal.SIGTERM)
        def run(*args, **kwargs):
            budget = self.check()
            timeout = kwargs.get('timeout')
            kwargs['timeout'] = min(float(timeout), budget) if timeout is not None else budget
            return self.previous_run(*args, **kwargs)
        subprocess.run = run
        signal.signal(signal.SIGTERM, self.terminate)
        return self

    def __exit__(self, *_args):
        subprocess.run = self.previous_run
        signal.signal(signal.SIGTERM, self.previous_signal)

    @contextmanager
    def cleanup(self):
        previous = self.cleanup_end
        self.cleanup_end = time.monotonic() + 25
        try:
            yield
        finally:
            self.cleanup_end = previous
