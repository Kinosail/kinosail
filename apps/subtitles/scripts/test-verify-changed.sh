#!/usr/bin/env bash
set -euo pipefail

app="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo="$(git -C "$app" rev-parse --show-toplevel)"
local_file="$app/scripts/.kinosail-subtitles-scope-test.sh"
sibling_file="$repo/apps/player/scripts/.kinosail-subtitles-sibling-test.sh"
trap 'rm -f "$local_file" "$sibling_file"' EXIT
printf '#!/usr/bin/env bash\ntrue\n' >"$local_file"
printf '#!/usr/bin/env bash\ntrue\n' >"$sibling_file"

base=origin/main
git -C "$repo" rev-parse --verify "$base^{commit}" >/dev/null 2>&1 || base=refs/kinosail/template
output="$(cd "$repo" && KINOSAIL_VERIFY_PLAN=1 KINOSAIL_VERIFY_WORKTREE=1 "$app/scripts/verify-changed.sh" "$base")"
grep -Fq 'PLAN max-loc:' <<<"$output"
grep -Fq 'PLAN diff-check:' <<<"$output"
grep -Fq '.kinosail-subtitles-scope-test.sh' <<<"$output"
if grep -Fq '.kinosail-subtitles-sibling-test.sh' <<<"$output"; then
  printf 'verify-changed included a sibling app path\n' >&2
  exit 1
fi
grep -Fq 'VERIFIED changed paths' <<<"$output"
printf 'verify-changed tests passed\n'
