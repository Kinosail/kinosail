#!/usr/bin/env bash
set -euo pipefail

selected="${KINOSAIL_BROWSER_PROJECT:-}"
matrix="${KINOSAIL_BROWSER_MATRIX:-}"
if [[ -n "$matrix" && "$matrix" != "full" ]]; then
  printf 'unsupported browser matrix: %s\n' "$matrix" >&2
  exit 2
fi
if [[ -n "$selected" ]]; then
  case "$selected" in
    chromium|firefox|webkit) printf '%s\n' "$selected" ;;
    *) printf 'unsupported browser project: %s\n' "$selected" >&2; exit 2 ;;
  esac
elif [[ "$matrix" == "full" ]]; then
  printf '%s\n' chromium firefox webkit
else
  printf '%s\n' chromium
fi
