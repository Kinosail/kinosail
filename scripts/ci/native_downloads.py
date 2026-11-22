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
    chips = {"x64": "Intel or AMD", "arm64": "ARM (Raspberry Pi)"}
    rows = "\n".join(
        f"| {distros} | {chips[arch]} | [Download .{extension}]({REPOSITORY}/releases/download/{tag}/{name}) |"
        for (_, arch, extension, distros), name in zip(DOWNLOADS, names()))
    return ("## Install Kinosail\n\n"
            f"**[Go to {SITE.removeprefix('https://').rstrip('/')}]({SITE}) and press Download.** The page picks the right file for your computer.\n\n"
            "On Linux, you can also paste this in a terminal. It picks the right file and checks it before it installs:\n\n"
            "```sh\ncurl -fsSL https://kinosail.com/install.sh | sh\n```\n\n"
            "Windows and Mac downloads are coming soon.\n\n"
            "<details><summary>All files in this release</summary>\n\n"
            "| Your Linux | Chip | File |\n| --- | --- | --- |\n"
            f"{rows}\n\n"
            "Run `uname -m` to check your chip. `x86_64` means Intel or AMD. `aarch64` means ARM.\n\n"
            "</details>\n\n"
            "Every file is listed in `SHA256SUMS`, which has a Sigstore signature from the Kinosail release workflow. "
            f"Docker users can keep using `ghcr.io/kinosail/kinosail-player:{version.removeprefix('v')}`.\n\n")

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
