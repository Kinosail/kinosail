#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [[ $# -lt 1 || $# -gt 2 ]]; then
  printf 'Usage: %s ios|tvos [simulator|device]\n' "$0" >&2
  exit 2
fi
case "$1" in
  ios) scheme='Kinosail-iOS'; platform='iOS' ;;
  tvos) scheme='Kinosail-tvOS'; platform='tvOS' ;;
  *) printf 'Choose ios or tvos.\n' >&2; exit 2 ;;
esac
case "${2:-simulator}" in
  simulator) destination="generic/platform=$platform Simulator"; signing=(CODE_SIGNING_ALLOWED=YES CODE_SIGN_IDENTITY=-) ;;
  device) destination="generic/platform=$platform"; signing=(CODE_SIGNING_ALLOWED=NO) ;;
  *) printf 'Choose simulator or device.\n' >&2; exit 2 ;;
esac

revision="$(git -C "$root" rev-parse HEAD)"
if [[ -n "$(git -C "$root" status --porcelain -- .)" ]]; then
  revision="$revision-working-tree"
fi
exec xcodebuild -project "$root/Kinosail.xcodeproj" -scheme "$scheme" \
  -configuration Debug -destination "$destination" \
  -derivedDataPath "$root/.build/$1-${2:-simulator}" -jobs 4 \
  "${signing[@]}" "KINOSAIL_SOURCE_REVISION=$revision" build
