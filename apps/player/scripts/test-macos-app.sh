#!/usr/bin/env bash
set -euo pipefail

dmg="${1:-}"
evidence="${2:-}"
if [[ $# -ne 2 || ! -f "$dmg" || -z "$evidence" ]]; then
	printf 'usage: %s Kinosail-macOS.dmg evidence-directory\n' "$0" >&2
	exit 2
fi
[[ "$(uname -s)" == Darwin && "${CI:-}" == true ]] || { printf 'run on a disposable macOS CI host\n' >&2; exit 2; }
mkdir -p "$evidence"
evidence="$(cd "$evidence" && pwd)"
work="$(mktemp -d "${TMPDIR:-/tmp}/kinosail-macos-test.XXXXXX")"
launcher=""
cleanup() {
	cp "$HOME/Library/Logs/Kinosail/kinosail.log" "$evidence/kinosail.log" 2>/dev/null || true
	pgrep -fl Kinosail >"$evidence/processes.txt" 2>&1 || true
	[[ -z "$launcher" ]] || kill -KILL "$launcher" 2>/dev/null || true
	pkill -KILL -f "$work/Applications/Kinosail.app/Contents/Helpers/kinosail" 2>/dev/null || true
	hdiutil detach -quiet "$work/mount" 2>/dev/null || true
	rm -rf -- "$work"
}
trap cleanup EXIT
step() { printf '%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$1" | tee -a "$evidence/steps.log"; }
healthy() { curl --insecure --silent --fail --max-time 2 https://localhost:38127/healthz >/dev/null; }
wait_healthy() {
	for _ in $(seq 1 120); do healthy && return 0; sleep 0.5; done
	return 1
}
server_pid() { pgrep -P "$launcher" -f 'Contents/Helpers/kinosail$' || true; }

step "mount the disk image"
hdiutil attach -quiet -nobrowse -readonly -mountpoint "$work/mount" "$dmg"
[[ -L "$work/mount/Applications" && -d "$work/mount/Kinosail.app" ]]
mkdir "$work/Applications"
ditto "$work/mount/Kinosail.app" "$work/Applications/Kinosail.app"
hdiutil detach -quiet "$work/mount"
app="$work/Applications/Kinosail.app"

step "check the signature and both architectures"
codesign --verify --strict --deep "$app"
for binary in MacOS/Kinosail Helpers/kinosail Helpers/ffmpeg Helpers/ffprobe Helpers/fpcalc; do
	archs=" $(lipo -archs "$app/Contents/$binary") "
	[[ "$archs" == *" arm64 "* && "$archs" == *" x86_64 "* ]] || { printf '%s is not universal\n' "$binary" >&2; exit 1; }
done
[[ "$(plutil -extract LSUIElement raw "$app/Contents/Info.plist")" == true ]]

step "start the menu bar app"
defaults write com.kinosail.server loginItemConfigured -bool true
defaults write com.kinosail.server openedSetup -bool true
! healthy || { printf 'another server already uses port 38127\n' >&2; exit 1; }
"$app/Contents/MacOS/Kinosail" >"$evidence/launcher.log" 2>&1 &
launcher=$!
wait_healthy || { printf 'Kinosail did not become healthy\n' >&2; exit 1; }
first="$(server_pid)"
[[ -n "$first" ]]
support="$HOME/Library/Application Support/Kinosail"
[[ -d "$support/data" && -d "$support/backups" && -d "$HOME/Library/Caches/Kinosail" ]]
[[ "$(stat -f %Lp "$support/data")" == 700 ]]

step "restart the server after it crashes"
kill -KILL "$first"
for _ in $(seq 1 60); do
	second="$(server_pid)"
	[[ -n "$second" && "$second" != "$first" ]] && break
	sleep 0.5
done
[[ -n "$second" && "$second" != "$first" ]] || { printf 'the menu bar app did not restart the server\n' >&2; exit 1; }
wait_healthy || { printf 'the restarted server did not become healthy\n' >&2; exit 1; }

step "quit stops the server cleanly"
kill -TERM "$launcher"
for _ in $(seq 1 60); do kill -0 "$launcher" 2>/dev/null || break; sleep 0.5; done
! kill -0 "$launcher" 2>/dev/null || { printf 'the menu bar app did not quit\n' >&2; exit 1; }
launcher=""
! pgrep -f "$app/Contents/Helpers/kinosail" >/dev/null || { printf 'the server kept running after quit\n' >&2; exit 1; }
! healthy || { printf 'port 38127 still answers after quit\n' >&2; exit 1; }
log="$HOME/Library/Logs/Kinosail/kinosail.log"
grep -q 'launcher closed; stopping' "$log" || { printf 'the server did not stop through the launcher pipe\n' >&2; exit 1; }
cp "$log" "$evidence/kinosail.log"

step "passed"
(cd "$(dirname "$dmg")" && shasum -a 256 "$(basename "$dmg")") >"$evidence/package.sha256"
(cd "$evidence" && shasum -a 256 steps.log kinosail.log launcher.log) >"$evidence/evidence.sha256"
printf '{"revision":"%s","system":"macos","macos":"%s","commands":["scripts/package-macos.sh","scripts/test-macos-app.sh"]}\n' \
	"$(git rev-parse HEAD)" "$(sw_vers -productVersion)" >"$evidence/run.json"
