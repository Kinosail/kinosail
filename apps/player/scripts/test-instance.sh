#!/usr/bin/env bash
set -euo pipefail

app="$(cd "$(dirname "$0")/.." && pwd)"
repo="$(git -C "$app" rev-parse --show-toplevel)"
cd "$app"
engine="${CONTAINER_ENGINE:-}"
if [[ -z "$engine" ]]; then
  if command -v podman >/dev/null; then engine=podman; else engine=docker; fi
fi
export KINOSAIL_TEST_ROOT="${KINOSAIL_TEST_ROOT:-$app/.kinosail-test}"
export KINOSAIL_BIND=127.0.0.1
export KINOSAIL_PORT="${KINOSAIL_PORT:-38127}"
compose=("$engine" compose --project-name "${KINOSAIL_TEST_PROJECT:-kinosail-test}" --file compose.yaml --file compose.test.yaml)

url() {
  local address
  address="$("${compose[@]}" port kinosail 38127)"
  printf 'https://127.0.0.1:%s\n' "${address##*:}"
}

wait_ready() {
  local base health=""
  base="$(url)"
  for _ in {1..80}; do
    health="$(curl --fail --silent --insecure "$base/healthz" || true)"
    [[ "$health" == '{"status":"ok"}' ]] && return
    sleep 0.25
  done
  printf 'test instance did not become healthy at %s\n' "$base" >&2
  return 1
}

totp_code() {
  local secret secret_hex counter digest offset chunk
  secret="$(<"$KINOSAIL_TEST_ROOT/totp-secret")"
  secret_hex="$(printf %s "$secret" | "$engine" run --rm --interactive --entrypoint base32 localhost/kinosail:dev --decode | xxd -p -c 256)"
  counter="$(printf '%016x' "$(($(date +%s) / 30))")"
  digest="$(printf %s "$counter" | xxd -r -p | openssl dgst -sha1 -mac HMAC -macopt "hexkey:$secret_hex" -binary | xxd -p -c 256)"
  offset=$((16#${digest:39:1}))
  chunk="${digest:$((offset * 2)):8}"
  printf '%06d\n' "$(((16#$chunk & 0x7fffffff) % 1000000))"
}

prepare() {
  local staging previous
  mkdir -p "$KINOSAIL_TEST_ROOT"
  if [[ "${KINOSAIL_TEST_IMAGE_READY:-}" != "1" ]]; then
    "$engine" build --file "$repo/apps/player/Containerfile" --tag localhost/kinosail:dev "$repo"
  fi
  "${compose[@]}" build kinosail
  staging="$(mktemp -d "$KINOSAIL_TEST_ROOT/media.XXXXXX")"
  KINOSAIL_TEST_IMAGE=localhost/kinosail:dev CONTAINER_ENGINE="$engine" ./scripts/generate-test-media.sh "$staging"
  previous="$KINOSAIL_TEST_ROOT/media.previous"
  rm -rf -- "$previous"
  if [[ -e "$KINOSAIL_TEST_ROOT/media" ]]; then mv "$KINOSAIL_TEST_ROOT/media" "$previous"; fi
  mv "$staging" "$KINOSAIL_TEST_ROOT/media"
  rm -rf -- "$previous"
}

setup_owner() {
  local base code response token secret confirmation onboarding
  base="$(url)"
  response="$KINOSAIL_TEST_ROOT/setup.json"
  code="$(curl --silent --insecure --output "$response" --write-out '%{http_code}' --header 'Content-Type: application/json' \
    --data '{"name":"Owner","password":"test-instance-password","device":"Public test instance","totp":true}' "$base/api/v1/setup")"
  if [[ "$code" == 201 ]]; then
    token="$(sed -n 's/.*"token":"\([^"]*\)".*/\1/p' "$response")"
    secret="$(sed -n 's/.*"secret":"\([A-Z2-7]*\)".*/\1/p' "$response")"
    [[ -n "$token" && "$secret" =~ ^[A-Z2-7]{32}$ ]]
    printf %s "$secret" >"$KINOSAIL_TEST_ROOT/totp-secret"
    chmod 600 "$KINOSAIL_TEST_ROOT/totp-secret"
    confirmation="$(curl --silent --insecure --output /dev/null --write-out '%{http_code}' --request PUT \
      --header 'Content-Type: application/json' --header "Authorization: Bearer $token" \
      --data "{\"code\":\"$(totp_code)\"}" "$base/api/v1/me/mfa")"
    [[ "$confirmation" == 200 ]]
    onboarding="$(curl --silent --insecure --output /dev/null --write-out '%{http_code}' --request PUT \
      --header 'Content-Type: application/json' --header "Authorization: Bearer $token" \
      --data '{"enabled":false}' "$base/api/v1/settings/onboarding")"
    [[ "$onboarding" == 200 ]]
  fi
  rm -f -- "$response"
  if [[ "$code" != 201 && "$code" != 409 ]]; then
    printf 'could not initialize test Owner Profile: HTTP %s\n' "$code" >&2
    return 1
  fi
}

action="${1:-}"
shift || true
case "$action" in
  up)
    prepare
    "${compose[@]}" down --volumes --remove-orphans
    "${compose[@]}" up --detach
    wait_ready
    setup_owner
    printf 'Kinosail test instance: %s\nOwner: Owner\nPassword: test-instance-password\n' "$(url)"
    ;;
  down)
    "${compose[@]}" down "$@"
    ;;
  url)
    url
    ;;
  verify)
    ./scripts/verify-test-instance.sh "$(url)"
    ;;
  browser)
    fixture_dir="$KINOSAIL_TEST_ROOT/ui-fixtures"
    KINOSAIL_UI_FIXTURE_DIR="$fixture_dir" go test ./internal/server -run TestWriteUIStateFixtures -count=1
    KINOSAIL_TEST_INSTANCE=1 KINOSAIL_UI_FIXTURE_DIR="$fixture_dir" KINOSAIL_TEST_TOTP_SECRET="$(<"$KINOSAIL_TEST_ROOT/totp-secret")" KINOSAIL_E2E_URL="$(url)" KINOSAIL_E2E_OUTPUT_DIR="$KINOSAIL_TEST_ROOT/playwright-results" pnpm --dir e2e test test-instance.spec.ts test-instance-production.spec.ts ui-happy-paths.spec.ts shortcuts.spec.ts jellyfin-setup.spec.ts playback-bandwidth.spec.ts playback-pause-repro.spec.ts playback-startup.spec.ts layout-audit.spec.ts masthead-spacing.spec.ts conditional-states.spec.ts password-reveal.spec.ts loading-review.spec.ts session-timeouts.spec.ts --workers="${KINOSAIL_E2E_WORKERS:-1}"
    ;;
  playback)
    fixture_dir="$KINOSAIL_TEST_ROOT/ui-fixtures"
    results="$KINOSAIL_TEST_ROOT/playback-results"
    KINOSAIL_UI_FIXTURE_DIR="$fixture_dir" go test ./internal/server -run TestWriteUIStateFixtures -count=1
    playwright=(env KINOSAIL_TEST_INSTANCE=1 KINOSAIL_UI_FIXTURE_DIR="$fixture_dir" KINOSAIL_TEST_TOTP_SECRET="$(<"$KINOSAIL_TEST_ROOT/totp-secret")" KINOSAIL_E2E_URL="$(url)" KINOSAIL_BROWSER_MATRIX=full pnpm --dir e2e exec playwright test)
    "${playwright[@]}" player-experience.spec.ts player-direct-fallback.spec.ts player-seeking.spec.ts player-quality.spec.ts player-duration.spec.ts playback-pause-repro.spec.ts playback-startup.spec.ts instant-playback.spec.ts instant-show-play.spec.ts loading-review.spec.ts --workers=1 --reporter=line --output="$results/player-matrix"
    "${playwright[@]}" playback-bandwidth.spec.ts --project=chromium --workers=1 --repeat-each=3 --reporter=line --output="$results/bandwidth"
    "${playwright[@]}" layout-audit.spec.ts --grep 'player shows and switches|player stays accessible|artwork-backed media' --workers=1 --reporter=line --output="$results/accessibility"
    ;;
  totp)
    totp_code
    ;;
  *)
    printf 'usage: %s {up|down [--volumes]|url|verify|browser|playback|totp}\n' "$0" >&2
    exit 2
    ;;
esac
