#!/usr/bin/env bash
set -euo pipefail

[[ $# == 2 ]] || { echo 'expected app and version' >&2; exit 2; }
app="$1"
version="$2"
case "$app" in player|subtitles) ;; *) echo 'invalid installer app' >&2; exit 2 ;; esac
[[ ${#version} -le 64 && "$version" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] || {
  echo 'invalid installer version' >&2; exit 2;
}
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo/apps/$app"
if [[ "$app" == player ]]; then
  python3 scripts/package-installer.py installer-release
  mv installer-release/* .
  rmdir installer-release
else
  tar --create --gzip --file kinosail-subtitles-install.tar.gz \
    scripts/install.sh scripts/uninstall.sh scripts/setup-remote-access.sh scripts/disable-remote-access.sh \
    compose.release.yaml compose.config.yaml compose.gpu.yaml compose.rkmpp.yaml compose.remote-https.yaml \
    kinosail.example.yaml .env.example README.md LICENSE LICENSING.md THIRD_PARTY_NOTICES.md \
    SECURITY.md CONTRIBUTING.md CLA.md CCLA.md TRADEMARKS.md third_party/hls.js/LICENSE third_party/htmx/LICENSE
  sha256sum kinosail-subtitles-install.tar.gz > kinosail-subtitles-install.tar.gz.sha256
fi
cosign sign-blob --yes --bundle "kinosail-$app-install.tar.gz.sigstore.json" "kinosail-$app-install.tar.gz"
./scripts/package-native-release.sh "$version" native-release
cosign sign-blob --yes --bundle native-release/kinosail-release.json.sigstore.json native-release/kinosail-release.json
