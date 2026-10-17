"""Sharded quick Go tests run every listed test exactly once from any working directory."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import go_shard

ROOT = Path(__file__).resolve().parents[2]
PLAYER = str(ROOT / "apps/player")
IMPORT = "github.com/MikeO7/kinosail-player"


def fake_run(calls, tests):
    def run(command, **kwargs):
        calls.append(command)
        if command[3:5] == ["list", "./..."]:
            out = "\n".join(f"{IMPORT}/{name}" for name in ("cmd/kinosail", "internal/backup", "internal/server")) + "\n"
        elif command[3:4] == ["list"]:
            out = f"{IMPORT}/internal/server\n"
        elif command[3:5] == ["test", "-list"]:
            out = "\n".join(tests) + "\nok  \tpkg\t0.1s\n"
        else:
            out = ""
        return subprocess.CompletedProcess(command, 0, out, "")
    return run


class GoShardTests(unittest.TestCase):
    def test_every_server_test_runs_once_and_other_packages_ride_the_first_shard(self):
        tests = [f"Test{index}" for index in range(5)] + ["ExampleThing", "FuzzParse"]
        seen = []
        for shard in ("1/2", "2/2"):
            calls = []
            with patch("go_shard.subprocess.run", side_effect=fake_run(calls, tests)):
                plan, chosen = go_shard.commands("apps/player", shard)
            self.assertTrue(all(command[1:3] == ["-C", PLAYER] for command in calls + plan))
            run_pattern = plan[0][plan[0].index("-run") + 1]
            self.assertTrue(run_pattern.startswith("^(") and run_pattern.endswith(")$"))
            names = run_pattern[2:-2].split("|")
            self.assertEqual(len(names), chosen)
            seen.extend(names)
            self.assertEqual(plan[0][-1], "./internal/server")
            self.assertIn("-count=1", plan[0])
            if shard == "1/2":
                self.assertEqual(plan[1][-2:], [f"{IMPORT}/cmd/kinosail", f"{IMPORT}/internal/backup"])
            else:
                self.assertEqual(len(plan), 1)
        self.assertEqual(sorted(seen), sorted(tests))

    def test_invalid_shards_modules_and_empty_shards_fail_before_running_tests(self):
        for shard in ("0/2", "3/2", "2/0", "1-2", "", "x/y", "1/200"):
            with self.subTest(shard=shard), patch("go_shard.subprocess.run") as run:
                with self.assertRaises(ValueError):
                    go_shard.commands("apps/player", shard)
                run.assert_not_called()
        with patch("go_shard.subprocess.run") as run:
            with self.assertRaises(ValueError):
                go_shard.commands("apps/subtitles", "1/2")
            run.assert_not_called()
        with patch("go_shard.subprocess.run", side_effect=fake_run([], ["TestOnly"])):
            with self.assertRaises(ValueError):
                go_shard.commands("apps/player", "2/2")

    def test_quick_mode_delegates_shards_from_the_module_directory(self):
        with tempfile.TemporaryDirectory() as directory:
            log = Path(directory) / "calls"
            fake = Path(directory) / "go"
            fake.write_text('#!/bin/sh\nprintf "%s|%s\\n" "$PWD" "$*" >> "$GO_CALL_LOG"\n'
                            'case "$*" in *"list ./..."*) printf "%s/internal/server\\n" "' + IMPORT + '" ;; *"list ./internal/server"*) printf "%s/internal/server\\n" "' + IMPORT + '" ;; *"test -list"*) printf "TestA\\nTestB\\n" ;; esac\n')
            fake.chmod(0o755)
            env = os.environ | {"PATH": directory + ":" + os.environ["PATH"], "GO_CALL_LOG": str(log),
                                "KINOSAIL_GO_TEST_MODE": "quick", "KINOSAIL_GO_SHARD": "2/2"}
            result = subprocess.run(["bash", str(ROOT / "scripts/ci/test-go.sh"), "player"], env=env,
                                    capture_output=True, text=True, cwd=directory)
            self.assertEqual(result.returncode, 0, result.stderr)
            calls = log.read_text().splitlines()
            self.assertTrue(all(f"-C {PLAYER} " in call for call in calls), calls)
            self.assertIn(f"-C {PLAYER} test -count=1 -run ^(TestB)$ ./internal/server", calls[-1])


if __name__ == "__main__":
    unittest.main()
