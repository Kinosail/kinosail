#!/usr/bin/env python3
"""Promote only when no newer main change affects this app; callers serialize."""
import os
import re
import subprocess

from affected import APPS, affected


def main():
    app, image, digest = os.environ["APP"], os.environ["IMAGE"], os.environ["DIGEST"]
    commit = os.environ["GITHUB_SHA"]
    if (app not in APPS or image != f"ghcr.io/kinosail/kinosail-{app}"
            or not re.fullmatch(r"sha256:[a-f0-9]{64}", digest)
            or not re.fullmatch(r"[a-f0-9]{40}", commit)
            or os.environ["GITHUB_EVENT_NAME"] != "push" or os.environ["GITHUB_REF"] != "refs/heads/main"):
        raise ValueError("invalid production promotion")
    # Checkout tokens are not persisted. Fetch the public main ref without credentials.
    subprocess.run(["git", "fetch", "origin", "main"], check=True, timeout=60)
    subprocess.run(["git", "merge-base", "--is-ancestor", commit, "origin/main"], check=True)
    result = subprocess.run(["git", "diff", "--name-only", "--no-renames", "-z", commit, "origin/main", "--"],
                            capture_output=True, check=True, timeout=60)
    if len(result.stdout) > 32 * 1024 * 1024 or (result.stdout and not result.stdout.endswith(b"\0")):
        raise ValueError("invalid promotion diff")
    paths = result.stdout.decode("utf-8", errors="surrogateescape").split("\0")[:-1]
    if affected(paths)[app]:
        print("Newer main changes affect this app; retain the current production tags.")
        return
    subprocess.run(["docker", "buildx", "imagetools", "create", "--tag", image + ":main",
                    "--tag", image + ":latest", image + "@" + digest], check=True)


if __name__ == "__main__":
    main()
