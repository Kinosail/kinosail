# Server discovery: operational handoff

## Incident and repair — 2026-09-08

The native Player setup screen reported “No servers found” on the TV. Player was running in a Docker bridge network, so its container-local Bonjour announcement was not visible on the physical LAN. A Mac browsing `_kinosail-player._tcp` also found no server before the repair.

The server host's standard Avahi service and socket were already masked. Those masks were preserved. A dedicated `kinosail-discovery.service` now runs the installed Avahi daemon using `/etc/avahi/kinosail.conf`. It advertises Player on the physical LAN interface only, with D-Bus and multicast reflection disabled. Avahi drops root privileges to its existing service account.

Host-owned files:

- `/etc/systemd/system/kinosail-discovery.service`: enabled at boot; runs `/usr/bin/avahi-daemon --file=/etc/avahi/kinosail.conf --no-chroot`; restarts on failure.
- `/etc/avahi/kinosail.conf`: limits announcements to the host's physical LAN interface; IPv4 enabled, IPv6 multicast disabled; workstation and hardware information publication disabled.
- `/etc/avahi/services/kinosail.service`: service name from `KINOSAIL_SERVER_NAME`, type `_kinosail-player._tcp`, port from `KINOSAIL_AUTH_URL`, TXT records `version=1`, `scheme=https`, and `url=<configured HTTPS origin>`.

The actual private connection address and LAN interface configuration remain on the host. This repair changed host configuration only; it did not alter the Player container, application code, or the standard masked Avahi units. These files are not managed by the repository deployment watcher.

## Evidence and limits

After starting the dedicated service, the Mac discovered the server and resolved its Bonjour record to the intended port and configured HTTPS origin. That HTTPS endpoint returned HTTP 200 from `/healthz` with certificate verification enabled. Player remained running and healthy. The service was confirmed active and enabled at boot; a reboot was not performed.

The user subsequently confirmed that TV server discovery worked. TV sign-in and phone approval are separate from that discovery result. Quality gates were not run because `.gates-disabled` remains in effect.

Avahi logged a warning about other mDNS stacks already running on the host. Discovery succeeded from the Mac despite that warning. If discovery becomes intermittent, inspect port 5353 ownership and multicast traffic before changing or disabling unrelated home services.

## Resume here

1. TV discovery is user-confirmed. Preserve that fix; confirm sign-in separately when changing device approval.
2. Preserve the dedicated host announcement across host migrations and configuration cleanup. Container rebuilds alone do not recreate it.
3. If the server name, HTTPS origin, published port, or physical LAN interface changes, update the host configuration accordingly and restart the dedicated service. Its advertisement is a static snapshot, not a live read of container settings.
4. Keep HTTPS certificate checks and the existing connection URL policy intact. Do not substitute container addresses or enable multicast reflection as a workaround.

On the server host, inspect the service using:

```sh
systemctl is-active kinosail-discovery.service
systemctl is-enabled kinosail-discovery.service
sudo journalctl -u kinosail-discovery.service -n 30 --no-pager
```

From a Mac on the same LAN, verify both browsing and resolution (stop each command with Control-C):

```sh
dns-sd -B _kinosail-player._tcp local.
dns-sd -L '<server name>' _kinosail-player._tcp local.
```

Then check `/healthz` using the advertised HTTPS origin with normal certificate verification. Do not publish the origin or raw resolution output in shared artifacts.

To roll back only this repair, stop and disable `kinosail-discovery.service`, remove the three files listed above, and reload systemd. Preserve the standard Avahi masks and unrelated mDNS services. Discovery will again be unavailable across the Docker bridge.
