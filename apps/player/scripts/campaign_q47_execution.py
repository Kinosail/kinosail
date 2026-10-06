#!/usr/bin/env python3
"""Owned process groups only; bounded captures stay private and are not artifacts."""
import os
import selectors
import signal
import subprocess
import time

CAPTURE_LIMIT = 16 * 1024 * 1024
CLEANUP_SECONDS = 7
PROCESS_RECORDS = []
PROOF_DEADLINE = None


def check_budget():
    if PROOF_DEADLINE is not None and time.monotonic() >= PROOF_DEADLINE:
        raise ValueError("campaign_budget")


def environment():
    keys = ("PATH", "HOME", "RUNNER_TEMP", "TMPDIR", "LANG", "LC_ALL", "TZ", "GEM_HOME", "GEM_PATH",
            "BUNDLE_PATH", "BUNDLE_GEMFILE", "BUNDLE_BIN", "PLAYWRIGHT_BROWSERS_PATH", "LD_LIBRARY_PATH")
    env = {key: os.environ[key] for key in keys if key in os.environ}
    env.update(CI="1", GOMAXPROCS="2", GOTOOLCHAIN="local", GOWORK="off", GO111MODULE="off",
               CGO_ENABLED="0", GOPROXY="off", BUNDLE_FROZEN="true", BUNDLE_DEPLOYMENT="true",
               PYTHONDONTWRITEBYTECODE="1")
    return env


def group_present(pid):
    try:
        os.killpg(pid, 0)
        return True
    except ProcessLookupError:
        return False
    except OSError:
        return None


def signal_owned(pid, signum):
    try:
        os.killpg(pid, signum)
        return True
    except ProcessLookupError:
        return False
    except OSError:
        return None


def settle_group(pid):
    for signum in (signal.SIGTERM, signal.SIGKILL):
        if signal_owned(pid, signum) is None:
            return False
        deadline = time.monotonic() + 2
        while time.monotonic() < deadline:
            state = group_present(pid)
            if state is False:
                return True
            if state is None:
                return False
            time.sleep(0.025)
    return group_present(pid) is False


def execute(command, bound, cwd, env, hook=None, label="owned-command"):
    started = time.monotonic()
    deadline = min(started + bound, PROOF_DEADLINE) if PROOF_DEADLINE is not None else started + bound
    result = {"launched": False, "exitCode": None, "durationMs": 0, "stopReason": None, "leaderExited": False,
              "groupAbsent": False, "captureComplete": False, "captureBytes": 0, "captureLimit": CAPTURE_LIMIT}
    data = bytearray()
    process = None
    eof = False
    interrupted = False
    previous = {}
    def handle(_number, _frame):
        nonlocal interrupted
        interrupted = True
    try:
        for number in (signal.SIGINT, signal.SIGTERM):
            previous[number] = signal.getsignal(number)
            signal.signal(number, handle)
        check_budget()
        process = subprocess.Popen(command, cwd=cwd, env=env, stdout=subprocess.PIPE,
                                   stderr=subprocess.STDOUT, start_new_session=True, stdin=subprocess.DEVNULL)
        result["launched"] = True
        os.set_blocking(process.stdout.fileno(), False)
        with selectors.DefaultSelector() as selector:
            selector.register(process.stdout, selectors.EVENT_READ)
            while True:
                if interrupted:
                    result["stopReason"] = "interrupted"; break
                if time.monotonic() >= deadline:
                    result["stopReason"] = "timeout"; break
                if hook is not None and not hook():
                    result["stopReason"] = "peer-prerequisite"; break
                for _key, _mask in selector.select(min(0.1, max(0, deadline - time.monotonic()))):
                    chunk = os.read(process.stdout.fileno(), 65536)
                    if not chunk:
                        eof = True
                        selector.unregister(process.stdout)
                        break
                    result["captureBytes"] += len(chunk)
                    if result["captureBytes"] > CAPTURE_LIMIT:
                        result["stopReason"] = "capture-bound"; break
                    data.extend(chunk)
                if result["stopReason"]:
                    break
                if eof and process.poll() is not None:
                    result["exitCode"] = process.returncode
                    break
    except (OSError, ValueError, RuntimeError):
        result["stopReason"] = "launch-or-capture-prerequisite"
    finally:
        if process is not None:
            if process.poll() is None or result["stopReason"] is not None:
                signal_owned(process.pid, signal.SIGKILL)
            try:
                result["exitCode"] = process.wait(timeout=3)
                result["leaderExited"] = True
            except subprocess.TimeoutExpired:
                result["stopReason"] = "leader-unsettled"
            result["groupAbsent"] = settle_group(process.pid)
            result["captureComplete"] = eof and result["captureBytes"] <= CAPTURE_LIMIT
            process.stdout.close()
        for number, handler in previous.items():
            signal.signal(number, handler)
        if interrupted:
            result["stopReason"] = "interrupted"
        result["durationMs"] = round((time.monotonic() - started) * 1000)
        PROCESS_RECORDS.append({"phase": label, "boundSeconds": bound, **result})
    return result, bytes(data)


def complete(result, exit_code):
    keys = ["launched", "exitCode", "durationMs", "stopReason", "leaderExited", "groupAbsent",
            "captureComplete", "captureBytes", "captureLimit"]
    return (type(result) is dict and set(result) == set(keys) and result["launched"] is True
            and type(result["exitCode"]) is int and result["exitCode"] == exit_code and result["stopReason"] is None
            and type(result["durationMs"]) is int and 0 <= result["durationMs"] <= 600_000
            and type(result["captureBytes"]) is int and 0 <= result["captureBytes"] <= CAPTURE_LIMIT
            and type(result["captureLimit"]) is int and result["captureLimit"] == CAPTURE_LIMIT
            and result["leaderExited"] is True and result["groupAbsent"] is True and result["captureComplete"] is True)


class Peer:
    """Fresh actual Go peer per repeat, max 105 seconds including normal shutdown."""
    def __init__(self, binary, site, cwd):
        self.binary, self.site, self.cwd = binary, site, cwd
        self.process = None
        self.started = 0.0
        self.buffer = bytearray()
        self.markers = []
        self.eof = False
        self.bad = False
        self.total = 0
        self.requested_stop = False

    def pump(self):
        if self.process is None:
            return False
        while not self.eof:
            try:
                chunk = os.read(self.process.stdout.fileno(), 256)
            except BlockingIOError:
                break
            except OSError:
                self.bad = True; break
            if not chunk:
                self.eof = True; break
            self.total += len(chunk)
            if self.total > 1024:
                self.bad = True; break
            self.buffer.extend(chunk)
            while b"\n" in self.buffer:
                line, _, remaining = self.buffer.partition(b"\n")
                self.buffer = bytearray(remaining)
                expected = (b'{"schemaVersion":1,"kind":"q47-ready","ready":true}' if len(self.markers) == 0
                            else b'{"schemaVersion":1,"kind":"q47-stopped","stopped":true}')
                if len(self.markers) >= 2 or line != expected:
                    self.bad = True; break
                self.markers.append("ready" if not self.markers else "stopped")
        return not self.bad

    def start(self):
        check_budget()
        self.started = time.monotonic()
        env = environment() | {"KINOSAIL_Q47_SITE": str(self.site)}
        self.process = subprocess.Popen([str(self.binary)], cwd=self.cwd, env=env, stdin=subprocess.DEVNULL,
                                        stdout=subprocess.PIPE, stderr=subprocess.STDOUT, start_new_session=True)
        os.set_blocking(self.process.stdout.fileno(), False)
        deadline = self.started + 5
        while time.monotonic() < deadline:
            if not self.pump() or self.process.poll() is not None:
                return False
            if self.markers == ["ready"]:
                return True
            time.sleep(0.025)
        return False

    def live(self):
        return (self.process is not None and time.monotonic() - self.started < 90
                and self.pump() and self.markers == ["ready"] and self.process.poll() is None)

    def stop(self):
        previous = {number: signal.getsignal(number) for number in (signal.SIGINT, signal.SIGTERM)}
        interrupted = False
        def handle(_number, _frame):
            nonlocal interrupted
            interrupted = True
        try:
            for number in previous:
                signal.signal(number, handle)
            receipt = self._stop()
        finally:
            for number, handler in previous.items():
                signal.signal(number, handler)
        receipt["interrupted"] = interrupted
        PROCESS_RECORDS.append({"phase": "fixture-peer", "boundSeconds": 105, **receipt})
        return receipt

    def _stop(self):
        if self.process is None:
            return {"ready": False, "stopped": False, "requestedStop": False, "exitCode": None,
                    "leaderExited": True, "groupAbsent": True, "captureComplete": True, "durationMs": 0}
        self.requested_stop = self.process.poll() is None and signal_owned(self.process.pid, signal.SIGTERM) is True
        deadline = min(self.started + 100, time.monotonic() + 6)
        while time.monotonic() < deadline:
            self.pump()
            if self.eof and self.process.poll() is not None:
                break
            time.sleep(0.025)
        if self.process.poll() is None:
            signal_owned(self.process.pid, signal.SIGKILL)
        exited = False
        try:
            self.process.wait(timeout=1)
            exited = True
        except subprocess.TimeoutExpired:
            pass
        absent = settle_group(self.process.pid)
        self.pump()
        self.process.stdout.close()
        return {"ready": self.markers[:1] == ["ready"], "stopped": self.markers == ["ready", "stopped"],
                "requestedStop": self.requested_stop, "exitCode": self.process.returncode,
                "leaderExited": exited, "groupAbsent": absent,
                "captureComplete": self.eof and not self.bad and not self.buffer,
                "durationMs": round((time.monotonic() - self.started) * 1000)}
