#!/usr/bin/env bash
# Fixed full-media owner; only its resources belong to this invocation.
set -euo pipefail
umask 077
if [[ $# != 3 && $# != 4 ]]; then echo 'requires project, discovery and fresh output' >&2; exit 2; fi
project="$1" discovery="$2" output="$3" profile="${4-library-owner}"
case "$profile" in library-owner|camera-fake) ;; *) exit 2 ;; esac
case "$project" in chromium|firefox) scheme=http ;; webkit) scheme=https ;; *) exit 2 ;; esac
engine="${CONTAINER_ENGINE:-docker}"
if [[ "$engine" != docker && "$engine" != podman || "${KINOSAIL_TEST_IMAGE_READY:-}" != '' && "${KINOSAIL_TEST_IMAGE_READY:-}" != 1 ]]; then exit 2; fi
while IFS= read -r variable; do
  case "$variable" in
    KINOSAIL_TEST_IMAGE_READY) ;;
    KINOSAIL_BROWSER_PROJECT) [[ "${!variable}" == "$project" ]] || exit 2 ;;
    KINOSAIL_*) echo 'unsupported library fixture environment override' >&2; exit 2 ;;
  esac
done < <(compgen -e)
app="$(cd "$(dirname "$0")/.." && pwd)"
repo="$(cd "$app/../.." && pwd)"
helper="$repo/scripts/ci/run-populated-settings.py"
selection=(--output "$output" --profile "$profile" --project "$project" --state fresh --discovery "$discovery")
python3 "$helper" --url "$scheme://localhost:38127" "${selection[@]}" --admit-only
source "$repo/scripts/ci/browser-fixture-tls.sh"
export KINOSAIL_BROWSER_TEST=1 KINOSAIL_BROWSER_PROJECT="$project"
validate_browser_fixture_tls
relay="$repo/scripts/ci/library-internal-relay.mjs"
node "$relay" topology "$engine" >/dev/null
owner="$(python3 -c 'import secrets; print(secrets.token_hex(16))')"
[[ "$owner" =~ ^[a-f0-9]{32}$ ]]
workspace="$(mktemp -d /tmp/kinosail-library.XXXXXX)"
workspace="$(cd "$workspace" && pwd)"
suffix="$$-$RANDOM"
network="kinosail-library-$suffix" container="kinosail-library-$suffix"
network_attempted=0 container_attempted=0 relay_pid="" network_id="" container_id=""
proof() { node "$relay" owned "$engine" "$1" "$2" "$owner" "$3"; }
phase=image
volumes=() volume_ids=()
cleanup() {
  local status="${1:-$?}" failed=0 volume index
  trap - EXIT
  set +e
  if [[ "$status" != 0 ]]; then
    # Only admitted fixed output is created; no logs, environment or daemon IDs.
    python3 - "$output/fixture-startup.json" "$phase" "$status" "$workspace/relay-failure.json" <<'PYTHON'
import json, os, pathlib, sys
facts = {'schemaVersion': 1, 'phase': sys.argv[2], 'exitCode': int(sys.argv[3]),
         'containerRunning': None, 'networkInternal': None, 'targetAdmitted': False}
try:
    path = pathlib.Path(sys.argv[4])
    if path.is_file() and not path.is_symlink() and path.stat().st_size <= 1024:
        value = json.loads(path.read_text())
        for key in ('containerRunning', 'networkInternal'):
            if value.get(key) is None or type(value.get(key)) is bool: facts[key] = value.get(key)
        facts['targetAdmitted'] = value.get('targetAdmitted') is True
except (OSError, ValueError, AttributeError):
    pass
destination = pathlib.Path(sys.argv[1])
try: destination.parent.mkdir()
except FileExistsError: pass
directory = os.open(destination.parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
try:
    fd = os.open(destination.name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=directory)
    with os.fdopen(fd, 'w') as file: file.write(json.dumps(facts) + '\n')
finally:
    os.close(directory)
PYTHON
  fi
  if [[ -n "$relay_pid" ]]; then
    kill "$relay_pid" 2>/dev/null
    wait "$relay_pid"; relay_pid=""
  fi
  if [[ "$container_attempted" == 1 && -z "$container_id" ]]; then
    container_id="$(proof container "$container" -)" || failed=1
  fi
  if [[ -n "$container_id" ]]; then
    if proof container "$container" "$container_id" >/dev/null; then "$engine" rm --force "$container_id" >/dev/null || failed=1; else failed=1; fi
  elif [[ "$container_attempted" == 1 ]]; then failed=1; fi
  remove_browser_fixture_trust || failed=1
  if [[ "${#volumes[@]}" -gt 0 ]]; then
    for ((index=0; index<${#volumes[@]}; index++)); do
      volume="${volumes[$index]}"
      if [[ "${volume_ids[$index]}" == - ]]; then
        volume_ids[index]="$(proof volume "$volume" -)" || { failed=1; continue; }
      fi
      if proof volume "$volume" "${volume_ids[$index]}" >/dev/null; then "$engine" volume rm "$volume" >/dev/null || failed=1; else failed=1; fi
    done
  fi
  if [[ "$network_attempted" == 1 && -z "$network_id" ]]; then
    network_id="$(proof network "$network" -)" || failed=1
  fi
  if [[ -n "$network_id" ]]; then
    if proof network "$network" "$network_id" >/dev/null; then "$engine" network rm "$network_id" >/dev/null || failed=1; else failed=1; fi
  elif [[ "$network_attempted" == 1 ]]; then failed=1; fi
  if [[ "$failed" == 0 ]]; then rm -rf -- "$workspace"; else echo 'Owned library fixture cleanup failed; retain its workspace' >&2; fi
  if [[ "$status" == 0 && "$failed" != 0 ]]; then status=1; fi
  exit "$status"
}
trap cleanup EXIT
trap 'exit 143' TERM
trap 'exit 130' INT
if [[ "${KINOSAIL_TEST_IMAGE_READY:-}" != 1 ]]; then
  "$engine" build --file "$app/Containerfile" --tag localhost/kinosail:dev "$repo"
fi
"$engine" build --file "$app/Containerfile.test" --tag localhost/kinosail:test-instance "$repo"
image_id="$("$engine" image inspect --format '{{.Id}}' localhost/kinosail:test-instance)"
if [[ ! "$image_id" =~ ^sha256:[a-f0-9]{64}$ ]]; then exit 2; fi
printf '%s\n' "$image_id" >"$workspace/image-id.txt"
phase=media
mkdir "$workspace/media"
(cd "$app"; KINOSAIL_TEST_IMAGE=localhost/kinosail:dev CONTAINER_ENGINE="$engine" ./scripts/generate-test-media.sh "$workspace/media")
"$engine" run --rm --network none --entrypoint ffmpeg localhost/kinosail:dev -version >"$workspace/ffmpeg.txt"
"$engine" run --rm --network none --entrypoint ffprobe localhost/kinosail:dev -version >"$workspace/ffprobe.txt"
phase=network
network_attempted=1
"$engine" network create --internal --label "org.kinosail.fixture-owner=$owner" "$network" >/dev/null
network_id="$(proof network "$network" -)"
for role in config cache backups; do
  volume="kinosail-library-$role-$suffix"
  volumes+=("$volume")
  volume_ids+=("-")
  "$engine" volume create --label "org.kinosail.fixture-owner=$owner" "$volume" >/dev/null
  volume_ids[${#volume_ids[@]}-1]="$(proof volume "$volume" -)"
done
phase=container
container_attempted=1
set +e
"$engine" run --detach --init --name "$container" --label "org.kinosail.fixture-owner=$owner" --network "$network" \
  --read-only --cap-drop ALL --security-opt no-new-privileges --pids-limit 256 \
  --tmpfs /tmp:rw,noexec,nosuid,nodev,size=256m \
  --volume "${volumes[0]}:/config" --volume "${volumes[1]}:/cache" --volume "${volumes[2]}:/backups" \
  --volume "$workspace/media:/media:ro" \
  --env "KINOSAIL_TLS_ENABLED=$([[ "$scheme" == https ]] && echo true || echo false)" \
  --env KINOSAIL_AUTH_URL=https://localhost:38127 \
  --env 'KINOSAIL_LIBRARIES=["Movies","Shows","Music","Audiobooks","Books","Photos"]' \
  --env KINOSAIL_TMDB_URL=http://127.0.0.1:8090/TMDB/api \
  --env KINOSAIL_TMDB_IMAGE_URL=http://127.0.0.1:8090/TMDB/images \
  --env KINOSAIL_TMDB_TOKEN=local-synthetic-test-catalogue \
  --env KINOSAIL_BACKUP_KEY=local-test-instance-backup-key \
  --entrypoint /bin/sh localhost/kinosail:test-instance -c 'httpd -f -p 127.0.0.1:8090 -h /media & exec kinosail' >/dev/null
status=$?
set -e
container_id="$(proof container "$container" - || true)"
if [[ "$status" != 0 ]]; then exit "$status"; fi
[[ -n "$container_id" ]] || exit 2
phase=relay
node "$relay" "$engine" "$container" "$network" "$image_id" "$owner" \
  >"$workspace/relay-ready.json" 2>"$workspace/relay-failure.json" &
relay_pid=$!
# Four bounded 3s metadata reads plus 2s listener; a real 1s watchdog margin.
port="$(python3 - "$workspace/relay-ready.json" "$relay_pid" <<'PYTHON'
import json, os, pathlib, sys, time
try:
    path = pathlib.Path(sys.argv[1])
    pid = int(sys.argv[2])
    if pid <= 0: raise ValueError()
    deadline = time.monotonic() + 15
    while True:
        if time.monotonic() >= deadline: raise ValueError()
        os.kill(pid, 0)
        if path.exists() and path.stat().st_size > 0: break
        time.sleep(min(0.05, max(0, deadline - time.monotonic())))
    if path.is_symlink() or not path.is_file() or path.stat().st_size > 1024: raise ValueError()
    text = path.read_text()
    value = json.loads(text)
    if (text.strip() != json.dumps(value, separators=(',', ':'))
        or set(value) != {'schemaVersion', 'port'} or type(value['schemaVersion']) is not int
        or value['schemaVersion'] != 1 or type(value['port']) is not int
        or not 1 <= value['port'] <= 65535): raise ValueError()
except (OSError, ValueError, TypeError):
    raise SystemExit(2)
print(value['port'])
PYTHON
)"
kill -0 "$relay_pid" 2>/dev/null
base="$scheme://localhost:$port"
phase=health
healthy=0
for _ in {1..80}; do
  # This readiness probe follows the existing test-instance owner. Browser/Owner use strict CA trust below.
  if [[ "$(curl --fail --silent --insecure --max-time 2 "$base/healthz" || true)" == '{"status":"ok"}' ]]; then healthy=1; break; fi
  sleep 0.25
done
[[ "$healthy" == 1 ]]
phase=trust
identifier="$("$engine" inspect --format '{{.Id}}' "$container")"
trust_browser_fixture_tls "$engine" "$identifier" "$workspace" "$suffix"
KINOSAIL_TEST_REVISION="$(git -C "$repo" rev-parse HEAD)"
export KINOSAIL_TEST_REVISION
phase=owner
set +e
python3 "$helper" --url "$base" "${selection[@]}"
status=$?
set -e
if [[ -d "$output" && ! -L "$output" ]]; then
  cp "$workspace/ffmpeg.txt" "$output/fixture-ffmpeg.txt"
  cp "$workspace/ffprobe.txt" "$output/fixture-ffprobe.txt"
  cp "$workspace/image-id.txt" "$output/fixture-image.txt"
  python3 - "$workspace/media" "$output/fixture-media.json" <<'PY'
import hashlib, json, pathlib, sys
root = pathlib.Path(sys.argv[1])
files = sorted(path for path in root.rglob('*') if path.is_file())
if not 1 <= len(files) <= 128 or any(path.is_symlink() for path in root.rglob('*')):
    raise SystemExit(2)
records = []
for path in files:
    digest = hashlib.sha256()
    with path.open('rb') as file:
        while chunk := file.read(65536): digest.update(chunk)
    records.append({'file': str(path.relative_to(root)), 'bytes': path.stat().st_size, 'sha256': digest.hexdigest()})
pathlib.Path(sys.argv[2]).write_text(json.dumps(records, indent=2) + '\n')
PY
fi
cleanup "$status"
