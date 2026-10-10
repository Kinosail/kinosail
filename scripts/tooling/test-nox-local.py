#!/usr/bin/env python3
"""Isolated source/race checks; live Nox cannot safely exercise these failures."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import time
import unittest
import plistlib
import sys

HERE = Path(__file__).resolve().parent


class LocalWatchTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.repo = self.root / "repo"
        self.repo.mkdir()
        subprocess.run(["git", "init", "-q", str(self.repo)], check=True)
        self.write(".gitignore", ".env\nsecret.go\n")
        for app in ("player", "subtitles"):
            self.write(f"apps/{app}/Containerfile", "FROM scratch\n")
            self.write(f"apps/{app}/internal/main.go", "package main\n")
        self.write("packages/shared.go", "package shared\n")
        subprocess.run(["git", "-C", str(self.repo), "add", "."], check=True)
        subprocess.run(["git", "-C", str(self.repo), "-c", "user.name=Test", "-c",
                        "user.email=test@example.invalid", "commit", "-qm", "fixture"], check=True)

    def write(self, path, content):
        dest = self.repo / path
        dest.parent.mkdir(parents=True, exist_ok=True)
        dest.write_text(content)
        return dest

    def source(self):
        spec = importlib.util.spec_from_file_location("nox_source", HERE / "nox_local_source.py")
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return module

    def test_snapshot_excludes_ignored_secrets_and_detects_deletion(self):
        source = self.source()
        initial = source.capture(self.repo, "player")
        self.write("apps/player/internal/.env", "PASSWORD=private\n")
        self.write("apps/player/internal/secret.go", "private\n")
        self.assertEqual(initial, source.capture(self.repo, "player"))
        self.write("apps/player/internal/new.go", "package main\n")
        snapshot = self.root / "snapshot"
        changed = source.capture(self.repo, "player", snapshot)
        self.assertNotEqual(initial["snapshot"], changed["snapshot"])
        self.assertFalse((snapshot / "apps/player/internal/.env").exists())
        child_snapshot = self.root / "child-snapshot"
        child = subprocess.run([sys.executable, str(HERE / "nox_local_source.py"), str(self.repo),
                                "player", "--json", "--destination", str(child_snapshot)],
                               check=True, capture_output=True, text=True)
        self.assertRegex(json.loads(child.stdout)["snapshot"], r"^[0-9a-f]{40}$")
        self.assertEqual((child_snapshot / "apps/player/internal/new.go").read_text(), "package main\n")
        self.assertFalse((child_snapshot / "apps/player/internal/secret.go").exists())
        (self.repo / "apps/player/internal/main.go").unlink()
        self.assertNotEqual(changed["snapshot"], source.capture(self.repo, "player")["snapshot"])

    def test_symlink_is_rejected_without_copying_its_target(self):
        secret = self.root / "private"
        secret.write_text("private")
        (self.repo / "packages/link.go").symlink_to(secret)
        with self.assertRaises(ValueError):
            self.source().capture(self.repo, "player", self.root / "snapshot")
        self.assertEqual(secret.read_text(), "private")

    def test_app_and_shared_changes_select_the_right_consumers(self):
        source = self.source()
        before = {a: source.capture(self.repo, a) for a in ("player", "subtitles")}
        self.write("apps/player/internal/main.go", "package changed\n")
        self.assertNotEqual(before["player"], source.capture(self.repo, "player"))
        self.assertEqual(before["subtitles"], source.capture(self.repo, "subtitles"))
        self.write("packages/shared.go", "package changed\n")
        self.assertNotEqual(before["subtitles"], source.capture(self.repo, "subtitles"))

    def test_saves_are_debounced_and_concurrent_watchers_are_rejected(self):
        runtime = HERE / "watch-nox-local.py"
        copied = self.root / "tooling"
        copied.mkdir()
        for path in (runtime, HERE / "nox_local_source.py"):
            (copied / path.name).write_bytes(path.read_bytes())
        log = self.root / "deployed"
        deploy = copied / "deploy-nox-local.sh"
        deploy.write_text('#!/bin/sh\nprintf "%s\\n" "$3" >> "$TEST_DEPLOY_LOG"\n')
        deploy.chmod(0o755)
        environment = dict(os.environ, TEST_DEPLOY_LOG=str(log))
        process = subprocess.Popen(["python3", str(copied / runtime.name), "--repo", str(self.repo),
                                    "--cache", str(self.root / "cache"), "--apps", "player",
                                    "--source-python", sys.executable,
                                    "--debounce", "0.3", "--poll", "0.05"], env=environment,
                                   stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        self.addCleanup(lambda: self.stop(process))
        self.until(lambda: log.exists())
        first = log.read_text().splitlines()
        for i in range(4):
            self.write("apps/player/internal/main.go", f"package save{i}\n")
            time.sleep(0.08)
        self.until(lambda: len(log.read_text().splitlines()) == 2)
        self.assertEqual(len(first), 1)
        self.assertEqual(log.read_text().splitlines()[-1], self.source().capture(self.repo, "player")["snapshot"])
        time.sleep(0.4)
        self.assertEqual(len(log.read_text().splitlines()), 2)
        second = subprocess.run(["python3", str(copied / runtime.name), "--repo", str(self.repo),
                                 "--cache", str(self.root / "cache"), "--once"], capture_output=True)
        self.assertNotEqual(second.returncode, 0)
        self.stop(process)

    def test_superseded_binary_build_cannot_reach_remote_image_or_service_changes(self):
        snapshot = self.root / "snapshot"
        metadata = self.source().capture(self.repo, "player", snapshot)
        tools = self.root / "bin"
        tools.mkdir()
        log = self.root / "ssh.log"
        for name, body in {
            "ssh": 'printf "%s\\n" "$*" >> "$TEST_SSH_LOG"\ncase "$*" in *"docker info"*) echo aarch64;; *"docker image inspect"*) printf "%s|arm64\\n" "$TEST_RUNTIME";; esac\n',
            "go": 'printf "package changed\\n" > "$TEST_SOURCE"\n',
            "tar": 'exit 99\n',
        }.items():
            file = tools / name
            file.write_text("#!/bin/sh\n" + body)
            file.chmod(0o755)
        env = dict(os.environ, PATH=f"{tools}:{os.environ['PATH']}", TEST_SSH_LOG=str(log),
                   TEST_RUNTIME=metadata["runtime"], TEST_SOURCE=str(self.repo / "packages/shared.go"))
        result = subprocess.run([str(HERE / "deploy-nox-local.sh"), "player", str(snapshot),
                                 metadata["snapshot"], metadata["runtime"], metadata["commit"],
                                 str(self.repo)], env=env, capture_output=True)
        self.assertEqual(result.returncode, 75, result.stderr.decode())
        self.assertNotIn("docker build", log.read_text())
        self.assertNotIn("docker load", log.read_text())
        self.assertNotIn("bash -s", log.read_text())

    def test_a_new_save_retries_immediately_after_a_failed_build(self):
        copied = self.root / "tooling"
        copied.mkdir()
        for name in ("watch-nox-local.py", "nox_local_source.py"):
            (copied / name).write_bytes((HERE / name).read_bytes())
        log = self.root / "attempts"
        deploy = copied / "deploy-nox-local.sh"
        deploy.write_text('#!/bin/sh\nprintf "%s\\n" "$3" >> "$TEST_DEPLOY_LOG"\nexit 1\n')
        deploy.chmod(0o755)
        process = subprocess.Popen(["python3", str(copied / "watch-nox-local.py"), "--repo", str(self.repo),
                                    "--cache", str(self.root / "cache"), "--apps", "player",
                                    "--debounce", "0.1", "--poll", "0.05"],
                                   env=dict(os.environ, TEST_DEPLOY_LOG=str(log)),
                                   stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        self.addCleanup(lambda: self.stop(process))
        self.until(lambda: log.exists())
        self.write("apps/player/internal/main.go", "package fixed\n")
        self.until(lambda: len(log.read_text().splitlines()) == 2)
        self.stop(process)

    def test_private_local_umask_does_not_make_the_container_binary_inexecutable(self):
        snapshot = self.root / "snapshot"
        metadata = self.source().capture(self.repo, "player", snapshot)
        tools = self.root / "bin"
        tools.mkdir()
        mode = self.root / "mode"
        for name, body in {
            "ssh": 'case "$*" in *"docker info"*) echo aarch64;; *"docker image inspect"*) printf "%s|arm64\\n" "$TEST_RUNTIME";; esac\n',
            "go": 'while [ "$#" -gt 0 ]; do if [ "$1" = -o ]; then shift; printf binary > "$1"; chmod 700 "$1"; exit; fi; shift; done\n',
            "tar": 'python3 -c "import pathlib,sys; print(oct(pathlib.Path(sys.argv[1]).stat().st_mode & 0o777))" "$2/kinosail" > "$TEST_MODE"\ntouch "$4"\n',
        }.items():
            file = tools / name
            file.write_text("#!/bin/sh\n" + body)
            file.chmod(0o755)
        env = dict(os.environ, PATH=f"{tools}:{os.environ['PATH']}", TEST_MODE=str(mode),
                   TEST_RUNTIME=metadata["runtime"])
        subprocess.run([str(HERE / "deploy-nox-local.sh"), "player", str(snapshot),
                        metadata["snapshot"], metadata["runtime"], metadata["commit"], str(self.repo)],
                       env=env, check=True, capture_output=True)
        permissions = int(mode.read_text().strip(), 8)
        self.assertEqual(permissions & 0o005, 0o005, "Container user must read and execute the binary")
        self.assertFalse(permissions & 0o002, "Container user must not rewrite the binary")

    def test_installer_records_the_selected_checkout_and_copies_runtime_tools(self):
        tools = self.root / "bin"
        tools.mkdir()
        for name in ("launchctl", "go", "podman", "ssh"):
            command = tools / name
            command.write_text("#!/bin/sh\nexit 0\n")
            command.chmod(0o755)
        (tools / "launchctl").write_text('#!/bin/sh\nif [ "$1" = bootstrap ] && [ ! -e "$TEST_STARTED" ]; then touch "$TEST_STARTED"; exit 5; fi\nexit 0\n')
        env = dict(os.environ, HOME=str(self.root / "home"), PATH=f"{tools}:{os.environ['PATH']}",
                   TEST_STARTED=str(self.root / "started"))
        subprocess.run(["python3", str(HERE / "install-nox-local.py"), "--repo", str(self.repo)],
                       env=env, check=True, capture_output=True)
        home = self.root / "home"
        plist = home / "Library/LaunchAgents/com.kinosail.nox-local-live.plist"
        config = plistlib.loads(plist.read_bytes())
        if sys.platform == "darwin":
            self.assertEqual(config["ProgramArguments"][0], "/usr/bin/python3")
            self.assertIn("--source-python", config["ProgramArguments"])
        self.assertIn(str(self.repo.resolve()), config["ProgramArguments"])
        self.assertTrue(config["RunAtLoad"])
        self.assertTrue(config["KeepAlive"])
        installed = home / "Library/Application Support/KinosailNoxLocal"
        self.assertTrue((installed / "nox_local_source.py").exists())
        self.assertTrue(os.access(installed / "deploy-nox-local.sh", os.X_OK))
        self.assertEqual((installed / "deploy-nox-remote.sh").read_bytes(),
                         (HERE / "deploy-nox-remote.sh").read_bytes())

    def test_a_superseded_one_shot_does_not_report_success(self):
        copied = self.root / "tooling"
        copied.mkdir()
        for name in ("watch-nox-local.py", "nox_local_source.py"):
            (copied / name).write_bytes((HERE / name).read_bytes())
        deploy = copied / "deploy-nox-local.sh"
        deploy.write_text("#!/bin/sh\nexit 75\n")
        deploy.chmod(0o755)
        result = subprocess.run(["python3", str(copied / "watch-nox-local.py"), "--repo", str(self.repo),
                                 "--cache", str(self.root / "cache"), "--once"], capture_output=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(b'"outcome": "superseded"', result.stdout)

    @staticmethod
    def stop(process):
        if process.poll() is None:
            process.terminate()
            process.communicate(timeout=5)

    def until(self, condition):
        deadline = time.monotonic() + 6
        while not condition():
            self.assertLess(time.monotonic(), deadline, "watcher did not reach expected state")
            time.sleep(0.05)


if __name__ == "__main__":
    unittest.main()
