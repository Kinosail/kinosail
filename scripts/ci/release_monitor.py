#!/usr/bin/env python3
"""Alert when a Kinosail signing identity, release tag, or update channel does not match an official release."""

import datetime
import json
import pathlib
import re
import subprocess
import sys

REPOSITORY = "https://github.com/Kinosail/kinosail"
WORKFLOWS = REPOSITORY + "/.github/workflows/"
ISSUER = "https://token.actions.githubusercontent.com"
TAG = re.compile(r"(player|subtitles)-v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)")
MAIN_IDENTITIES = {WORKFLOWS + "publish.yml@refs/heads/main", WORKFLOWS + "release-trust.yml@refs/heads/main"}
CHANNEL_MARGIN = datetime.timedelta(days=4)
CHANNEL_URL = "https://github.com/Kinosail/kinosail/releases/download/player-channel/channel.json"


def found_subjects(text):
    subjects = []
    for line in text.splitlines():
        if line.strip():
            entry = json.loads(line)
            if not isinstance(entry, dict):
                raise ValueError("invalid identity monitor output")
            subjects.append((str(entry.get("CertSubject", "")), entry.get("Index"), str(entry.get("UUID", ""))))
    return subjects


def unexpected_identities(subjects, official_tag):
    problems = []
    for subject, index, uuid in subjects:
        tag = subject.removeprefix(WORKFLOWS + "release.yml@refs/tags/")
        if subject in MAIN_IDENTITIES or (tag != subject and TAG.fullmatch(tag) and official_tag(tag)):
            continue
        problems.append(f"Unexpected Sigstore signature for {subject or 'an unknown identity'} (Rekor index {index}, entry {uuid[:80]})")
    return problems


def audit_tags(tags, on_main, released, release_running):
    problems = []
    for tag in tags:
        if not TAG.fullmatch(tag):
            problems.append(f"Unexpected release-like tag {tag[:80]}")
        elif not on_main(tag):
            problems.append(f"Release tag {tag} points to a commit that is not on main")
        elif not released(tag) and not release_running(tag):
            problems.append(f"Release tag {tag} has no published release and no running release workflow")
    return problems


def channel_problems(channel, now):
    try:
        expires = datetime.datetime.strptime(channel["expires"], "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=datetime.timezone.utc)
    except (KeyError, TypeError, ValueError):
        return ["The update channel cannot be read"]
    if channel.get("app") != "player" or not TAG.fullmatch("player-" + str(channel.get("latest"))):
        return ["The update channel announces an invalid release"]
    if expires - now < CHANNEL_MARGIN:
        return [f"The update channel expires at {channel['expires']}; the channel workflow is not refreshing it"]
    return []


def run(*command):
    return subprocess.run(command, capture_output=True, text=True, check=False)


def git_tag_on_main(tag):
    commit = run("git", "rev-parse", "--verify", f"refs/tags/{tag}^{{commit}}").stdout.strip()
    return bool(commit) and run("git", "merge-base", "--is-ancestor", commit, "origin/main").returncode == 0


def published_tags():
    result = run("gh", "release", "list", "--repo", "Kinosail/kinosail", "--limit", "1000", "--exclude-drafts", "--json", "tagName")
    if result.returncode:
        raise ValueError("GitHub releases cannot be listed")
    return {release["tagName"] for release in json.loads(result.stdout)}


def release_running(tag):
    result = run("gh", "run", "list", "--repo", "Kinosail/kinosail", "--workflow", "release.yml", "--branch", tag, "--json", "status")
    return result.returncode == 0 and any(item.get("status") != "completed" for item in json.loads(result.stdout or "[]"))


def verified_channel(directory):
    files = [directory / "channel.json", directory / "channel.json.sigstore.json"]
    for url, path in zip((CHANNEL_URL, CHANNEL_URL + ".sigstore.json"), files):
        if run("curl", "--fail", "--silent", "--show-error", "--location", "--max-filesize", "65536", "--output", str(path), url).returncode:
            return None
    if run("cosign", "verify-blob", "--bundle", str(files[1]), "--certificate-identity", WORKFLOWS + "release-trust.yml@refs/heads/main",
           "--certificate-oidc-issuer", ISSUER, str(files[0])).returncode:
        return None
    return json.loads(files[0].read_text())


def main():
    if len(sys.argv) == 3 and sys.argv[1] == "rekor":
        path = pathlib.Path(sys.argv[2])
        tags = published_tags()
        problems = unexpected_identities(found_subjects(path.read_text() if path.exists() else ""), lambda tag: git_tag_on_main(tag) and (tag in tags or release_running(tag)))
    elif len(sys.argv) == 3 and sys.argv[1] == "audit":
        tags = run("git", "tag", "--list", "player-*", "subtitles-*").stdout.split()
        published = published_tags()
        problems = audit_tags([tag for tag in tags if tag != "player-channel"], git_tag_on_main, published.__contains__, release_running)
        channel = verified_channel(pathlib.Path(sys.argv[2]))
        problems += ["The update channel is missing or its signature does not verify"] if channel is None else channel_problems(channel, datetime.datetime.now(datetime.timezone.utc))
    else:
        raise ValueError("usage: release_monitor.py rekor IDENTITIES_JSON | audit WORK_DIRECTORY")
    for problem in problems:
        print(problem)
    if problems:
        sys.exit(1)


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, json.JSONDecodeError) as error:
        print(f"release_monitor: {error}", file=sys.stderr)
        sys.exit(2)
