#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../tooling/gates-pause.sh"
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
[[ "$(uname -s):$(uname -m)" == Linux:x86_64 ]] || { echo 'source tool runner requires Linux x86_64' >&2; exit 1; }
[[ $# -eq 0 ]] || { echo 'source tool runner takes no arguments' >&2; exit 1; }
tools="$(mktemp -d)"
trap 'rm -rf "$tools"' EXIT

# Hashes recorded from the selected upstream release assets. Verify the complete
# archives before extracting the two known executable paths; never use latest.
# This maintained actionlint fork validates native parallel steps, which the
# rhysd/actionlint releases do not support (upstream issues #693 and #694).
curl --proto '=https' --tlsv1.2 --fail --location --max-time 120 --max-filesize 16777216 \
  --output "$tools/shellcheck.tar.gz" \
  https://github.com/koalaman/shellcheck/releases/download/v0.11.0/shellcheck-v0.11.0.linux.x86_64.tar.gz
curl --proto '=https' --tlsv1.2 --fail --location --max-time 120 --max-filesize 16777216 \
  --output "$tools/actionlint.tar.gz" \
  https://github.com/kjanat/actionlint/releases/download/v1.17.0/actionlint_1.17.0_linux_amd64.tar.gz
printf '%s  %s\n' \
  b7af85e41cc99489dcc21d66c6d5f3685138f06d34651e6d34b42ec6d54fe6f6 "$tools/shellcheck.tar.gz" \
  620abd485a12b6ab1125b844a876414e1d5bd2af8a3125b27f82b01d0d9d6e5a "$tools/actionlint.tar.gz" | sha256sum --check --strict

tar -xzf "$tools/shellcheck.tar.gz" -C "$tools" shellcheck-v0.11.0/shellcheck
tar -xzf "$tools/actionlint.tar.gz" -C "$tools" actionlint
export PATH="$tools/shellcheck-v0.11.0:$tools:$PATH"
cd "$repo"
bash scripts/tooling/test-actionlint-parallel.sh
./scripts/tooling/shellcheck-tracked.sh
actionlint
