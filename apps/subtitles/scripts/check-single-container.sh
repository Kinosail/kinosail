#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../../../scripts/tooling/gates-pause.sh"
set -euo pipefail

for file in compose.yaml compose.release.yaml compose.test.yaml compose.remote-https.yaml; do
  services="$(awk '
    $0 == "services:" { inside = 1; next }
    inside && /^[^ ]/ { inside = 0 }
    inside && /^  [A-Za-z0-9_-]+:$/ { sub(/^  /, ""); sub(/:$/, ""); print }
  ' "$file")"
  expected="kinosail"
  # Public HTTPS adds the restricted gateway; application state remains in kinosail.
  if [[ "$file" == compose.remote-https.yaml ]]; then expected=$'kinosail\npublic-gateway'; fi
  if [[ "$services" != "$expected" ]]; then
    printf '%s has unexpected services; found: %s\n' "$file" "${services:-none}" >&2
    exit 1
  fi
done
