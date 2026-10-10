#!/usr/bin/env python3
"""Capture the bounded, nonignored build inputs of one local application."""
import hashlib
import os
from pathlib import Path
import stat
import subprocess
import sys

APPS = ("player", "subtitles")
LICENSES = ("LICENSE", "LICENSING.md", "THIRD_PARTY_NOTICES.md")


def capture(repo, app, destination=None):
    if app not in APPS:
        raise ValueError("unknown app")
    repo = Path(repo).resolve(strict=True)
    roots = [f"apps/{app}/{part}" for part in ("cmd", "internal", "third_party", "Containerfile",
                                              "go.mod", "go.sum", *LICENSES)] + ["packages"]
    listing = subprocess.check_output(["git", "-C", str(repo), "ls-files", "-z", "--cached",
                                       "--others", "--exclude-standard", "--", *roots])
    names = sorted(set(filter(None, listing.decode().split("\0"))))
    if len(names) > 20000:
        raise ValueError("source file limit exceeded")
    digest, runtime = hashlib.sha256(), hashlib.sha256()
    total = 0
    for name in names:
        relative = Path(name)
        if (relative.is_absolute() or ".." in relative.parts or len(name) > 4096
                or any(ord(c) < 32 for c in name)):
            raise ValueError("invalid source path")
        if (any(p.startswith(".") or p in ("testdata", "__pycache__", "e2e", "engineering")
                for p in relative.parts) or name.endswith("_test.go")
                or relative.suffix in (".pem", ".key", ".p12", ".pfx")):
            continue
        path = repo / relative
        if any((repo / Path(*relative.parts[:i])).is_symlink() for i in range(1, len(relative.parts))):
            raise ValueError("source directory symlink rejected")
        try:
            descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
        except FileNotFoundError:
            continue
        except OSError as error:
            raise ValueError("source file could not be safely opened") from error
        with os.fdopen(descriptor, "rb") as source:
            info = os.fstat(source.fileno())
            if not stat.S_ISREG(info.st_mode) or info.st_size > 16 * 1024 * 1024:
                raise ValueError("source file type or size rejected")
            content = source.read(16 * 1024 * 1024 + 1)
        total += len(content)
        if len(content) > 16 * 1024 * 1024 or total > 128 * 1024 * 1024:
            raise ValueError("source byte limit exceeded")
        record = name.encode() + b"\0" + hashlib.sha256(content).digest()
        digest.update(record)
        if (relative.name == "Containerfile" or "third_party" in relative.parts
                or relative.name in LICENSES or relative.name.startswith("OFL-")):
            runtime.update(record)
        if destination is not None:
            target = Path(destination) / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(content)
    commit = subprocess.check_output(["git", "-C", str(repo), "rev-parse", "HEAD"], text=True).strip()
    return {"snapshot": digest.hexdigest()[:40], "sha256": digest.hexdigest(),
            "runtime": runtime.hexdigest(), "commit": commit}


if __name__ == "__main__":
    try:
        print(capture(Path(sys.argv[1]), sys.argv[2])["snapshot"])
    except (OSError, ValueError, subprocess.CalledProcessError, IndexError):
        print("level=warn operation=nox_source outcome=invalid_source", file=sys.stderr)
        sys.exit(2)
