"""One owned process group, bounded capture and one absolute cleanup reserve."""
import math
import os
import selectors
import signal
import subprocess
import time

CAP, LINE_CAP = 16 * 1024 * 1024, 65536


def accepted(value, exits=(0,)):
    return (type(value.get("exitCode")) is int and value["exitCode"] in exits and
            value.get("stopReason") is None and value.get("ownedProcessExited") is True and
            value.get("ownedGroupSettled") is True and value.get("captureSettled") is True and value.get("handlersRestored") is True)


def group_state(pid):
    try: os.killpg(pid, 0); return "present"
    except ProcessLookupError: return "absent"
    except OSError: return "unconfirmed"


def kill_group(pid):
    try: os.killpg(pid, signal.SIGKILL); return True
    except ProcessLookupError: return True
    except OSError: return False


def execute(command, active, total_bound, projection, receipt, cwd):
    if (type(active) not in (int, float) or type(total_bound) not in (int, float) or
            not math.isfinite(active) or not math.isfinite(total_bound) or active <= 0 or total_bound != active + 5 or
            type(command) is not list or not command or
            any(type(part) is not str or not part or "\0" in part for part in command)):
        raise ValueError("process-selection")
    began = time.monotonic()
    active_end, final_end = began + active, began + total_bound
    child, selector, eof, pending, count = None, None, False, b"", 0
    reason, close_ok = None, True
    previous = {s: signal.getsignal(s) for s in (signal.SIGINT, signal.SIGTERM)}
    def interrupted(_signum, _frame):
        nonlocal reason
        reason = "interrupted"
    receipt.update(activeSeconds=active, totalBoundSeconds=total_bound, processLaunched=False,
                   exitCode=None, ownedProcessExited=False, ownedGroupSettled=False, captureSettled=False)
    try:
        for signum in previous: signal.signal(signum, interrupted)
        environment = dict(os.environ, GOMAXPROCS="2", GOPROXY="off", GOTOOLCHAIN="local")
        child = subprocess.Popen(command, cwd=cwd, env=environment, stdout=subprocess.PIPE,
                                 stderr=subprocess.STDOUT, start_new_session=True)
        receipt["processLaunched"] = True
        os.set_blocking(child.stdout.fileno(), False)
        selector = selectors.DefaultSelector()
        selector.register(child.stdout, selectors.EVENT_READ)
        while reason is None and not eof:
            remaining = active_end - time.monotonic()
            if remaining <= 0: reason = "active-timeout"; break
            if not selector.select(min(remaining, 0.25)): continue
            block = os.read(child.stdout.fileno(), 65536)
            if not block:
                eof = True
                if pending and projection is not None: projection.consume(pending)
                pending = b""
                break
            count += len(block)
            if count > CAP: reason = "output-bound"; break
            if projection is not None and getattr(projection, "binary", False) is True:
                projection.consume(block)
                continue
            pending += block
            while b"\n" in pending:
                line, _, pending = pending.partition(b"\n")
                if len(line) > LINE_CAP: reason = "output-bound"; break
                if projection is not None: projection.consume(line)
            if len(pending) > LINE_CAP: reason = "output-bound"
        while eof and reason is None:
            remaining = active_end - time.monotonic()
            if remaining <= 0: reason = "active-timeout"; break
            try: child.wait(timeout=min(remaining, 0.25)); break
            except subprocess.TimeoutExpired: pass
    except Exception:
        reason = reason or "process-boundary"
    finally:
        if child is not None:
            try: live = child.poll() is None
            except Exception: live = True; reason = reason or "leader-poll"
            if reason is not None or live:
                if not kill_group(child.pid): reason = reason or "group-stop"
            remaining = max(0, final_end - time.monotonic())
            try:
                child.wait(timeout=remaining)
                receipt["ownedProcessExited"] = True
            except Exception: reason = reason or "leader-unsettled"
            while receipt["ownedProcessExited"] and time.monotonic() < final_end:
                state = group_state(child.pid)
                if state == "absent": receipt["ownedGroupSettled"] = True; break
                if state == "unconfirmed": break
                kill_group(child.pid)
                time.sleep(min(0.025, max(0, final_end - time.monotonic())))
            if not receipt["ownedGroupSettled"]: reason = reason or "group-unsettled"
            receipt["exitCode"] = child.returncode
            try: child.stdout.close()
            except Exception: close_ok = False
        if selector is not None:
            try: selector.close()
            except (OSError, ValueError): close_ok = False
        receipt["captureSettled"] = eof and not pending and close_ok
        if not close_ok: reason = reason or "capture-close"
        if time.monotonic() > final_end: reason = reason or "settlement-timeout"
        receipt["handlersRestored"] = True
        for signum, handler in previous.items():
            try: signal.signal(signum, handler)
            except Exception:
                receipt["handlersRestored"] = False
                reason = reason or "handler-restore"
        receipt.update(stopReason=reason, durationSeconds=round(time.monotonic() - began, 3), capturedBytes=count)
