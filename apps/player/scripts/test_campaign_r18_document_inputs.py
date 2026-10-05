"""Pure mocked traversal failures; never actual tool/filesystem proof."""
from pathlib import Path
from types import SimpleNamespace
from unittest import mock
import stat
import os
import unittest

from campaign_r18_document_inputs import Budget, iter_tool_paths, open_regular, stream_regular, read_small
from campaign_r18_document_sources import import_execute


def scan(entries):
    context = mock.MagicMock()
    context.__enter__.return_value = iter(SimpleNamespace(name=name) for name in entries)
    return context


def info(directory):
    return SimpleNamespace(st_dev=1, st_ino=7, st_size=0,
                           st_mode=(stat.S_IFDIR if directory else stat.S_IFREG) | 0o755,
                           st_mtime_ns=2, st_ctime_ns=3, st_uid=4, st_gid=5, st_nlink=1)


class TraversalControls(unittest.TestCase):
    def test_entry_count_stops_before_unbounded_retention_or_sort(self):
        for root_names, tool_names in ((["linux_amd64"], [f"tool{i}" for i in range(129)]),
                                      ([f"arch{i}" for i in range(33)], [])):
            with self.subTest(root_count=len(root_names)):
                scans = [scan(root_names)] + [scan(tool_names) for _name in root_names]
                lookup = lambda name, **_options: info(not name.startswith("tool"))
                with mock.patch("campaign_r18_document_inputs.open_directory", return_value=10), mock.patch("campaign_r18_document_inputs.os.open", return_value=11), mock.patch("campaign_r18_document_inputs.os.fstat", return_value=info(True)), mock.patch("campaign_r18_document_inputs.os.stat", side_effect=lookup), mock.patch("campaign_r18_document_inputs.os.scandir", side_effect=scans), mock.patch("campaign_r18_document_inputs.os.close"), mock.patch("campaign_r18_document_inputs.time.monotonic", return_value=1):
                    with self.assertRaises(ValueError):
                        list(iter_tool_paths(Path("/fixture/tool"), 100))

    def test_expired_traversal_does_not_enter_scandir(self):
        with mock.patch("campaign_r18_document_inputs.os.scandir") as scan_call, mock.patch("campaign_r18_document_inputs.time.monotonic", return_value=100):
            with self.assertRaises(ValueError):
                list(iter_tool_paths(Path("/fixture/tool"), 100))
            scan_call.assert_not_called()


class InputBoundaryControls(unittest.TestCase):
    def info(self, inode=7, size=3, mode=stat.S_IFREG | 0o644):
        return SimpleNamespace(st_dev=1, st_ino=inode, st_size=size, st_mode=mode,
                               st_mtime_ns=2, st_ctime_ns=3, st_uid=4, st_gid=5, st_nlink=1)

    def reader(self, info=None, reads=(b"abc", b""), fstats=None, close_effect=None):
        info = info or self.info()
        return (mock.patch("campaign_r18_document_inputs.open_regular", side_effect=[(10, 11, "source"), (12, 13, "source")]),
                mock.patch("campaign_r18_document_inputs.os.fstat", side_effect=fstats, return_value=info),
                mock.patch("campaign_r18_document_inputs.os.stat", return_value=info),
                mock.patch("campaign_r18_document_inputs.os.read", side_effect=reads),
                mock.patch("campaign_r18_document_inputs.os.close", side_effect=close_effect),
                mock.patch("campaign_r18_document_inputs.time.monotonic", return_value=1))

    def call_reader(self, contexts, limit=3, deadline=100):
        from contextlib import ExitStack
        with ExitStack() as stack:
            patches = [stack.enter_context(item) for item in contexts]
            result = read_small(Path("/fixture/source"), limit, deadline)
            return result, patches

    def test_open_chain_uses_nofollow_nonblocking_cloexec_for_every_owned_descriptor(self):
        with mock.patch("campaign_r18_document_inputs.os.open", side_effect=[10, 11, 12]) as opened, mock.patch("campaign_r18_document_inputs.os.close") as closed:
            self.assertEqual(open_regular(Path("/fixture/source")), (12, 11, "source"))
            required = os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC
            self.assertTrue(all(call.args[1] & required == required for call in opened.call_args_list))
            self.assertTrue(all(call.args[1] & os.O_DIRECTORY for call in opened.call_args_list[:2]))
            closed.assert_called_once_with(10)

    def test_descriptor_success_is_stable_and_every_owned_descriptor_closes(self):
        result, patches = self.call_reader(self.reader())
        self.assertEqual(result, b"abc")
        self.assertEqual(sorted(call.args[0] for call in patches[4].call_args_list), [10, 11, 12, 13])

    def test_fifo_and_pre_read_size_limit_fail_without_reading(self):
        for info in (self.info(mode=stat.S_IFIFO | 0o600), self.info(size=4)):
            contexts = self.reader(info=info)
            with contexts[0], contexts[1], contexts[2], contexts[3] as read, contexts[4] as close, contexts[5]:
                with self.assertRaises(ValueError):
                    read_small(Path("/fixture/source"), 3, 100)
                read.assert_not_called()
                self.assertEqual(close.call_count, 2)

    def test_growth_shrink_and_changed_descriptor_identity_are_rejected(self):
        for reads, stats in (((b"abcd", b""), None), ((b"ab", b""), None),
                             ((b"abc", b""), [self.info(), self.info(inode=8)])):
            with self.subTest(reads=reads):
                with self.assertRaises(ValueError):
                    self.call_reader(self.reader(reads=reads, fstats=stats))

    def test_named_path_swap_and_unconfirmed_close_are_rejected(self):
        contexts = self.reader()
        contexts = contexts[:2] + (mock.patch("campaign_r18_document_inputs.os.stat", side_effect=[self.info(), self.info(), self.info(inode=8)]),) + contexts[3:]
        with self.assertRaises(ValueError):
            self.call_reader(contexts)
        with self.assertRaises(ValueError):
            self.call_reader(self.reader(close_effect=[OSError("fixture"), None, None, None]))

    def test_expired_descriptor_read_never_reads_or_accepts_bytes(self):
        with self.assertRaises(ValueError):
            self.call_reader(self.reader(), deadline=1)

    def test_tool_count_byte_and_deadline_budgets_fail_before_reservation(self):
        with mock.patch("campaign_r18_document_inputs.time.monotonic", return_value=1):
            budget = Budget(100, max_count=1, max_bytes=3)
            budget.reserve(3)
            for size in (0, 1, True, -1):
                with self.subTest(size=size), self.assertRaises(ValueError):
                    budget.reserve(size)
            self.assertEqual((budget.count, budget.bytes), (1, 3))
            with self.assertRaises(ValueError):
                Budget(1).reserve(0)
            with self.assertRaises(ValueError):
                Budget(100, max_bytes=2).reserve(3)

    def test_invalid_helper_bytes_reject_before_any_import_execution(self):
        with mock.patch("campaign_r18_document_sources.read_small", return_value=b"private_invalid_helper()"), mock.patch("builtins.exec") as executed:
            with self.assertRaises(ValueError):
                import_execute(100)
            executed.assert_not_called()

if __name__ == "__main__":
    unittest.main()
