#!/usr/bin/env python3
"""Manually verify real saved-source deployments and isolated rollback on Nox."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import time

from nox_local_source import APPS, capture

HERE = Path(__file__).resolve().parent
CONTAINERS = {"player": "kinosail", "subtitles": "kinosail-subtitles-dev"}
SSH = ["ssh", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=yes", "nox@server.nox"]
SANDBOX = "/tmp/kinosail-local-looptest"
REPO = "localhost/kinosail-looptest"
SERVICE = "kinosail-looptest"
GOOD, BAD = "a" * 40, "b" * 40


def save_evidence(args, evidence):
    evidence.setdefault("outcome", "failed")
    evidence["artifacts"] = {file.name: hashlib.sha256(file.read_bytes()).hexdigest()
                             for file in args.output.iterdir() if file.is_file() and file.name != "e2e.json"}
    (args.output / "e2e.json").write_text(json.dumps(evidence, indent=2) + "\n")


def remote(command, data=None, check=True, log=None):
    result = subprocess.run(SSH + [command], input=data, capture_output=True, timeout=180)
    if log:
        with log.open("ab") as output:
            output.write(result.stdout + result.stderr)
    if check and result.returncode:
        raise RuntimeError("remote_command_failed")
    return result


def deployed(app, expected):
    container = CONTAINERS[app]
    result = remote("docker inspect --format "
                    "'{{index .Config.Labels \"org.opencontainers.image.revision\"}}|"
                    "{{.State.Status}}|{{.State.Health.Status}}' " + container)
    assert result.stdout.decode().strip() == f"{expected}|running|healthy", "remote_revision_or_health"
    version = remote(f"docker exec {container} kinosail version").stdout.decode().strip()
    assert version == f"local-{expected[:12]}", "remote_binary_version"
    remote(f"docker exec {container} kinosail healthcheck")
    return dict(snapshot=expected, state="running", health="healthy", binary_version=version)


def wait_for(args, expected, started, evidence, name):
    deadline = time.monotonic() + 180
    while time.monotonic() < deadline:
        ready = True
        for app, snapshot in expected.items():
            receipt = args.cache / f"{app}-latest.json"
            try:
                value = json.loads(receipt.read_text())
                ready &= value["snapshot"] == snapshot and value["outcome"] == "healthy"
            except (OSError, ValueError, KeyError):
                ready = False
        if ready:
            receipt_seconds = round(time.monotonic() - started, 2)
            try:
                state = {app: deployed(app, sha) for app, sha in expected.items()}
            except (AssertionError, RuntimeError):
                time.sleep(1)
                continue
            result = dict(journey=name, seconds_to_receipt=receipt_seconds,
                          seconds_including_verification=round(time.monotonic() - started, 2), apps=state)
            evidence["journeys"].append(result)
            print(json.dumps(result), flush=True)
            return
        time.sleep(1)
    raise RuntimeError(f"{name}_timed_out")


def snapshots(repo):
    return {app: capture(repo, app)["snapshot"] for app in APPS}


def rollback(args, evidence):
    log = args.output / "rollback-private.log"
    setup = f"""set -eu
cat > {SANDBOX}/compose.yaml <<'COMPOSE'
name: kinosail-looptest
services:
  {SERVICE}:
    container_name: {SERVICE}
    image: {REPO}:nox-dev
COMPOSE
docker build --quiet --tag {REPO}:nox-{GOOD[:12]} - <<'DOCKERFILE'
FROM localhost/kinosail:nox-local-runtime
LABEL org.opencontainers.image.revision={GOOD}
HEALTHCHECK --interval=1s --timeout=30s --start-period=10s --retries=2 CMD kinosail healthcheck
DOCKERFILE
"""
    # Refuse to claim a preexisting fixture. Cleanup only resources created by this run.
    remote(f"set -e; if test -e {SANDBOX} || docker inspect {SERVICE} >/dev/null 2>&1 || "
           f"docker image inspect {REPO}:nox-dev >/dev/null 2>&1; then exit 2; fi; mkdir -m 700 {SANDBOX}")
    try:
        remote("bash -s", setup.encode(), log=log)
        helper = (HERE / "deploy-nox-remote.sh").read_bytes()
        def promote(sha, check=True):
            command = (f"env KINOSAIL_NOX_COMPOSE_DIR={SANDBOX} bash -s -- {sha} "
                       f"{REPO}:nox-{sha[:12]} {REPO}:nox-dev {SERVICE} {SERVICE} {REPO}")
            return remote(command, helper, check=check, log=log)
        promote(GOOD)
        remote("bash -s", f"""set -eu
docker build --quiet --tag {REPO}:nox-{BAD[:12]} - <<'DOCKERFILE'
FROM {REPO}:nox-{GOOD[:12]}
LABEL org.opencontainers.image.revision={BAD}
ENTRYPOINT ["/bin/false"]
DOCKERFILE
""".encode(), log=log)
        marker = remote("date +%s").stdout.decode().strip()
        assert marker.isdigit(), "invalid_event_timestamp"
        rejected = promote(BAD, check=False)
        assert rejected.returncode != 0, "unhealthy_image_accepted"
        events = remote(f"docker events --since {marker} --until $(date +%s) "
                        f"--filter type=container --filter event=die "
                        f"--filter label=org.opencontainers.image.revision={BAD} "
                        "--format '{{index .Actor.Attributes \"exitCode\"}}'").stdout.decode().splitlines()
        assert "1" in events, "failed_candidate_never_started"
        for _ in range(30):
            value = remote("docker inspect --format "
                           "'{{index .Config.Labels \"org.opencontainers.image.revision\"}}|"
                           "{{.State.Status}}|{{.State.Health.Status}}' " + SERVICE).stdout.decode().strip()
            if value == f"{GOOD}|running|healthy":
                break
            time.sleep(1)
        else:
            raise RuntimeError("rollback_did_not_restore_health")
        for app, sha in snapshots(args.repo).items():
            deployed(app, sha)
        result = dict(journey="isolated_remote_rollback", rejected_exit=rejected.returncode,
                      candidate_exit_observed=True, restored_snapshot=GOOD, health="healthy", live_apps_unchanged=True)
        evidence["journeys"].append(result)
        print(json.dumps(result), flush=True)
    finally:
        cleanup = f"""set -eu
if test -f {SANDBOX}/compose.yaml; then
  cd {SANDBOX}
  docker compose down --volumes --remove-orphans
fi
docker image rm {REPO}:nox-dev {REPO}:nox-{BAD[:12]} {REPO}:nox-{GOOD[:12]} >/dev/null 2>&1 || true
rm -rf {SANDBOX}
"""
        remote("bash -s", cleanup.encode(), log=log)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", type=Path, required=True, help="Checkout already watched by the running updater")
    parser.add_argument("--cache", type=Path, default=Path.home() / "Library/Caches/KinosailNoxLocal")
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--watcher-mode", choices=("foreground", "login-agent"), required=True)
    parser.add_argument("--rollback-only", action="store_true", help="Exercise only the disposable remote fixture")
    args = parser.parse_args()
    args.repo = args.repo.resolve(strict=True)
    os.umask(0o077)
    args.output.mkdir(parents=True, exist_ok=True)
    canaries = [args.repo / "apps/player/internal/server/nox_local_live_canary.go",
                args.repo / "packages/nox_local_live_canary.go"]
    assert all(not file.exists() for file in canaries), "canary_already_exists"
    baseline = snapshots(args.repo)
    evidence = dict(command=[sys.executable, *sys.argv], watcher_mode=args.watcher_mode,
                    test_revision=subprocess.check_output(["git", "-C", str(HERE), "rev-parse", "HEAD"], text=True).strip(),
                    source_revision=capture(args.repo, "player")["commit"], baseline=baseline,
                    environment=dict(platform="macOS", target="linux/arm64", debounce_seconds=3), journeys=[])
    evidence["tools_sha256"] = {name: hashlib.sha256((HERE / name).read_bytes()).hexdigest()
                                for name in ("test-nox-local-e2e.py", "watch-nox-local.py", "nox_local_source.py",
                                             "deploy-nox-local.sh", "deploy-nox-remote.sh", "install-nox-local.py")}
    owned, successful = [], False
    if args.rollback_only:
        try:
            rollback(args, evidence)
            evidence["outcome"] = "passed"
        finally:
            save_evidence(args, evidence)
        return 0
    try:
        wait_for(args, baseline, time.monotonic(), evidence, "initial_live_health")
        started = time.monotonic()
        intermediate = []
        with canaries[0].open("x") as output:
            owned.append(canaries[0])
            output.write("package server\n// Direct deployment canary: initial save.\n")
        intermediate.append(snapshots(args.repo)["player"])
        for index in range(2):
            time.sleep(0.2)
            with canaries[0].open("a") as output:
                output.write(f"// Debounced save {index}.\n")
            intermediate.append(snapshots(args.repo)["player"])
        expected = snapshots(args.repo)
        assert expected["subtitles"] == baseline["subtitles"], "app_source_selection"
        wait_for(args, expected, started, evidence, "player_save_burst")
        with canaries[1].open("x") as output:
            owned.append(canaries[1])
            output.write("package kinosail\n// Shared direct deployment canary.\n")
        wait_for(args, snapshots(args.repo), time.monotonic(), evidence, "shared_save_both_apps")
        events = []
        for line in (args.cache / "watcher.log").read_text().splitlines():
            try:
                events.append(json.loads(line))
            except ValueError:
                pass
        promoted = {event.get("snapshot") for event in events if event.get("outcome") == "healthy"}
        assert not promoted.intersection(intermediate[:-1]), "intermediate_save_promoted"
        evidence["debounce_intermediate_snapshots_absent"] = True
        successful = True
    finally:
        for file in owned:
            file.unlink(missing_ok=True)
        try:
            wait_for(args, baseline, time.monotonic(), evidence, "restore_original_source")
            rollback(args, evidence)
            evidence["outcome"] = "passed" if successful else "failed"
        except BaseException:
            evidence["outcome"] = "failed"
            raise
        finally:
            save_evidence(args, evidence)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
