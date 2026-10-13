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
mkdir "$fixture/base-bin"
for command in bash dirname; do
  ln -s "$(command -v "$command")" "$fixture/base-bin/$command"
done
for scanner in trivy syft; do
  mv "$fixture/bin/$scanner" "$fixture/bin/$scanner.hidden"
  status=0
  PATH="$fixture/bin:$fixture/base-bin" "$repo/scripts/tooling/scan-deployment-image.sh" "$fixture/image.tar" "$fixture/evidence" >"$fixture/output" 2>&1 || status=$?
  [[ "$status" == 127 ]] || { echo "missing $scanner lacks a scanner preflight exit" >&2; exit 1; }
  grep -Fq "level=warn operation=image_scan outcome=rejected failure_class=scanner_missing scanner=$scanner" "$fixture/output"
  [[ ! -e "$SCAN_TEST_TRACE" && ! -e "$fixture/evidence" ]]
  mv "$fixture/bin/$scanner.hidden" "$fixture/bin/$scanner"
done
"$repo/scripts/tooling/scan-deployment-image.sh" "$fixture/image.tar" "$fixture/evidence" >"$fixture/output" 2>&1
[[ "$(wc -l < "$SCAN_TEST_TRACE" | tr -d ' ')" == 3 && -s "$fixture/evidence/archive.sha256" ]]
echo 'image scan arguments validated before scans and evidence writes'
