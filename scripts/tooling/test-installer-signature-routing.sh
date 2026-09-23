#!/usr/bin/env bash
# Sourced by both installer tests after verifying their version-tag signer.
# shellcheck disable=SC2154 # fixture is initialized by each caller.

KINOSAIL_INSTALL_TEST_LEGACY_SIGNATURE=1 PATH="$fixture/bin:$PATH" "$fixture/app/scripts/install.sh" "$fixture/media" 9080 >/dev/null
grep -Fq 'repository=default' "$fixture/compose.log"
sed -i.bak '/^KINOSAIL_VERSION=/d' "$fixture/app/.env"
rm "$fixture/app/.env.bak"

before="$(grep -c 'up --detach' "$fixture/compose.log")"
if KINOSAIL_INSTALL_TEST_FAIL_SIGNATURES=1 PATH="$fixture/bin:$PATH" "$fixture/app/scripts/install.sh" "$fixture/media" 9080 >/dev/null 2>&1; then
	echo "unverified image must not be installed" >&2
	exit 1
fi
[[ "$(grep -c 'up --detach' "$fixture/compose.log")" == "$before" ]]
