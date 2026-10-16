#!/usr/bin/env bash
set -euo pipefail

app="$1"
media="$2"
if docker compose version >/dev/null 2>&1; then
  KINOSAIL_MEDIA_PATH="$media" docker compose --file "$app/compose.release.yaml" --file "$app/compose.nvidia.yaml" config --format json | python3 -c '
import json, sys
service = json.load(sys.stdin)["services"]["kinosail"]
driver_caps = {"compute", "compat32", "graphics", "utility", "video", "display"}
requests = service["deploy"]["resources"]["reservations"]["devices"]
assert [r["driver"] for r in requests] == ["nvidia"], requests
assert "gpu" in requests[0]["capabilities"], requests
assert not driver_caps & set(requests[0]["capabilities"]), "device request capabilities replace NVIDIA_DRIVER_CAPABILITIES"
assert set(service["environment"]["NVIDIA_DRIVER_CAPABILITIES"].split(",")) >= {"compute", "video", "utility"}
'
elif [[ -n "${GITHUB_ACTIONS:-}" ]]; then
  echo 'docker compose is required to render compose.nvidia.yaml in CI' >&2
  exit 1
fi
