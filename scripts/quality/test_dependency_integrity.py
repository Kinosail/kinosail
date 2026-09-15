#!/usr/bin/env python3
import hashlib
import importlib.util
import sys
sys.dont_write_bytecode = True
from pathlib import Path
import tempfile
import unittest
from unittest import mock

SPEC = importlib.util.spec_from_file_location(
    "integrity", Path(__file__).with_name("check-dependency-integrity.py")
)
INTEGRITY = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(INTEGRITY)


class IntegrityTest(unittest.TestCase):
    def test_accepts_only_reviewed_bytes(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "asset"
            path.write_bytes(b"reviewed")
            expected = hashlib.sha256(b"reviewed").hexdigest()
            INTEGRITY.check_file(path, expected)
            path.write_bytes(b"changed")
            with self.assertRaises(ValueError):
                INTEGRITY.check_file(path, expected)

    def test_rejects_nonregular_or_oversized_artifacts_before_hashing(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "large"
            with path.open("wb") as asset:
                asset.truncate((8 << 20) + 1)
            for invalid in (Path(directory), path):
                with mock.patch.object(INTEGRITY.hashlib, "file_digest") as digest:
                    with self.assertRaises(ValueError):
                        INTEGRITY.check_file(invalid, "unused")
                    digest.assert_not_called()

    def test_rejects_version_or_replace_before_file_access(self):
        for module in ({"Version": "unknown"}, {"Version": INTEGRITY.GOVAD_VERSION, "Replace": {"Dir": "/unreviewed"}}):
            with mock.patch.object(INTEGRITY, "check_file") as check:
                with self.assertRaises(ValueError):
                    INTEGRITY.verify(Path("."), module)
                check.assert_not_called()


if __name__ == "__main__":
    unittest.main()
