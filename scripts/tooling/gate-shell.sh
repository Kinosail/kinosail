#!/usr/bin/env bash
# Make gate recipes and package scripts share the same persistent switch.
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/gates-pause.sh"
exec /bin/sh "$@"
