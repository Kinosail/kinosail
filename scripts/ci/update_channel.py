#!/usr/bin/env python3
"""Build the update channel statement that native updaters trust for freshness and revocation."""

import datetime
import json
import os
import pathlib
import re
import subprocess
import sys

APPS = ("player", "subtitles")
VERSION = re.compile(r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)")
LIFETIME = datetime.timedelta(days=7)
MAXIMUM_REVOCATIONS = 100
REQUIRED_ASSETS = {"kinosail-release.json", "kinosail-release.json.sigstore.json"}
REVOCATIONS = pathlib.Path(".github/update-revocations.json")


def version_key(version):
    return tuple(int(part) for part in VERSION.fullmatch(version).groups())


def load_revocations(text, app):
    data = json.loads(text)
    if not isinstance(data, dict) or set(data) != set(APPS) or app not in APPS:
        raise ValueError("invalid revocation list")
    revoked = data[app]
    if (not isinstance(revoked, list) or len(revoked) > MAXIMUM_REVOCATIONS or len(set(map(str, revoked))) != len(revoked)
            or any(not isinstance(version, str) or not VERSION.fullmatch(version) for version in revoked)):
        raise ValueError("invalid revocation list")
    return sorted(revoked, key=version_key)


def choose_latest(releases, app, revoked):
    prefix = app + "-"
    candidates = []
    for release in releases:
        tag = release.get("tag_name") if isinstance(release, dict) else None
        if not isinstance(tag, str) or release.get("draft") is not False or release.get("prerelease") is not False:
            continue
        version = tag[len(prefix):] if tag.startswith(prefix) else ""
        assets = {asset.get("name") for asset in release.get("assets", []) if isinstance(asset, dict)}
        if VERSION.fullmatch(version) and version not in revoked and REQUIRED_ASSETS <= assets:
            candidates.append(version)
    if not candidates:
        raise ValueError("no release can be announced")
    return max(candidates, key=version_key)


def statement(app, latest, revoked, now):
    if app not in APPS or not VERSION.fullmatch(latest) or latest in revoked:
        raise ValueError("invalid update channel")
    issued = now.astimezone(datetime.timezone.utc).replace(microsecond=0)
    stamp = "%Y-%m-%dT%H:%M:%SZ"
    return {"schemaVersion": 1, "app": app, "latest": latest, "revoked": revoked,
            "issued": issued.strftime(stamp), "expires": (issued + LIFETIME).strftime(stamp)}


def published_releases():
    result = subprocess.run(["gh", "api", "--paginate", "--slurp", "repos/Kinosail/kinosail/releases?per_page=100"],
                            check=True, capture_output=True, text=True)
    return [release for page in json.loads(result.stdout) for release in page]


def main():
    if len(sys.argv) != 3 or sys.argv[1] not in APPS:
        raise ValueError("usage: update_channel.py APP OUTPUT")
    app, output = sys.argv[1], pathlib.Path(sys.argv[2])
    revoked = load_revocations(REVOCATIONS.read_text(), app)
    latest = choose_latest(published_releases(), app, revoked)
    output.write_text(json.dumps(statement(app, latest, revoked, datetime.datetime.now(datetime.timezone.utc)), separators=(",", ":")))
    if "GITHUB_OUTPUT" in os.environ:
        with open(os.environ["GITHUB_OUTPUT"], "a") as github_output:
            github_output.write(f"latest={latest}\n")


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        print(f"update_channel: {error}", file=sys.stderr)
        sys.exit(1)
