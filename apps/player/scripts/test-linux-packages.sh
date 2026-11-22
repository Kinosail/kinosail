#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
evidence="${KINOSAIL_EVIDENCE_DIR:-$root/.verification/linux-packages}"
version="${KINOSAIL_PACKAGE_VERSION:-v0.0.0}"
case "$(uname -m)" in
	x86_64) arch=amd64 label=x64 ;;
	aarch64 | arm64) arch=arm64 label=arm64 ;;
	*) printf 'unsupported architecture\n' >&2; exit 2 ;;
esac
[[ "$(uname -s)" == Linux && -d /run/systemd/system ]] || { printf 'run on a disposable Linux systemd host\n' >&2; exit 2; }
command -v docker >/dev/null || { printf 'docker is required for the RPM journey\n' >&2; exit 2; }
work="$(mktemp -d "${TMPDIR:-/tmp}/kinosail-linux-packages.XXXXXX")"
trap 'docker rm -f kinosail-rpm-test >/dev/null 2>&1 || true; rm -rf -- "$work"' EXIT
rm -rf "$evidence"
mkdir -p "$evidence/deb" "$evidence/rpm"
packages="${KINOSAIL_PACKAGE_OUTPUT:-$work/packages}"
"$root/scripts/package-linux.sh" "$version" "$arch" "$packages"
sudo "$root/scripts/test-linux-package.sh" "$packages/Kinosail-Linux-$label.deb" "$evidence/deb"
printf 'FROM fedora:43\nRUN dnf install -y systemd iproute procps-ng util-linux findutils && dnf clean all\nSTOPSIGNAL SIGRTMIN+3\nCMD ["/sbin/init"]\n' >"$work/Containerfile"
docker build --quiet --file "$work/Containerfile" --tag kinosail-rpm-test:fedora-43 "$work" >/dev/null
docker run --detach --name kinosail-rpm-test --privileged --cgroupns=host --volume /sys/fs/cgroup:/sys/fs/cgroup:rw \
	--volume "$packages:/packages:ro" --volume "$root/scripts:/scripts:ro" kinosail-rpm-test:fedora-43 >/dev/null
for _ in $(seq 1 30); do
	docker exec kinosail-rpm-test test -d /run/systemd/system && break
	sleep 1
done
status=0
docker exec kinosail-rpm-test /scripts/test-linux-package.sh "/packages/Kinosail-Linux-$label.rpm" /evidence || status=$?
docker cp kinosail-rpm-test:/evidence/. "$evidence/rpm/"
sudo chown -R "$(id -u):$(id -g)" "$evidence"
(cd "$packages" && sha256sum "Kinosail-Linux-$label.deb" "Kinosail-Linux-$label.rpm") >"$evidence/packages.sha256"
printf '{"revision":"%s","arch":"%s","commands":["scripts/package-linux.sh %s","scripts/test-linux-package.sh"]}\n' \
	"$(git -C "$root" rev-parse HEAD)" "$arch" "$version $arch" >"$evidence/run.json"
exit "$status"
