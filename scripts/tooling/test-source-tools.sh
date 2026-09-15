#!/usr/bin/env bash
set -euo pipefail
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT
mkdir "$fixture/bin"
export SOURCE_TOOL_TRACE="$fixture/trace"
cat > "$fixture/bin/uname" <<'STUB'
#!/usr/bin/env bash
if [[ "$1" == -s ]]; then echo Linux; else echo x86_64; fi
STUB
cat > "$fixture/bin/curl" <<'STUB'
#!/usr/bin/env bash
printf 'download\n' >> "$SOURCE_TOOL_TRACE"
STUB
cat > "$fixture/bin/sha256sum" <<'STUB'
#!/usr/bin/env bash
cat >/dev/null
exit 1
STUB
cat > "$fixture/bin/tar" <<'STUB'
#!/usr/bin/env bash
printf 'extracted\n' >> "$SOURCE_TOOL_TRACE"
STUB
chmod +x "$fixture/bin/"*
if PATH="$fixture/bin:$PATH" "$repo/scripts/ci/check-source-tools.sh" >"$fixture/output" 2>&1; then
  echo 'accepted corrupt archives' >&2; exit 1
fi
[[ "$(wc -l < "$SOURCE_TOOL_TRACE" | tr -d ' ')" == 2 ]]
if grep -q extracted "$SOURCE_TOOL_TRACE"; then echo "extracted unverified archive" >&2; exit 1; fi
rm "$SOURCE_TOOL_TRACE"
if PATH="$fixture/bin:$PATH" "$repo/scripts/ci/check-source-tools.sh" unexpected >"$fixture/output" 2>&1; then
  echo 'accepted extra arguments' >&2; exit 1
fi
[[ ! -e "$SOURCE_TOOL_TRACE" ]]
echo 'source tool integrity failures stop before extraction or execution'
