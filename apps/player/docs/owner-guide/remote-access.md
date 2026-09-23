---
title: Configure remote access
description: Set up viewing away from home and keep Owner management private.
section: Own the Server
---

# Configure remote access

Public viewing and management while away are separate and off by default. Use public HTTPS for approved Viewers. Keep Owner management at home, or explicitly pair your own device in **Settings → Manage while away**. Both paths connect directly to your Server.

## Choose the boundary

| Path | Use | Router requirement |
| --- | --- | --- |
| LAN HTTPS | Devices on the trusted home network | None from the internet |
| Private management | Individually paired Owner devices, with two-step sign-in | Forward UDP `51821` in the standard installation |
| Public HTTPS | Remote-enabled Viewers and compatible Jellyfin apps | Forward TCP `443` |

Kinosail does not operate a hosted relay, tunnel service, media proxy, or cache. The optional private management tunnel runs on your own Server. Carrier-grade NAT or blocked inbound traffic needs public reachability from the internet provider.

## Run the remote-access wizard

Complete local setup first. From the bundle root, run:

```sh
./scripts/setup-remote-access.sh
```

The wizard configures public HTTPS and asks for a DuckDNS subdomain label. It stores the DuckDNS token in `secrets/duckdns_token` with mode `600` and writes only the token file path to `.env`.

For public HTTPS, the wizard sets `KINOSAIL_REMOTE_MODE=https`, `KINOSAIL_AUTH_URL=https://<name>.duckdns.org`, and `KINOSAIL_REMOTE_LISTEN=:8443`. Forward router TCP `443` to the host's TCP `443`; do not expose TCP `80` or the LAN port. Test from cellular data with a remote-enabled Viewer.

## Add the router rule

Open **Settings → Watch away from home → Set up access away from home**. The page shows your configured public address, the steps to follow, and current Server checks. It does not open ports automatically.

1. On home Wi-Fi, open the router's app or settings page. Use the router administrator sign-in, which may differ from your Wi-Fi password.
2. In **Connected devices**, find the computer or NAS running Kinosail. Reserve its local IP address so it will not change. Routers may call this **Address reservation** or **DHCP reservation**.
3. Open **Port forwarding** (sometimes **Virtual server** or **NAT forwarding**) and add a custom rule:

| Router field | What to enter |
| --- | --- |
| Name / service | `Kinosail HTTPS` |
| Device / internal IP | The Server computer's reserved local IP |
| Protocol | **TCP only** |
| External / public port | **443** |
| Internal / private port | **443** |

If there are start and end port fields, put `443` in both. Save the rule. On the Server computer's firewall, allow incoming TCP `443` while keeping the firewall enabled. The standard container installation maps host `443` to container `8443`; the router must target host `443`.

Never use DMZ, open a range, forward the LAN port, or enable router administration from the internet. These instructions apply to the standard public HTTPS installation, not a custom reverse proxy.

Official router guides: [TP-Link](https://www.tp-link.com/us/support/faq/1379/), [ASUS](https://www.asus.com/us/support/faq/1037906/), [NETGEAR](https://kb.netgear.com/24289/How-do-I-set-up-port-forwarding-to-a-local-server-on-my-NETGEAR-router), [eero](https://eero.com/support/articles/how-do-i-set-up-port-forwarding), [Google Nest Wifi / Google Wifi](https://support.google.com/googlehome/answer/6274503?hl=en), and [Xfinity Gateway](https://www.xfinity.com/support/articles/xfi-port-forwarding). Use the Kinosail values above instead of any example ports in a guide. For another model, use the manufacturer's support site.

## Prepare a Viewer and test from outside

Before enabling public HTTPS, create a Viewer Profile, select only the Libraries they need, and enable its remote access. Leave **Allow downloads** off if you want playback only. Sign in as that Viewer at the local Server address, open **Account**, and set up an authenticator app. Keep the home device signed in for approval. After changing the authentication hostname, use password plus authenticator locally; passkeys remain tied to their registered hostname.

Turn off Wi-Fi on your phone. Open the public HTTPS address shown in Settings in a browser and choose **Get a sign-in code**. The browser opens your library after approval; the code expires after a few minutes and **Cancel sign-in** withdraws it. In Kinosail or a compatible Jellyfin app, add that address and choose **Quick Connect**. On the signed-in home device, open Quick Connect as that Viewer, check that the device and code match, then approve. Only approve a code you initiated. Enable Jellyfin apps in Settings first if using one.

Play an item and seek. A browser can also use a passkey already registered for the public address. Registering a new passkey and approving Quick Connect remain local operations; the public listener blocks them.

A successful home-Wi-Fi test does not prove outside access. **Check again** refreshes local Server checks only.

- **Timeout:** check the Server IP, both port fields, firewall, and DuckDNS public address. With two routers, forward outer TCP `443` to the inner router's WAN address, then inner TCP `443` to the Server.
- **Still blocked:** ask your internet provider, “Do I have a public IPv4 address, and can I receive incoming TCP 443 connections?” Double NAT or carrier-grade NAT (CGNAT) can prevent access. Port forwarding alone cannot fix CGNAT; request a public address or use an owner-controlled VPN compatible with your network.
- **Certificate warning:** stop and fix the hostname, mapping, or certificate error in Settings. Never bypass warnings at the public address.
- **Sign-in rejected:** check Viewer remote permission, use a passkey or approved Quick Connect, and confirm it is not an Owner Profile.

## Prepare public HTTPS safely

Open **Settings → Watch away from home → Set up access away from home**. Public streaming is ready only when authorization state, dedicated public HTTPS, the deny-by-default route policy, listener, certificate, a remote-enabled Viewer with a passkey or authenticator, and the persistent kill switch pass.

Normal public sign-in allows only explicitly remote-enabled Viewers with strong authentication. Optional Owner-created Media Shares grant only their selected media. SCIM provisioning stays on the private network, even with a valid provisioning token. API keys and machine integrations cannot use the Owner device tunnel. Owners, API keys, setup, Profile management, backups, diagnostics, MCP/OAuth administration, and other administrative routes remain unavailable on the public listener. Public sessions use a secure, HttpOnly cookie and are bound to the public access channel. They are revoked when permission is removed or the kill switch runs.

A stolen Viewer session can expose allowed media, viewing history, playlists, and enabled download access until revoked. It must not grant Server administration. The standard HTTPS Compose override runs the public TLS gateway in a separate unprivileged container. It has no media, configuration, account, backup, or DNS-token mounts. A process-wide Linux filter prevents it from opening IP sockets after its listener starts. It can connect only to the public application and public-certificate Unix sockets; the application always applies the public Viewer policy to that traffic.

A server or operating-system exploit is a broader threat. The main application still handles the allowed public viewing requests and holds management state. A vulnerability in that application or the host could exceed Viewer permissions. The gateway is an additional boundary, not a guarantee that every full compromise is limited to watching movies. Keep Kinosail, the host, and router updated, limit mounts and permissions, and retain recovery backups.

## Manage your Server while away

At home, Owner management works as before. Public viewing never grants Owner access. To manage while away:

1. Finish trusted HTTPS setup and enable two-step sign-in for your Owner Profile. A public DuckDNS HTTPS hostname also works. If you use that hostname, Kinosail renews the private tunnel's certificate through DNS and maintains its public address even when the **public-access kill switch** is on. A custom endpoint hostname needs its own DNS maintenance.
2. Open **Settings → Manage while away** on your home network. Enter your public Server hostname and UDP port, normally `your-server.duckdns.org:51821`, and choose **Enable private management**. A valid HTTPS certificate must already be available; if issuance is still starting, wait and try again.
3. In the router, forward **UDP 51821** to **UDP 51821** on the Server's reserved LAN IP. This is separate from the TCP `443` rule used for viewing. The standard container maps this host port to internal UDP `51821`. A custom external UDP port may be any port from `1024` through `65535`; use the exact mapping shown on the management page.
4. Install the [WireGuard app](https://www.wireguard.com/install/) on your Owner device. While at home, give the device a recognizable name and download its private profile. Import that file into WireGuard, then delete the downloaded copy. Never share it. Pair each device separately; up to 32 devices are supported.
5. Turn on the tunnel, open the HTTPS management address shown on the page, and sign in as the Owner who paired this device using a passkey or password plus authenticator. The tunnel proves the device; sign-in proves the Owner. Turning off the tunnel makes that session unusable. It cannot be replayed on the LAN, public HTTPS, or another paired device.
6. Turn Wi-Fi off and try opening Settings from cellular data before leaving home. “On” means the Server started its private listener; it does not prove that your router or internet provider permits the connection.

The tunnel routes only the Server's virtual address, `10.92.0.1/32`. It cannot route to the rest of your LAN, host services, or other paired devices. It does not install a host network interface or need elevated container privileges. While it is active, device DNS uses the Server: the management hostname resolves inside the tunnel and other A/AAAA lookups use the Server's resolver. Other network traffic follows the device's normal connection. If the Server is offline or this device is revoked, turn off its tunnel to restore normal DNS. If your home internet address changes, reconnect the tunnel to resolve the new endpoint. If the browser bypasses system DNS using its own secure-DNS setting, use automatic/system DNS for this connection. Never bypass an HTTPS warning.

Enabling management and pairing new devices require the home network. Pairing, revoking devices, disabling management, and exporting a portable backup require Owner sign-in within the last ten minutes. Management sessions require strong sign-in within eight hours and still follow your configured session limits. Changing the Owner's credentials or permissions invalidates existing pairings; pair again at home.

Revoke a lost device on the management page. Revocation closes its active connections. **Turn off management while away** disconnects all devices and erases the Server tunnel identity and pairings. Re-enabling creates new keys; old profiles remain unusable. Remove the management UDP router rule when you no longer need it. Portable backups do not include management pairing keys; pair devices again after restoring onto a new Server.

For local recovery without a working login or device, run this inside the Server container using your installation's Compose files:

```sh
podman compose exec -T kinosail kinosail owner-access-disable
```

For a directly installed binary, run `kinosail owner-access-disable` as the Server's operating-system user with its normal configuration. The command records a durable recovery lock; a running Server stops the tunnel within two seconds, and it stays off after restart. If saving fails, stop the Server and resolve the storage error before restarting. Home-network Owner sign-in remains available.

## Stop remote access

Use the **Emergency public-access kill switch** in **Settings → Watch away from home**, then select **Disable public access now**. This closes the public application path, revokes public authorization, and keeps it off after restart. In the isolated gateway installation, the gateway may still answer with an unavailable response until it is stopped. LAN access and separately enabled private management stay available. Remove the Kinosail router rule too when you no longer need remote access.

You can also run:

```sh
./scripts/disable-remote-access.sh
```

The script sets `KINOSAIL_REMOTE_MODE=off`, restores the saved local authentication address (including an originally automatic address), stops the old service before recreating the installed release or source service, and stops an active `wg-quick@kinosail` interface. Setup reruns preserve the original local address. A failed recreation leaves the old public listener stopped; fix the reported error before restarting locally. It does not remove local Profiles or Library Content. This script restores the local authentication hostname, so update your management bookmark if it changes. To stop private management too, use its separate control or the local recovery command above.

## Understand trusted HTTPS

**Trusted HTTPS** is the smoothest secure path for phones, TVs, and apps. It is different from public remote access and remains optional. It obtains a Let's Encrypt certificate through DuckDNS or deSEC while Kinosail remains on the LAN. DuckDNS is easiest. deSEC supports narrower tokens for more privacy. Trusted HTTPS publishes the private LAN address in public DNS and the hostname in certificate-transparency logs. It does not open a router port.

Follow [Connect browsers and Jellyfin apps]({{ '/getting-started/connect-devices/' | relative_url }}) for trusted HTTPS setup and device certificate choices.

## Source of truth

Sources: `README.md`, `scripts/setup-remote-access.sh`, `scripts/disable-remote-access.sh`, `internal/server/remote_readiness.go`, `internal/server/settings_http.go`, `internal/server/management_access.go`, `packages/owneraccess`, `packages/publicgateway`, and `compose.remote-https.yaml`.
