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
if [[ -L .verification || -e .verification && ! -d .verification ]]; then exit 2; fi
mkdir -p .verification
output="$repo/.verification/verify-changed"
mkdir "$output"
phase=gates head="" base=""
finish() {
  local original="${1:-$?}" collected
  trap - EXIT
  set +e
  python3 - "$output" "$phase" "$original" "$head" "$base" <<'RECEIPT'
import hashlib, json, os, pathlib, stat, sys
root = pathlib.Path(sys.argv[1])
results, oversized = [], False
path = root / 'results.txt'
if path.exists():
    for line in path.read_text().splitlines():
        app, status, evidence = line.split()
        files = []
        for name in (app + '.log', app + '-stages.tar.gz'):
            path = root / name
            if name.endswith('-stages.tar.gz') and int(evidence) != 0: continue
            if not path.exists(): continue
            info = path.lstat()
            if not stat.S_ISREG(info.st_mode): raise SystemExit(2)
            limit = 4194304 if name.endswith('.log') else 16777216
            if info.st_size > limit:
                oversized = True
                os.replace(path, root / (name + '.overflow'))
                files.append({'file': name + '.overflow', 'bytes': info.st_size, 'artifactAdmitted': False})
                continue
            digest = hashlib.sha256()
            with path.open('rb') as file:
                while chunk := file.read(65536): digest.update(chunk)
            files.append({'file': name, 'bytes': info.st_size, 'sha256': digest.hexdigest(), 'artifactAdmitted': True})
        results.append({'app': app, 'argv': ['make', '-C', 'apps/' + app, 'verify-changed', 'BASE=' + sys.argv[5]],
                        'exit': int(status), 'stageEvidenceExit': int(evidence), 'files': files})
final_status = 2 if oversized and int(sys.argv[3]) == 0 else int(sys.argv[3])
(root / 'receipt.json').write_text(json.dumps({'schemaVersion': 1, 'head': sys.argv[4] or None,
    'base': sys.argv[5] or None, 'baseRef': 'origin/main',
    'phase': 'artifact-admission' if oversized else sys.argv[2], 'exit': final_status,
    'verificationStageCacheRestored': False, 'oversizedAppLog': oversized, 'results': results}, indent=2) + '\n')
raise SystemExit(2 if oversized else 0)
RECEIPT
  collected=$?
  if [[ "$original" == 0 && "$collected" != 0 ]]; then original="$collected"; fi
  exit "$original"
}
trap finish EXIT
test ! -e .gates-disabled
phase=fetch
git fetch --no-tags origin main >"$output/fetch.log" 2>&1
head="$(git rev-parse HEAD)" base="$(git rev-parse origin/main)"
common="$(git rev-parse --path-format=absolute --git-common-dir)"
[[ "$head" =~ ^[a-f0-9]{40}$ && "$base" =~ ^[a-f0-9]{40}$ ]]
phase=cache-admission
# This checkout has no restored verification-stage cache; Go dependency caches are separate.
[[ ! -e "$common/kinosail-verify-cache" && ! -L "$common/kinosail-verify-cache" ]] || exit 2
for directory in "$common/kinosail-verify-logs" "${apps[@]/#/$common/kinosail-verify-logs/apps-}"; do
  [[ ! -L "$directory" && ( ! -e "$directory" || -d "$directory" ) ]] || exit 2
done
git diff --check "$base...HEAD"
printf '%s\n' "$head" "$base" >"$output/revisions.txt"
status=0
for app in "${apps[@]}"; do
  phase='base-stability'
  [[ "$(git rev-parse origin/main)" == "$base" ]] || exit 2
  phase=target
  set +e
  make -C "apps/$app" verify-changed BASE="$base" >"$output/$app.log" 2>&1
  result=$?
  python3 - "$common/kinosail-verify-logs/apps-$app" "$output/$app-stages.tar.gz" <<'STAGES'
import os, pathlib, stat, sys, tarfile
output = pathlib.Path(sys.argv[2])
temporary = output.with_name(output.name + '.tmp')
try:
    directory = pathlib.Path(sys.argv[1])
    if directory.is_symlink() or not directory.is_dir(): raise ValueError()
    files = []
    with os.scandir(directory) as entries:
        for entry in entries:
            if len(files) == 128: raise ValueError()
            files.append(pathlib.Path(entry.path))
    if not files: raise ValueError()
    files.sort()
    total = 0
    with tarfile.open(temporary, 'x:gz') as archive:
        for path in files:
            info = path.lstat()
            if not stat.S_ISREG(info.st_mode) or path.suffix != '.log' or info.st_size > 4194304:
                raise ValueError()
            total += info.st_size
            if total > 16777216: raise ValueError()
            archive.add(path, arcname=path.name, recursive=False)
    os.replace(temporary, output)
except (OSError, ValueError):
    raise SystemExit(2)
finally:
    temporary.unlink(missing_ok=True)
STAGES
  evidence=$?
  set -e
  printf '%s %s %s\n' "$app" "$result" "$evidence" >>"$output/results.txt"
  if [[ "$result" != 0 || "$evidence" != 0 ]]; then status=1; fi
  phase='base-stability'
  [[ "$(git rev-parse origin/main)" == "$base" ]] || exit 2
done
phase=complete
finish "$status"
