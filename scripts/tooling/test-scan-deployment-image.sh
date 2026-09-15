#!/usr/bin/env bash
set -euo pipefail
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT
mkdir "$fixture/bin"
export SCAN_TEST_TRACE="$fixture/trace"
for command in syft trivy; do
  cat >"$fixture/bin/$command" <<'STUB'
#!/usr/bin/env bash
printf 'executed\n' >> "$SCAN_TEST_TRACE"
STUB
  chmod +x "$fixture/bin/$command"
done
export PATH="$fixture/bin:$PATH"
printf image >"$fixture/image.tar"
ln -s "$fixture/image.tar" "$fixture/link.tar"
reject() {
  if "$repo/scripts/tooling/scan-deployment-image.sh" "$@" >"$fixture/output" 2>&1; then
    echo 'accepted invalid scan arguments' >&2; exit 1
  fi
  [[ ! -e "$SCAN_TEST_TRACE" && ! -e "$fixture/evidence" ]]
}
reject
reject "$fixture/image.tar"
reject "$fixture/image.tar" "$fixture/evidence" extra
reject relative.tar "$fixture/evidence"
reject "$fixture/missing.tar" "$fixture/evidence"
reject "$fixture/link.tar" "$fixture/evidence"
reject "$fixture" "$fixture/evidence"
reject "$fixture/image.tar" relative
reject "$fixture/image.tar" "$fixture/link.tar"
reject "$fixture/image.tar" "$fixture/invalid"$'\n'
reject "/$(printf '%04100d' 0)" "$fixture/evidence"
reject "$fixture/image.tar" "/$(printf '%04100d' 0)"
"$repo/scripts/tooling/scan-deployment-image.sh" "$fixture/image.tar" "$fixture/evidence" >"$fixture/output" 2>&1
[[ "$(wc -l < "$SCAN_TEST_TRACE" | tr -d ' ')" == 3 && -s "$fixture/evidence/archive.sha256" ]]
echo 'image scan arguments validated before scans and evidence writes'
