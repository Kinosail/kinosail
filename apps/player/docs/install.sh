#!/bin/sh
set -eu

repository="https://github.com/Kinosail/kinosail/releases/latest/download"
fail() { printf 'Kinosail install: %s\n' "$1" >&2; exit 1; }

[ "$(uname -s)" = Linux ] || fail "this script is for Linux. Get the installer for your computer at https://kinosail.com/download/"
case "$(uname -m)" in
  x86_64 | amd64) arch=x64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) fail "Kinosail needs a 64-bit Intel, AMD, or ARM computer. Docker is an option: https://kinosail.com/quickstart/" ;;
esac
if command -v apt-get >/dev/null 2>&1; then
  format=deb
elif command -v dnf >/dev/null 2>&1 || command -v zypper >/dev/null 2>&1 || command -v yum >/dev/null 2>&1; then
  format=rpm
else
  fail "no supported package manager found (apt, dnf, yum, or zypper). Docker is an option: https://kinosail.com/quickstart/"
fi
[ -d /run/systemd/system ] || fail "Kinosail runs as a systemd service, and systemd is not running here"
command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v sha256sum >/dev/null 2>&1 || fail "sha256sum is required"
if [ "$(id -u)" = 0 ]; then sudo=""; else
  command -v sudo >/dev/null 2>&1 || fail "run this as root or install sudo"
  sudo=sudo
fi

file="Kinosail-Linux-$arch.$format"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT INT TERM
printf 'Downloading %s...\n' "$file"
curl --fail --location --proto '=https' --tlsv1.2 --retry 3 --output "$work/$file" "$repository/$file"
curl --fail --location --proto '=https' --tlsv1.2 --retry 3 --silent --output "$work/SHA256SUMS" "$repository/SHA256SUMS"
expected="$(awk -v name="$file" '$2 == name { print $1 }' "$work/SHA256SUMS")"
[ -n "$expected" ] || fail "the release checksum list does not include $file"
actual="$(sha256sum "$work/$file" | awk '{ print $1 }')"
[ "$expected" = "$actual" ] || fail "the download is damaged or changed (checksum mismatch). Nothing was installed"
if command -v cosign >/dev/null 2>&1; then
  curl --fail --location --proto '=https' --tlsv1.2 --retry 3 --silent --output "$work/SHA256SUMS.sigstore.json" "$repository/SHA256SUMS.sigstore.json"
  cosign verify-blob --bundle "$work/SHA256SUMS.sigstore.json" \
    --certificate-identity-regexp '^https://github\.com/Kinosail/kinosail/\.github/workflows/release\.yml@refs/tags/player-v[0-9]+\.[0-9]+\.[0-9]+$' \
    --certificate-oidc-issuer https://token.actions.githubusercontent.com "$work/SHA256SUMS" >/dev/null 2>&1 ||
    fail "the release signature did not verify. Nothing was installed"
fi
chmod 0644 "$work/$file"
chmod 0755 "$work"
printf 'Installing Kinosail...\n'
if [ "$format" = deb ]; then
  $sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y "$work/$file"
elif command -v dnf >/dev/null 2>&1; then
  $sudo dnf install -y "$work/$file"
elif command -v zypper >/dev/null 2>&1; then
  $sudo zypper --non-interactive install --allow-unsigned-rpm "$work/$file"
else
  $sudo yum install -y "$work/$file"
fi
