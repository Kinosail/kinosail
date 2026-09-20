#!/usr/bin/env python3
from concurrent.futures import ThreadPoolExecutor
import json
import os
from pathlib import Path

if __name__ == "__main__" and (Path(__file__).resolve().parents[2] / ".gates-disabled").is_file():
    print("Quality gates are disabled until explicitly enabled (.gates-disabled).")
    raise SystemExit(0)
import subprocess
import sys


LIMIT = 80.0
TARGETS = {"darwin", "linux", "windows"}


def target_for(path: str) -> str | None:
    stem = Path(path).stem
    suffix = stem.rsplit("_", 1)[-1]
    if suffix == "other":
        return "windows"
    return suffix if suffix in TARGETS else None


def source_files(repo: Path) -> list[str]:
    output = subprocess.check_output(
        ["git", "ls-files", "--cached", "--others", "--exclude-standard", "*.go"], cwd=repo, text=True
    )
    return [
        path
        for path in output.splitlines()
        if "/third_party/" not in path
        and not path.endswith("_test.go")
        and (repo / path).is_file()
    ]


def report(tool: str, repo: Path, paths: list[str]) -> dict:
    path = paths[0]
    environment = os.environ.copy()
    target = target_for(path)
    if target:
        environment.update({"CGO_ENABLED": "0", "GOARCH": "amd64", "GOOS": target})
    result = subprocess.run(
        [tool, "halstead-package", *paths],
        cwd=repo,
        env=environment,
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode:
        detail = result.stderr.strip() or result.stdout.strip()
        raise RuntimeError(f"{path}: {detail}")
    return json.loads(result.stdout)


def main() -> int:
    if len(sys.argv) != 3:
        print("usage: check-halstead.py TOOL REPOSITORY", file=sys.stderr)
        return 2
    tool, root = sys.argv[1:]
    repo = Path(root).resolve()
    paths = source_files(repo)

    def analyze(group: list[str]) -> list[str]:
        try:
            analyses = report(tool, repo, group)
        except (RuntimeError, json.JSONDecodeError) as error:
            return [str(error)]
        failures = []
        for path in group:
            analysis = analyses[path]
            for function in analysis.get("functions") or []:
                difficulty = function["metrics"]["difficulty"]
                if difficulty >= LIMIT:
                    line = function["start"]["line"]
                    failures.append(
                        f"{path}:{line}: {function['name']} has Halstead difficulty "
                        f"{difficulty:.2f}; maximum is less than {LIMIT:.0f}"
                    )
        return failures

    groups = {}
    for path in paths:
        groups.setdefault((str(Path(path).parent), target_for(path)), []).append(path)
    if not groups:
        return 0
    worker_count = min(4, len(groups))
    with ThreadPoolExecutor(max_workers=worker_count) as executor:
        failures = [failure for result in executor.map(analyze, groups.values()) for failure in result]
    if failures:
        print("\n".join(failures), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
