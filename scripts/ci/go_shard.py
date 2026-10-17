#!/usr/bin/env python3
"""Split one Go module's quick tests across runners; every test runs exactly once and an empty shard fails."""

import argparse
import re
import subprocess
import sys

HEAVY = {"apps/player": "./internal/server"}
NAME = re.compile(r"^(Test|Example|Fuzz)[A-Za-z0-9_]*$")


def parse_shard(value):
    match = re.fullmatch(r"([1-9][0-9]?)/([1-9][0-9]?)", value or "")
    if not match or int(match.group(1)) > int(match.group(2)):
        raise ValueError("shard must look like index/count with 1 <= index <= count")
    return int(match.group(1)), int(match.group(2))


def list_tests(directory, package):
    result = subprocess.run(["go", "-C", directory, "test", "-list", ".*", package],
                            capture_output=True, text=True, check=True, timeout=1800)
    names = [line.strip() for line in result.stdout.splitlines()]
    tests = sorted({name for name in names if NAME.fullmatch(name)})
    if not tests:
        raise ValueError(f"{package} lists no tests")
    return tests


def partition(tests, index, count):
    chosen = [name for position, name in enumerate(tests) if position % count == index - 1]
    if not chosen:
        raise ValueError("shard would run no tests")
    return chosen


def commands(directory, shard):
    index, count = parse_shard(shard)
    heavy = HEAVY.get(directory)
    if heavy is None:
        raise ValueError(f"{directory} has no sharding plan")
    packages = subprocess.run(["go", "-C", directory, "list", "./..."], capture_output=True, text=True,
                              check=True, timeout=600).stdout.split()
    heavy_import = subprocess.run(["go", "-C", directory, "list", heavy], capture_output=True, text=True,
                                  check=True, timeout=600).stdout.strip()
    if heavy_import not in packages:
        raise ValueError(f"{heavy} is not a package of {directory}")
    chosen = partition(list_tests(directory, heavy), index, count)
    pattern = "^(" + "|".join(re.escape(name) for name in chosen) + ")$"
    plan = [["go", "-C", directory, "test", "-count=1", "-run", pattern, heavy]]
    if index == 1:
        others = [package for package in packages if package != heavy_import]
        if others:
            plan.append(["go", "-C", directory, "test", "-count=1", *others])
    return plan, len(chosen)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("directory")
    parser.add_argument("shard")
    args = parser.parse_args()
    plan, chosen = commands(args.directory, args.shard)
    print(f"shard {args.shard}: {chosen} heavy tests, {len(plan)} go test invocations", flush=True)
    for command in plan:
        shown = [part if len(part) < 120 else part[:117] + "..." for part in command]
        print("+", " ".join(shown), flush=True)
        subprocess.run(command, check=True)


if __name__ == "__main__":
    main()
