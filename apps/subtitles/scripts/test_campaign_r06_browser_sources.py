"""Source fingerprint controls using in-memory file objects only."""
import hashlib
import io
import stat
from types import SimpleNamespace
import unittest

from campaign_r06_browser_sources import fingerprint


class MemoryFile:
    def __init__(self, data=b"", mode=stat.S_IFREG | 0o644, size=None):
        self.data = data
        self.mode = mode
        self.size = len(data) if size is None else size
        self.opened = 0

    def lstat(self):
        return SimpleNamespace(st_mode=self.mode, st_size=self.size)

    def open(self, mode):
        if mode != "rb":
            raise AssertionError("read-only-mode")
        self.opened += 1
        return io.BytesIO(self.data)


class SourceFingerprintControls(unittest.TestCase):
    def reject_shape(self, path, **options):
        with self.assertRaisesRegex(ValueError, "^file-shape$"):
            fingerprint(path, **options)
        self.assertEqual(path.opened, 0)

    def test_empty_tracked_regular_source_has_git_identity(self):
        path = MemoryFile()
        self.assertEqual(fingerprint(path, limit=32*1024*1024), {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
            "mode": 0o644,
            "gitBlob": "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391",
        })
        self.assertEqual(path.opened, 1)

    def test_empty_executable_is_rejected(self):
        self.reject_shape(MemoryFile(mode=stat.S_IFREG | 0o755), executable=True)

    def test_symlink_is_rejected_before_open(self):
        self.reject_shape(MemoryFile(b"target", stat.S_IFLNK | 0o777))

    def test_directory_is_rejected_before_open(self):
        self.reject_shape(MemoryFile(mode=stat.S_IFDIR | 0o755))

    def test_source_size_cap_is_preserved(self):
        self.reject_shape(MemoryFile(b"x", size=32*1024*1024+1), limit=32*1024*1024)

    def test_negative_size_is_rejected(self):
        self.reject_shape(MemoryFile(size=-1))

    def test_required_executable_mode_is_preserved(self):
        self.reject_shape(MemoryFile(b"binary"), executable=True)

    def test_nonempty_executable_hashes_actual_bytes(self):
        data = b"owned tool\n"
        path = MemoryFile(data, stat.S_IFREG | 0o755)
        result = fingerprint(path, executable=True)
        self.assertEqual(result, {
            "bytes": len(data), "sha256": hashlib.sha256(data).hexdigest(),
            "mode": 0o755, "gitBlob": hashlib.sha1(b"blob 11\0" + data).hexdigest(),
        })
        self.assertEqual(path.opened, 1)


if __name__ == "__main__":
    unittest.main()
