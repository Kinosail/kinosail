"""Validated storage and Git primitives for the worktree guard."""

from __future__ import annotations

import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import time


MAX_LEASE_BYTES = 8192
MAX_PATH_BYTES = 4096
MIN_TTL = 60
MAX_TTL = 604800
DEFAULT_TTL = 14400
TASK_RE = re.compile(r"[A-Za-z0-9][A-Za-z0-9._/-]{0,127}\Z")
LEASE_KEYS = {"version", "task", "worktree", "ttl_seconds", "expires_at"}


class GuardError(RuntimeError):
    pass


def git(repo: Path, *args: str, check: bool = True) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["git", "-C", str(repo), *args],
        check=check,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )


def repository(path: str | None = None) -> Path:
    candidate = Path(path or os.getcwd()).resolve()
    result = git(candidate, "rev-parse", "--show-toplevel")
    return Path(result.stdout.strip()).resolve()


def common_dir(repo: Path) -> Path:
    result = git(repo, "rev-parse", "--path-format=absolute", "--git-common-dir")
    return Path(result.stdout.strip()).resolve()


def worktrees(repo: Path) -> list[dict[str, str]]:
    raw = git(repo, "worktree", "list", "--porcelain", "-z").stdout
    entries: list[dict[str, str]] = []
    current: dict[str, str] = {}
    for field in raw.split("\0"):
        if not field:
            if current:
                entries.append(current)
                current = {}
            continue
        key, _, value = field.partition(" ")
        current[key] = value
    if current:
        entries.append(current)
    return entries


def canonical_worktree(value: str | None, repo: Path) -> Path:
    path = Path(value).resolve() if value else repo
    if len(os.fsencode(path)) > MAX_PATH_BYTES:
        raise GuardError("worktree path is too long")
    if str(path) not in {entry["worktree"] for entry in worktrees(repo)}:
        raise GuardError(f"not a registered worktree: {path}")
    return path


def lease_dir(repo: Path) -> Path:
    return common_dir(repo) / "kinosail-worktrees" / "leases"


def lease_path(repo: Path, path: Path) -> Path:
    digest = hashlib.sha256(os.fsencode(str(path))).hexdigest()
    return lease_dir(repo) / f"{digest}.json"


def validate_task(task: str) -> str:
    if not TASK_RE.fullmatch(task):
        raise GuardError("task must be 1-128 ASCII letters, digits, '.', '_', '/', or '-'")
    return task


def validate_ttl(ttl: int) -> int:
    if ttl < MIN_TTL or ttl > MAX_TTL:
        raise GuardError(f"ttl must be between {MIN_TTL} and {MAX_TTL} seconds")
    return ttl


def read_lease(repo: Path, path: Path) -> dict[str, object] | None:
    file = lease_path(repo, path)
    try:
        size = file.stat().st_size
    except FileNotFoundError:
        return None
    if size > MAX_LEASE_BYTES:
        raise GuardError(f"oversized lease for {path}")
    try:
        data = json.loads(file.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        raise GuardError(f"malformed lease for {path}: {exc}") from exc
    if not isinstance(data, dict) or set(data) != LEASE_KEYS:
        raise GuardError(f"lease for {path} has unknown or missing fields")
    if data["version"] != 1 or data["worktree"] != str(path):
        raise GuardError(f"lease identity mismatch for {path}")
    if not isinstance(data["task"], str):
        raise GuardError(f"invalid task in lease for {path}")
    validate_task(data["task"])
    if not isinstance(data["ttl_seconds"], int) or isinstance(data["ttl_seconds"], bool):
        raise GuardError(f"invalid ttl in lease for {path}")
    validate_ttl(data["ttl_seconds"])
    if (
        not isinstance(data["expires_at"], int)
        or isinstance(data["expires_at"], bool)
        or not 0 <= data["expires_at"] <= 2**63 - 1
    ):
        raise GuardError(f"invalid expiry in lease for {path}")
    return data


def write_lease(repo: Path, path: Path, task: str, ttl: int) -> None:
    directory = lease_dir(repo)
    directory.mkdir(mode=0o700, parents=True, exist_ok=True)
    payload = {
        "version": 1,
        "task": task,
        "worktree": str(path),
        "ttl_seconds": ttl,
        "expires_at": int(time.time()) + ttl,
    }
    fd, temporary = tempfile.mkstemp(prefix=".lease-", dir=directory)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as output:
            json.dump(payload, output, separators=(",", ":"), sort_keys=True)
            output.write("\n")
        os.chmod(temporary, 0o600)
        os.replace(temporary, lease_path(repo, path))
    finally:
        Path(temporary).unlink(missing_ok=True)


def primary(entries: list[dict[str, str]]) -> str:
    for entry in entries:
        if entry.get("branch") == "refs/heads/main":
            return entry["worktree"]
    return entries[0]["worktree"]


def is_dirty(path: Path) -> bool:
    return bool(git(path, "status", "--porcelain=v1", "--untracked-files=all").stdout)


def is_merged(repo: Path, head: str, main_head: str) -> bool:
    return git(repo, "merge-base", "--is-ancestor", head, main_head, check=False).returncode == 0
