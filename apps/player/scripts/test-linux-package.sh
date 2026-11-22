#!/usr/bin/env bash
# shellcheck disable=SC2329 # Journey steps are invoked by name through step.
set -euo pipefail

package="${1:-}"
evidence="${2:-}"
if [[ $# -ne 2 || ! -f "$package" || ! "$package" =~ \.(deb|rpm)$ || -z "$evidence" ]]; then
	printf 'usage: %s Kinosail-Linux-ARCH.deb|rpm evidence-directory\n' "$0" >&2
	exit 2
fi
[[ "$(id -u)" == 0 ]] || { printf 'run as root on a disposable systemd host\n' >&2; exit 2; }
[[ -d /run/systemd/system ]] || { printf 'systemd is required\n' >&2; exit 2; }
mkdir -p "$evidence"
package="$(cd "$(dirname "$package")" && pwd)/$(basename "$package")"
format="${package##*.}"
results=()

step() {
	local name="$1"
	shift
	if "$@" >>"$evidence/steps.log" 2>&1; then
		results+=("{\"step\":\"$name\",\"result\":\"pass\"}")
	else
		results+=("{\"step\":\"$name\",\"result\":\"fail\"}")
		finish 1
	fi
}

install_package() {
	if [[ "$format" == deb ]]; then
		DEBIAN_FRONTEND=noninteractive apt-get install -y --reinstall "$package"
	else
		dnf install -y "$package" || dnf reinstall -y "$package"
	fi
}

remove_package() {
	if [[ "$format" == deb ]]; then apt-get remove -y kinosail; else dnf remove -y kinosail; fi
}

as_kinosail() {
	local defaults
	mapfile -t defaults < <(grep -Ev '^(#|$)' /usr/lib/kinosail/kinosail.defaults)
	runuser -u kinosail -- env "${defaults[@]}" "$@"
}

healthy() {
	for _ in $(seq 1 60); do
		if as_kinosail /usr/lib/kinosail/kinosail healthcheck; then return 0; fi
		sleep 1
	done
	return 1
}

loopback_only() {
	local listeners
	listeners="$(ss -Htln 'sport = :38127')"
	grep -q '127.0.0.1:38127' <<<"$listeners" && ! grep -Eq '(\*|0\.0\.0\.0|\[::\]):38127' <<<"$listeners"
}

data_owned() {
	[[ "$(stat -c %U /var/lib/kinosail)" == kinosail && "$(stat -c %U /var/cache/kinosail)" == kinosail ]]
}

runtime_runs() {
	as_kinosail /usr/lib/kinosail/runtime/ffmpeg -hide_banner -version | head -1 | grep -q 'ffmpeg version'
	as_kinosail /usr/lib/kinosail/runtime/ffprobe -hide_banner -version | head -1 | grep -q 'ffprobe version'
	as_kinosail /usr/lib/kinosail/runtime/fpcalc -version | grep -q 'fpcalc version'
}

write_marker() { runuser -u kinosail -- sh -c 'echo kept > /var/lib/kinosail/package-test-marker'; }
marker_kept() { grep -qx kept /var/lib/kinosail/package-test-marker; }
service_gone() { ! systemctl is-active --quiet kinosail.service && [[ ! -e /usr/lib/kinosail/kinosail ]]; }
config_edit_survives() {
	printf '\nKINOSAIL_SERVER_NAME=Package test\n' >>/etc/kinosail/kinosail.env
	install_package
	grep -qx 'KINOSAIL_SERVER_NAME=Package test' /etc/kinosail/kinosail.env
}
purged() {
	if [[ "$format" == deb ]]; then
		apt-get purge -y kinosail
		[[ ! -e /var/lib/kinosail ]]
	else
		true
	fi
}

finish() {
	local status="$1"
	journalctl -u kinosail.service --no-pager -o short-iso >"$evidence/service.log" 2>&1 || true
	local joined
	joined="$(IFS=,; printf '%s' "${results[*]}")"
	printf '{"schemaVersion":1,"package":"%s","sha256":"%s","host":"%s","kernel":"%s","results":[%s],"serviceLogSha256":"%s","status":"%s"}\n' \
		"$(basename "$package")" "$(sha256sum "$package" | awk '{print $1}')" "$(sed -n 's/^PRETTY_NAME="\{0,1\}\([^"]*\)"\{0,1\}$/\1/p' /etc/os-release)" \
		"$(uname -sr)" "$joined" "$(sha256sum "$evidence/service.log" | awk '{print $1}')" "$([[ "$status" == 0 ]] && echo pass || echo fail)" \
		>"$evidence/linux-package-evidence.json"
	exit "$status"
}

step install install_package
step service-enabled systemctl is-enabled --quiet kinosail.service
step service-active systemctl is-active --quiet kinosail.service
step healthy healthy
step loopback-only loopback_only
step data-owned data_owned
step runtime-runs runtime_runs
step write-marker write_marker
step repair-install install_package
step healthy-after-repair healthy
step config-edit-survives config_edit_survives
step marker-after-repair marker_kept
step remove remove_package
step service-gone service_gone
step data-kept-after-remove marker_kept
step reinstall install_package
step healthy-after-reinstall healthy
step marker-after-reinstall marker_kept
step purge purged
finish 0
