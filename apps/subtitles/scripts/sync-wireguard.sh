#!/usr/bin/env bash
set -euo pipefail

config="${1:-/etc/wireguard/kinosail.conf}"
[[ -f "$config" ]] || { echo "WireGuard configuration not found: $config" >&2; exit 1; }
temporary="$(mktemp)"
trap 'rm -f "$temporary"' EXIT
wg-quick strip "$config" >"$temporary"
wg syncconf kinosail "$temporary"
