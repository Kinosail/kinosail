"""Only four bounded private reads and exclusive fixed publication; no cleanup."""
import os
from pathlib import Path
import stat

from campaign_r18_document_inputs import (
    FLAGS, checked_close, identity, open_directory, read_small, tick,
)

NAMES = ("receipt.json", "results.json", "source-manifest.json", "artifact-manifest.json")
CAPS = dict(zip(NAMES, (512 * 1024, 128 * 1024, 16 * 1024 * 1024, 4096), strict=True))


def read_bundle(directory, deadline):
    directory = Path(directory)
    descriptor = open_directory(directory, deadline)
    newer = None
    try:
        before = os.fstat(descriptor)
        names = set()
        with os.scandir(descriptor) as entries:
            for entry in entries:
                tick(deadline)
                if entry.name not in NAMES or entry.name in names or len(names) >= 4:
                    raise ValueError("artifact-name-boundary")
                info = os.stat(entry.name, dir_fd=descriptor, follow_symlinks=False)
                if not stat.S_ISREG(info.st_mode) or not 0 < info.st_size <= CAPS[entry.name]:
                    raise ValueError("artifact-regular-boundary")
                names.add(entry.name)
        if names != set(NAMES):
            raise ValueError("artifact-incomplete")
        raw = {name: read_small(directory / name, CAPS[name], deadline) for name in NAMES}
        newer = open_directory(directory, deadline)
        if identity(os.fstat(descriptor)) != identity(before) or identity(os.fstat(newer)) != identity(before):
            raise ValueError("artifact-directory-drift")
        tick(deadline)
        return raw
    finally:
        checked_close(descriptor, *(() if newer is None else (newer,)))


def fresh_output(temp, deadline, token):
    temp = Path(temp)
    descriptor = open_directory(temp, deadline)
    try:
        for _attempt in range(8):
            tick(deadline)
            value = token()
            if type(value) is not str or len(value) != 32 or any(c not in "0123456789abcdef" for c in value):
                raise ValueError("private-token-boundary")
            name = "r18-document-format-" + value
            try:
                os.stat(name, dir_fd=descriptor, follow_symlinks=False)
            except FileNotFoundError:
                return temp / name
        raise ValueError("private-name-unavailable")
    finally:
        checked_close(descriptor)


def publish(raw, root, deadline):
    if type(raw) is not dict or set(raw) != set(NAMES):
        raise ValueError("publication-allowlist")
    parent = open_directory(Path(root), deadline)
    output, directory = None, Path(root) / ".verification/campaign-proof/R18"
    try:
        for name in (".verification", "campaign-proof"):
            tick(deadline)
            try:
                os.mkdir(name, mode=0o700, dir_fd=parent)
            except FileExistsError:
                pass
            newer = os.open(name, FLAGS | os.O_DIRECTORY, dir_fd=parent)
            older, parent = parent, newer
            checked_close(older)
        tick(deadline)
        os.mkdir("R18", mode=0o700, dir_fd=parent)
        output = os.open("R18", FLAGS | os.O_DIRECTORY, dir_fd=parent)
        for name in NAMES:
            data = raw[name]
            if type(data) is not bytes or not 0 < len(data) <= CAPS[name]:
                raise ValueError("publication-byte-boundary")
            tick(deadline)
            descriptor = os.open(name, os.O_WRONLY | os.O_CREAT | os.O_EXCL |
                                 os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC, 0o600, dir_fd=output)
            try:
                sent = 0
                while sent < len(data):
                    tick(deadline)
                    count = os.write(descriptor, data[sent:sent + 65536])
                    if count <= 0:
                        raise ValueError("publication-write-incomplete")
                    sent += count
                info = os.fstat(descriptor)
                if not stat.S_ISREG(info.st_mode) or info.st_size != len(data):
                    raise ValueError("publication-file-boundary")
            finally:
                checked_close(descriptor)
        tick(deadline)
        fresh = open_directory(directory, deadline)
        try:
            if identity(os.fstat(fresh)) != identity(os.fstat(output)):
                raise ValueError("publication-directory-drift")
        finally:
            checked_close(fresh)
        if read_bundle(directory, deadline) != raw:
            raise ValueError("publication-content-drift")
        return directory
    finally:
        checked_close(parent, *(() if output is None else (output,)))
