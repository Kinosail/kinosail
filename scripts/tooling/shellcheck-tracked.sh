#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/gates-pause.sh"
set -euo pipefail

files=()
while IFS= read -r -d '' file; do
  [[ -f "$file" ]] && files+=("$file")
done < <(git ls-files -z -- '*.sh')

((${#files[@]} == 0)) || shellcheck "${files[@]}"
