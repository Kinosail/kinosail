#!/usr/bin/env bash
# Literal affected-app verification, with a freshly fetched base and stage evidence.
set -euo pipefail
umask 077
if [[ $# != 1 && $# != 2 ]]; then exit 2; fi
case "$1" in both) apps=(player subtitles) ;; player|subtitles) apps=("$1") ;; *) exit 2 ;; esac
if [[ $# == 2 && "$2" != --admit-only ]]; then exit 2; fi
for variable in KINOSAIL_VERIFY_PLAN KINOSAIL_VERIFY_WORKTREE KINOSAIL_PACKAGES_VERIFIED KINOSAIL_E2E_URL; do
  [[ -z "${!variable:-}" ]] || exit 2
done
[[ $# != 2 ]] || exit 0
repo="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$repo"
test ! -e .gates-disabled
mkdir -p .verification
output="$repo/.verification/verify-changed"
mkdir "$output"
git fetch --no-tags origin main >"$output/fetch.log" 2>&1
head="$(git rev-parse HEAD)" base="$(git rev-parse origin/main)"
common="$(git rev-parse --path-format=absolute --git-common-dir)"
[[ "$head" =~ ^[a-f0-9]{40}$ && "$base" =~ ^[a-f0-9]{40}$ ]]
# This checkout has no restored verification-stage cache; Go dependency caches are separate.
test ! -e "$common/kinosail-verify-cache"
git diff --check origin/main...HEAD
printf '%s\n' "$head" "$base" >"$output/revisions.txt"
status=0
for app in "${apps[@]}"; do
  set +e
  make -C "apps/$app" verify-changed BASE=origin/main >"$output/$app.log" 2>&1
  result=$?
  set -e
  printf '%s %s\n' "$app" "$result" >>"$output/results.txt"
  [[ "$result" == 0 ]] || status=1
  # Retain the actual stage logs, including failure details, with fixed bounded ownership.
  python3 - "$common/kinosail-verify-logs/apps-$app" "$output/$app-stages.tar.gz" <<'PY'
import os, pathlib, stat, sys, tarfile
directory = pathlib.Path(sys.argv[1])
if directory.is_symlink() or not directory.is_dir(): raise SystemExit(2)
files = sorted(directory.iterdir())
if not 1 <= len(files) <= 128: raise SystemExit(2)
total = 0
with tarfile.open(sys.argv[2], 'w:gz') as archive:
    for path in files:
        info = path.lstat()
        if not stat.S_ISREG(info.st_mode) or path.suffix != '.log' or info.st_size > 4194304:
            raise SystemExit(2)
        total += info.st_size
        if total > 16777216: raise SystemExit(2)
        archive.add(path, arcname=path.name, recursive=False)
PY
done
python3 - "$output" <<'PY'
import hashlib, json, pathlib, sys
root = pathlib.Path(sys.argv[1])
head, base = (root / 'revisions.txt').read_text().splitlines()
results = []
for line in (root / 'results.txt').read_text().splitlines():
    app, status = line.split()
    files = []
    for name in (app + '.log', app + '-stages.tar.gz'):
        path = root / name
        files.append({'file': name, 'bytes': path.stat().st_size,
                      'sha256': hashlib.sha256(path.read_bytes()).hexdigest()})
    results.append({'app': app, 'argv': ['make', '-C', 'apps/' + app, 'verify-changed', 'BASE=origin/main'],
                    'exit': int(status), 'files': files})
(root / 'receipt.json').write_text(json.dumps({'schemaVersion': 1, 'head': head, 'base': base,
    'baseRef': 'origin/main', 'verificationStageCacheRestored': False, 'results': results}, indent=2) + '\n')
PY
exit "$status"
