#!/usr/bin/env python3
"""Split one Go module's quick tests across runners by measured weight; every test runs exactly once and an empty shard fails.

Refresh the weights from a hosted run: dispatch ci.yml, which sets KINOSAIL_GO_TEST_TIMINGS=1, then pipe both
Go (player) job logs into: python3 scripts/ci/go_shard.py --record apps/player
"""

import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[2]
HEAVY = {"apps/player": "./internal/server"}
NAME = re.compile(r"^(Test|Example|Fuzz)[A-Za-z0-9_]*$")
WEIGHTS = Path(__file__).with_name("go_test_weights.json")
DEFAULT_WEIGHT = 0.3
RECORD_MINIMUM = 1.0
VERBOSE = re.compile(r"(?:^|\s)(=== PAUSE|--- PASS:|--- SKIP:) ((?:Test|Example|Fuzz)[A-Za-z0-9_]*)(?: \((\d+(?:\.\d+)?)s\))?\s*$")


def parse_shard(value):
    match = re.fullmatch(r"([1-9][0-9]?)/([1-9][0-9]?)", value or "")
    if not match or int(match.group(1)) > int(match.group(2)):
        raise ValueError("shard must look like index/count with 1 <= index <= count")
    return int(match.group(1)), int(match.group(2))


def list_tests(directory, package):
    result = subprocess.run(["go", "-C", directory, "test", "-list", ".*", package],
                            stdout=subprocess.PIPE, text=True, check=True, timeout=1800)
    names = [line.strip() for line in result.stdout.splitlines()]
    tests = sorted({name for name in names if NAME.fullmatch(name)})
    if not tests:
        raise ValueError(f"{package} lists no tests")
    return tests


def load_weights(module, path=WEIGHTS):
    try:
        data = json.loads(Path(path).read_text())
    except (OSError, ValueError) as error:
        raise ValueError(f"cannot read test weights: {error}") from error
    if not isinstance(data, dict) or not isinstance(data.get(module, {}), dict):
        raise ValueError("test weights must map a module to test names and seconds")
    weights = {}
    for name, seconds in data.get(module, {}).items():
        if not NAME.fullmatch(name) or isinstance(seconds, bool) or not isinstance(seconds, (int, float)) or seconds <= 0:
            raise ValueError(f"invalid test weight for {name!r}")
        weights[name] = float(seconds)
    return weights


def partition(tests, index, count, weights=None):
    weights = weights or {}
    loads, shards = [0.0] * count, [[] for _ in range(count)]
    for name in sorted(tests, key=lambda name: (-weights.get(name, DEFAULT_WEIGHT), name)):
        lightest = min(range(count), key=lambda shard: (loads[shard], shard))
        loads[lightest] += weights.get(name, DEFAULT_WEIGHT)
        shards[lightest].append(name)
    chosen = sorted(shards[index - 1])
    if not chosen:
        raise ValueError("shard would run no tests")
    return chosen


def record(lines, cores=4):
    seconds, parallel = {}, set()
    for line in lines:
        match = VERBOSE.search(line)
        if match:
            if match.group(1) == "=== PAUSE":
                parallel.add(match.group(2))
            elif match.group(3):
                seconds.setdefault(match.group(2), 0.0 if match.group(1) == "--- SKIP:" else float(match.group(3)))
            continue
        try:
            event = json.loads(line)
        except ValueError:
            continue
        if not isinstance(event, dict) or not NAME.fullmatch(str(event.get("Test", ""))):
            continue
        if event.get("Action") == "pause":
            parallel.add(event["Test"])
        elif event.get("Action") in ("pass", "skip"):
            seconds.setdefault(event["Test"], 0.0 if event["Action"] == "skip" else float(event.get("Elapsed", 0)))
    weights = {name: round(value / (cores if name in parallel else 1), 1) for name, value in seconds.items()}
    return {name: value for name, value in sorted(weights.items()) if value >= RECORD_MINIMUM}


def commands(module, shard):
    index, count = parse_shard(shard)
    heavy = HEAVY.get(module)
    if heavy is None:
        raise ValueError(f"{module} has no sharding plan")
    weights = load_weights(module)
    directory = str(ROOT / module)
    packages = subprocess.run(["go", "-C", directory, "list", "./..."], stdout=subprocess.PIPE, text=True,
                              check=True, timeout=600).stdout.split()
    heavy_import = subprocess.run(["go", "-C", directory, "list", heavy], stdout=subprocess.PIPE, text=True,
                                  check=True, timeout=600).stdout.strip()
    if heavy_import not in packages:
        raise ValueError(f"{heavy} is not a package of {module}")
    chosen = partition(list_tests(directory, heavy), index, count, weights)
    pattern = "^(" + "|".join(re.escape(name) for name in chosen) + ")$"
    timings = ["-v"] if os.environ.get("KINOSAIL_GO_TEST_TIMINGS") == "1" else []
    plan = [["go", "-C", directory, "test", "-count=1", *timings, "-run", pattern, heavy]]
    if index == 1:
        others = [package for package in packages if package != heavy_import]
        if others:
            plan.append(["go", "-C", directory, "test", "-count=1", *others])
    return plan, len(chosen)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--record", action="store_true", help="read go test -json from standard input and store the weights")
    parser.add_argument("module")
    parser.add_argument("shard", nargs="?", default="")
    args = parser.parse_args()
    if args.record:
        if args.module not in HEAVY:
            raise ValueError(f"{args.module} has no sharding plan")
        data = json.loads(WEIGHTS.read_text()) if WEIGHTS.exists() else {}
        data[args.module] = record(sys.stdin)
        WEIGHTS.write_text(json.dumps(data, indent=1, sort_keys=True) + "\n")
        print(f"recorded {len(data[args.module])} weights for {args.module}")
        return
    plan, chosen = commands(args.module, args.shard)
    print(f"shard {args.shard}: {chosen} heavy tests, {len(plan)} go test invocations", flush=True)
    for command in plan:
        shown = [part if len(part) < 120 else part[:117] + "..." for part in command]
        print("+", " ".join(shown), flush=True)
        subprocess.run(command, check=True)


if __name__ == "__main__":
    main()
