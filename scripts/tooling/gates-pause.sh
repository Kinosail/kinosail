#!/usr/bin/env bash
# Source before a gate does work. The marker has no expiry or environment override.
if [[ -f "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/.gates-disabled" ]]; then
  printf 'Quality gates are disabled until explicitly enabled (.gates-disabled).\n'
  exit 0
fi
