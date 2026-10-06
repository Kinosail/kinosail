"""Bound and settle only owned hosted subprocess groups; export no raw output."""
import os
import selectors
import signal
import subprocess
import time
from campaign_r06_sources import APP, ROOT

def signal_owned(pid, signum):
    try:
        os.killpg(pid, signum)
        return "present", None
    except ProcessLookupError:
        return "absent", None
    except OSError as error:
        return "unconfirmed", type(error).__name__


def settle_owned(pid):
    # The leader has already been reaped. Prove the whole owned group absent.
    for signum in (signal.SIGTERM, signal.SIGKILL):
        state, error = signal_owned(pid, 0)
        if state != "present":
            return state == "absent", error
        state, error = signal_owned(pid, signum)
        if state != "present":
            return state == "absent", error
        limit = time.monotonic() + 2
        while time.monotonic() < limit:
            state, error = signal_owned(pid, 0)
            if state != "present":
                return state == "absent", error
            time.sleep(0.025)
    return False, "TimeoutExpired"


def stop_owned(process, receipt):
    # The leader may have exited while a descendant still holds stdout.
    _state, error = signal_owned(process.pid, signal.SIGKILL)
    if error:
        receipt["processStopFailureClass"] = error
        if process.poll() is None:
            try:
                process.kill()
            except ProcessLookupError:
                pass
            except OSError as failure:
                receipt["leaderStopFailureClass"] = type(failure).__name__


def execute(command, bound, projection, receipt, cwd=APP):
    environment = dict(os.environ, GOMAXPROCS="2", GOPROXY="off",
                       GOTOOLCHAIN="local", GOWORK=str(ROOT / "go.work"))
    process, selector, reason, total, pending = None, None, {"stop": None}, 0, b""
    eof = False
    started = time.monotonic()
    deadline = started + bound
    previous = {s: signal.getsignal(s) for s in (signal.SIGINT, signal.SIGTERM)}
    def interrupted(_signum, _frame):
        reason["stop"] = "interrupted"
    receipt.update({"exitCode": None, "ownedProcessExited": False, "ownedGroupSettled": False, "captureSettled": False})
    try:
        for signum in previous:
            signal.signal(signum, interrupted)
        process = subprocess.Popen(command, cwd=cwd, env=environment, stdout=subprocess.PIPE,
                                   stderr=subprocess.STDOUT, start_new_session=True)
        receipt["processLaunched"] = True
        os.set_blocking(process.stdout.fileno(), False)
        selector = selectors.DefaultSelector()
        selector.register(process.stdout, selectors.EVENT_READ)
        eof = False
        while not reason["stop"] and not eof:
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                reason["stop"] = "external-timeout"
                break
            if not selector.select(min(remaining, 0.25)):
                continue
            data = os.read(process.stdout.fileno(), 65536)
            if not data:
                eof = True
                if pending and projection is not None:
                    projection.consume(pending)
                    if projection.prerequisite:
                        reason["stop"] = "prerequisite-failure"
                pending = b""
                continue
            total += len(data)
            pending += data
            if total > 16 * 1024 * 1024:
                reason["stop"] = "output-bound"
                break
            while b"\n" in pending:
                line, _separator, pending = pending.partition(b"\n")
                if len(line) > 65536:
                    reason["stop"] = "output-bound"
                    break
                if projection is not None:
                    projection.consume(line)
                    if projection.prerequisite:
                        reason["stop"] = "prerequisite-failure"
                        break
            if len(pending) > 65536:
                reason["stop"] = "output-bound"
        # EOF is not proof of process exit. Preserve its natural result first.
        while eof and not reason["stop"]:
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                reason["stop"] = "external-timeout"
                break
            try:
                process.wait(timeout=min(remaining, 0.25))
                break
            except subprocess.TimeoutExpired:
                continue
    except OSError as error:
        receipt["processBoundaryFailureClass"] = type(error).__name__
        reason["stop"] = "process-boundary-failure"
    finally:
        if process:
            if reason["stop"] or process.returncode is None:
                stop_owned(process, receipt)
            try:
                process.wait(timeout=3)
                receipt["ownedProcessExited"] = True
            except subprocess.TimeoutExpired:
                receipt["settlementFailureClass"] = "TimeoutExpired"
                receipt["settlementFailurePhase"] = "leader-reap"
            if receipt["ownedProcessExited"]:
                settled, error = settle_owned(process.pid)
                receipt["ownedGroupSettled"] = settled
                if error:
                    receipt["settlementFailureClass"] = error
                    receipt["settlementFailurePhase"] = "owned-group"
            if not receipt["ownedGroupSettled"] and not reason["stop"]:
                reason["stop"] = "process-settlement-failure"
            receipt["exitCode"] = process.returncode
            process.stdout.close()
        if selector:
            selector.close()
        receipt["captureSettled"] = eof and not pending
        receipt.update({"seconds": round(time.monotonic() - started, 3), "stopReason": reason["stop"]})
        for signum, handler in previous.items():
            signal.signal(signum, handler)


def complete_process(execution):
    return (execution.get("exitCode") == 0 and not execution.get("stopReason")
            and execution.get("ownedProcessExited") and execution.get("ownedGroupSettled") and execution.get("captureSettled"))

