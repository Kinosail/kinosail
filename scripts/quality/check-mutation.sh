#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../tooling/gates-pause.sh"
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tools="${TMPDIR:-/tmp}/kinosail-quality-tools"
workers="${KINOSAIL_MUTATION_WORKERS:-2}"
timeout_coefficient="${KINOSAIL_MUTATION_TIMEOUT_COEFFICIENT:-50}"
real_git="$(command -v git)"
status=0
apps=("$@")
diff_arguments=()
if [[ -n "${KINOSAIL_MUTATION_DIFF:-}" ]]; then
	diff_arguments=(--diff "$KINOSAIL_MUTATION_DIFF")
fi
if (( ${#apps[@]} == 0 )); then
	apps=(player subtitles packages)
fi
selected=()
for app in "${apps[@]}"; do
	case "$app" in
	player|subtitles|packages) ;;
	*) printf 'unknown app: %s\n' "$app" >&2; exit 2 ;;
	esac
	relative="apps/$app"
	if [[ "$app" == packages ]]; then relative=packages; fi
	if (( ${#diff_arguments[@]} > 0 )) && ! git -C "$repo" diff --name-only "$KINOSAIL_MUTATION_DIFF" -- "$relative" | grep -qE '\.go$'; then
		printf '%s mutation check skipped: no changed Go files\n' "$app"
		continue
	fi
	selected+=("$app")
done
if (( ${#selected[@]} == 0 )); then
	exit 0
fi
mkdir -p "$tools"
GOBIN="$tools" go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0
git_wrapper="$tools/git-relative-diff"
mkdir -p "$git_wrapper"
ln -sf "$repo/scripts/quality/git-relative-diff.sh" "$git_wrapper/git"
for app in "${selected[@]}"; do
	if [[ "$app" == packages ]]; then directory="$repo/packages"; else directory="$repo/apps/$app"; fi
	gremlins_args=(
		unleash
		--exclude-files '(^|/)node_modules/'
		--exclude-files '(^|/)third_party/'
	)
	if (( ${#diff_arguments[@]} > 0 )); then
		gremlins_args+=("${diff_arguments[@]}")
	fi
	if [[ "$app" == packages ]]; then
		gremlins_args+=(--exclude-files '(^|/)(archivetest|commandtest|configurationtest|servertest)/')
	fi
	mkdir -p "$directory/.verification"
	cd "$directory"
	if ! GOWORK=off KINOSAIL_REAL_GIT="$real_git" PATH="$git_wrapper:$PATH" "$tools/gremlins" \
		"${gremlins_args[@]}" \
		--integration \
		--invert-assignments \
		--invert-bitwise \
		--invert-bwassign \
		--invert-logical \
		--invert-loopctrl \
		--output .verification/mutation.json \
		--output-statuses lctv \
		--remove-self-assignments \
		--threshold-efficacy 100 \
		--threshold-mcover 100 \
		--timeout-coefficient "$timeout_coefficient" \
		--workers "$workers"; then
		status=1
	fi
	if [[ -s .verification/mutation.json ]] && grep -Eq '"status"[[:space:]]*:[[:space:]]*"(LIVED|NOT COVERED|TIMED OUT)"' .verification/mutation.json; then
		printf '%s mutation check has unresolved or surviving mutants\n' "$app" >&2
		status=1
	fi
done
exit "$status"
