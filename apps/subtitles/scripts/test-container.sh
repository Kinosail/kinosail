#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../../../scripts/tooling/gates-pause.sh"
set -euo pipefail
trap 'if [[ -s "$media_dir/compatible.m3u8" ]]; then sed -n "1,40p" "$media_dir/compatible.m3u8" >&2; fi; if [[ -n "${container:-}" ]]; then "$engine" logs "$container" >&2; fi; echo "container test failed at line $LINENO" >&2' ERR

app="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo="$(git -C "$app" rev-parse --show-toplevel)"
cd "$app"
image="${KINOSAIL_TEST_IMAGE:-localhost/kinosail-subtitles:test}"
container=""
mcp_jobs=()
media_dir="$(mktemp -d)"
mcp_dir="$(mktemp -d)"
engine="${CONTAINER_ENGINE:-}"
suffix="$$-$RANDOM"
config_volume="kinosail-test-config-$suffix"
cache_volume="kinosail-test-cache-$suffix"
backup_volume="kinosail-test-backups-$suffix"

if [[ -z "$engine" ]]; then
  if command -v podman >/dev/null; then
    engine="podman"
  else
    engine="docker"
  fi
fi

cleanup() {
  if ((${#mcp_jobs[@]})); then
    kill "${mcp_jobs[@]}" >/dev/null 2>&1 || true
    wait "${mcp_jobs[@]}" >/dev/null 2>&1 || true
  fi
  if [[ -n "$container" ]]; then
    "$engine" rm --force "$container" >/dev/null
  fi
  "$engine" volume rm --force "$config_volume" "$cache_volume" "$backup_volume" >/dev/null 2>&1 || true
  if [[ "${KINOSAIL_CLEAN_TEST_IMAGE:-}" == "1" ]]; then
    "$engine" image rm --force "$image" >/dev/null 2>&1 || true
  fi
  rm -rf -- "${media_dir:?}"
  rm -rf -- "${mcp_dir:?}"
}
trap cleanup EXIT

revision=0123456789abcdef0123456789abcdef01234567
build=(build --file "$repo/apps/subtitles/Containerfile" --tag "$image" --build-arg VERSION=test --build-arg "REVISION=$revision")
run=(run)
if [[ "$(basename "$engine")" == "podman" ]]; then
  build+=(--format docker)
fi
if [[ "$(basename "$engine")" == "docker" ]] && "$engine" buildx version >/dev/null 2>&1 && "$engine" image inspect "$image" >/dev/null 2>&1; then
  build+=(--cache-from "$image")
fi
if [[ -n "${KINOSAIL_PLATFORM:-}" ]]; then
  build+=(--platform "$KINOSAIL_PLATFORM")
  run+=(--platform "$KINOSAIL_PLATFORM")
fi
if [[ "${KINOSAIL_TEST_IMAGE_READY:-}" == "1" ]]; then
  "$engine" image inspect "$image" >/dev/null
else
  "$engine" "${build[@]}" "$repo"
  [[ "$("$engine" image inspect --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' "$image")" == "$revision" ]]
fi
"$engine" volume create "$config_volume" >/dev/null
"$engine" volume create "$cache_volume" >/dev/null
"$engine" volume create "$backup_volume" >/dev/null
"$engine" "${run[@]}" --rm --entrypoint fpcalc "$image" -version >/dev/null
fingerprint="$("$engine" "${run[@]}" --rm --entrypoint sh "$image" -c 'ffmpeg -hide_banner -loglevel error -f lavfi -i sine=frequency=440:duration=15 /tmp/fingerprint.wav && fpcalc -json /tmp/fingerprint.wav')"
python3 -c 'import json, sys; result = json.load(sys.stdin); assert result["duration"] == 15 and isinstance(result["fingerprint"], str) and result["fingerprint"]' <<<"$fingerprint"
if "$engine" "${run[@]}" --rm --entrypoint fpcalc "$image" -json /etc/os-release >/dev/null 2>&1; then
  echo 'fpcalc accepted a non-audio file' >&2
  exit 1
fi
"$engine" "${run[@]}" --rm --entrypoint test "$image" -r /licenses/LICENSE
"$engine" "${run[@]}" --rm --entrypoint test "$image" -r /licenses/third_party/hls.js/LICENSE
"$engine" "${run[@]}" --rm --entrypoint test "$image" -r /licenses/third_party/htmx/LICENSE
encoders="$("$engine" "${run[@]}" --rm --entrypoint ffmpeg "$image" -hide_banner -encoders 2>/dev/null)"
filters="$("$engine" "${run[@]}" --rm --entrypoint ffmpeg "$image" -hide_banner -filters 2>/dev/null)"
architecture="$("$engine" "${run[@]}" --rm --entrypoint uname "$image" -m)"
host_architecture="$(uname -m)"
[[ "$host_architecture" == "arm64" ]] && host_architecture="aarch64"
grep --quiet ' signature ' <<<"$filters"
for encoder in libx264 libx265 libsvtav1 libvpx-vp9; do
  grep --quiet "$encoder" <<<"$encoders"
  test_encoder="$encoder"
  encoder_options=(-threads 1)
  if [[ "$encoder" == "libsvtav1" ]]; then
    encoder_options=(-preset 10 -threads 1)
    if [[ "$architecture" != "$host_architecture" ]]; then
      echo "Skipping SVT-AV1 execution under cross-architecture emulation" >&2
      continue
    fi
  fi
  "$engine" "${run[@]}" --rm --entrypoint ffmpeg "$image" -hide_banner -loglevel error -f lavfi -i testsrc2=size=128x72:rate=1:duration=1 -frames:v 1 -c:v "$test_encoder" "${encoder_options[@]}" -f null -
done
[[ "$("$engine" "${run[@]}" --rm --entrypoint ffmpeg "$image" -hide_banner -loglevel error -f lavfi -i testsrc2=size=320x180:rate=1:duration=1 -vf 'fps=1,scale=9:8:force_original_aspect_ratio=decrease,pad=9:8:(ow-iw)/2:(oh-ih)/2,format=gray' -frames:v 1 -an -f rawvideo -pix_fmt gray - | wc -c | tr -d ' ')" == 72 ]]
[[ "$("$engine" "${run[@]}" --rm --entrypoint ffmpeg "$image" -hide_banner -loglevel error -f lavfi -i testsrc2=size=320x180:rate=1:duration=1 -vf 'fps=1,scale=160:90:force_original_aspect_ratio=decrease,pad=160:90:(ow-iw)/2:(oh-ih)/2,format=gray' -frames:v 1 -an -f rawvideo -pix_fmt gray - | wc -c | tr -d ' ')" == 14400 ]]
"$engine" "${run[@]}" --rm --entrypoint dpkg "$image" -s mesa-va-drivers >/dev/null
"$engine" "${run[@]}" --rm --entrypoint dpkg "$image" -s libarchive-tools >/dev/null
if [[ "$architecture" == "x86_64" ]]; then
  "$engine" "${run[@]}" --rm --entrypoint dpkg "$image" -s i965-va-driver intel-media-va-driver >/dev/null
  grep --quiet h264_qsv <<<"$encoders"
  grep --quiet hevc_qsv <<<"$encoders"
  grep --quiet av1_qsv <<<"$encoders"
  grep --quiet vp9_qsv <<<"$encoders"
  grep --quiet h264_nvenc <<<"$encoders"
  grep --quiet hevc_nvenc <<<"$encoders"
  grep --quiet av1_nvenc <<<"$encoders"
  grep --quiet h264_vaapi <<<"$encoders"
  grep --quiet hevc_vaapi <<<"$encoders"
  grep --quiet av1_vaapi <<<"$encoders"
  grep --quiet vp9_vaapi <<<"$encoders"
else
  grep --quiet h264_nvenc <<<"$encoders"
  grep --quiet hevc_nvenc <<<"$encoders"
  grep --quiet av1_nvenc <<<"$encoders"
  grep --quiet h264_rkmpp <<<"$encoders"
  grep --quiet hevc_rkmpp <<<"$encoders"
fi
grep --quiet h264_v4l2m2m <<<"$encoders"
grep --quiet hevc_v4l2m2m <<<"$encoders"
"$engine" "${run[@]}" --rm --entrypoint ffmpeg "$image" -hide_banner -loglevel error -f lavfi -i color=c=blue:s=1280x720:d=12 -c:v ffv1 -f matroska - >"$media_dir/Arrival.mkv"
"$engine" "${run[@]}" --rm --entrypoint ffmpeg "$image" -hide_banner -loglevel error -f lavfi -i testsrc2=size=320x180:rate=24:duration=2 -c:v mpeg2video -f mpegts - >"$media_dir/Transport.ts"
ln "$media_dir/Arrival.mkv" "$media_dir/Beta.mkv"
ln "$media_dir/Arrival.mkv" "$media_dir/Gamma.mkv"
chmod a+rwx "$media_dir"
chmod a+r "$media_dir"/*.mkv "$media_dir"/*.ts

start_server() {
  local publish="127.0.0.1::38128"
  local auth_url=""
  if [[ $# -eq 1 ]]; then
    publish="127.0.0.1:$1:38128"
    auth_url="https://localhost:$1"
  fi
  container="$("$engine" "${run[@]}" --detach --publish "$publish" --env "KINOSAIL_AUTH_URL=$auth_url" --env KINOSAIL_BACKUP_DIR=/backups --env KINOSAIL_BACKUP_KEY=container-test-backup-key --volume "$config_volume:/config" --volume "$cache_volume:/cache" --volume "$backup_volume:/backups" --volume "$media_dir:/media:rw" "$image")"
  mapped_port="$("$engine" port "$container" 38128/tcp)"
  url="https://localhost:${mapped_port##*:}"
  health_host="${auth_url#https://}"
  if [[ -z "$health_host" ]]; then
    health_host="localhost:38128"
  fi

  health=""
  for _ in {1..240}; do
    health="$(curl --fail --silent --insecure --header "Host: $health_host" "$url/healthz" || true)"
    [[ "$health" == '{"status":"ok"}' ]] && return
    sleep 0.25
  done
  return 1
}

expect_status() {
  local expected="$1"
  shift
  local actual
  actual="$(curl --silent --insecure --output "$media_dir/security-response" --write-out '%{http_code}' "$@")"
  if [[ "$actual" != "$expected" ]]; then
    echo "expected HTTP $expected, got $actual: $(cat "$media_dir/security-response")" >&2
    return 1
  fi
}

start_server
port="${url##*:}"
"$engine" rm --force "$container" >/dev/null
container=""
start_server "$port"

expect_status 401 "$url/api/v1/settings"
expect_status 403 --request POST --header 'Origin: https://attacker.example' --data 'name=Attacker&password=attacker-password' "$url/setup"
expect_status 421 --header 'Host: attacker.example' "$url/healthz"
dd if=/dev/zero of="$media_dir/oversized-request" bs=1048577 count=1 2>/dev/null
expect_status 413 --request POST --header 'Content-Type: application/json' --data-binary "@$media_dir/oversized-request" "$url/api/v1/setup"
grep --quiet 'Set up your Server.' < <(curl --fail --silent --insecure "$url/setup")
headers="$(curl --fail --silent --insecure --dump-header - --output /dev/null "$url/healthz")"
grep -qi '^strict-transport-security: max-age=31536000' <<<"$headers"
grep -qi "^content-security-policy: default-src 'self'" <<<"$headers"
grep -qi '^x-content-type-options: nosniff' <<<"$headers"

curl --fail --silent --insecure --cookie-jar "$media_dir/cookies" --data 'name=Owner&password=test-instance-password&totp=true' "$url/setup" --output "$media_dir/setup"
session="$(awk '$6 == "__Host-kinosail_session" { print $7 }' "$media_dir/cookies" | tail -1)"
[[ -n "$session" ]]
csrf="$(printf 'kinosail-csrf\0%s' "$session" | openssl dgst -sha256 -binary | openssl base64 -A | tr '+/' '-_' | tr -d '=')"
[[ "$csrf" =~ ^[A-Za-z0-9_-]{43}$ ]]
secret="$(sed -n 's|.*<code>\([A-Z2-7]\{32\}\)</code>.*|\1|p' "$media_dir/setup" | head -1)"
[[ "$secret" =~ ^[A-Z2-7]{32}$ ]]
secret_hex="$(printf %s "$secret" | "$engine" "${run[@]}" --rm --interactive --entrypoint base32 "$image" --decode | xxd -p -c 256)"
counter="$(printf '%016x' "$(($(date +%s) / 30))")"
digest="$(printf %s "$counter" | xxd -r -p | openssl dgst -sha1 -mac HMAC -macopt "hexkey:$secret_hex" -binary | xxd -p -c 256)"
offset=$((16#${digest:39:1}))
chunk="${digest:$((offset * 2)):8}"
code="$(printf '%06d' "$(((16#$chunk & 0x7fffffff) % 1000000))")"
expect_status 303 --cookie "$media_dir/cookies" --header "Origin: $url" --header "X-Kinosail-CSRF: $csrf" --data "code=$code" "$url/account/mfa/enable"
if [[ "${KINOSAIL_BROWSER_TEST:-}" == "1" ]]; then
  KINOSAIL_TEST_INSTANCE=1 KINOSAIL_TEST_TOTP_SECRET="$secret" KINOSAIL_E2E_URL="$url" KINOSAIL_E2E_OUTPUT_DIR="$media_dir/playwright-results" pnpm --dir e2e test subtitle-dashboard.spec.ts
  exit
fi
mkfifo "$mcp_dir/input"
exec 9<>"$mcp_dir/input"
# Keep all twelve relay processes concurrent without twelve remote-engine SSH handshakes.
# shellcheck disable=SC2016 # Expansion belongs to the container shell.
"$engine" exec --interactive "$container" sh -c 'exec 3<&0; for index in $(seq 1 12); do kinosail mcp-stdio <&3 & done; wait' <"$mcp_dir/input" >/dev/null 2>"$mcp_dir/relay.log" &
mcp_jobs+=("$!")
mcp_relays=0
# Cold starts initialize localized templates before the command can hand off.
mcp_deadline=$((SECONDS + 60))
while ((SECONDS < mcp_deadline)); do
  mcp_relays="$("$engine" exec "$container" sh -c "grep -l '^Name:[[:space:]]*socat$' /proc/[0-9]*/status 2>/dev/null | wc -l" || true)"
  [[ "$mcp_relays" == 12 ]] && break
  sleep 0.1
done
if [[ "$mcp_relays" != 12 ]]; then
  printf 'expected 12 MCP relays, found %s\n' "$mcp_relays" >&2
  cat "$mcp_dir/relay.log" >&2
  # shellcheck disable=SC2016 # Expansion belongs to the container shell.
  "$engine" exec "$container" sh -c 'for status in /proc/[0-9]*/status; do head -7 "$status"; done' >&2
  exit 1
fi
"$engine" exec "$container" sh -c "for status in \$(grep -l '^Name:[[:space:]]*socat$' /proc/[0-9]*/status); do grep -q '^Threads:[[:space:]]*1$' \"\$status\" || exit 1; done"
exec 9>&-
kill "${mcp_jobs[@]}" >/dev/null 2>&1 || true
wait "${mcp_jobs[@]}" >/dev/null 2>&1 || true
mcp_jobs=()
expect_status 401 --cookie "$media_dir/cookies" --header 'Authorization: Bearer invalid' "$url/api/v1/settings"
expect_status 403 --cookie "$media_dir/cookies" --request POST --header 'Origin: https://attacker.example' "$url/api/v1/backups"
expect_status 404 --request POST --header 'Content-Type: application/json' --data '{}' "$url/Sessions/Logout?api_key=$session"
curl --fail --silent --insecure --cookie "$media_dir/cookies" --request POST --header "Origin: $url" --header "X-Kinosail-CSRF: $csrf" "$url/api/v1/backups" --output /dev/null
"$engine" "${run[@]}" --rm --entrypoint sh --volume "$backup_volume:/backups:ro" "$image" -c 'find /backups -name "kinosail-*.backup" -print -quit | grep -q .'
home="$(curl --fail --silent --insecure --cookie "$media_dir/cookies" "$url/")"
grep --quiet Kinosail <<<"$home"
library="$(curl --fail --silent --insecure --cookie "$media_dir/cookies" "$url/api/v1/subtitle-library?view=library")"
grep --quiet '"title":"Transport"' <<<"$library"
grep --quiet '"file":"Transport.ts"' <<<"$library"
"$engine" exec "$container" sh -c 'touch /media/.kinosail-write-test && rm /media/.kinosail-write-test'
cp "$media_dir/Arrival.mkv" "$media_dir/Dune.mkv"
for _ in {1..240}; do
  library="$(curl --fail --silent --insecure --cookie "$media_dir/cookies" "$url/api/v1/subtitle-library?view=library")"
  grep --quiet '"title":"Dune"' <<<"$library" && break
  sleep 0.25
done
grep --quiet '"title":"Dune"' <<<"$library"
printf '1\n00:00:01,000 --> 00:00:02,000\nContainer sidecar\n' >"$media_dir/Dune.en.srt"
curl --fail --silent --insecure --cookie "$media_dir/cookies" --request POST --header "Origin: $url" --header "X-Kinosail-CSRF: $csrf" "$url/scan" --output /dev/null
library="$(curl --fail --silent --insecure --cookie "$media_dir/cookies" "$url/api/v1/subtitle-library?view=library")"
grep -Eq '"title":"Dune"[^}]*"tracks":"en"[^}]*"ready":true' <<<"$library"
