#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../../../scripts/tooling/gates-pause.sh"
set -euo pipefail

output="$(mktemp -d)"
second="$(mktemp -d)"
rejected="$(mktemp -d)"
trap 'rm -rf "$output" "$second" "$rejected"' EXIT

printf -v oversized_version 'v%063d.0.0' 0
if "$(dirname "$0")/package-native-release.sh" "$oversized_version" "$rejected" >/dev/null 2>&1; then
	printf 'native release accepted an oversized version\n' >&2
	exit 1
fi
[[ -z "$(find "$rejected" -mindepth 1 -print -quit)" ]]

"$(dirname "$0")/package-native-release.sh" v1.2.3 "$output"
"$(dirname "$0")/package-native-release.sh" v1.2.3 "$second"
files=(kinosail-native-installation.json kinosail-release.json)
for target in linux-amd64 linux-arm64 macos-amd64 macos-arm64; do
	file="kinosail-core-v1.2.3-$target.tar.gz"
	[[ -s "$output/$file" ]]
	files+=("$file")
	tar -tzf "$output/$file" | grep -Fx native-installation.json >/dev/null
done
for target in windows-amd64 windows-arm64; do
	file="kinosail-core-v1.2.3-$target.zip"
	[[ -s "$output/$file" ]]
	files+=("$file")
	unzip -Z1 "$output/$file" | grep -Fx native-installation.json >/dev/null
done
for file in "${files[@]}"; do
	cmp -s "$output/$file" "$second/$file" || { printf 'native release is not reproducible: %s\n' "$file" >&2; exit 1; }
done
[[ "$(grep -o '"os":' "$output/kinosail-release.json" | wc -l | tr -d ' ')" == 6 ]]
grep -Fq '"schemaVersion":2,"version":"v1.2.3","updateSchema":2,"stateSchema":2,"minimumStateSchema":1,"configurationSchema":1,"minimumConfigurationSchema":1' "$output/kinosail-release.json"
contract_digest="$(shasum -a 256 "$output/kinosail-native-installation.json" | awk '{print $1}')"
contract_size="$(wc -c <"$output/kinosail-native-installation.json" | tr -d ' ')"
grep -Fq '"installation":{"schemaVersion":3,"file":"kinosail-native-installation.json","sha256":"'"$contract_digest"'","size":'"$contract_size"'}' "$output/kinosail-release.json"
if grep -Eqi 'beta|channel|latest' "$output/kinosail-release.json"; then
	printf 'release manifest contains a release channel\n' >&2
	exit 1
fi
case "$(uname -m)" in
	x86_64) host_arch=amd64 ;;
	arm64 | aarch64) host_arch=arm64 ;;
	*) printf 'unsupported test architecture\n' >&2; exit 1 ;;
esac
case "$(uname -s)" in
	Darwin) host_os=macos ;;
	Linux) host_os=linux ;;
	*) printf 'unsupported test operating system\n' >&2; exit 1 ;;
esac
mkdir "$output/host"
tar -xzf "$output/kinosail-core-v1.2.3-$host_os-$host_arch.tar.gz" -C "$output/host"
[[ "$("$output/host/kinosail" version)" == v1.2.3 ]]
"$output/host/kinosail" update-artifact <"$output/kinosail-release.json" | grep -Fq "kinosail-core-v1.2.3-$host_os-$host_arch.tar.gz"
python3 "$(dirname "$0")/test-native-contract.py" "$output/host/native-installation.json"
"$(dirname "$0")/test-native-contract.sh"
archive_script="$(dirname "$0")/package-native-archive.py"
if python3 "$archive_script" rar "$output/invalid" "$output/host/kinosail" "$output/kinosail-native-installation.json" LICENSE THIRD_PARTY_NOTICES.md >/dev/null 2>&1; then
	printf 'archive helper accepted an unknown format\n' >&2
	exit 1
fi
[[ ! -e "$output/invalid" ]]
touch "$output/existing"
if python3 "$archive_script" tar.gz "$output/existing" "$output/host/kinosail" "$output/kinosail-native-installation.json" LICENSE THIRD_PARTY_NOTICES.md >/dev/null 2>&1; then
	printf 'archive helper replaced an existing output\n' >&2
	exit 1
fi
[[ ! -s "$output/existing" ]]
if python3 "$archive_script" tar.gz "$output/missing-output" "$output/missing-input" "$output/kinosail-native-installation.json" LICENSE THIRD_PARTY_NOTICES.md >/dev/null 2>&1; then
	printf 'archive helper accepted a missing input\n' >&2
	exit 1
fi
[[ ! -e "$output/missing-output" ]]
manifest_digest="$(shasum -a 256 "$output/kinosail-release.json" | awk '{print $1}')"
if "$(dirname "$0")/package-native-release.sh" v1.2.3 "$output" >/dev/null 2>&1; then
	printf 'release packager replaced existing output\n' >&2
	exit 1
fi
[[ "$(shasum -a 256 "$output/kinosail-release.json" | awk '{print $1}')" == "$manifest_digest" ]]
