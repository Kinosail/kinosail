#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../../../scripts/tooling/gates-pause.sh"
set -euo pipefail

services="$(awk '
  $0 == "services:" { inside = 1; next }
  inside && /^[^ ]/ { inside = 0 }
  inside && /^  [A-Za-z0-9_-]+:$/ { sub(/^  /, ""); sub(/:$/, ""); print }
' compose.yaml)"
if [[ "$services" != "dashboard" ]]; then
  printf 'compose.yaml must define exactly one service named dashboard; found: %s\n' "${services:-none}" >&2
  exit 1
fi
