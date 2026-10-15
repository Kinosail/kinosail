#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
runtime="$repo/.verification/code-comments-venv"
requirements="$repo/scripts/tooling/code-comments-requirements.txt"
if [[ ! -f "$runtime/requirements.txt" ]] || ! cmp -s "$requirements" "$runtime/requirements.txt"; then
  python3 -m venv "$runtime"
  "$runtime/bin/python" -m pip install --disable-pip-version-check --no-deps -r "$requirements"
  cp "$requirements" "$runtime/requirements.txt"
fi
cd "$repo"
"$runtime/bin/python" "$repo/scripts/tooling/check-code-comments.py" "$@"
"$runtime/bin/python" "$repo/scripts/tooling/test-code-comments.py"
