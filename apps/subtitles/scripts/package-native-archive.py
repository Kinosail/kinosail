#!/usr/bin/env python3
"""Create a byte-for-byte reproducible Kinosail native archive."""

import argparse
import gzip
import os
import pathlib
import shutil
import stat
import tarfile
import zipfile


ARCHIVE_FILES = (
    ("executable", None, 0o755),
    ("contract", "native-installation.json", 0o644),
    ("license", "LICENSE", 0o644),
    ("notices", "THIRD_PARTY_NOTICES.md", 0o644),
)


def inputs(arguments):
    files = []
    for attribute, archive_name, mode in ARCHIVE_FILES:
        source = pathlib.Path(getattr(arguments, attribute))
        if not source.is_file() or not stat.S_ISREG(source.stat().st_mode):
            raise ValueError(f"invalid input file: {source}")
        files.append((source, archive_name or source.name, mode))
    return files


def write_tar(output, files):
    with output.open("wb") as raw:
        with gzip.GzipFile(filename="", mode="wb", fileobj=raw, compresslevel=9, mtime=0) as compressed:
            with tarfile.open(fileobj=compressed, mode="w", format=tarfile.USTAR_FORMAT) as archive:
                for source, name, mode in files:
                    info = tarfile.TarInfo(name)
                    info.size = source.stat().st_size
                    info.mode = mode
                    info.mtime = 0
                    info.uid = 0
                    info.gid = 0
                    info.uname = "root"
                    info.gname = "root"
                    with source.open("rb") as content:
                        archive.addfile(info, content)


def write_zip(output, files):
    with zipfile.ZipFile(output, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        for source, name, mode in files:
            info = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
            info.compress_type = zipfile.ZIP_DEFLATED
            info.create_system = 3
            info.external_attr = mode << 16
            with source.open("rb") as content, archive.open(info, "w") as destination:
                shutil.copyfileobj(content, destination)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("format", choices=("tar.gz", "zip"))
    parser.add_argument("output")
    parser.add_argument("executable")
    parser.add_argument("contract")
    parser.add_argument("license")
    parser.add_argument("notices")
    arguments = parser.parse_args()
    output = pathlib.Path(arguments.output)
    if output.exists() or not output.parent.is_dir():
        raise ValueError(f"invalid output path: {output}")
    temporary = output.with_name(f".{output.name}.{os.getpid()}.tmp")
    if temporary.exists():
        raise ValueError(f"temporary output already exists: {temporary}")
    try:
        writers = {"tar.gz": write_tar, "zip": write_zip}
        writers[arguments.format](temporary, inputs(arguments))
        os.replace(temporary, output)
    finally:
        temporary.unlink(missing_ok=True)


if __name__ == "__main__":
    main()
