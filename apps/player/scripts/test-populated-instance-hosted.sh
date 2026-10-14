#!/usr/bin/env bash
# Admit the existing synthetic catalog before expanding populated browser QA.
set -euo pipefail

if [[ "${CI:-}" != true || "${GITHUB_ACTIONS:-}" != true || "${RUNNER_OS:-}" != Linux || "$(uname -s)" != Linux || "${CONTAINER_ENGINE:-docker}" != docker ]]; then
  echo 'Populated admission requires a disposable Linux Actions runner' >&2
  exit 2
fi
output="${KINOSAIL_E2E_ARTIFACT_DIR:-}"
if [[ "$output" != /* || ! -d "$output" || -L "$output" || ! -d "${RUNNER_TEMP:-}" ]]; then
  echo 'Populated admission requires an existing private artifact directory and runner temporary directory' >&2
  exit 2
fi
cd "$(dirname "$0")/.."
export CONTAINER_ENGINE=docker KINOSAIL_TEST_IMAGE_READY=1 KINOSAIL_PORT=0 KINOSAIL_MANAGEMENT_BIND=127.0.0.1
KINOSAIL_TEST_ROOT="$(mktemp -d "$RUNNER_TEMP/kinosail-populated.XXXXXX")"
export KINOSAIL_TEST_ROOT
export KINOSAIL_TEST_PROJECT="kinosail-populated-$$-$RANDOM"

cleanup() {
  local status=$? cleanup_status=0 remaining_containers=0 remaining_volumes=0 fixture_removed=0
  trap - EXIT
  ./scripts/test-instance.sh down --volumes >"$output/cleanup.log" 2>&1 || cleanup_status=$?
  remaining_containers="$(docker ps -aq --filter "label=com.docker.compose.project=$KINOSAIL_TEST_PROJECT" | awk 'NF {n++} END {print n+0}')" || cleanup_status=1
  remaining_volumes="$(docker volume ls -q --filter "label=com.docker.compose.project=$KINOSAIL_TEST_PROJECT" | awk 'NF {n++} END {print n+0}')" || cleanup_status=1
  if [[ "$remaining_containers" != 0 || "$remaining_volumes" != 0 ]]; then cleanup_status=1; fi
  rm -rf -- "${KINOSAIL_TEST_ROOT:?}" || cleanup_status=1
  if [[ ! -e "$KINOSAIL_TEST_ROOT" ]]; then fixture_removed=1; else cleanup_status=1; fi
  if [[ "$status" == 0 && "$cleanup_status" != 0 ]]; then status=1; fi
  python3 - "$output/fixture-lifecycle.json" "$status" "$cleanup_status" "$remaining_containers" "$remaining_volumes" "$fixture_removed" <<'PY'
import json,sys
from pathlib import Path
result,cleanup,containers,volumes,removed=map(int,sys.argv[2:])
Path(sys.argv[1]).write_text(json.dumps({'result':'passed' if result==0 else 'failed',
    'exitCode':result,'cleanupExitCode':cleanup,'remainingContainers':containers,
    'remainingVolumes':volumes,'privateFixtureRemoved':bool(removed)},indent=2)+'\n')
PY
  exit "$status"
}
trap cleanup EXIT

revision="$(git rev-parse HEAD)"
[[ "$revision" =~ ^[a-f0-9]{40}$ ]]
export KINOSAIL_TEST_REVISION="$revision"
./scripts/test-instance.sh up
identity="$(docker image inspect --format '{{.Id}} {{index .Config.Labels "org.opencontainers.image.revision"}}' localhost/kinosail:test-instance)"
read -r image_id image_revision <<<"$identity"
[[ "$image_id" =~ ^sha256:[a-f0-9]{64}$ && "$image_revision" == "$revision" ]]
printf '%s\n' "$identity" >"$output/test-image-identity.txt"
./scripts/test-instance.sh verify
export KINOSAIL_TEST_INSTANCE=1 KINOSAIL_BROWSER_PROJECT=chromium KINOSAIL_E2E_WORKERS=1
KINOSAIL_TEST_TOTP_SECRET="$(<"$KINOSAIL_TEST_ROOT/totp-secret")"
export KINOSAIL_TEST_TOTP_SECRET
KINOSAIL_E2E_URL="$(./scripts/test-instance.sh url)"
export KINOSAIL_E2E_URL
title='Connection choices stay optional and secure by default'
pnpm --dir e2e exec playwright test test-instance.spec.ts --grep "$title" --project=chromium --workers=1 --retries=0 --max-failures=1
python3 - "$output/results-chromium.json" "$title" <<'PY'
import json,sys
from pathlib import Path
report=json.loads(Path(sys.argv[1]).read_text())
def specs(suite):
    yield from suite.get('specs',[])
    for child in suite.get('suites',[]):yield from specs(child)
cases=[spec for suite in report['suites'] for spec in specs(suite)]
assert len(cases)==1 and cases[0]['title']==sys.argv[2]
tests=cases[0]['tests'];assert len(tests)==1
test=tests[0];assert test['projectName']=='chromium' and test['status']=='expected'
assert len(test['results'])==1
result=test['results'][0]
assert result['status']=='passed' and result['retry']==0 and not result.get('errors')
assert not report.get('errors')
PY
