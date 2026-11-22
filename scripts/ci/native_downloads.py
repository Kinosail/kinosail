#!/usr/bin/env python3
"""Define the stable native download names and verify a release carries exactly those files."""

import pathlib
import re
import sys

DOWNLOADS = (
    ("Linux", "x64", "deb", "Debian, Ubuntu, Linux Mint, Pop!_OS, Raspberry Pi OS (64-bit)"),
    ("Linux", "arm64", "deb", "Debian, Ubuntu, Raspberry Pi OS on ARM (64-bit)"),
    ("Linux", "x64", "rpm", "Fedora, RHEL, Rocky, AlmaLinux, openSUSE"),
    ("Linux", "arm64", "rpm", "Fedora, RHEL, Rocky, AlmaLinux on ARM"),
)
SITE = "https://kinosail.com/download/"
REPOSITORY = "https://github.com/Kinosail/kinosail"


def names():
    return [f"Kinosail-{system}-{arch}.{extension}" for system, arch, extension, _ in DOWNLOADS]


def verify(directory):
    root = pathlib.Path(directory)
    present = sorted(path.name for path in root.iterdir())
    if present != sorted(names()):
        raise ValueError("native release files do not match the published download names")
    for name in present:
        path = root / name
        if path.is_symlink() or not path.is_file() or path.stat().st_size < 1 << 20:
            raise ValueError(f"native release file is missing or too small: {name}")


def notes(app, version):
    if app not in ("player", "subtitles") or not re.fullmatch(r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)", version):
        raise ValueError("invalid release notes input")
    if app != "player":
        return f"Kinosail Subtitles {version}. Install it with the Docker instructions in the README.\n"
    tag = f"player-{version}"
    rows = "\n".join(
        f"| {system} {arch.upper() if arch == 'x64' else 'ARM64'} | {distros} | [{name}]({REPOSITORY}/releases/download/{tag}/{name}) |"
        for (system, arch, _, distros), name in zip(DOWNLOADS, names()))
    return (f"## Download Kinosail {version}\n\n"
            f"**Not sure which file you need? Go to [{SITE}]({SITE}).** It picks the right one for your computer.\n\n"
            "| Computer | Works with | File |\n| --- | --- | --- |\n"
            f"{rows}\n\n"
            "Linux one-step install:\n\n"
            "```sh\ncurl -fsSL https://kinosail.com/install.sh | sh\n```\n\n"
            f"Check downloads with `SHA256SUMS` and its Sigstore bundle. Docker users can keep using `ghcr.io/kinosail/kinosail-player:{version.removeprefix('v')}`.\n\n")


def main():
    if len(sys.argv) == 3 and sys.argv[1] == "verify":
        verify(sys.argv[2])
    elif len(sys.argv) == 4 and sys.argv[1] == "notes":
        sys.stdout.write(notes(sys.argv[2], sys.argv[3]))
    else:
        raise ValueError("usage: native_downloads.py verify DIR | notes APP VERSION")


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError) as error:
        print(f"native_downloads: {error}", file=sys.stderr)
        sys.exit(1)
