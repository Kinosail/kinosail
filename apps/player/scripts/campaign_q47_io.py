#!/usr/bin/env python3
"""Bounded descriptor-owned reads and fingerprints of known proof inputs."""
from contextlib import contextmanager
import hashlib
import os
import stat
from campaign_q47_execution import check_budget


def stable(details):
    return (details.st_dev, details.st_ino, details.st_mode, details.st_size,
            details.st_mtime_ns, details.st_ctime_ns)


@contextmanager
def descriptor(path, limit, mode=None):
    check_budget()
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    try:
        details = os.fstat(fd)
        if (not stat.S_ISREG(details.st_mode) or not 0 <= details.st_size <= limit
                or mode is not None and details.st_mode & 0o777 != mode):
            raise ValueError("descriptor_boundary")
        yield fd, details
        if stable(os.fstat(fd)) != stable(details):
            raise ValueError("descriptor_changed")
    finally:
        # A close failure is a failed prerequisite, including during another failure.
        os.close(fd)


def read_bounded(path, limit, mode=None):
    data = bytearray()
    with descriptor(path, limit, mode) as (fd, details):
        while len(data) <= limit:
            check_budget()
            chunk = os.read(fd, min(65536, limit + 1 - len(data)))
            if not chunk:
                break
            data.extend(chunk)
        if len(data) > limit or len(data) != details.st_size:
            raise ValueError("read_boundary")
    return bytes(data)


def fingerprint(path, limit=32 * 1024 * 1024):
    check_budget()
    initial = path.lstat()
    digest = hashlib.sha256()
    total = 0
    with descriptor(path, limit) as (fd, details):
        if stable(details) != stable(initial):
            raise ValueError("file_changed")
        blob = hashlib.sha1(b"blob " + str(details.st_size).encode() + b"\0")
        while True:
            check_budget()
            chunk = os.read(fd, min(65536, limit + 1 - total))
            if not chunk:
                break
            total += len(chunk)
            if total > limit:
                raise ValueError("file_bound")
            digest.update(chunk); blob.update(chunk)
        if total != details.st_size:
            raise ValueError("file_changed")
    if stable(path.lstat()) != stable(initial):
        raise ValueError("file_changed")
    return {"bytes": total, "sha256": digest.hexdigest(), "gitBlob": blob.hexdigest()}



@contextmanager
def directory_descriptor(path):
    check_budget()
    fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_NONBLOCK)
    try:
        details = os.fstat(fd)
        if not stat.S_ISDIR(details.st_mode):
            raise ValueError("directory_boundary")
        yield fd
        if stable(os.fstat(fd)) != stable(details):
            raise ValueError("directory_changed")
    finally:
        os.close(fd)


def bounded_paths(root, maximum):
    if type(maximum) is not int or not 1 <= maximum <= 20_000:
        raise ValueError("tree_limit")
    pending, count = [root], 0
    while pending:
        check_budget()
        directory = pending.pop()
        with directory_descriptor(directory) as fd, os.scandir(fd) as entries:
            for entry in entries:
                check_budget()
                count += 1
                if count > maximum:
                    raise ValueError("tree_entry_bound")
                if type(entry.name) is not str or entry.name in ("", ".", "..") or "/" in entry.name:
                    raise ValueError("tree_name")
                path = directory / entry.name
                relative = path.relative_to(root).as_posix()
                if len(relative) > 512 or any(char in relative for char in "\n\r\t\0"):
                    raise ValueError("tree_name")
                if entry.is_dir(follow_symlinks=False):
                    if len(pending) >= maximum:
                        raise ValueError("tree_pending_bound")
                    pending.append(path)
                yield path
