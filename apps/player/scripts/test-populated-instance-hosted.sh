#!/usr/bin/env bash
# Admit the existing synthetic catalog before expanding populated browser QA.
set -euo pipefail

if [[ "${CI:-}" != true || "${GITHUB_ACTIONS:-}" != true || "${RUNNER_OS:-}" != Linux || "$(uname -s)" != Linux || "${CONTAINER_ENGINE:-docker}" != docker ]]; then
  echo 'Populated admission requires a disposable Linux Actions runner' >&2
  exit 2
fi
suite="${KINOSAIL_POPULATED_SUITE:-admission}"
case "$suite" in
  admission|selected|browse|configuration) ;;
  *) echo 'Unknown populated fixture suite' >&2; exit 2 ;;
esac
output="${KINOSAIL_E2E_ARTIFACT_DIR:-}"
if [[ "$output" != /* || ! -d "$output" || -L "$output" || ! -d "${RUNNER_TEMP:-}" ]]; then
  echo 'Populated admission requires an existing private artifact directory and runner temporary directory' >&2
  exit 2
fi
cd "$(dirname "$0")/.."
# shellcheck source=scripts/ci/browser-fixture-tls.sh
source ../../scripts/ci/browser-fixture-tls.sh
export CONTAINER_ENGINE=docker KINOSAIL_TEST_IMAGE_READY=1 KINOSAIL_PORT=0 KINOSAIL_MANAGEMENT_BIND=127.0.0.1
KINOSAIL_TEST_ROOT="$(mktemp -d "$RUNNER_TEMP/kinosail-populated.XXXXXX")"
export KINOSAIL_TEST_ROOT
export KINOSAIL_TEST_PROJECT="kinosail-populated-$$-$RANDOM"
stage=source
nss_database=""

cleanup() {
  local status=$? cleanup_status=0 remaining_containers=0 remaining_volumes=0 fixture_removed=0 trust_removed=1 inventory
  trap - EXIT
  if [[ -n "$nss_database" ]]; then
    certutil -D -d "sql:$nss_database" -n "$KINOSAIL_TEST_PROJECT" >>"$output/cleanup.log" 2>&1 || cleanup_status=1
    trust_removed=0
    if inventory="$(certutil -L -d "sql:$nss_database" 2>>"$output/cleanup.log")" && ! grep -q "^${KINOSAIL_TEST_PROJECT}[[:space:]]" <<<"$inventory"; then
      trust_removed=1
    else
      cleanup_status=1
    fi
  fi
  ./scripts/test-instance.sh down --volumes >>"$output/cleanup.log" 2>&1 || cleanup_status=$?
  remaining_containers="$(docker ps -aq --filter "label=com.docker.compose.project=$KINOSAIL_TEST_PROJECT" | awk 'NF {n++} END {print n+0}')" || cleanup_status=1
  remaining_volumes="$(docker volume ls -q --filter "label=com.docker.compose.project=$KINOSAIL_TEST_PROJECT" | awk 'NF {n++} END {print n+0}')" || cleanup_status=1
  if [[ "$remaining_containers" != 0 || "$remaining_volumes" != 0 ]]; then cleanup_status=1; fi
  rm -rf -- "${KINOSAIL_TEST_ROOT:?}" || cleanup_status=1
  if [[ ! -e "$KINOSAIL_TEST_ROOT" ]]; then fixture_removed=1; else cleanup_status=1; fi
  if [[ "$status" == 0 && "$cleanup_status" != 0 ]]; then status=1; fi
  python3 - "$output/fixture-lifecycle.json" "$status" "$cleanup_status" "$remaining_containers" "$remaining_volumes" "$fixture_removed" "$stage" "$trust_removed" "$suite" <<'PY'
import json,sys
from pathlib import Path
result,cleanup,containers,volumes,removed=map(int,sys.argv[2:7])
Path(sys.argv[1]).write_text(json.dumps({'result':'passed' if result==0 else 'failed',
    'level':'info' if result==0 else 'error','stage':sys.argv[7],
    'exitCode':result,'cleanupExitCode':cleanup,'remainingContainers':containers,
    'remainingVolumes':volumes,'privateFixtureRemoved':bool(removed),
    'browserTrustRemoved':sys.argv[8]=='1','suite':sys.argv[9]},indent=2)+'\n')
PY
  exit "$status"
}
trap cleanup EXIT

revision="$(git rev-parse HEAD)"
[[ "$revision" =~ ^[a-f0-9]{40}$ ]]
export KINOSAIL_TEST_REVISION="$revision"
stage=setup
./scripts/test-instance.sh up
stage=image-identity
identity="$(docker image inspect --format '{{.Id}} {{index .Config.Labels "org.opencontainers.image.revision"}}' localhost/kinosail:test-instance)"
read -r image_id image_revision <<<"$identity"
[[ "$image_id" =~ ^sha256:[a-f0-9]{64}$ && "$image_revision" == "$revision" ]]
printf '%s\n' "$identity" >"$output/test-image-identity.txt"
stage=catalog-verification
./scripts/test-instance.sh verify
stage=browser-trust
container="$(docker compose --project-name "$KINOSAIL_TEST_PROJECT" --file compose.yaml --file compose.test.yaml ps --quiet kinosail)"
[[ "$container" =~ ^[a-f0-9]{12,64}$ ]]
certificate="$KINOSAIL_TEST_ROOT/browser-fixture-ca.crt"
docker exec "$container" kinosail tls-certificate >"$certificate"
validate_browser_fixture_ca "$certificate"
cp "$certificate" "$output/browser-fixture-ca.crt"
database="${HOME:?}/.local/share/pki/nssdb"
if [[ -d "$HOME/.pki/nssdb" ]]; then database="$HOME/.pki/nssdb"; fi
[[ ! -L "$database" ]]
mkdir -p "$database"
if [[ ! -f "$database/cert9.db" ]]; then certutil -N --empty-password -d "sql:$database"; fi
certutil -L -d "sql:$database" >/dev/null
if certutil -L -d "sql:$database" -n "$KINOSAIL_TEST_PROJECT" >/dev/null 2>&1; then
  echo 'Populated fixture trust nickname already exists' >&2
  exit 2
fi
nss_database="$database"
certutil -A -d "sql:$database" -t 'C,,' -n "$KINOSAIL_TEST_PROJECT" -i "$certificate"
export NODE_EXTRA_CA_CERTS="$certificate"
export KINOSAIL_TEST_INSTANCE=1 KINOSAIL_BROWSER_PROJECT=chromium KINOSAIL_E2E_WORKERS=1
KINOSAIL_TEST_TOTP_SECRET="$(<"$KINOSAIL_TEST_ROOT/totp-secret")"
export KINOSAIL_TEST_TOTP_SECRET
KINOSAIL_E2E_URL="$(./scripts/test-instance.sh url)"
export KINOSAIL_E2E_URL
title='Connection choices stay optional and secure by default'
if [[ "$suite" == admission ]]; then
  stage=connection-settings
  pnpm --dir e2e exec playwright test test-instance.spec.ts --grep "$title" --project=chromium --workers=1 --retries=0 --max-failures=1
elif [[ "$suite" == selected ]]; then
  stage=selected-suite
  ./scripts/test-instance.sh browser --project=chromium --retries=0 --max-failures=1
elif [[ "$suite" == browse ]]; then
  stage=browse-suite
  pnpm --dir e2e exec playwright test browse-readability.spec.ts collections-loading.spec.ts curation-count.spec.ts dependency-thanks.spec.ts layout-audit-library.spec.ts layout-audit-shell.spec.ts mobile-quick-connect.spec.ts navigation-repeat.spec.ts show-card-viewer-policy.spec.ts show-episode-ledger.spec.ts static-compression.spec.ts --project=chromium --workers=1 --retries=0 --max-failures=1
else
  stage=configuration-suite
  pnpm --dir e2e exec playwright test layout-audit-configuration.spec.ts --project=chromium --workers=1 --retries=0 --max-failures=1
fi
stage=browser-result
python3 - "$output/results-chromium.json" "$title" "$suite" <<'PY'
import json,sys
from pathlib import Path
report=json.loads(Path(sys.argv[1]).read_text())
def specs(suite):
    yield from suite.get('specs',[])
    for child in suite.get('suites',[]):yield from specs(child)
cases=[spec for suite in report['suites'] for spec in specs(suite)]
if sys.argv[3]=='admission':assert len(cases)==1 and cases[0]['title']==sys.argv[2]
elif sys.argv[3]=='selected':assert len(cases)>=82
elif sys.argv[3]=='browse':assert len(cases)==23
else:assert len(cases)==11
for case in cases:
    tests=case['tests'];assert len(tests)==1
    test=tests[0];assert test['projectName']=='chromium' and test['status']=='expected'
    assert len(test['results'])==1
    result=test['results'][0]
    assert result['status']=='passed' and result['retry']==0 and not result.get('errors')
assert not report.get('errors')
PY
stage=complete
