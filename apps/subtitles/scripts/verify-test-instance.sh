#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../../../scripts/tooling/gates-pause.sh"
set -euo pipefail

base="${1:-https://127.0.0.1:38128}"
fixture="$(mktemp -d)"
trap 'rm -rf -- "${fixture:?}"' EXIT

curl --fail --silent --insecure --cookie-jar "$fixture/cookies" --data "name=Owner&password=test-instance-password&code=$(./scripts/test-instance.sh totp)" "$base/login" --output /dev/null

inventory=""
for _ in {1..80}; do
	inventory="$(curl --fail --silent --insecure --cookie "$fixture/cookies" "$base/api/v1/subtitle-library?view=library")"
  [[ "$inventory" == *'"total":'* ]] && break
  sleep 0.25
done

grep -Fq '"language":"en"' <<<"$inventory"
grep -Fq '"ready":' <<<"$inventory"
grep -Fq '"wanted":' <<<"$inventory"
grep -Fq 'Example Movie' <<<"$inventory"
grep -Fq 'Example Episode One' <<<"$inventory"
grep -Fq '"tracks":"Default"' <<<"$inventory"

home="$(curl --fail --silent --insecure --cookie "$fixture/cookies" "$base/")"
grep -Fq '<h1>Overview</h1>' <<<"$home"
grep -Fq 'files need subtitles' <<<"$home"
grep -Fq '<meter' <<<"$home"
grep -Fq 'Subtitle library' <<<"$home"

wanted="$(curl --fail --silent --insecure --cookie "$fixture/cookies" "$base/?view=wanted")"
grep -Fq '<h1>Wanted</h1>' <<<"$wanted"
grep -Fq 'files need subtitles' <<<"$wanted"

settings="$(curl --fail --silent --insecure --cookie "$fixture/cookies" "$base/settings")"
grep -Fq 'Subtitle settings' <<<"$settings"
grep -Fq 'Preferred languages' <<<"$settings"
grep -Fq 'SubDL' <<<"$settings"

[[ "$(curl --silent --insecure --cookie "$fixture/cookies" --output /dev/null --write-out '%{http_code}' "$base/?view=wanted&view=library")" == 400 ]]
[[ "$(curl --silent --insecure --cookie "$fixture/cookies" --request POST --header 'Content-Type: application/json' --data '{"language":"en","Language":"fr"}' --output /dev/null --write-out '%{http_code}' "$base/api/v1/subtitle-library/fetch-wanted")" == 400 ]]

printf 'Verified subtitle coverage, wanted inventory, strict input rejection, settings, and responsive browser entrypoint at %s\n' "$base"
