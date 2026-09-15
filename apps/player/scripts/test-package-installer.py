#!/usr/bin/env python3
"""Focused installer bundle regressions; run only when quality gates are enabled."""

import hashlib
import importlib.util
from pathlib import Path
import sys
import tarfile
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location(
    "package_installer", Path(__file__).with_name("package-installer.py")
)
packager = importlib.util.module_from_spec(spec)
spec.loader.exec_module(packager)


class InstallerBundleTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.source = self.root / "source"
        self.source.mkdir()
        self.destination = self.root / "release"
        for name in packager.FILES:
            path = self.source / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(name)
        (self.source / "scripts/install.sh").chmod(0o755)
        (self.source / ".env").write_text("must stay private")
        self.root_patch = patch.object(packager, "ROOT", self.source)
        self.root_patch.start()
        self.addCleanup(self.root_patch.stop)

    def run_packager(self, *arguments):
        with patch.object(sys, "argv", ["package-installer.py", *arguments]):
            packager.main()

    def test_bundle_and_checksum(self):
        self.run_packager(str(self.destination))
        archive = self.destination / "kinosail-player-install.tar.gz"
        with tarfile.open(archive) as bundle:
            self.assertEqual(set(bundle.getnames()), set(packager.FILES))
            self.assertEqual(bundle.getmember("scripts/install.sh").mode, 0o755)
            for member in bundle.getmembers():
                self.assertEqual((member.uid, member.gid, member.mtime), (0, 0, 0))
                self.assertEqual((member.uname, member.gname), ("root", "root"))
            self.assertEqual(bundle.extractfile("README.md").read(), b"README.md")
        self.assertEqual(
            archive.with_name(archive.name + ".sha256").read_text(),
            f"{hashlib.sha256(archive.read_bytes()).hexdigest()}  {archive.name}\n",
        )

    def test_invalid_arguments_have_no_output(self):
        for arguments in ((), ("--unknown",), (str(self.destination), "extra")):
            with self.subTest(arguments=arguments), self.assertRaises(SystemExit):
                self.run_packager(*arguments)
            self.assertFalse(self.destination.exists())

    def test_existing_destination_is_preserved(self):
        self.destination.mkdir()
        sentinel = self.destination / "keep"
        sentinel.write_text("unchanged")
        with self.assertRaises(SystemExit):
            self.run_packager(str(self.destination))
        self.assertEqual(list(self.destination.iterdir()), [sentinel])
        self.assertEqual(sentinel.read_text(), "unchanged")

    def test_bad_source_has_no_output(self):
        source = self.source / "README.md"
        source.unlink()
        for kind in ("missing", "directory", "symlink"):
            with self.subTest(kind=kind):
                if kind == "directory":
                    source.mkdir()
                elif kind == "symlink":
                    source.symlink_to(self.source / ".env")
                with self.assertRaises(SystemExit):
                    self.run_packager(str(self.destination))
                self.assertFalse(self.destination.exists())
                if kind == "directory":
                    source.rmdir()


if __name__ == "__main__":
    unittest.main()
