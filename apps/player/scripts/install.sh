#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
media="${1:-}"
port="${2:-38127}"
mode="${3:-}"

if [[ -z "$media" || "$media" != /* || ! -d "$media" || "$media" == *$'\n'* || "$media" == *"'"* || $# -gt 3 || (-n "$mode" && "$mode" != "--lan") ]]; then
  echo "usage: $0 /absolute/path/to/media [port] [--lan]" >&2
  exit 2
fi
if [[ ! "$port" =~ ^[0-9]+$ || ${#port} -gt 5 ]] || ((10#$port < 1 || 10#$port > 65535)); then
  echo "port must be between 1 and 65535" >&2
  exit 2
fi
cd "$root"
gpu_device=""
gpu_backend=""
remote_mode=""
version=""
configured_port=""
read_env_value() {
  local key="$1" count value
  count="$(grep -c "^${key}=" .env || true)"
  if ((count > 1)); then
    echo "$key must occur at most once in .env" >&2
    return 2
  fi
  value="$(sed -n "s/^${key}=//p" .env)"
  if ((${#value} > 256)); then
    echo "$key is too long" >&2
    return 2
  fi
  printf '%s' "$value"
}
if [[ -e .env ]]; then
  gpu_device="$(read_env_value KINOSAIL_GPU_DEVICE)"
  gpu_backend="$(read_env_value KINOSAIL_GPU_BACKEND)"
  remote_mode="$(read_env_value KINOSAIL_REMOTE_MODE)"
  version="$(read_env_value KINOSAIL_VERSION)"
  configured_port="$(read_env_value KINOSAIL_PORT)"
fi
if [[ -n "$configured_port" ]] && { [[ ! "$configured_port" =~ ^[0-9]+$ || ${#configured_port} -gt 5 ]] || ((10#$configured_port < 1 || 10#$configured_port > 65535)); }; then
  echo "KINOSAIL_PORT must be between 1 and 65535" >&2
  exit 2
fi
gpu_backend="${gpu_backend:-auto}"
case "$gpu_backend" in
  auto | off | rkmpp | device) ;;
  *) echo "KINOSAIL_GPU_BACKEND must be auto, off, rkmpp, or device" >&2; exit 2 ;;
esac
if [[ -n "$gpu_device" ]]; then
  [[ "$gpu_backend" == "device" ]] || { echo "KINOSAIL_GPU_DEVICE requires KINOSAIL_GPU_BACKEND=device" >&2; exit 2; }
  if [[ "$gpu_device" =~ ^nvidia\.com/gpu=(all|[0-9]+)$ ]]; then
    :
  else
    source_device="${gpu_device%%:*}"
    container_device="${gpu_device#*:}"
    valid_gpu_path='^/dev/(dri(/[A-Za-z0-9_-]+)?|nvidia([0-9]+|ctl|-uvm|-uvm-tools|-modeset)|video[0-9]+)$'
    if [[ "$source_device" == "$gpu_device" || "$container_device" == *:* || ! "$source_device" =~ $valid_gpu_path || ! "$container_device" =~ $valid_gpu_path ]]; then
      echo "KINOSAIL_GPU_DEVICE must be a canonical GPU or video-encoder /dev source:destination mapping, or an NVIDIA CDI device" >&2
      exit 2
    fi
  fi
elif [[ "$gpu_backend" == "device" ]]; then
  echo "KINOSAIL_GPU_DEVICE is required when KINOSAIL_GPU_BACKEND=device" >&2
  exit 2
fi
case "${remote_mode:-off}" in
  off | https | wireguard) ;;
  *) echo "KINOSAIL_REMOTE_MODE must be off, wireguard, or https" >&2; exit 2 ;;
esac
version="${version:-latest}"
if [[ ! "$version" =~ ^[A-Za-z0-9._-]+$ ]] || ((${#version} > 64)); then
  echo "KINOSAIL_VERSION contains invalid characters" >&2
  exit 1
fi
if command -v podman >/dev/null && podman compose version >/dev/null 2>&1; then
  compose=(podman compose)
elif command -v docker >/dev/null && docker compose version >/dev/null 2>&1; then
  compose=(docker compose)
else
  echo "install Podman Compose or Docker Compose first" >&2
  exit 1
fi
find_device() {
  find "$1" -maxdepth 1 -name "$2" -print -quit 2>/dev/null || true
}
device_group() {
  local group
  group="$(stat -c '%g' "$1" 2>/dev/null || stat -f '%g' "$1" 2>/dev/null || true)"
  [[ "$group" =~ ^[0-9]+$ ]] && printf '%s' "$group" || printf '10001'
}
render_device="$(find_device /dev/dri 'renderD*')"
mpp_device="$(find_device /dev mpp_service)"
rga_device="$(find_device /dev rga)"
dma_heap="$(find_device /dev dma_heap)"
if [[ "$gpu_backend" == "auto" ]]; then
  if [[ -n "$render_device" && -n "$mpp_device" && -n "$rga_device" && -n "$dma_heap" ]]; then
    gpu_backend=rkmpp
  elif command -v nvidia-ctk >/dev/null && nvidia-ctk cdi list 2>/dev/null | grep -Fxq 'nvidia.com/gpu=all'; then
    gpu_backend=device
    gpu_device=nvidia.com/gpu=all
  elif [[ -n "$render_device" ]]; then
    gpu_backend=device
    gpu_device=/dev/dri:/dev/dri
  else
    gpu_backend=off
  fi
fi
export KINOSAIL_GPU_DEVICE="$gpu_device"
group_device="${render_device:-/dev/null}"
if [[ "$gpu_backend" == "device" && "$gpu_device" == /dev/* && "${gpu_device%%:*}" != "/dev/dri" ]]; then
  group_device="${gpu_device%%:*}"
fi
KINOSAIL_GPU_GROUP="$(device_group "$group_device")"
KINOSAIL_GPU_GROUP_1="$KINOSAIL_GPU_GROUP"
KINOSAIL_GPU_GROUP_2="$(device_group "${mpp_device:-/dev/null}")"
export KINOSAIL_GPU_GROUP KINOSAIL_GPU_GROUP_1 KINOSAIL_GPU_GROUP_2
command -v cosign >/dev/null || { echo "cosign is required to verify Kinosail releases" >&2; exit 1; }
umask 077
previous_env="$(mktemp "$root/.env.previous.XXXXXX")"
backup_temporary=""
trap 'rm -f -- "$previous_env" "$backup_temporary"' EXIT
address="https://localhost:$port"
if [[ ! -e .env ]]; then
  if [[ "$mode" == "--lan" ]]; then
    echo "install on localhost and create the Owner Profile before enabling LAN access" >&2
    exit 1
  fi
  printf "KINOSAIL_MEDIA_PATH='%s'\nKINOSAIL_BIND=127.0.0.1\nKINOSAIL_PORT=%s\n" "$media" "$port" >.env
else
	echo "Using existing $root/.env"
	address="the port configured in $root/.env"
fi
cp .env "$previous_env"
project=("${compose[@]}" --file compose.release.yaml)
[[ ! -f kinosail.yaml ]] || project+=(--file compose.config.yaml)
case "$gpu_backend" in
  off) ;;
  rkmpp) project+=(--file compose.rkmpp.yaml) ;;
  device) project+=(--file compose.gpu.yaml) ;;
esac
case "${remote_mode:-off}" in
  off) ;;
  https) project+=(--file compose.remote-https.yaml) ;;
  wireguard) project+=(--file compose.remote-wireguard.yaml) ;;
esac
mkdir -p secrets
if [[ ! -s secrets/backup_key ]]; then
  od -An -N32 -tx1 /dev/urandom | tr -d ' \n' >secrets/backup_key
  chmod 600 secrets/backup_key
fi
if [[ "$mode" == "--lan" ]]; then
  command -v curl >/dev/null || { echo "curl is required to verify Owner setup" >&2; exit 1; }
  setup_scheme=https
  setup_status="$(curl --silent --insecure --output /dev/null --write-out '%{http_code}' "https://127.0.0.1:$configured_port/setup" || true)"
  if [[ "$setup_status" != 303 ]]; then
    setup_scheme=http
    setup_status="$(curl --silent --output /dev/null --write-out '%{http_code}' "http://127.0.0.1:$configured_port/setup" || true)"
  fi
  if [[ ! "$configured_port" =~ ^[0-9]+$ || "$setup_status" != 303 ]]; then
    echo "create the Owner Profile at $setup_scheme://localhost:${configured_port:-$port} before enabling LAN access" >&2
    exit 1
  fi
  lan_ip="$(hostname -I 2>/dev/null | awk '{print $1}')" || lan_ip=""
  if [[ -z "$lan_ip" ]] && command -v route >/dev/null && command -v ipconfig >/dev/null; then
    interface="$(route -n get default 2>/dev/null | awk '/interface:/{print $2; exit}')" || interface=""
    [[ -z "$interface" ]] || lan_ip="$(ipconfig getifaddr "$interface" 2>/dev/null || true)"
  fi
  temporary="$(mktemp "$root/.env.XXXXXX")"
  awk -v ip="$lan_ip" -v port="$configured_port" '
    BEGIN { bind = hosts = auth = 0 }
    /^KINOSAIL_BIND=/ { print "KINOSAIL_BIND=0.0.0.0"; bind = 1; next }
    /^KINOSAIL_TLS_HOSTS=/ { if ($0 == "KINOSAIL_TLS_HOSTS=" && ip != "") print "KINOSAIL_TLS_HOSTS=[\"" ip "\"]"; else print; hosts = 1; next }
    /^KINOSAIL_AUTH_URL=/ { if ($0 == "KINOSAIL_AUTH_URL=" && ip != "") print "KINOSAIL_AUTH_URL=https://" ip ":" port; else print; auth = 1; next }
    { print }
    END {
      if (!bind) print "KINOSAIL_BIND=0.0.0.0"
      if (!hosts && ip != "") print "KINOSAIL_TLS_HOSTS=[\"" ip "\"]"
      if (!auth && ip != "") print "KINOSAIL_AUTH_URL=https://" ip ":" port
    }
  ' .env >"$temporary"
  chmod 600 "$temporary"
  mv "$temporary" .env
  address="$setup_scheme://${lan_ip:-<server-LAN-IP>}:$configured_port"
fi
was_running=""
if "${project[@]}" ps --status running --quiet kinosail 2>/dev/null | grep -q .; then
	was_running=yes
  mkdir -p backups
  backup="backups/kinosail-before-update-$(date -u +%Y%m%dT%H%M%SZ).kinosail-backup"
  backup_temporary="$(mktemp "$root/backups/.kinosail-update.XXXXXX")"
  "${project[@]}" run --rm --no-deps kinosail backup >"$backup_temporary"
  "${project[@]}" run --rm --no-deps -T kinosail backup verify <"$backup_temporary"
  chmod 600 "$backup_temporary"
  mv "$backup_temporary" "$backup"
  backup_temporary=""
  echo "Recovery backup: $root/$backup"
fi
image="ghcr.io/mikeo7/kinosail-player:$version"
KINOSAIL_IMAGE="$image" "${project[@]}" pull
digest="$("${compose[0]}" image inspect --format '{{index .RepoDigests 0}}' "$image")"
if [[ ! "$digest" =~ ^ghcr\.io/mikeo7/kinosail-player@sha256:[a-f0-9]{64}$ ]]; then
  echo "pulled image did not resolve to an expected Kinosail digest" >&2
  exit 1
fi
cosign verify --certificate-identity-regexp '^https://github\.com/MikeO7/kinosail/\.github/workflows/player-release\.yml@refs/tags/player-v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$' --certificate-oidc-issuer https://token.actions.githubusercontent.com "$digest" >/dev/null
if [[ -f kinosail.yaml ]]; then
  KINOSAIL_IMAGE="$digest" "${project[@]}" run --rm --no-deps kinosail config validate
fi
temporary="$(mktemp "$root/.env.XXXXXX")"
awk -v image="$digest" '
  BEGIN { found = 0 }
  /^KINOSAIL_IMAGE=/ { print "KINOSAIL_IMAGE=" image; found = 1; next }
  { print }
  END { if (!found) print "KINOSAIL_IMAGE=" image }
' .env >"$temporary"
chmod 600 "$temporary"
mv "$temporary" .env
healthy=""
if "${project[@]}" up --detach; then
  for _ in {1..40}; do
    if "${project[@]}" exec -T kinosail kinosail healthcheck; then
      healthy=yes
      break
    fi
    sleep 0.5
  done
fi
if [[ -z "$healthy" ]]; then
	"${project[@]}" logs --tail 50 kinosail >&2 || true
	if [[ -n "$was_running" ]]; then
		rollback="$(mktemp "$root/.env.XXXXXX")"
		cp "$previous_env" "$rollback"
		chmod 600 "$rollback"
		mv "$rollback" .env
		"${project[@]}" stop kinosail
		if ! "${project[@]}" run --rm --no-deps -T kinosail restore <"$backup"; then
			echo "Kinosail update and state rollback both failed" >&2
			exit 1
		fi
		"${project[@]}" up --detach
		for _ in {1..40}; do
			if "${project[@]}" exec -T kinosail kinosail healthcheck; then
				echo "Kinosail restored the previous verified image" >&2
				echo "Kinosail update failed and was rolled back" >&2
				exit 1
			fi
			sleep 0.5
		done
		echo "Kinosail update and rollback both failed" >&2
		exit 1
	fi
	echo "Kinosail did not become healthy" >&2
  exit 1
fi
echo "Kinosail is ready at $address"
if [[ "$mode" != "--lan" ]]; then
  echo "Create the Owner Profile, then rerun this command with --lan to share Kinosail on your local network."
fi
