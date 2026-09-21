#!/usr/bin/env bash
set -euo pipefail

source_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT
mkdir -p "$fixture/app/scripts" "$fixture/bin" "$fixture/media"
cp "$source_root/scripts/install.sh" "$source_root/scripts/uninstall.sh" "$fixture/app/scripts/"
cp "$source_root/compose.release.yaml" "$source_root/compose.config.yaml" "$source_root/compose.gpu.yaml" "$source_root/compose.rkmpp.yaml" "$source_root/compose.remote-https.yaml" "$fixture/app/"
grep -Fq "\${KINOSAIL_BIND:-127.0.0.1}" "$fixture/app/compose.release.yaml"
grep -Fq "name: \"\${KINOSAIL_PROJECT_NAME:-kinosail}\"" "$fixture/app/compose.release.yaml"
# shellcheck disable=SC2016 # The literal Compose interpolation is the assertion target.
grep -Fq 'image: "${KINOSAIL_IMAGE:-ghcr.io/kinosail/kinosail-player:latest}"' "$fixture/app/compose.release.yaml"
grep -Fq "KINOSAIL_AUTH_URL: \"\${KINOSAIL_AUTH_URL:-}\"" "$fixture/app/compose.release.yaml"
grep -Fq "KINOSAIL_TLS_ENABLED: \"\${KINOSAIL_TLS_ENABLED:-}\"" "$fixture/app/compose.release.yaml"
grep -Fq "KINOSAIL_TLS_HOSTS: \"\${KINOSAIL_TLS_HOSTS:-}\"" "$fixture/app/compose.release.yaml"
grep -Fq 'cap_drop: [ALL]' "$fixture/app/compose.release.yaml"
grep -Fq 'user: "10001:10001"' "$fixture/app/compose.release.yaml"
grep -Fq '/run/secrets/kinosail_backup_key:ro' "$fixture/app/compose.release.yaml"
grep -Fq '/tmp:rw,noexec,nosuid,nodev,size=256m' "$fixture/app/compose.release.yaml"
grep -Fq 'KINOSAIL_GPU_GROUP:-10001' "$fixture/app/compose.gpu.yaml"
grep -Fq 'KINOSAIL_GPU_GROUP_1:-10001' "$fixture/app/compose.rkmpp.yaml"
export KINOSAIL_IMAGE=unverified-image
export KINOSAIL_INSTALL_TEST_LOG="$fixture/compose.log" KINOSAIL_INSTALL_TEST_STATE="$fixture/running" KINOSAIL_INSTALL_TEST_DATA="$fixture/private-state"
printf initial >"$fixture/private-state"
cat >"$fixture/bin/podman" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
printf 'gpu=%s groups=%s,%s %s\n' "${KINOSAIL_GPU_DEVICE:-}" "${KINOSAIL_GPU_GROUP_1:-}" "${KINOSAIL_GPU_GROUP_2:-}" "$*" >>"$KINOSAIL_INSTALL_TEST_LOG"
[[ "${KINOSAIL_IMAGE:-}" != "unverified-image" ]] || { echo 'unverified inherited image reached Compose' >&2; exit 91; }
case "$*" in
  *"compose version"*) echo "test compose" ;;
  *"config --images"*) echo "ghcr.io/kinosail/kinosail-player:latest" ;;
  *"image inspect"*) echo "ghcr.io/kinosail/kinosail-player@sha256:${KINOSAIL_INSTALL_TEST_DIGEST:-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa}" ;;
  *"ps --status running --quiet kinosail"*) [[ -f "$KINOSAIL_INSTALL_TEST_STATE" ]] && echo container || true ;;
  *"run --rm --no-deps -T kinosail backup verify"*)
    [[ -z "${KINOSAIL_INSTALL_TEST_FAIL_VERIFY:-}" ]] || exit 1
    cat >/dev/null ;;
  *"run --rm --no-deps -T kinosail restore"*) cat >"$KINOSAIL_INSTALL_TEST_DATA" ;;
  *"run --rm --no-deps kinosail backup"*) cat "$KINOSAIL_INSTALL_TEST_DATA" ;;
  *"run --rm --no-deps kinosail config validate"*) printf 'Configuration is valid\n' ;;
  *"stop kinosail"*) rm -f "$KINOSAIL_INSTALL_TEST_STATE" ;;
  *"up --detach"*)
    touch "$KINOSAIL_INSTALL_TEST_STATE"
    if [[ -n "${KINOSAIL_INSTALL_TEST_FAIL_DIGEST:-}" ]] && grep -q "sha256:$KINOSAIL_INSTALL_TEST_FAIL_DIGEST" .env; then
      printf migrated >"$KINOSAIL_INSTALL_TEST_DATA"
      [[ -z "${KINOSAIL_INSTALL_TEST_FAIL_START:-}" ]] || exit 1
    fi ;;
  *"logs --tail 50 kinosail"*) [[ -z "${KINOSAIL_INSTALL_TEST_FAIL_LOGS:-}" ]] ;;
  *"exec -T kinosail kinosail healthcheck"*)
    if [[ -n "${KINOSAIL_INSTALL_TEST_FAIL_DIGEST:-}" ]] && grep -q "sha256:$KINOSAIL_INSTALL_TEST_FAIL_DIGEST" .env; then exit 1; fi ;;
  *"down --remove-orphans"*) rm -f "$KINOSAIL_INSTALL_TEST_STATE" ;;
esac
FAKE
chmod +x "$fixture/bin/podman"
cat >"$fixture/bin/cosign" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
printf 'cosign %s\n' "$*" >>"$KINOSAIL_INSTALL_TEST_LOG"
FAKE
chmod +x "$fixture/bin/cosign"
cat >"$fixture/bin/curl" <<'FAKE'
#!/usr/bin/env bash
printf '%s' "${KINOSAIL_INSTALL_TEST_SETUP_STATUS:-303}"
FAKE
chmod +x "$fixture/bin/curl"
cat >"$fixture/bin/hostname" <<'FAKE'
#!/usr/bin/env bash
[[ "${1:-}" == "-I" ]] && printf '192.0.2.55\n'
FAKE
chmod +x "$fixture/bin/hostname"
cat >"$fixture/bin/find" <<'FAKE'
#!/usr/bin/env bash
case "${KINOSAIL_INSTALL_TEST_GPU:-}:$*" in
  dri:*'/dev/dri '*'renderD*'*) printf '/dev/dri/renderD129\n' ;;
  rkmpp:*'/dev/dri '*'renderD*'*) printf '/dev/dri/renderD128\n' ;;
  rkmpp:*'/dev '*'mpp_service'*) printf '/dev/mpp_service\n' ;;
  rkmpp:*'/dev '*' rga '*) printf '/dev/rga\n' ;;
  rkmpp:*'/dev '*'dma_heap'*) printf '/dev/dma_heap\n' ;;
esac
FAKE
chmod +x "$fixture/bin/find"
cat >"$fixture/bin/stat" <<'FAKE'
#!/usr/bin/env bash
case "${*: -1}" in
  *renderD129) printf '109\n' ;;
  *renderD128) printf '105\n' ;;
  *mpp_service) printf '44\n' ;;
	*video11) printf '81\n' ;;
  *) printf '10001\n' ;;
esac
FAKE
chmod +x "$fixture/bin/stat"
cat >"$fixture/bin/nvidia-ctk" <<'FAKE'
#!/usr/bin/env bash
[[ "${KINOSAIL_INSTALL_TEST_GPU:-}" == nvidia && "$*" == "cdi list" ]] && printf 'nvidia.com/gpu=all\n'
FAKE
chmod +x "$fixture/bin/nvidia-ctk"
assert_gpu_rejected() {
  local device="$1" backend="${2:-device}"
  printf "KINOSAIL_MEDIA_PATH='%s'\nKINOSAIL_BIND=127.0.0.1\nKINOSAIL_PORT=9080\nKINOSAIL_GPU_DEVICE=%s\nKINOSAIL_GPU_BACKEND=%s\n" "$fixture/media" "$device" "$backend" >"$fixture/app/.env"
  cp "$fixture/app/.env" "$fixture/before.env"
  : >"$fixture/compose.log"
  if PATH="$fixture/bin:$PATH" "$fixture/app/scripts/install.sh" "$fixture/media" 9080 >/dev/null 2>&1; then
    echo "invalid GPU configuration must fail: $device / $backend" >&2
    exit 1
  fi
  cmp "$fixture/before.env" "$fixture/app/.env"
  [[ ! -s "$fixture/compose.log" && ! -e "$fixture/app/secrets" ]]
}
assert_gpu_rejected '/dev/../../etc/shadow:/dev/dri'
assert_gpu_rejected '/dev/dri/../card0:/dev/dri/card0'
assert_gpu_rejected '/dev/dri:/dev/dri:rw'
assert_gpu_rejected '/dev/null:/dev/null'
assert_gpu_rejected '/dev/dri:/dev/dri' rkmpp
assert_gpu_rejected '' device
printf "KINOSAIL_MEDIA_PATH='%s'\nKINOSAIL_GPU_BACKEND=auto\nKINOSAIL_GPU_BACKEND=off\n" "$fixture/media" >"$fixture/app/.env"
cp "$fixture/app/.env" "$fixture/before.env"
: >"$fixture/compose.log"
if PATH="$fixture/bin:$PATH" "$fixture/app/scripts/install.sh" "$fixture/media" 9080 >/dev/null 2>&1; then
  echo "duplicate GPU configuration must fail" >&2
  exit 1
fi
cmp "$fixture/before.env" "$fixture/app/.env"
[[ ! -s "$fixture/compose.log" && ! -e "$fixture/app/secrets" ]]
for key in KINOSAIL_REMOTE_MODE KINOSAIL_VERSION KINOSAIL_PORT; do
  printf "KINOSAIL_MEDIA_PATH='%s'\n%s=value\n%s=other\n" "$fixture/media" "$key" "$key" >"$fixture/app/.env"
  cp "$fixture/app/.env" "$fixture/before.env"
  if PATH="$fixture/bin:$PATH" "$fixture/app/scripts/install.sh" "$fixture/media" 9080 >/dev/null 2>&1; then
    echo "duplicate $key configuration must fail" >&2
    exit 1
  fi
  cmp "$fixture/before.env" "$fixture/app/.env"
  [[ ! -s "$fixture/compose.log" && ! -e "$fixture/app/secrets" ]]
done
for version in edge 01.2.3 v1.2.3 1.2 1.2.3-beta '1.2.3/other'; do
  printf "KINOSAIL_MEDIA_PATH='%s'\nKINOSAIL_VERSION=%s\n" "$fixture/media" "$version" >"$fixture/app/.env"
  cp "$fixture/app/.env" "$fixture/before.env"
  if PATH="$fixture/bin:$PATH" "$fixture/app/scripts/install.sh" "$fixture/media" 9080 >/dev/null 2>&1; then
    echo "invalid release selector must fail: $version" >&2
    exit 1
  fi
  cmp "$fixture/before.env" "$fixture/app/.env"
  [[ ! -s "$fixture/compose.log" && ! -e "$fixture/app/secrets" ]]
done
printf -v oversized_version '%065d' 0
printf "KINOSAIL_MEDIA_PATH='%s'\nKINOSAIL_VERSION=%s\n" "$fixture/media" "$oversized_version" >"$fixture/app/.env"
cp "$fixture/app/.env" "$fixture/before.env"
if PATH="$fixture/bin:$PATH" "$fixture/app/scripts/install.sh" "$fixture/media" 9080 >/dev/null 2>&1; then
  echo "oversized version configuration must fail" >&2
  exit 1
fi
cmp "$fixture/before.env" "$fixture/app/.env"
[[ ! -s "$fixture/compose.log" && ! -e "$fixture/app/secrets" ]]
printf "KINOSAIL_MEDIA_PATH='%s'\nKINOSAIL_PORT=70000\n" "$fixture/media" >"$fixture/app/.env"
cp "$fixture/app/.env" "$fixture/before.env"
if PATH="$fixture/bin:$PATH" "$fixture/app/scripts/install.sh" "$fixture/media" 9080 >/dev/null 2>&1; then
  echo "out-of-range port configuration must fail" >&2
  exit 1
fi
cmp "$fixture/before.env" "$fixture/app/.env"
[[ ! -s "$fixture/compose.log" && ! -e "$fixture/app/secrets" ]]
printf -v oversized_device '/dev/dri:/dev/dri%0240d' 0
assert_gpu_rejected "$oversized_device"
rm "$fixture/app/.env" "$fixture/before.env" "$fixture/compose.log"
cat >"$fixture/bin/sleep" <<'FAKE'
#!/usr/bin/env bash
exit 0
FAKE
chmod +x "$fixture/bin/sleep"
if PATH="$fixture/bin:$PATH" "$fixture/app/scripts/install.sh" "$fixture/media" 9080 media.example.com >/dev/null 2>&1; then
  echo "remote-hostname mode must stay out of the local installer" >&2
  exit 1
fi
PATH="$fixture/bin:$PATH" "$fixture/app/scripts/install.sh" "$fixture/media" 9080 >/dev/null
[[ "$(wc -c <"$fixture/app/secrets/backup_key" | tr -d ' ')" == 64 ]]
[[ -n "$(find "$fixture/app/secrets/backup_key" -perm 600 -print)" ]]
cp "$fixture/app/.env" "$fixture/before.env"
export KINOSAIL_INSTALL_TEST_FAIL_VERIFY=1
if PATH="$fixture/bin:$PATH" "$fixture/app/scripts/install.sh" "$fixture/media" 9080 >/dev/null 2>&1; then
	echo "damaged recovery backup must stop the update" >&2
	exit 1
fi
unset KINOSAIL_INSTALL_TEST_FAIL_VERIFY
cmp "$fixture/before.env" "$fixture/app/.env"
[[ -e "$fixture/running" ]]
if compgen -G "$fixture/app/backups/.kinosail-update.*" >/dev/null; then
	echo "failed backup left a temporary file" >&2
	exit 1
fi
rm "$fixture/before.env"
printf 'version: 1\nserver:\n  name: File Home\n' >"$fixture/app/kinosail.yaml"
printf 'KINOSAIL_GPU_BACKEND=invalid\n' >>"$fixture/app/.env"
log_size="$(wc -c <"$fixture/compose.log")"
if PATH="$fixture/bin:$PATH" "$fixture/app/scripts/install.sh" "$fixture/media" 9080 >/dev/null 2>&1; then
  echo "invalid GPU backend must fail" >&2
  exit 1
fi
[[ "$(wc -c <"$fixture/compose.log")" == "$log_size" ]]
sed -i.bak 's/^KINOSAIL_GPU_BACKEND=invalid$/KINOSAIL_GPU_BACKEND=off/' "$fixture/app/.env"
rm "$fixture/app/.env.bak"
export KINOSAIL_INSTALL_TEST_SETUP_STATUS=200
if PATH="$fixture/bin:$PATH" "$fixture/app/scripts/install.sh" "$fixture/media" 9080 --lan >/dev/null 2>&1; then
  echo "LAN exposure must fail before Owner setup" >&2
  exit 1
fi
grep -q '^KINOSAIL_BIND=127.0.0.1$' "$fixture/app/.env"
sed -i.bak 's/^KINOSAIL_GPU_BACKEND=off$/KINOSAIL_GPU_BACKEND=device/' "$fixture/app/.env"
rm "$fixture/app/.env.bak"
printf 'KINOSAIL_GPU_DEVICE=/dev/video11:/dev/video11\n' >>"$fixture/app/.env"
export KINOSAIL_INSTALL_TEST_SETUP_STATUS=303
PATH="$fixture/bin:$PATH" "$fixture/app/scripts/install.sh" "$fixture/media" 9080 --lan >/dev/null
sed -i.bak -e 's|^KINOSAIL_GPU_DEVICE=.*$|KINOSAIL_GPU_DEVICE=|' -e 's/^KINOSAIL_GPU_BACKEND=device$/KINOSAIL_GPU_BACKEND=rkmpp/' "$fixture/app/.env"
rm "$fixture/app/.env.bak"
PATH="$fixture/bin:$PATH" "$fixture/app/scripts/install.sh" "$fixture/media" 9080 >/dev/null
sed -i.bak 's/^KINOSAIL_GPU_BACKEND=rkmpp$/KINOSAIL_GPU_BACKEND=auto/' "$fixture/app/.env"
rm "$fixture/app/.env.bak"
export KINOSAIL_INSTALL_TEST_GPU=dri
PATH="$fixture/bin:$PATH" "$fixture/app/scripts/install.sh" "$fixture/media" 9080 >/dev/null
export KINOSAIL_INSTALL_TEST_GPU=nvidia
PATH="$fixture/bin:$PATH" "$fixture/app/scripts/install.sh" "$fixture/media" 9080 >/dev/null
export KINOSAIL_INSTALL_TEST_GPU=rkmpp
PATH="$fixture/bin:$PATH" "$fixture/app/scripts/install.sh" "$fixture/media" 9080 >/dev/null
unset KINOSAIL_INSTALL_TEST_GPU
KINOSAIL_IMAGE="" PATH="$fixture/bin:$PATH" "$fixture/app/scripts/uninstall.sh" >/dev/null
grep -q '^KINOSAIL_BIND=0.0.0.0$' "$fixture/app/.env"
grep -Fq 'KINOSAIL_TLS_HOSTS=["192.0.2.55"]' "$fixture/app/.env"
grep -q '^KINOSAIL_AUTH_URL=https://192.0.2.55:9080$' "$fixture/app/.env"
grep -q '^KINOSAIL_IMAGE=ghcr.io/kinosail/kinosail-player@sha256:a\{64\}$' "$fixture/app/.env"
grep -Fq 'cosign verify --certificate-identity https://github.com/Kinosail/kinosail/.github/workflows/delivery.yml@refs/heads/main --certificate-oidc-issuer https://token.actions.githubusercontent.com' "$fixture/compose.log"
grep -q 'exec -T kinosail kinosail healthcheck' "$fixture/compose.log"
grep -q 'run --rm --no-deps kinosail backup' "$fixture/compose.log"
grep -q 'run --rm --no-deps -T kinosail backup verify' "$fixture/compose.log"
grep -q -- '--file compose.config.yaml.*run --rm --no-deps kinosail config validate' "$fixture/compose.log"
grep -q -- '--file compose.release.yaml.*--file compose.gpu.yaml' "$fixture/compose.log"
grep -q -- '--file compose.release.yaml.*--file compose.rkmpp.yaml' "$fixture/compose.log"
grep -q 'gpu=/dev/dri:/dev/dri groups=109,10001 .*--file compose.release.yaml.*--file compose.gpu.yaml' "$fixture/compose.log"
grep -q 'gpu=/dev/video11:/dev/video11 groups=81,10001 .*--file compose.release.yaml.*--file compose.gpu.yaml' "$fixture/compose.log"
grep -q 'gpu=nvidia.com/gpu=all .*--file compose.release.yaml.*--file compose.gpu.yaml' "$fixture/compose.log"
grep -q 'groups=105,44 .*--file compose.release.yaml.*--file compose.rkmpp.yaml' "$fixture/compose.log"
compgen -G "$fixture/app/backups/kinosail-before-update-*.kinosail-backup" >/dev/null
[[ -d "$fixture/media" && ! -e "$fixture/running" ]]

sed -i.bak '/^KINOSAIL_VERSION=/d' "$fixture/app/.env"
rm "$fixture/app/.env.bak"
printf 'KINOSAIL_VERSION=1.2.3\n' >>"$fixture/app/.env"
PATH="$fixture/bin:$PATH" "$fixture/app/scripts/install.sh" "$fixture/media" 9080 >/dev/null
grep -Fq 'cosign verify --certificate-identity https://github.com/Kinosail/kinosail/.github/workflows/player-release.yml@refs/tags/player-v1.2.3 --certificate-oidc-issuer https://token.actions.githubusercontent.com' "$fixture/compose.log"
sed -i.bak '/^KINOSAIL_VERSION=/d' "$fixture/app/.env"
rm "$fixture/app/.env.bak"

touch "$fixture/running"
printf original >"$fixture/private-state"
export KINOSAIL_INSTALL_TEST_DIGEST=cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
export KINOSAIL_INSTALL_TEST_FAIL_DIGEST="$KINOSAIL_INSTALL_TEST_DIGEST"
if PATH="$fixture/bin:$PATH" "$fixture/app/scripts/install.sh" "$fixture/media" 9080 --lan >/dev/null 2>&1; then
	echo "unhealthy update must fail after rollback" >&2
	exit 1
fi
grep -q '^KINOSAIL_IMAGE=ghcr.io/kinosail/kinosail-player@sha256:a\{64\}$' "$fixture/app/.env"
[[ -e "$fixture/running" ]]
[[ "$(cat "$fixture/private-state")" == original ]]
grep -q 'stop kinosail' "$fixture/compose.log"
grep -q 'run --rm --no-deps -T kinosail restore' "$fixture/compose.log"

# Compose may fail after replacing the container and changing persistent state.
# Even unavailable logs must not prevent restoring the old image and backup.
for failure in start logs both; do
  unset KINOSAIL_INSTALL_TEST_FAIL_START KINOSAIL_INSTALL_TEST_FAIL_LOGS
  [[ "$failure" == logs ]] || export KINOSAIL_INSTALL_TEST_FAIL_START=1
  [[ "$failure" == start ]] || export KINOSAIL_INSTALL_TEST_FAIL_LOGS=1
  cp "$fixture/app/.env" "$fixture/before.env"
  : >"$fixture/compose.log"
  if PATH="$fixture/bin:$PATH" "$fixture/app/scripts/install.sh" "$fixture/media" 9080 >"$fixture/failure.log" 2>&1; then
    echo "failed update must return failure after rollback: $failure" >&2
    exit 1
  fi
  cmp "$fixture/before.env" "$fixture/app/.env"
  [[ -e "$fixture/running" && "$(cat "$fixture/private-state")" == original ]]
  grep -q 'run --rm --no-deps -T kinosail restore' "$fixture/compose.log"
  grep -q 'Kinosail update failed and was rolled back' "$fixture/failure.log"
done
unset KINOSAIL_INSTALL_TEST_FAIL_START KINOSAIL_INSTALL_TEST_FAIL_LOGS

# A failed first install has no previous state to restore.
rm "$fixture/running" "$fixture/app/.env"
: >"$fixture/compose.log"
export KINOSAIL_INSTALL_TEST_FAIL_START=1
if PATH="$fixture/bin:$PATH" "$fixture/app/scripts/install.sh" "$fixture/media" 9080 >"$fixture/failure.log" 2>&1; then
  echo "failed first start must not report success" >&2
  exit 1
fi
grep -q 'Kinosail did not become healthy' "$fixture/failure.log"
if grep -q 'run --rm --no-deps -T kinosail restore' "$fixture/compose.log"; then
  echo "first install must not attempt a nonexistent rollback" >&2
  exit 1
fi
