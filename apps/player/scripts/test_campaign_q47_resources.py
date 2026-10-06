#!/usr/bin/env python3
"""Fictional descriptor, traversal and cleanup controls; no real IO or processes."""
import unittest
from unittest import mock
from types import SimpleNamespace
import os
import stat
import campaign_q47_sources as sources
import campaign_q47_execution as processes
import campaign_q47_admission as admission
import campaign_q47_io as bounded_io
from pathlib import Path

H = "a" * 64
complete = processes.complete


class ResourceControls(unittest.TestCase):
    def test_private_report_owns_bounded_descriptor_and_checked_close(self):
        details = SimpleNamespace(st_mode=stat.S_IFREG | 0o600, st_dev=1, st_ino=2, st_size=3,
                                  st_mtime_ns=4, st_ctime_ns=5)
        path = mock.Mock(); path.lstat.return_value = details
        path.read_bytes.side_effect = AssertionError("unbounded pathname read")
        for fault in ("none", "overflow", "changed", "close", "symlink"):
            with self.subTest(fault=fault), mock.patch.multiple(
                    os, open=mock.DEFAULT, fstat=mock.DEFAULT, read=mock.DEFAULT, close=mock.DEFAULT) as calls:
                calls["open"].return_value = 7
                calls["fstat"].return_value = details
                calls["read"].side_effect = [b"{}\n", b""]
                if fault == "overflow": calls["read"].side_effect = [b"x" * 262_145]
                if fault == "changed":
                    calls["fstat"].side_effect = [details, SimpleNamespace(**(vars(details) | {"st_ino": 8}))]
                if fault == "close": calls["close"].side_effect = OSError("fictional close failure")
                if fault == "symlink": calls["open"].side_effect = OSError("fictional no-follow rejection")
                if fault == "none": self.assertEqual(sources.private_report(path), {})
                else:
                    with self.assertRaises((ValueError, OSError)): sources.private_report(path)
                self.assertEqual(calls["open"].call_args.args[1] & (os.O_NOFOLLOW | os.O_NONBLOCK),
                                 os.O_NOFOLLOW | os.O_NONBLOCK)
                if fault != "symlink": calls["close"].assert_called_once_with(7)
    def test_tree_limits_and_budget_are_incremental_for_every_entry(self):
        for function, limit, count in ((sources.installed_tree, 20_000, 3), (sources.generated, 10_000, 5)):
            for directories in (True, False):
                with self.subTest(function=function.__name__, directories=directories):
                    root, path, seen = mock.Mock(), mock.MagicMock(), []
                    path.__lt__.return_value = False
                    root.resolve.return_value = root
                    path.relative_to.return_value.as_posix.return_value = "fictional"
                    path.is_symlink.return_value = False
                    path.is_dir.return_value = directories; path.is_file.return_value = not directories
                    def entries():
                        for index in range(limit + 2 if directories else 100):
                            seen.append(index); yield path
                    root.rglob.side_effect = lambda _: entries()
                    with mock.patch.object(sources, "bounded_paths", side_effect=lambda *_: entries(), create=True), mock.patch.object(
                            sources, "check_budget") as budget, mock.patch.object(sources, "fingerprint", return_value={"bytes": 64 * 1024 * 1024, "sha256": H, "gitBlob": "a" * 40}):
                        with self.assertRaises(ValueError): function(root)
                        self.assertEqual(len(seen), limit + 1 if directories else count)
                        self.assertGreaterEqual(budget.call_count, len(seen))
            root.rglob.side_effect = lambda _: iter([path])
            for symlink in (False, True):
                path.is_symlink.return_value = symlink
                with self.subTest(function=function.__name__, symlink=symlink), mock.patch.object(
                        sources, "bounded_paths", side_effect=lambda *_: iter([path]), create=True), mock.patch.object(
                        sources, "check_budget", side_effect=ValueError("fictional budget")), mock.patch.object(
                        os, "readlink", return_value="fictional"):
                    with self.assertRaisesRegex(ValueError, "fictional budget"): function(root)

    def test_interrupt_during_execute_and_peer_cleanup_cannot_be_success(self):
        for window in ("wait", "group"):
            with self.subTest(window=window):
                handlers, process, selector = {}, mock.Mock(), mock.MagicMock()
                process.stdout.fileno.return_value = 7; process.poll.return_value = 0; process.returncode = 0
                selector.__enter__.return_value.select.return_value = [(None, None)]
                def install(number, handler): handlers[number] = handler
                def interrupt(): handlers[processes.signal.SIGTERM](0, None)
                def wait(**_kw):
                    if window == "wait": interrupt()
                    return 0
                def group(_pid):
                    if window == "group": interrupt()
                    return True
                process.wait.side_effect = wait
                with mock.patch.object(processes.signal, "signal", side_effect=install), mock.patch.object(
                        processes.signal, "getsignal", return_value=lambda *_: None), mock.patch.object(
                        processes.subprocess, "Popen", return_value=process), mock.patch.object(
                        processes.selectors, "DefaultSelector", return_value=selector), mock.patch.object(
                        os, "set_blocking"), mock.patch.object(os, "read", return_value=b""), mock.patch.object(
                        processes, "settle_group", side_effect=group):
                    result, _ = processes.execute(["fictional"], 1, None, {})
                self.assertEqual(result["stopReason"], "interrupted")
                self.assertFalse(complete(result, 0))
        handlers, peer = {}, processes.Peer(None, None, None)
        def cleanup():
            handler = handlers[processes.signal.SIGTERM]
            if callable(handler): handler(0, None)
            return {"leaderExited": True, "groupAbsent": True}
        with mock.patch.object(processes.signal, "signal", side_effect=lambda n, h: handlers.update({n: h})), mock.patch.object(
                processes.signal, "getsignal", return_value=lambda *_: None), mock.patch.object(peer, "_stop", side_effect=cleanup):
            self.assertTrue(peer.stop().get("interrupted", False))

    def test_normal_checksums(self):
        self.assertNotEqual(processes.environment().get("GOSUMDB"), "off")

    def test_fixed_public_selector(self):
        base = {"CAMPAIGN_PROOF": "Q47", "CAMPAIGN_Q47_SUITE": "primary"}
        self.assertTrue(admission.selector_valid(["fictional"], base))
        self.assertTrue(admission.selector_valid(["fictional"], base | {"KINOSAIL_Q47_SUITE": "primary"}))
        for variable in ("CAMPAIGN_PROOF", "CAMPAIGN_Q47_SUITE", "KINOSAIL_Q47_SUITE"):
            for value in ("contracts", "none", "", "PRIMARY"):
                self.assertFalse(admission.selector_valid(["fictional"], base | {variable: value}))
        for variable in ("CAMPAIGN_PROOF", "CAMPAIGN_Q47_SUITE"):
            self.assertFalse(admission.selector_valid(["fictional"], {k: v for k, v in base.items() if k != variable}))
        for argv in ([], ["fictional", "extra"]):
            self.assertFalse(admission.selector_valid(argv, base))

    def test_fingerprint_fifo_swap_is_nonblocking_and_close_checked(self):
        regular = SimpleNamespace(st_mode=stat.S_IFREG | 0o600, st_dev=1, st_ino=2, st_size=3,
                                  st_mtime_ns=4, st_ctime_ns=5)
        fifo = SimpleNamespace(**(vars(regular) | {"st_mode": stat.S_IFIFO | 0o600}))
        path, stream = mock.Mock(), mock.MagicMock()
        path.lstat.return_value = regular
        stream.__enter__.return_value.fileno.return_value = 7
        with mock.patch.multiple(os, open=mock.DEFAULT, fstat=mock.DEFAULT, read=mock.DEFAULT,
                                 close=mock.DEFAULT, fdopen=mock.DEFAULT) as calls:
            calls["open"].return_value = 7; calls["fstat"].return_value = fifo
            calls["fdopen"].return_value = stream
            with self.assertRaises(ValueError): sources.fingerprint(path)
            self.assertEqual(calls["open"].call_args.args[1] & os.O_NONBLOCK, os.O_NONBLOCK)
            calls["close"].assert_called_once_with(7)


    def test_scandir_consumption_is_incremental_and_symlink_dirs_not_followed(self):
        seen, entry, iterator = [], mock.Mock(), mock.MagicMock()
        entry.name = "fictional-link"
        entry.is_dir.side_effect = lambda *, follow_symlinks: follow_symlinks
        def entries():
            for index in range(100):
                seen.append(index); yield entry
        iterator.__enter__.side_effect = lambda: entries()
        details = SimpleNamespace(st_mode=stat.S_IFDIR | 0o700, st_dev=1, st_ino=2, st_size=0,
                                  st_mtime_ns=4, st_ctime_ns=5)
        with mock.patch.multiple(os, open=mock.DEFAULT, fstat=mock.DEFAULT,
                                 close=mock.DEFAULT, scandir=mock.DEFAULT) as calls:
            calls["open"].return_value = 7; calls["fstat"].return_value = details
            calls["scandir"].return_value = iterator
            with self.assertRaises(ValueError): list(bounded_io.bounded_paths(Path("/fictional"), 3))
            self.assertEqual(len(seen), 4)
            calls["scandir"].assert_called_once_with(7)
            calls["close"].assert_called_once_with(7)
            self.assertEqual(calls["open"].call_args.args[1] & (os.O_NOFOLLOW | os.O_NONBLOCK),
                             os.O_NOFOLLOW | os.O_NONBLOCK)
            self.assertTrue(all(call.kwargs == {"follow_symlinks": False} for call in entry.is_dir.call_args_list))
            iterator.__exit__.assert_called_once()

    def test_scandir_budget_stops_producer_and_closes_owned_directory(self):
        seen, entry, iterator = [], mock.Mock(), mock.MagicMock()
        entry.name = "fictional-file"; entry.is_dir.return_value = False
        def entries():
            for index in range(100):
                seen.append(index); yield entry
        def budget():
            if len(seen) >= 2: raise ValueError("fictional traversal budget")
        iterator.__enter__.side_effect = lambda: entries()
        details = SimpleNamespace(st_mode=stat.S_IFDIR | 0o700, st_dev=1, st_ino=2, st_size=0,
                                  st_mtime_ns=4, st_ctime_ns=5)
        with mock.patch.multiple(os, open=mock.DEFAULT, fstat=mock.DEFAULT,
                                 close=mock.DEFAULT, scandir=mock.DEFAULT) as calls, mock.patch.object(
                bounded_io, "check_budget", side_effect=budget):
            calls["open"].return_value = 7; calls["fstat"].return_value = details
            calls["scandir"].return_value = iterator
            with self.assertRaisesRegex(ValueError, "fictional traversal budget"):
                list(bounded_io.bounded_paths(Path("/fictional"), 3))
            self.assertEqual(len(seen), 2)
            calls["close"].assert_called_once_with(7)
            iterator.__exit__.assert_called_once()


if __name__ == "__main__":
    unittest.main()
