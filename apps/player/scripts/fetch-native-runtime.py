#!/usr/bin/env python3
"""Download, verify, and extract the pinned native runtime tools for one platform."""

import argparse
import hashlib
import json
import os
import pathlib
import shutil
import sys
import tarfile
import tempfile
import urllib.request
import zipfile

ROOT = pathlib.Path(__file__).resolve().parents[1]
PLATFORMS = ("linux-amd64", "linux-arm64", "macos-amd64", "macos-arm64", "windows-amd64", "windows-arm64")
MAX_DOWNLOAD = 128 << 20
MAX_MEMBER = 256 << 20
ALLOWED_HOSTS = ("https://github.com/jellyfin/jellyfin-ffmpeg/releases/download/",
                 "https://github.com/acoustid/chromaprint/releases/download/")


def load(platform):
    pins = json.loads((ROOT / "packaging/runtime.json").read_text(encoding="utf-8"))
    if pins.get("schemaVersion") != 1 or platform not in pins["downloads"]:
        raise ValueError("unsupported runtime platform")
    entries = []
    for entry in pins["downloads"][platform]:
        component = pins["components"][entry["component"]]
        url, digest, size = entry["url"], entry["sha256"], entry["size"]
        if not url.startswith(ALLOWED_HOSTS) or len(digest) != 64 or not 0 < size <= MAX_DOWNLOAD:
            raise ValueError("invalid runtime pin")
        entries.append((url, digest, size, component["commands"]))
    return entries


def download(url, digest, size, cache):
    target = cache / digest
    if target.is_file() and sha256(target) == digest:
        return target
    partial = target.with_suffix(".partial")
    hasher, total = hashlib.sha256(), 0
    with urllib.request.urlopen(url, timeout=120) as response, partial.open("wb") as output:
        while chunk := response.read(1 << 20):
            total += len(chunk)
            if total > size:
                raise ValueError("runtime download exceeds its pinned size")
            hasher.update(chunk)
            output.write(chunk)
    if total != size or hasher.hexdigest() != digest:
        partial.unlink()
        raise ValueError("runtime download does not match its pinned digest")
    partial.replace(target)
    return target


def sha256(path):
    hasher = hashlib.sha256()
    with path.open("rb") as source:
        while chunk := source.read(1 << 20):
            hasher.update(chunk)
    return hasher.hexdigest()


def members(archive, url):
    if url.endswith(".zip"):
        with zipfile.ZipFile(archive) as bundle:
            for info in bundle.infolist():
                if not info.is_dir():
                    yield info.filename, info.file_size, lambda info=info, bundle=bundle: bundle.open(info)
        return
    with tarfile.open(archive) as bundle:
        for info in bundle.getmembers():
            if info.isfile():
                yield info.name, info.size, lambda info=info, bundle=bundle: bundle.extractfile(info)


def extract(archive, url, commands, windows, destination):
    wanted = {command + (".exe" if windows else ""): command for command in commands}
    found = set()
    for name, size, opener in members(archive, url):
        base = pathlib.PurePosixPath(name).name
        if base not in wanted:
            continue
        if base in found or size > MAX_MEMBER or ".." in pathlib.PurePosixPath(name).parts:
            raise ValueError("runtime archive has an unexpected executable layout")
        output = destination / base
        with opener() as source, output.open("wb") as target:
            shutil.copyfileobj(source, target)
        output.chmod(0o755)
        found.add(base)
    if found != set(wanted):
        raise ValueError("runtime archive is missing a required executable")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("platform", choices=PLATFORMS)
    parser.add_argument("destination")
    parser.add_argument("--cache", default=os.environ.get("KINOSAIL_RUNTIME_CACHE", ""))
    arguments = parser.parse_args()
    destination = pathlib.Path(arguments.destination)
    if destination.exists() and any(destination.iterdir()):
        raise ValueError("runtime destination must be empty")
    destination.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="kinosail-runtime-") as temporary:
        cache = pathlib.Path(arguments.cache or temporary)
        cache.mkdir(parents=True, exist_ok=True)
        for url, digest, size, commands in load(arguments.platform):
            archive = download(url, digest, size, cache)
            extract(archive, url, commands, arguments.platform.startswith("windows"), destination)


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, KeyError, tarfile.TarError, zipfile.BadZipFile) as error:
        print(f"fetch-native-runtime: {error}", file=sys.stderr)
        sys.exit(1)
