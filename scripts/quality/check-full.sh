#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../tooling/gates-pause.sh"
set -euo pipefail

quality="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
"$quality/check-static.sh"
"$quality/check-coverage.sh"
"$quality/check-crap.sh"
"$quality/check-mutation.sh"
"$quality/check-native-mutation.sh"
