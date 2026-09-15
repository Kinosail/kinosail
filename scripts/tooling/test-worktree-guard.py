#!/usr/bin/env python3
"""Focused behavioral tests for the worktree lease and cleanup guard."""

from __future__ import annotations

import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


GUARD = Path(__file__).with_name("worktree_guard.py")


class WorktreeGuardTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = Path(tempfile.mkdtemp(prefix="kinosail-worktree-test."))
        self.repo = self.temp / "repo"
        self.repo.mkdir()
        self.git("init", "-b", "main")
        self.git("config", "user.name", "Worktree Test")
        self.git("config", "user.email", "worktree-test@localhost")
        (self.repo / "tracked").write_text("initial\n", encoding="utf-8")
        self.git("add", "tracked")
        self.commit("initial")

    def tearDown(self) -> None:
        shutil.rmtree(self.temp, ignore_errors=True)

    def git(self, *args: str, cwd: Path | None = None) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            ["git", "-c", "core.hooksPath=/dev/null", "-C", str(cwd or self.repo), *args],
            check=True,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )

    def commit(self, message: str, cwd: Path | None = None) -> None:
        self.git("commit", "-m", message, cwd=cwd)

    def run_guard(
        self, *args: str, cwd: Path | None = None, check: bool = False, env: dict[str, str] | None = None
    ) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [sys.executable, str(GUARD), *args],
            cwd=cwd or self.repo,
            check=check,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            env=env,
        )

    def add_worktree(self, name: str = "task") -> Path:
        path = self.temp / name
        self.git("worktree", "add", "-b", name, str(path))
        return path

    def lease_file(self, path: Path) -> Path:
        common = Path(self.git("rev-parse", "--path-format=absolute", "--git-common-dir").stdout.strip())
        digest = hashlib.sha256(os.fsencode(str(path.resolve()))).hexdigest()
        return common / "kinosail-worktrees" / "leases" / f"{digest}.json"

    def expire(self, path: Path) -> None:
        lease = self.lease_file(path)
        data = json.loads(lease.read_text(encoding="utf-8"))
        data["expires_at"] = 1
        lease.write_text(json.dumps(data), encoding="utf-8")

    def test_lease_heartbeat_and_audit(self) -> None:
        path = self.add_worktree()
        self.run_guard("lease", "--task", "agent/task", cwd=path, check=True)
        before = self.lease_file(path).read_text(encoding="utf-8")
        self.run_guard("heartbeat", "--task", "agent/task", cwd=path, check=True)
        self.assertEqual(self.run_guard("audit").returncode, 0)
        self.assertTrue(before)

    def test_unknown_oversized_and_conflicting_input_has_no_side_effects(self) -> None:
        path = self.add_worktree()
        self.run_guard("lease", "--task", "owner", cwd=path, check=True)
        lease = self.lease_file(path)
        original = lease.read_bytes()
        self.assertNotEqual(self.run_guard("lease", "--task", "other", cwd=path).returncode, 0)
        self.assertNotEqual(self.run_guard("lease", "--task", "x" * 129, cwd=path).returncode, 0)
        self.assertNotEqual(self.run_guard("lease", "--task", "owner", "--ttl", "59", cwd=path).returncode, 0)
        self.assertNotEqual(self.run_guard("lease", "--task", "owner", "--ttl", "604801", cwd=path).returncode, 0)
        self.assertNotEqual(self.run_guard("lease", "--task", "owner", "--unknown", cwd=path).returncode, 0)
        self.assertEqual(lease.read_bytes(), original)

    def test_malformed_and_oversized_leases_are_rejected_without_cleanup(self) -> None:
        path = self.add_worktree()
        self.run_guard("lease", "--task", "owner", cwd=path, check=True)
        lease = self.lease_file(path)
        data = json.loads(lease.read_text(encoding="utf-8"))
        data["unknown"] = True
        lease.write_text(json.dumps(data), encoding="utf-8")
        self.assertNotEqual(self.run_guard("cleanup").returncode, 0)
        self.assertTrue(path.is_dir())
        lease.write_text("{", encoding="utf-8")
        self.assertNotEqual(self.run_guard("cleanup").returncode, 0)
        self.assertTrue(path.is_dir())
        lease.write_bytes(b"x" * 8193)
        self.assertNotEqual(self.run_guard("cleanup").returncode, 0)
        self.assertTrue(path.is_dir())

    def test_cleanup_preserves_dirty_and_unique_work(self) -> None:
        dirty = self.add_worktree("dirty")
        self.run_guard("lease", "--task", "dirty", cwd=dirty, check=True)
        self.expire(dirty)
        (dirty / "untracked").write_text("keep\n", encoding="utf-8")
        self.assertNotEqual(self.run_guard("cleanup").returncode, 0)
        self.assertTrue((dirty / "untracked").exists())

        unique = self.add_worktree("unique")
        (unique / "tracked").write_text("unique\n", encoding="utf-8")
        self.git("add", "tracked", cwd=unique)
        self.commit("unique", cwd=unique)
        self.assertNotEqual(self.run_guard("cleanup").returncode, 0)
        self.assertTrue(unique.is_dir())

    def test_cleanup_removes_only_clean_merged_worktree(self) -> None:
        path = self.add_worktree()
        self.assertNotEqual(self.run_guard("audit").returncode, 0)
        self.run_guard("cleanup", check=True)
        self.assertFalse(path.exists())

    def test_finish_serializes_fast_forward_and_removes_task(self) -> None:
        path = self.add_worktree()
        (path / "tracked").write_text("finished\n", encoding="utf-8")
        self.git("add", "tracked", cwd=path)
        self.commit("finished", cwd=path)
        self.run_guard("lease", "--task", "finish", cwd=path, check=True)
        fake_bin = self.temp / "bin"
        fake_bin.mkdir()
        fake_make = fake_bin / "make"
        fake_make.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8")
        fake_make.chmod(0o755)
        env = os.environ.copy()
        env["PATH"] = f"{fake_bin}{os.pathsep}{env['PATH']}"
        result = self.run_guard("finish", "--task", "finish", cwd=path, env=env)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse(path.exists())
        self.assertEqual((self.repo / "tracked").read_text(encoding="utf-8"), "finished\n")
        self.assertNotIn("task", self.git("branch", "--format=%(refname:short)").stdout.splitlines())


if __name__ == "__main__":
    unittest.main()
