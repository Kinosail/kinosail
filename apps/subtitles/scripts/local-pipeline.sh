#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../../../scripts/tooling/gates-pause.sh"
set -uo pipefail

app="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo="$(git -C "$app" rev-parse --show-toplevel)"
result="${KINOSAIL_PIPELINE_RESULT_FILE:-$(mktemp "${TMPDIR:-/tmp}/kinosail-pipeline-result.XXXXXX")}"
failures=0
stage_log=""
engine=""
image=""
tagged_dev=0
run_id="$$-$(git -C "$repo" rev-parse --short HEAD)"
: >"$result"

cleanup() {
  [[ -z "$stage_log" ]] || rm -f "$stage_log"
  (( tagged_dev == 0 )) || "$engine" image rm localhost/kinosail-subtitles:dev >/dev/null 2>&1 || true
  [[ -z "$image" || -z "$engine" ]] || "$engine" image rm --force "$image" >/dev/null 2>&1 || true
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

failure_ids() {
  local stage="$1" log="$2" ids markers
  ids="$(sed -nE \
    -e 's/^--- FAIL: ([^ (]+).*/test:\1/p' \
    -e 's/^FAIL[[:space:]]+([^[:space:]]+).*/package:\1/p' \
    -e 's/^make(\[[0-9]+\])?: \*\*\* \[([^]]+)\].*/make:\2/p' \
    -e 's/^([^:[:space:]][^:]*):[0-9]+:[0-9]+: (.*)$/diagnostic:\1:\2/p' \
    -e 's/^[[:space:]]*[0-9]+\) \[[^]]+\] › ([^:]+):[0-9]+:[0-9]+ › (.*)$/browser:\1:\2/p' \
    -e 's/^.*container test failed at line ([0-9]+).*$/container-line:\1/p' \
    -e 's/^.*(WARNING: DATA RACE).*$/runtime:\1/p' \
    -e 's/^.*(panic: .*)$/runtime:\1/p' \
    -e 's/^(Error: .*)$/runtime:\1/p' \
    -e 's/^(error: .*)$/runtime:\1/p' \
    -e 's/^.*(command not found.*)$/runtime:\1/p' \
    -e 's/^.*(leaks found: [0-9]+).*$/secrets:\1/p' \
    "$log" | LC_ALL=C sort -u)"
  markers="$(sed -nE \
    -e '/^(--- FAIL: |FAIL[[:space:]]|make(\[[0-9]+\])?: \*\*\* |[^:[:space:]][^:]*:[0-9]+:[0-9]+: |.*container test failed at line |.*WARNING: DATA RACE|.*panic: |Error: |error: |.*command not found|.*leaks found: )/p' \
    "$log" | sed -E \
      -e 's/request_id=[^ ]+/request_id=ID/g' \
      -e 's/[0-9]+\.[0-9]+s/DURATION/g' \
      -e 's#/var/folders/[^ ]+#TMP#g' \
      -e 's#/tmp/[^ ]+#TMP#g')"
  printf 'stage:%s\n' "$stage"
  if [[ -n "$ids" ]]; then
    printf '%s\n' "$ids"
  else
    printf 'unclassified:%s\n' "$(shasum -a 256 "$log" | awk '{print $1}')"
  fi
  printf 'fingerprint:%s\n' "$(printf '%s\n' "$markers" | shasum -a 256 | awk '{print $1}')"
}

run_stage() {
  local stage="$1" status started elapsed
  shift
  stage_log="$(mktemp "${TMPDIR:-/tmp}/kinosail-${stage}.XXXXXX")"
  started="$(date +%s)"
  if "$@" >"$stage_log" 2>&1; then status=0; else status=$?; fi
  elapsed="$(($(date +%s) - started))"
  if (( status != 0 )); then
    failure_ids "$stage" "$stage_log" >>"$result"
    failures=$((failures + 1))
    printf 'FAIL %s (%ss); log: %s\n' "$stage" "$elapsed" "$stage_log" >&2
    tail -60 "$stage_log" >&2
    stage_log=""
  else
    printf 'PASS %s (%ss)\n' "$stage" "$elapsed"
    rm -f "$stage_log"
    stage_log=""
  fi
}

cd "$app" || exit
engine="${CONTAINER_ENGINE:-}"
if [[ -z "$engine" ]]; then if command -v podman >/dev/null; then engine=podman; else engine=docker; fi; fi
image="localhost/kinosail-test:${run_id}"
run_stage check make check
run_stage container-native env KINOSAIL_TEST_IMAGE="$image" make container-test
image_ready=()
if "$engine" image inspect "$image" >/dev/null 2>&1; then
  "$engine" tag "$image" localhost/kinosail-subtitles:dev
  tagged_dev=1
  image_ready=(KINOSAIL_TEST_IMAGE_READY=1)
fi
run_stage populated-instance env "${image_ready[@]}" make test-instance-check
run_stage browser-chromium env CI=1 KINOSAIL_TEST_IMAGE="$image" "${image_ready[@]}" make browser-test
run_stage performance make performance-test
run_stage fuzz sh -c "../../scripts/tooling/with-go-module.sh go test -list '^FuzzRestoreWritesOnlyConfigurationState$' ./internal/backup | grep -Fx FuzzRestoreWritesOnlyConfigurationState && ../../scripts/tooling/with-go-module.sh go test -run '^$' -fuzz '^FuzzRestoreWritesOnlyConfigurationState$' -fuzztime 30s ./internal/backup"

LC_ALL=C sort -u -o "$result" "$result"
if (( failures != 0 )); then
  printf '\nLocal pipeline failed in %d stage(s). Signature: %s\n' "$failures" "$result" >&2
  exit 1
fi
printf '\nLocal pipeline passed.\n'
