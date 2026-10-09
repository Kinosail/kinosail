"""Failure-first one absolute owned-process settlement reserve."""
import io
from types import SimpleNamespace
import unittest
from unittest.mock import patch
import campaign_r06_restore_runtime_process as process

class Pipe:
    def fileno(self): return 7
    def close(self): return None

class Child:
    pid = 12345
    def __init__(self, fail_close=False):
        self.stdout = Pipe()
        self.returncode = None
        if fail_close: self.stdout.close = lambda: (_ for _ in ()).throw(OSError("fictional close"))
    def wait(self, timeout=None): self.returncode = 0; return 0
    def poll(self): return self.returncode
    def kill(self): self.returncode = -9

class Selector:
    def register(self, *args): pass
    def select(self, timeout): return [(None, None)]
    def close(self): pass

class RestoreProcessControls(unittest.TestCase):
    def run_owned(self, chunks=(b"",), child=None, alive=False):
        child = child or Child(); receipt = {}
        clock = [0.0]; signals = []
        def now(): clock[0] += 0.05; return clock[0]
        def kill(pid, signum):
            signals.append((pid, signum))
            if signum == 0 and not alive: raise ProcessLookupError
        output = iter(chunks)
        def read(*args): return next(output, b"")
        with patch.object(process.subprocess, "Popen", return_value=child), \
             patch.object(process.selectors, "DefaultSelector", return_value=Selector()), \
             patch.object(process.os, "read", side_effect=read), \
             patch.object(process.os, "set_blocking"), \
             patch.object(process.os, "killpg", side_effect=kill), \
             patch.object(process.signal, "getsignal", return_value=None), \
             patch.object(process.signal, "signal"), \
             patch.object(process.time, "monotonic", side_effect=now), \
             patch.object(process.time, "sleep"):
            process.execute(["fictional-owned-command"], 1, 6, None, receipt, "/owned")
        return receipt, signals

    def test_natural_eof_preserves_zero_and_owned_settlement(self):
        value, signals = self.run_owned()
        self.assertTrue(process.accepted(value, (0,)))
        self.assertEqual(value["exitCode"], 0)
        self.assertTrue(value["captureSettled"])
        self.assertEqual(value["activeSeconds"], 1)
        self.assertEqual(value["totalBoundSeconds"], 6)

    def test_group_survives_leader_and_blocks_acceptance(self):
        value, signals = self.run_owned(alive=True)
        self.assertFalse(process.accepted(value, (0,)))
        self.assertFalse(value["ownedGroupSettled"])
        self.assertLessEqual(value["durationSeconds"], 6.2)

    def test_checked_capture_close_failure_blocks(self):
        value, _ = self.run_owned(child=Child(fail_close=True))
        self.assertFalse(process.accepted(value, (0,)))
        self.assertFalse(value["captureSettled"])

    def test_output_overflow_is_not_a_valid_partial_capture(self):
        value, _ = self.run_owned((b"x"*65537, b""))
        self.assertFalse(process.accepted(value, (0,)))
        self.assertEqual(value["stopReason"], "output-bound")

    def test_boolean_exit_unknown_stop_and_missing_settlement_block(self):
        good, _ = self.run_owned()
        for changes in ({"exitCode": True}, {"exitCode": 2}, {"stopReason": "interrupted"},
                        {"captureSettled": False}, {"ownedProcessExited": False}, {"ownedGroupSettled": False}):
            self.assertFalse(process.accepted({**good, **changes}, (0,1)))
        self.assertTrue(process.accepted({**good, "exitCode": 1}, (0,1)))

    def test_budget_has_exactly_one_five_second_reserve(self):
        for active, total in ((1, 5), (1, 8), (0, 5), (True, 6), (1, float("inf"))):
            with self.assertRaises(ValueError):
                process.execute(["fictional-owned-command"], active, total, None, {}, "/owned")

    def test_invalid_command_blocks_before_process_launch(self):
        for command in ([], "shell command", ["valid", None], ["contains\0nul"]):
            with patch.object(process.subprocess, "Popen") as launched, self.assertRaises(ValueError):
                process.execute(command, 1, 6, None, {}, "/owned")
            launched.assert_not_called()

    def exception_owned(self, failure):
        child = Child(); clock=[0.0]; handlers={}; restored=[]; killed=[]; closed=[]
        selector=Selector()
        child.stdout.close=lambda: closed.append("pipe")
        selector.close=lambda: closed.append("selector")
        original={process.signal.SIGINT:object(),process.signal.SIGTERM:object()}
        def now(): clock[0]+=0.05;return clock[0]
        def install(signum, handler):
            handlers[signum]=handler
            if handler is original[signum]: restored.append(signum)
        def select(_timeout):
            if failure=="select": raise RuntimeError("fictional select")
            if failure=="interrupt": handlers[process.signal.SIGTERM](process.signal.SIGTERM,None)
            return [(None,None)]
        selector.select=select
        def read(*_args):
            if failure=="read": raise OSError("fictional read")
            return b"one-line\n" if failure=="projection" else b""
        def wait(timeout=None):
            if failure=="wait" and not killed: raise RuntimeError("fictional wait")
            child.returncode=-9 if killed else 0; return child.returncode
        child.wait=wait
        def kill(pid, signum):
            if signum==0: raise ProcessLookupError
            killed.append((pid,signum));child.returncode=-9
        projector=SimpleNamespace(consume=lambda line: (_ for _ in ()).throw(ValueError("fictional projection")))
        receipt={}
        with patch.object(process.subprocess,"Popen",return_value=child), \
             patch.object(process.selectors,"DefaultSelector",return_value=selector), \
             patch.object(process.os,"read",side_effect=read), patch.object(process.os,"set_blocking"), \
             patch.object(process.os,"killpg",side_effect=kill), \
             patch.object(process.signal,"getsignal",side_effect=lambda signum:original[signum]), \
             patch.object(process.signal,"signal",side_effect=install), \
             patch.object(process.time,"monotonic",side_effect=now), patch.object(process.time,"sleep"):
            process.execute(["fictional-owned-command"],1,6,projector if failure=="projection" else None,receipt,"/owned")
        return receipt,killed,closed,restored,original

    def test_read_select_projection_and_wait_exceptions_settle_with_same_end(self):
        for failure in ("read","select","projection","wait"):
            value,killed,closed,restored,original=self.exception_owned(failure)
            self.assertFalse(process.accepted(value))
            self.assertTrue(killed)
            self.assertTrue(value["ownedProcessExited"])
            self.assertTrue(value["ownedGroupSettled"])
            self.assertEqual(sorted(closed),["pipe","selector"])
            self.assertEqual(set(restored),set(original))
            self.assertLessEqual(value["durationSeconds"],6.2)
            self.assertEqual(value["totalBoundSeconds"],6)
            self.assertIn(value["stopReason"],("process-boundary","interrupted"))

    def test_real_installed_interruption_callback_settles_and_restores(self):
        value,killed,closed,restored,original=self.exception_owned("interrupt")
        self.assertFalse(process.accepted(value))
        self.assertEqual(value["stopReason"],"interrupted")
        self.assertTrue(killed)
        self.assertTrue(value["ownedProcessExited"])
        self.assertTrue(value["ownedGroupSettled"])
        self.assertEqual(sorted(closed),["pipe","selector"])
        self.assertEqual(set(restored),set(original))
        self.assertLessEqual(value["durationSeconds"],6.2)

    def test_completed_natural_stream_and_tail_are_projected_once(self):
        lines=[]
        # Capture must consume the last non-newline tail at natural EOF.
        projector=SimpleNamespace(consume=lines.append)
        original=process.execute
        def invoke(command,active,total,projection,receipt,cwd):
            return original(command,active,total,projector,receipt,cwd)
        with patch.object(process,"execute",side_effect=invoke):
            value,_=self.run_owned((b"one\ntwo",b""))
        self.assertTrue(process.accepted(value))
        self.assertEqual(lines,[b"one",b"two"])

    def test_persistent_wait_and_capture_exception_still_restore_handlers(self):
        child=Child();receipt={};clock=[0.0];closed=[];restored=[];killed=[]
        child.wait=lambda timeout=None: (_ for _ in ()).throw(RuntimeError("fictional persistent wait"))
        child.stdout.close=lambda: closed.append("pipe")
        selector=Selector();selector.close=lambda: closed.append("selector")
        def now():clock[0]+=0.05;return clock[0]
        def kill(pid,signum):
            if signum==0:raise ProcessLookupError
            killed.append(signum)
        with patch.object(process.subprocess,"Popen",return_value=child), \
             patch.object(process.selectors,"DefaultSelector",return_value=selector), \
             patch.object(process.os,"read",return_value=b""),patch.object(process.os,"set_blocking"), \
             patch.object(process.os,"killpg",side_effect=kill), \
             patch.object(process.signal,"getsignal",return_value="prior"), \
             patch.object(process.signal,"signal",side_effect=lambda signum,handler: restored.append(signum) if handler=="prior" else None), \
             patch.object(process.time,"monotonic",side_effect=now),patch.object(process.time,"sleep"):
            process.execute(["fictional-owned-command"],1,6,None,receipt,"/owned")
        self.assertFalse(process.accepted(receipt))
        self.assertFalse(receipt["ownedProcessExited"])
        self.assertTrue(killed)
        self.assertEqual(sorted(closed),["pipe","selector"])
        self.assertEqual(set(restored),{process.signal.SIGINT,process.signal.SIGTERM})
        self.assertLessEqual(receipt["durationSeconds"],6.2)

    def test_owned_binary_metadata_capture_has_no_partial_line_acceptance(self):
        blocks=[]
        collector=SimpleNamespace(binary=True,consume=blocks.append)
        original=process.execute
        def invoke(command,active,total,projection,receipt,cwd):
            return original(command,active,total,collector,receipt,cwd)
        with patch.object(process,"execute",side_effect=invoke):
            value,_=self.run_owned((b"a"*65537+b"\0",b""))
        self.assertTrue(process.accepted(value))
        self.assertEqual(blocks,[b"a"*65537+b"\0"])
