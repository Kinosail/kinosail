#!/usr/bin/env bash
# Fixed full-media owner; only its resources belong to this invocation.
set -euo pipefail
umask 077
if [[ $# != 3 ]]; then echo 'requires project, discovery and fresh output' >&2; exit 2; fi
project="$1" discovery="$2" output="$3"
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
selection=(--output "$output" --profile library-owner --project "$project" --state fresh --discovery "$discovery")
python3 "$helper" --url "$scheme://localhost:38127" "${selection[@]}" --admit-only
source "$repo/scripts/ci/browser-fixture-tls.sh"
export KINOSAIL_BROWSER_TEST=1 KINOSAIL_BROWSER_PROJECT="$project"
validate_browser_fixture_tls
workspace="$(mktemp -d /tmp/kinosail-library.XXXXXX)"
workspace="$(cd "$workspace" && pwd)"
suffix="$$-$RANDOM"
network="kinosail-library-$suffix" container="kinosail-library-$suffix"
network_created=0 container_started=0
volumes=()
cleanup() {
  local status="${1:-$?}" failed=0 volume
  trap - EXIT
  set +e
  if [[ "$container_started" == 1 ]]; then "$engine" rm --force "$container" >/dev/null || failed=1; fi
  remove_browser_fixture_trust || failed=1
  if [[ "${#volumes[@]}" -gt 0 ]]; then
    for volume in "${volumes[@]}"; do "$engine" volume rm "$volume" >/dev/null || failed=1; done
  fi
  if [[ "$network_created" == 1 ]]; then "$engine" network rm "$network" >/dev/null || failed=1; fi
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
mkdir "$workspace/media"
(cd "$app"; KINOSAIL_TEST_IMAGE=localhost/kinosail:dev CONTAINER_ENGINE="$engine" ./scripts/generate-test-media.sh "$workspace/media")
"$engine" run --rm --network none --entrypoint ffmpeg localhost/kinosail:dev -version >"$workspace/ffmpeg.txt"
"$engine" run --rm --network none --entrypoint ffprobe localhost/kinosail:dev -version >"$workspace/ffprobe.txt"
"$engine" network create --internal "$network" >/dev/null
network_created=1
for role in config cache backups; do
  volume="kinosail-library-$role-$suffix"
  "$engine" volume create "$volume" >/dev/null
  volumes+=("$volume")
done
container_started=1
"$engine" run --detach --init --name "$container" --network "$network" --publish 127.0.0.1::38127 \
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
address="$("$engine" port "$container" 38127/tcp)"
if [[ ! "$address" =~ ^127\.0\.0\.1:([0-9]{1,5})$ ]]; then exit 2; fi
port=$((10#${BASH_REMATCH[1]}))
if [[ "$port" -lt 1 || "$port" -gt 65535 ]]; then exit 2; fi
base="$scheme://localhost:$port"
healthy=0
for _ in {1..80}; do
  # This readiness probe follows the existing test-instance owner. Browser/Owner use strict CA trust below.
  if [[ "$(curl --fail --silent --insecure --max-time 2 "$base/healthz" || true)" == '{"status":"ok"}' ]]; then healthy=1; break; fi
  sleep 0.25
done
[[ "$healthy" == 1 ]]
identifier="$("$engine" inspect --format '{{.Id}}' "$container")"
trust_browser_fixture_tls "$engine" "$identifier" "$workspace" "$suffix"
KINOSAIL_TEST_REVISION="$(git -C "$repo" rev-parse HEAD)"
export KINOSAIL_TEST_REVISION
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
