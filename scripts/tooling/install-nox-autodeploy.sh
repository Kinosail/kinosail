#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
app="${1:-}"
if (( $# != 1 )); then
  printf 'usage: %s {player|subtitles|dashboard}\n' "$0" >&2
  exit 2
fi

# shellcheck source=scripts/tooling/nox-app.sh
source "$script_dir/nox-app.sh"
load_nox_app "$app"
root="$install_root"
cache="$install_cache"
label="$launch_label"
success="$install_success"

agents="$HOME/Library/LaunchAgents"
plist="$agents/$label.plist"
mkdir -p "$root" "$cache" "$agents"
install -m 755 "$repo/scripts/tooling/deploy-nox-app.sh" "$root/deploy-nox-app.sh"
install -m 755 "$repo/scripts/tooling/scan-deployment-image.sh" "$root/scan-deployment-image.sh"
install -m 644 "$repo/scripts/tooling/gates-pause.sh" "$root/gates-pause.sh"
install -m 755 "$repo/scripts/tooling/deploy-nox-remote.sh" "$root/deploy-nox-remote.sh"
install -m 755 "$repo/scripts/tooling/watch-nox-main.sh" "$root/watch-nox-main.sh"
install -m 644 "$repo/scripts/tooling/nox-app.sh" "$root/nox-app.sh"

sed -e "s|@ROOT@|$root|g" -e "s|@CACHE@|$cache|g" -e "s|@APP@|$app|g" \
  -e "s|@LABEL@|$label|g" -e "s|@ROOT_VARIABLE@|$root_variable|g" >"$plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>@LABEL@</string>
<key>ProgramArguments</key><array><string>@ROOT@/watch-nox-main.sh</string><string>@APP@</string></array>
<key>EnvironmentVariables</key><dict>
<key>PATH</key><string>/opt/homebrew/bin:/opt/podman/bin:/usr/local/bin:/usr/bin:/bin</string>
<key>@ROOT_VARIABLE@</key><string>@CACHE@</string>
</dict>
<key>RunAtLoad</key><true/><key>KeepAlive</key><true/>
<key>StandardOutPath</key><string>@CACHE@/deploy.log</string>
<key>StandardErrorPath</key><string>@CACHE@/deploy-error.log</string>
</dict></plist>
PLIST

launchctl bootout "gui/$(id -u)/$label" >/dev/null 2>&1 || true
# Re-evaluate main after an installer refresh; the selection policy may have changed.
rm -f "$cache/deployed"
launchctl bootstrap "gui/$(id -u)" "$plist"
printf '%s\n' "$success"
