"""Descriptor-owned input reads and incremental tool inventory bounds."""
import os
from pathlib import Path
import re
import stat
import time

FLAGS = os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC
MAX_TOOL_COUNT = 128
MAX_TOOL_BYTES = 512 * 1024 * 1024


def tick(deadline):
    if deadline is not None and time.monotonic() >= deadline:
        raise ValueError("input-deadline")


def identity(info):
    return (info.st_dev, info.st_ino, info.st_mode, info.st_size, info.st_mtime_ns,
            info.st_ctime_ns, info.st_uid, info.st_gid, info.st_nlink)


def checked_close(*descriptors):
    uncertain = False
    for descriptor in descriptors:
        try:
            os.close(descriptor)
        except OSError:
            uncertain = True
    if uncertain:
        raise ValueError("descriptor-close-unconfirmed")


def open_directory(path, deadline=None):
    path = Path(path)
    if not path.is_absolute() or ".." in path.parts or len(path.parts) > 128:
        raise ValueError("directory-path-boundary")
    tick(deadline)
    descriptor = os.open(path.anchor, FLAGS | os.O_DIRECTORY)
    try:
        for part in path.parts[1:]:
            tick(deadline)
            newer = os.open(part, FLAGS | os.O_DIRECTORY, dir_fd=descriptor)
            older, descriptor = descriptor, newer
            checked_close(older)
        tick(deadline)
        return descriptor
    except BaseException:
        checked_close(descriptor)
        raise


def open_regular(path, deadline=None):
    path = Path(path)
    if not path.name or path.name in (".", ".."):
        raise ValueError("regular-path-boundary")
    parent = open_directory(path.parent, deadline)
    try:
        tick(deadline)
        descriptor = os.open(path.name, FLAGS, dir_fd=parent)
        return descriptor, parent, path.name
    except BaseException:
        checked_close(parent)
        raise


def regular(info, limit):
    if not stat.S_ISREG(info.st_mode) or type(info.st_size) is not int or not 0 <= info.st_size <= limit:
        raise ValueError("regular-size-boundary")


def verify_path(path, expected, deadline):
    descriptor, parent, leaf = open_regular(path, deadline)
    try:
        tick(deadline)
        observed = os.fstat(descriptor)
        named = os.stat(leaf, dir_fd=parent, follow_symlinks=False)
        if identity(observed) != identity(expected) or identity(named) != identity(expected):
            raise ValueError("named-input-changed")
    finally:
        checked_close(descriptor, parent)


def stream_regular(path, limit, deadline, begin, consume):
    if type(limit) is not int or not 0 <= limit <= 128 * 1024 * 1024:
        raise ValueError("input-limit")
    tick(deadline)
    descriptor, parent, leaf = open_regular(path, deadline)
    try:
        before = os.fstat(descriptor)
        regular(before, limit)
        named = os.stat(leaf, dir_fd=parent, follow_symlinks=False)
        if identity(named) != identity(before):
            raise ValueError("opened-input-changed")
        tick(deadline)
        begin(before)
        count = 0
        while True:
            tick(deadline)
            data = os.read(descriptor, min(65536, limit - count + 1))
            tick(deadline)
            if not data:
                break
            count += len(data)
            if count > limit or count > before.st_size:
                raise ValueError("input-growth")
            consume(data)
        after = os.fstat(descriptor)
        named = os.stat(leaf, dir_fd=parent, follow_symlinks=False)
        if count != before.st_size or identity(after) != identity(before) or identity(named) != identity(before):
            raise ValueError("descriptor-input-changed")
        verify_path(path, before, deadline)
        tick(deadline)
        return before
    finally:
        checked_close(descriptor, parent)


def read_small(path, limit, deadline=None):
    chunks = []
    stream_regular(path, limit, deadline, lambda _info: None, chunks.append)
    return b"".join(chunks)


class Budget:
    def __init__(self, deadline, max_count=MAX_TOOL_COUNT, max_bytes=MAX_TOOL_BYTES):
        if type(max_count) is not int or not 1 <= max_count <= MAX_TOOL_COUNT:
            raise ValueError("tool-count-limit")
        if type(max_bytes) is not int or not 0 <= max_bytes <= MAX_TOOL_BYTES:
            raise ValueError("tool-byte-limit")
        self.deadline, self.max_count, self.max_bytes = deadline, max_count, max_bytes
        self.count, self.bytes = 0, 0

    def reserve(self, size):
        tick(self.deadline)
        if type(size) is not int or size < 0 or self.count + 1 > self.max_count or self.bytes + size > self.max_bytes:
            raise ValueError("tool-inventory-budget")
        self.count += 1
        self.bytes += size


def entry_name(name):
    if not isinstance(name, str) or not re.fullmatch(r"[A-Za-z0-9_.-]{1,255}", name) or name in (".", ".."):
        raise ValueError("tool-entry-name")
    return name


def iter_tool_paths(directory, deadline):
    tick(deadline)
    top = open_directory(directory, deadline)
    roots, files = 0, 0
    try:
        with os.scandir(top) as entries:
            for entry in entries:
                tick(deadline)
                roots += 1
                if roots > 32:
                    raise ValueError("tool-directory-count")
                name = entry_name(entry.name)
                before = os.stat(name, dir_fd=top, follow_symlinks=False)
                if stat.S_ISREG(before.st_mode):
                    continue
                if not stat.S_ISDIR(before.st_mode):
                    raise ValueError("tool-directory-shape")
                child = os.open(name, FLAGS | os.O_DIRECTORY, dir_fd=top)
                try:
                    if identity(os.fstat(child)) != identity(before):
                        raise ValueError("tool-directory-changed")
                    with os.scandir(child) as leaves:
                        for leaf in leaves:
                            tick(deadline)
                            files += 1
                            if files > MAX_TOOL_COUNT:
                                raise ValueError("tool-entry-count")
                            filename = entry_name(leaf.name)
                            info = os.stat(filename, dir_fd=child, follow_symlinks=False)
                            if not stat.S_ISREG(info.st_mode):
                                raise ValueError("tool-entry-shape")
                            yield directory / name / filename
                    tick(deadline)
                finally:
                    checked_close(child)
        tick(deadline)
    finally:
        checked_close(top)
