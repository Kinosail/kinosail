#!/usr/bin/env python3
"""Parse one app version tag before any release side effect."""

import os
import re


def parse(tag):
    if not isinstance(tag, str) or len(tag) > 80:
        raise ValueError("invalid release tag")
    match = re.fullmatch(r"(player|subtitles)-v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)", tag)
    if not match:
        raise ValueError("invalid release tag")
    app, major, minor, patch = match.groups()
    return {"app": app, "version": f"v{major}.{minor}.{patch}",
            "container_version": f"{major}.{minor}.{patch}"}


def main():
    tag = os.environ["GITHUB_REF_NAME"]
    if (os.environ["GITHUB_EVENT_NAME"] != "push" or os.environ["GITHUB_REF"] != f"refs/tags/{tag}"
            or os.environ["GITHUB_REPOSITORY"] != "Kinosail/kinosail"):
        raise ValueError("release requires a Kinosail version tag push")
    fields = parse(tag)
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        for name, value in fields.items():
            output.write(f"{name}={value}\n")


if __name__ == "__main__":
    main()
