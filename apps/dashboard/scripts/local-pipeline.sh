#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../../../scripts/tooling/gates-pause.sh"
set -euo pipefail

app="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$app"

make check
make container-test
make test-instance-check
make browser-test
