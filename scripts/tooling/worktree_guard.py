#!/usr/bin/env python3
"""Lease, audit, safely clean, and integrate Kinosail Git worktrees."""

from __future__ import annotations

import argparse
import fcntl
import os
from pathlib import Path
import subprocess
import sys
import time

sys.dont_write_bytecode = True

from worktree_guard_lib import (
    DEFAULT_TTL,
    GuardError,
    canonical_worktree,
    common_dir,
    git,
    is_dirty,
    is_merged,
    lease_path,
    primary,
    read_lease,
    repository,
    validate_task,
    validate_ttl,
    worktrees,
    write_lease,
)


def lease_command(args: argparse.Namespace) -> None:
    repo = repository(args.repo)
    path = canonical_worktree(args.worktree, repo)
    task = validate_task(args.task)
    ttl = validate_ttl(args.ttl)
    existing = read_lease(repo, path)
    if existing and existing["task"] != task and not args.replace:
        raise GuardError(f"worktree is leased to {existing['task']}; use --replace explicitly")
    write_lease(repo, path, task, ttl)
    print(f"leased {path} to {task} for {ttl}s")


def heartbeat_command(args: argparse.Namespace) -> None:
    repo = repository(args.repo)
    path = canonical_worktree(args.worktree, repo)
    entries = worktrees(repo)
    if str(path) == primary(entries):
        print(f"primary worktree needs no lease: {path}")
        return
    lease = read_lease(repo, path)
    if lease is None:
        if args.if_present:
            raise GuardError(f"active secondary worktree has no lease: {path}")
        raise GuardError(f"no lease for {path}")
    if args.task is not None and lease["task"] != validate_task(args.task):
        raise GuardError(f"lease belongs to {lease['task']}, not {args.task}")
    write_lease(repo, path, str(lease["task"]), int(lease["ttl_seconds"]))
    print(f"renewed lease for {path}")


def release_command(args: argparse.Namespace) -> None:
    repo = repository(args.repo)
    path = canonical_worktree(args.worktree, repo)
    lease = read_lease(repo, path)
    if lease is None:
        if args.missing_ok:
            return
        raise GuardError(f"no lease for {path}")
    if lease["task"] != validate_task(args.task):
        raise GuardError(f"lease belongs to {lease['task']}, not {args.task}")
    lease_path(repo, path).unlink()
    print(f"released lease for {path}")


def inactive(repo: Path) -> tuple[list[tuple[dict[str, str], str]], list[str]]:
    entries = worktrees(repo)
    main_path = Path(primary(entries))
    main_head = git(main_path, "rev-parse", "HEAD").stdout.strip()
    failures: list[tuple[dict[str, str], str]] = []
    active: list[str] = []
    now = int(time.time())
    for entry in entries:
        path = Path(entry["worktree"])
        if str(path) == str(main_path):
            continue
        try:
            lease = read_lease(repo, path)
        except GuardError as exc:
            failures.append((entry, str(exc)))
            continue
        if lease is not None and int(lease["expires_at"]) > now:
            active.append(f"{path} ({lease['task']})")
            continue
        status = "expired lease" if lease else "unleased"
        if "prunable" in entry:
            state = "prunable"
        elif is_dirty(path):
            state = "dirty"
        elif is_merged(repo, entry["HEAD"], main_head):
            state = "clean and merged"
        else:
            state = "clean with unique commits"
        failures.append((entry, f"{status}; {state}"))
    return failures, active


def audit_command(args: argparse.Namespace) -> None:
    repo = repository(args.repo)
    failures, active = inactive(repo)
    for item in active:
        print(f"active: {item}")
    if failures:
        for entry, reason in failures:
            print(f"inactive: {entry['worktree']}: {reason}", file=sys.stderr)
        raise GuardError(f"worktree audit failed with {len(failures)} inactive worktree(s)")
    print("worktree audit passed")


def cleanup_command(args: argparse.Namespace) -> None:
    repo = repository(args.repo)
    entries = worktrees(repo)
    main_path = Path(primary(entries))
    main_head = git(main_path, "rev-parse", "HEAD").stdout.strip()
    removed = 0
    refused = 0
    now = int(time.time())
    prunable = [entry for entry in entries if "prunable" in entry]
    safe_prunable: list[tuple[dict[str, str], Path]] = []
    for entry in prunable:
        path = Path(entry["worktree"])
        try:
            lease = read_lease(repo, path)
            active = lease is not None and int(lease["expires_at"]) > now
        except GuardError as exc:
            print(f"refused: {path}: {exc}", file=sys.stderr)
            refused += 1
            continue
        if active or not is_merged(repo, entry["HEAD"], main_head):
            reason = "active lease" if active else "unique commits"
            print(f"refused: {path}: {reason}", file=sys.stderr)
            refused += 1
            continue
        safe_prunable.append((entry, path))
    if safe_prunable and len(safe_prunable) == len(prunable):
        git(repo, "worktree", "prune")
        for _, path in safe_prunable:
            lease_path(main_path, path).unlink(missing_ok=True)
            print(f"removed: {path}")
            removed += 1

    for entry in entries:
        path = Path(entry["worktree"])
        if path == main_path or "prunable" in entry:
            continue
        try:
            lease = read_lease(repo, path)
        except GuardError as exc:
            print(f"refused: {path}: {exc}", file=sys.stderr)
            refused += 1
            continue
        if lease is not None and int(lease["expires_at"]) > now:
            continue
        if not is_merged(repo, entry["HEAD"], main_head):
            print(f"refused: {path}: unique commits", file=sys.stderr)
            refused += 1
            continue
        if is_dirty(path):
            print(f"refused: {path}: dirty", file=sys.stderr)
            refused += 1
            continue
        else:
            git(repo, "worktree", "remove", str(path))
        lease_path(main_path, path).unlink(missing_ok=True)
        print(f"removed: {path}")
        removed += 1
    print(f"worktree cleanup: removed={removed} refused={refused}")
    if refused:
        raise GuardError("unsafe worktrees were preserved")


def finish_command(args: argparse.Namespace) -> None:
    repo = repository(args.repo)
    path = canonical_worktree(args.worktree, repo)
    entries = worktrees(repo)
    main_path = Path(primary(entries))
    if path == main_path:
        raise GuardError("finish must run for a secondary worktree")
    task = validate_task(args.task)
    lease = read_lease(repo, path)
    if lease is None or lease["task"] != task:
        raise GuardError("finish requires the matching active lease")
    if is_dirty(path):
        raise GuardError("commit or discard task-owned changes before finish")
    branch_ref = next((e.get("branch") for e in entries if e["worktree"] == str(path)), None)
    if not branch_ref or not branch_ref.startswith("refs/heads/"):
        raise GuardError("finish requires a named local branch")
    branch = branch_ref.removeprefix("refs/heads/")
    state_root = common_dir(repo) / "kinosail-worktrees"
    state_root.mkdir(mode=0o700, parents=True, exist_ok=True)
    lock = state_root / "integration.lock"
    lock_handle = lock.open("a+", encoding="utf-8")
    os.chmod(lock, 0o600)
    try:
        fcntl.flock(lock_handle, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except BlockingIOError as exc:
        lock_handle.close()
        raise GuardError("another main integration is active") from exc
    try:
        if is_dirty(main_path):
            raise GuardError("primary main worktree is dirty")
        if git(main_path, "remote", "get-url", "origin", check=False).returncode == 0:
            git(main_path, "fetch", "--quiet", "origin", "main")
        main_head = git(main_path, "rev-parse", "HEAD").stdout.strip()
        remote = git(main_path, "rev-parse", "--verify", "origin/main", check=False)
        task_head = git(path, "rev-parse", "HEAD").stdout.strip()
        if remote.returncode == 0 and is_merged(repo, task_head, remote.stdout.strip()):
            if not is_merged(repo, main_head, remote.stdout.strip()):
                raise GuardError("local main has unique commits; reconcile it first")
            target = "origin/main"
        else:
            if remote.returncode == 0 and not is_merged(repo, remote.stdout.strip(), main_head):
                raise GuardError("local main is behind origin/main; reconcile it first")
            if not is_merged(repo, main_head, task_head):
                raise GuardError("task branch is not based on current main; rebase it and retry")
            target = branch
        if not (path / ".gates-disabled").is_file():
            subprocess.run(["make", "-C", str(path), "tooling-check"], check=True)
        git(main_path, "merge", "--ff-only", target)
        os.chdir(main_path)
        git(main_path, "worktree", "remove", str(path))
        lease_path(main_path, path).unlink(missing_ok=True)
        git(main_path, "branch", "-d", branch)
        print(f"integrated {task_head} into local main and removed {path}")
    finally:
        fcntl.flock(lock_handle, fcntl.LOCK_UN)
        lock_handle.close()


def parser() -> argparse.ArgumentParser:
    result = argparse.ArgumentParser(allow_abbrev=False)
    result.add_argument("--repo")
    commands = result.add_subparsers(dest="command", required=True)

    lease = commands.add_parser("lease", allow_abbrev=False)
    lease.add_argument("--task", required=True)
    lease.add_argument("--ttl", type=int, default=DEFAULT_TTL)
    lease.add_argument("--worktree")
    lease.add_argument("--replace", action="store_true")
    lease.set_defaults(run=lease_command)

    heartbeat = commands.add_parser("heartbeat", allow_abbrev=False)
    heartbeat.add_argument("--task")
    heartbeat.add_argument("--worktree")
    heartbeat.add_argument("--if-present", action="store_true")
    heartbeat.set_defaults(run=heartbeat_command)

    release = commands.add_parser("release", allow_abbrev=False)
    release.add_argument("--task", required=True)
    release.add_argument("--worktree")
    release.add_argument("--missing-ok", action="store_true")
    release.set_defaults(run=release_command)

    audit = commands.add_parser("audit", allow_abbrev=False)
    audit.set_defaults(run=audit_command)

    cleanup = commands.add_parser("cleanup", allow_abbrev=False)
    cleanup.set_defaults(run=cleanup_command)

    finish = commands.add_parser("finish", allow_abbrev=False)
    finish.add_argument("--task", required=True)
    finish.add_argument("--worktree")
    finish.set_defaults(run=finish_command)
    return result


def main() -> int:
    try:
        args = parser().parse_args()
        args.run(args)
    except (GuardError, subprocess.CalledProcessError) as exc:
        print(f"worktree guard: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
