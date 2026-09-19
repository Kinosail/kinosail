#!/usr/bin/env python3
"""Prepare the Player container installer locally without signing or publishing."""

import argparse
import hashlib
import os
from pathlib import Path
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parent.parent
FILES = (
    "scripts/install.sh",
    "scripts/uninstall.sh",
    "scripts/setup-remote-access.sh",
    "scripts/disable-remote-access.sh",
    "compose.release.yaml",
    "compose.config.yaml",
    "compose.gpu.yaml",
    "compose.rkmpp.yaml",
    "compose.remote-https.yaml",
    "kinosail.example.yaml",
    ".env.example",
    "README.md",
    "LICENSE",
    "LICENSING.md",
    "THIRD_PARTY_NOTICES.md",
    "SECURITY.md",
    "CONTRIBUTING.md",
    "CLA.md",
    "CCLA.md",
    "TRADEMARKS.md",
    "third_party/hls.js/LICENSE",
    "third_party/htmx/LICENSE",
)


def release_metadata(member):
    member.uid = member.gid = 0
    member.uname = member.gname = "root"
    member.mtime = 0
    member.pax_headers = {}
    member.mode = 0o755 if member.mode & 0o111 else 0o644
    return member


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("destination", type=Path, help="new directory for the installer and checksum")
    args = parser.parse_args()
    destination = args.destination.absolute()
    if destination.exists() or destination.is_symlink():
        parser.error("destination must not already exist")
    for name in FILES:
        source = ROOT / name
        if not source.is_file() or source.is_symlink():
            parser.error(f"missing or non-regular installer input: {name}")
    # Assemble completely before creating the destination.
    with tempfile.TemporaryDirectory(prefix=".player-installer-", dir=destination.parent) as temporary:
        archive = Path(temporary) / "kinosail-player-install.tar.gz"
        with tarfile.open(archive, "w:gz") as bundle:
            for name in FILES:
                bundle.add(ROOT / name, arcname=name, filter=release_metadata)
        digest = hashlib.sha256(archive.read_bytes()).hexdigest()
        archive.with_name(archive.name + ".sha256").write_text(
            f"{digest}  {archive.name}\n", encoding="utf-8"
        )
        destination.mkdir()
        for source in Path(temporary).iterdir():
            os.rename(source, destination / source.name)
    print(f"Prepared unsigned installer: {destination / archive.name}")


if __name__ == "__main__":
    main()
