#!/usr/bin/env python3
"""Deploy debounced local source snapshots directly to Nox."""
import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time

from nox_local_source import APPS, capture

HERE = Path(__file__).resolve().parent


def report(level, app, snapshot, outcome, **fields):
    print(json.dumps(dict(level=level, operation="nox_local_update", app=app,
                          snapshot=snapshot, outcome=outcome, **fields)), flush=True)


def run(args):
    args.cache.mkdir(parents=True, exist_ok=True, mode=0o700)
    os.chmod(args.cache, 0o700)
    with (args.cache / "watcher.lock").open("w") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            report("error", "all", "", "watcher_already_running")
            return 1
        return watch(args)


def watch(args):
    pending, deployed, retry_after = {}, {}, {}
    state = args.cache / "deployed.json"
    report("info", "all", "", "watching", debounce_seconds=args.debounce)
    while True:
        failed = False
        for app in args.apps:
            now = time.monotonic()
            try:
                source = capture(args.repo, app)
                sha = source["snapshot"]
                if pending.get(app, (None,))[0] != sha:
                    pending[app] = (sha, now)
                    retry_after[app] = 0
                if sha == deployed.get(app):
                    continue
                if not args.once and (now - pending[app][1] < args.debounce or now < retry_after.get(app, 0)):
                    continue
                started = time.monotonic()
                with tempfile.TemporaryDirectory(prefix=f"{app}-", dir=args.cache) as folder:
                    snapshot = Path(folder)
                    captured = capture(args.repo, app, snapshot)
                    if captured != source or capture(args.repo, app) != source:
                        report("info", app, sha, "superseded")
                        failed = True
                        continue
                    log = args.cache / f"{app}-build.log"
                    report("info", app, sha, "building")
                    with log.open("wb") as output:
                        process = subprocess.Popen([str(HERE / "deploy-nox-local.sh"), app, str(snapshot),
                                                    sha, source["runtime"], source["commit"], str(args.repo)],
                                                   stdout=output, stderr=subprocess.STDOUT, start_new_session=True)
                        try:
                            code = process.wait()
                        except BaseException:
                            try:
                                os.killpg(process.pid, signal.SIGTERM)
                            except ProcessLookupError:
                                pass
                            process.wait()
                            raise
                    seconds = round(time.monotonic() - started, 2)
                    if code:
                        report("info" if code == 75 else "warn", app, sha,
                               "superseded" if code == 75 else "deployment_failed", exit_code=code, seconds=seconds)
                        failed = True
                        retry_after[app] = time.monotonic() + (0 if code == 75 else 30)
                        continue
                    evidence = dict(source, app=app, outcome="healthy", seconds=seconds,
                                    build_log_sha256=hashlib.sha256(log.read_bytes()).hexdigest())
                    (args.cache / f"{app}-latest.json").write_text(json.dumps(evidence, indent=2) + "\n")
                    deployed[app] = sha
                    temporary = state.with_suffix(".tmp")
                    temporary.write_text(json.dumps(dict(repo=str(args.repo), apps=deployed)) + "\n")
                    temporary.replace(state)
                    report("info", app, sha, "healthy", seconds=seconds)
            except (OSError, ValueError, subprocess.CalledProcessError):
                report("warn", app, "", "source_or_environment_failed")
                failed = True
        if args.once:
            return int(failed)
        time.sleep(args.poll)


def duration(value):
    number = float(value)
    if not 0.01 <= number <= 300:
        raise argparse.ArgumentTypeError("duration must be between 0.01 and 300 seconds")
    return number


def main():
    parser = argparse.ArgumentParser(description=__doc__, allow_abbrev=False)
    parser.add_argument("--repo", type=Path, default=HERE.parents[1])
    parser.add_argument("--cache", type=Path, default=Path.home() / "Library/Caches/KinosailNoxLocal")
    parser.add_argument("--apps", nargs="+", choices=APPS, default=list(APPS))
    parser.add_argument("--debounce", type=duration, default=3)
    parser.add_argument("--poll", type=duration, default=1)
    parser.add_argument("--once", action="store_true")
    args = parser.parse_args()
    args.repo = args.repo.resolve(strict=True)
    args.cache = args.cache.resolve()
    os.umask(0o077)
    signal.signal(signal.SIGTERM, lambda *_: sys.exit(0))
    try:
        return run(args)
    except KeyboardInterrupt:
        return 0


if __name__ == "__main__":
    sys.exit(main())
