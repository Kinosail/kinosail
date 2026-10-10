#!/usr/bin/env python3
"""Install the direct local-source watcher as a macOS login agent."""
import argparse
import os
from pathlib import Path
import plistlib
import shutil
import subprocess
import sys
import time

HERE = Path(__file__).resolve().parent
LABEL = "com.kinosail.nox-local-live"


def main():
    parser = argparse.ArgumentParser(description=__doc__, allow_abbrev=False)
    parser.add_argument("--repo", type=Path, default=HERE.parents[1])
    parser.add_argument("--stop", action="store_true")
    args = parser.parse_args()
    home = Path.home()
    agent = home / f"Library/LaunchAgents/{LABEL}.plist"
    domain = f"gui/{os.getuid()}"
    if args.stop:
        subprocess.run(["launchctl", "bootout", f"{domain}/{LABEL}"], capture_output=True)
        agent.unlink(missing_ok=True)
        print("Direct Nox local-source updates stopped.")
        return 0
    repo = args.repo.resolve(strict=True)
    subprocess.run(["git", "-C", str(repo), "rev-parse", "--git-dir"], check=True, capture_output=True)
    # Apple's runtime gives launchd a stable platform identity for Local Network access.
    # The Homebrew runtime can be denied here even when its GUI permission is enabled.
    python = "/usr/bin/python3" if sys.platform == "darwin" else shutil.which("python3")
    for tool in ("go", "podman", "ssh"):
        if not shutil.which(tool):
            raise ValueError(f"missing tool: {tool}")
    root = home / "Library/Application Support/KinosailNoxLocal"
    cache = home / "Library/Caches/KinosailNoxLocal"
    os.umask(0o077)
    for directory in (root, cache, agent.parent):
        directory.mkdir(parents=True, exist_ok=True)
    for name in ("watch-nox-local.py", "nox_local_source.py", "deploy-nox-local.sh",
                 "deploy-nox-remote.sh", "nox-app.sh"):
        shutil.copyfile(HERE / name, root / name)
        (root / name).chmod(0o700 if name.endswith(".sh") else 0o600)
    config = dict(Label=LABEL, ProgramArguments=[python, str(root / "watch-nox-local.py"), "--repo", str(repo)],
                  EnvironmentVariables=dict(PATH="/opt/homebrew/bin:/opt/podman/bin:/usr/local/bin:/usr/bin:/bin"),
                  RunAtLoad=True, KeepAlive=True, ThrottleInterval=10,
                  StandardOutPath=str(cache / "watcher.log"), StandardErrorPath=str(cache / "watcher-error.log"))
    if sys.platform == "darwin":
        config["ProgramArguments"] += ["--source-python", shutil.which("python3")]
    # Stop the hosted-main watchers so they cannot overwrite a local development snapshot.
    for label in ("com.kinosail.deploy-nox", "com.kinosail.subtitles.deploy-nox", LABEL):
        subprocess.run(["launchctl", "bootout", f"{domain}/{label}"], capture_output=True)
    for label in ("com.kinosail.deploy-nox", "com.kinosail.subtitles.deploy-nox"):
        (agent.parent / f"{label}.plist").unlink(missing_ok=True)
    agent.write_bytes(plistlib.dumps(config))
    for attempt in range(15):
        result = subprocess.run(["launchctl", "bootstrap", domain, str(agent)], capture_output=True)
        if result.returncode == 0:
            break
        if result.returncode != 5 or attempt == 14:
            raise ValueError("agent bootstrap failed")
        time.sleep(0.2)
    print(f"Direct Nox updates installed. Watching saved source in {repo} after a three-second debounce.")
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, ValueError, subprocess.CalledProcessError):
        print("level=error operation=nox_local_install outcome=installation_failed", file=sys.stderr)
        sys.exit(1)
