#!/usr/bin/env bash
# The hosted graph exercises execution; this catches a linter upgrade that
# rejects native parallel steps or stops validating their children and waits.
set -euo pipefail
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT
cat > "$fixture/valid.yml" <<'YAML'
name: Native parallel validation
on: push
jobs:
  check:
    runs-on: ubuntu-24.04
    steps:
      - parallel:
          - run: 'true'
          - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
      - id: build
        background: true
        run: 'true'
      - wait: build
      - run: 'true'
YAML
actionlint - < "$fixture/valid.yml"

# The errors must come from child validation, not rejection of parallel itself.
# Use an unknown step key to test parsing inside a group without depending on
# optional shellcheck or repository secrets.
sed '/          - run:/a\
            unknown-step-key: true
' "$fixture/valid.yml" > "$fixture/invalid.yml"
if actionlint - < "$fixture/invalid.yml" > "$fixture/output" 2>&1; then
  echo 'accepted an unknown key inside a parallel group' >&2; exit 1
fi
grep -Fq 'unexpected key "unknown-step-key"' "$fixture/output"

sed 's/wait: build/wait: missing/' "$fixture/valid.yml" > "$fixture/invalid.yml"
if actionlint - < "$fixture/invalid.yml" > "$fixture/output" 2>&1; then
  echo 'accepted a wait for a missing background step' >&2; exit 1
fi
grep -Fq '"missing" is not the ID of a preceding background step' "$fixture/output"

sed 's/background: true/background: false/' "$fixture/valid.yml" > "$fixture/invalid.yml"
if actionlint - < "$fixture/invalid.yml" > "$fixture/output" 2>&1; then
  echo 'accepted a wait for a foreground step' >&2; exit 1
fi
grep -Fq '"build" is not the ID of a preceding background step' "$fixture/output"
echo 'native parallel children and synchronization references are validated'
