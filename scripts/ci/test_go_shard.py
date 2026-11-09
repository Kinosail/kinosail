"""Sharded quick Go tests run every listed test exactly once from any working directory."""
import json
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

    def test_measured_weights_balance_the_shards_and_every_test_still_runs_once(self):
        tests = [f"Test{index:02d}" for index in range(40)]
        weights = {"Test00": 90.0, "Test02": 60.0, "Test04": 45.0, "Test06": 30.0, "Test08": 30.0, "TestRemoved": 500.0}
        shards = [go_shard.partition(tests, index, 2, weights) for index in (1, 2)]
        self.assertEqual(sorted(shards[0] + shards[1]), tests)
        self.assertFalse(set(shards[0]) & set(shards[1]))
        loads = [sum(weights.get(name, go_shard.DEFAULT_WEIGHT) for name in shard) for shard in shards]
        self.assertLess(abs(loads[0] - loads[1]), 31)
        alternating = sum(weights.get(name, go_shard.DEFAULT_WEIGHT) for name in tests[::2])
        self.assertGreater(alternating - max(loads), 100)
        self.assertEqual(shards, [go_shard.partition(list(reversed(tests)), index, 2, weights) for index in (1, 2)])
        self.assertEqual(go_shard.partition(tests, 1, 2, {}), tests[::2])
        three = [go_shard.partition(tests, index, 3, weights) for index in (1, 2, 3)]
        self.assertEqual(sorted(sum(three, [])), tests)

    def test_invalid_weight_files_fail_and_the_recorded_file_is_valid(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "weights.json"
            for content in ("[]", '{"apps/player": []}', '{"apps/player": {"TestA": -1}}', '{"apps/player": {"TestA": "slow"}}',
                            '{"apps/player": {"TestA": true}}', '{"apps/player": {"bad name": 2}}', "not json"):
                path.write_text(content)
                with self.subTest(content=content), self.assertRaises(ValueError):
                    go_shard.load_weights("apps/player", path)
            path.write_text('{"apps/other": {"TestA": 3}}')
            self.assertEqual(go_shard.load_weights("apps/player", path), {})
            path.write_text('{"apps/player": {"TestA": 3, "FuzzB": 1.5}}')
            self.assertEqual(go_shard.load_weights("apps/player", path), {"TestA": 3.0, "FuzzB": 1.5})
        recorded = go_shard.load_weights("apps/player")
        self.assertGreater(len(recorded), 10)
        self.assertEqual(list(json.loads(go_shard.WEIGHTS.read_text())["apps/player"]), sorted(recorded))

    def test_recording_reads_json_events_and_verbose_logs_and_discounts_parallel_tests(self):
        events = [{"Action": "pass", "Test": "TestSlow", "Elapsed": 12.34}, {"Action": "pass", "Test": "TestSlow/case", "Elapsed": 12.0},
                  {"Action": "pass", "Test": "TestQuick", "Elapsed": 0.4}, {"Action": "fail", "Test": "TestFailed", "Elapsed": 9.0},
                  {"Action": "run", "Test": "TestSlow"}, {"Action": "pass", "Elapsed": 80.0}, {"Action": "pass", "Test": "FuzzEdge", "Elapsed": 1.0},
                  {"Action": "pause", "Test": "TestShared"}, {"Action": "pass", "Test": "TestShared", "Elapsed": 20.0},
                  {"Action": "pause", "Test": "TestSharedQuick"}, {"Action": "pass", "Test": "TestSharedQuick", "Elapsed": 3.0}]
        lines = [json.dumps(event) for event in events] + ["not json", "", "[]", "7"]
        self.assertEqual(go_shard.record(lines), {"FuzzEdge": 1.0, "TestShared": 5.0, "TestSlow": 12.3})
        log = ["2026-11-09T22:45:59.1Z === RUN   TestSerial", "2026-11-09T22:46:59.1Z --- PASS: TestSerial (60.04s)",
               "2026-11-09T22:46:59.1Z === PAUSE TestWide", "2026-11-09T22:47:59.1Z --- PASS: TestWide (40.00s)",
               "2026-11-09T22:47:59.1Z     --- PASS: TestSerial/case (55.00s)", "2026-11-09T22:47:59.1Z --- FAIL: TestBroken (30.00s)",
               "2026-11-09T22:47:59.1Z --- PASS: TestTiny (0.20s)", "2026-11-09T22:47:59.1Z --- SKIP: TestSkipped (9.00s)"]
        self.assertEqual(go_shard.record(log), {"TestSerial": 60.0, "TestWide": 10.0})

    def test_timing_mode_adds_verbose_output_to_the_heavy_package_only(self):
        for value, verbose in (("1", True), ("", False), ("true", False)):
            with self.subTest(value=value), patch.dict(os.environ, {"KINOSAIL_GO_TEST_TIMINGS": value}), \
                    patch("go_shard.subprocess.run", side_effect=fake_run([], ["TestA", "TestB"])):
                plan, _ = go_shard.commands("apps/player", "1/2")
            self.assertEqual("-v" in plan[0], verbose)
            self.assertNotIn("-v", plan[1])

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
