#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/gates-pause.sh"
set -euo pipefail

[[ $# -eq 2 ]] || { echo 'usage: scan-deployment-image.sh ARCHIVE EVIDENCE_DIRECTORY' >&2; exit 2; }
archive="$1"
evidence="$2"
[[ ${#archive} -le 4096 && "$archive" == /* && -f "$archive" && ! -L "$archive" && "$archive" != *$'\n'* ]] || { echo 'invalid image archive' >&2; exit 2; }
[[ ${#evidence} -le 4096 && "$evidence" == /* && "$evidence" != *$'\n'* && ! -L "$evidence" ]] || { echo 'invalid evidence directory' >&2; exit 2; }
command -v trivy >/dev/null
command -v syft >/dev/null
mkdir -p "$evidence"
shasum -a 256 "$archive" >"$evidence/archive.sha256"
# Keep the complete findings separately from the existing release policy gate.
syft scan "docker-archive:$archive" -o "cyclonedx-json=$evidence/sbom.json"
trivy image --input "$archive" --scanners vuln --format json --output "$evidence/vulnerabilities.json"
trivy image --input "$archive" --scanners vuln --skip-db-update \
  --severity HIGH,CRITICAL --ignore-unfixed --exit-code 1 >"$evidence/gate.log"
printf 'Image evidence: %s\n' "$evidence"
