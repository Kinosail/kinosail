#!/usr/bin/env python3
"""Keep nested pre-push fixtures outside the repository being published."""

from __future__ import annotations

import os
from pathlib import Path
import shlex
import shutil
import subprocess
import sys
import tempfile
import unittest


TOOLS = Path(__file__).resolve().parent


class PrePushEnvironmentTest(unittest.TestCase):
    def test_linked_worktree_hook_does_not_modify_its_repository(self) -> None:
        self.assert_isolated(False)

    def test_explicit_worktree_hook_does_not_modify_its_repository(self) -> None:
        self.assert_isolated(True)

    def assert_isolated(self, explicit_worktree: bool) -> None:
        clean_env = {key: value for key, value in os.environ.items() if not key.startswith("GIT_")}
        with tempfile.TemporaryDirectory(prefix="kinosail-pre-push-test.") as directory:
            root = Path(directory)
            source = root / "source"
            worktree = root / "task"
            source.mkdir()

            def git(*args: str, cwd: Path = source) -> str:
                return subprocess.run(
                    ["git", "-c", "core.hooksPath=/dev/null", "-C", str(cwd), *args],
                    env=clean_env, check=True, capture_output=True, text=True,
                ).stdout.strip()

            git("init", "-b", "main")
            git("config", "user.name", "Pre-push Fixture")
            git("config", "user.email", "pre-push-fixture@localhost")
            (source / "keep").write_text("preserve this repository\n", encoding="utf-8")
            git("add", "keep")
            git("commit", "-m", "initial")
            baseline = git("rev-parse", "HEAD")
            git("worktree", "add", "-b", "task", str(worktree))

            def executable(path: Path, content: str) -> None:
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(content, encoding="utf-8")
                path.chmod(0o755)

            hook = worktree / "scripts/tooling/pre-push-main.sh"
            hook.parent.mkdir(parents=True)
            shutil.copyfile(TOOLS / "pre-push-main.sh", hook)
            hook.chmod(0o755)
            executable(hook.with_name("worktree_guard.py"), "#!/bin/sh\nexit 0\n")
            executable(worktree / "scripts/quality/check-full.sh", "#!/bin/sh\nexit 0\n")
            for app in ("player", "subtitles", "dashboard"):
                executable(worktree / f"apps/{app}/scripts/pre-push-main.sh", "#!/bin/sh\ncat >/dev/null\n")
            binary = root / "bin"
            executable(binary / "make", "#!/bin/sh\ncase \"$*\" in\n"
                       "  *tooling-check*) exec " + shlex.quote(sys.executable) + " "
                       + shlex.quote(str(TOOLS / "test-worktree-guard.py")) + ";;\nesac\n")
            git("add", ".", cwd=worktree)
            git("commit", "-m", "hook fixture", cwd=worktree)
            head = git("rev-parse", "HEAD", cwd=worktree)
            git_dir = Path(git("rev-parse", "--absolute-git-dir", cwd=worktree))
            config = source / ".git/config"
            index = git_dir / "index"
            git("status", "--porcelain", cwd=worktree)
            before = (config.read_bytes(), index.read_bytes())
            hook_env = clean_env | {
                "PATH": f"{binary}{os.pathsep}{clean_env['PATH']}",
                "GIT_DIR": str(git_dir), "GIT_COMMON_DIR": str(source / ".git"),
                "GIT_INDEX_FILE": str(index),
                "GIT_PREFIX": "nested/", "GIT_CONFIG_COUNT": "1",
                "GIT_CONFIG_KEY_0": "user.name", "GIT_CONFIG_VALUE_0": "Injected identity",
            }
            if explicit_worktree:
                hook_env["GIT_WORK_TREE"] = str(worktree)
            result = subprocess.run(
                [str(hook), "origin", "unused"], cwd=worktree, env=hook_env,
                input=f"refs/heads/task {head} refs/heads/main {baseline}\n",
                capture_output=True, text=True, check=False,
            )
            self.assertEqual(git("rev-parse", "HEAD", cwd=worktree), head, result.stderr)
            self.assertEqual((config.read_bytes(), index.read_bytes()), before)
            self.assertEqual(git("rev-parse", "main", cwd=worktree), baseline)
            self.assertEqual(git("status", "--porcelain", cwd=worktree), "")
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
