---
title: Connect phones, TVs, and Jellyfin apps
description: Use trusted HTTPS, secure local access, and optional Jellyfin support for household devices.
section: Start here
last_reviewed: 2026-08-29
---

# Connect phones, TVs, and Jellyfin apps

Kinosail keeps HTTPS on. It does not require public internet access, a router port, or a media relay for devices on your home network.

Use **Trusted HTTPS** before you enable Jellyfin apps. It avoids private-certificate errors without a manual certificate install.

| Choice | Use it when | What changes |
| --- | --- | --- |
| **Secure local access** | You use the Kinosail web app | This is on by default. No public DNS record is created. |
| **Trusted HTTPS** | You use phones, TVs, or Jellyfin apps | Kinosail gets a publicly trusted certificate for a supported provider hostname. |
| **Jellyfin apps** | You want a compatible Jellyfin or Swiftfin client | Kinosail enables its compatibility routes after trusted HTTPS is configured. |

You can keep only secure local access. You can also use trusted HTTPS without Jellyfin apps. Jellyfin apps cannot be enabled without trusted HTTPS.

## Why the wizard requires trusted HTTPS

Many Jellyfin apps reject private or self-signed certificates. Some clients cannot install a local certificate at all.

Kinosail requires a publicly trusted certificate so household members do not install a certificate or bypass a security warning. Kinosail does not offer an HTTP or ignore-certificate fallback.

The setup wizard keeps both steps together. Save trusted HTTPS, enable Jellyfin apps, then restart Kinosail once.

Kinosail does not use self-signed certificates for Jellyfin compatibility. Many players reject them, and Kinosail does not ask household members to install a certificate.

## Choose a DNS provider

The wizard supports two providers. New setups select DuckDNS because it has the easiest setup.

| Provider | Recommendation | Why choose it |
| --- | --- | --- |
| [DuckDNS](https://www.duckdns.org/) | **Easiest** | It has the shortest setup and broad homelab support. Its account token can update every hostname in the account. |
| [deSEC](https://desec.io/) | **More privacy** | It is nonprofit and open source. Its narrow token policies can limit Kinosail to one hostname and its certificate records. |

DNSSEC protects DNS data. It does not replace the publicly trusted Let's Encrypt certificate that Jellyfin clients need.

Existing DuckDNS setups continue to work after an upgrade. Kinosail detects the stored DuckDNS format automatically.

## Find the address to enter

Sign in as an Owner. Open **Settings → Access**.

When Jellyfin support is enabled, the **Jellyfin apps** section shows the exact **Connect address**. Enter the complete address in the app. Keep `https://` and the port.

The address has this form:

```text
https://myhome.dedyn.io:38127
```

Use the address shown by your Server. Do not change it to `http://`. Do not use the public remote-access address unless you configured remote access separately.

## Connect a Jellyfin app

1. Sign in to Kinosail as an Owner.
2. Open the setup wizard and go to **Devices**. You can also open **Settings → Access**.
3. Complete **Trusted HTTPS**.
4. Select **Allow compatible Jellyfin apps to connect**.
5. Select **Save Jellyfin choice**.
6. Restart Kinosail once.
7. Open Kinosail at the new trusted address.
8. Copy the displayed **Connect address**.
9. Enter the complete address in the Jellyfin-compatible app.
10. Use Quick Connect when the app supports it. Otherwise, sign in with a Viewer Profile.

Use a Viewer Profile for normal playback. Jellyfin compatibility does not grant Owner controls through the client.

Jellyfin support is off by default on a new installation. Turning it off again hides the compatible routes. It does not disable the Kinosail web app or versioned API.

## Set up trusted HTTPS

This setup does not require installing the Kinosail local certificate.

1. Choose DuckDNS or deSEC in the wizard.
2. Create a hostname with that provider.
3. Create or copy a provider token. Use the narrowest token policy available.
4. Open the setup wizard and go to **Devices**.
5. Enter the complete hostname. For DuckDNS, the short subdomain also works.
6. Enter the provider token.
7. Enter the Kinosail LAN address as a private IPv4 address or a local hostname such as `server.nox`.
8. Accept the Let's Encrypt subscriber agreement.
9. Select **Save trusted HTTPS**.
10. Enable Jellyfin apps on the same wizard page.
11. Restart Kinosail when the page shows **Restart required**.
12. Open the new trusted HTTPS address.
13. Sign in with your password and multi-factor authentication.
14. Add passkeys again for the new address.

For DuckDNS, enter `myhome` or `myhome.duckdns.org`. Kinosail keeps existing stored DuckDNS settings working automatically.

For deSEC, create a `dedyn.io` hostname. Create a narrow token that can create, update, and remove its apex `A` and `_acme-challenge` `TXT` records.

The provider publishes the private LAN address in public DNS. The hostname also appears in public certificate-transparency logs. Kinosail stores the token in its protected secrets file and does not show it again.

This local trusted HTTPS option does not open a router port. It does not make Kinosail available away from home. It does not relay media.

## Use Quick Connect on limited-input devices

Use Quick Connect when the Jellyfin-compatible app offers it. It is the preferred sign-in method.

1. Start Quick Connect in the new app.
2. Keep the displayed code visible.
3. Open Kinosail from an already authenticated device.
4. Select **Connect** in the main navigation.
5. Enter the displayed code and select **Authorize device**.
6. Confirm that the active Profile has the intended Viewer access.
7. Return to the new app.

Reject a request that you did not start. A Quick Connect code cannot create Owner access or bypass Viewer policy.

Approval works only from the local network or WireGuard. The approving session must have completed strong authentication within ten minutes.

The request expires and works once. Start a new request if the code expires or the approval page asks you to sign in again.

## Understand the Jellyfin security boundary

Kinosail keeps its normal security rules unless a client needs a narrow compatibility exception.

| Control | Kinosail behavior |
| --- | --- |
| Compatibility routes | They are off by default. Disabling Jellyfin support hides them again. |
| Transport | Publicly trusted HTTPS is required. Jellyfin support cannot enable HTTP or private-certificate fallback. |
| Quick Connect | Approval is local or WireGuard only. It requires recent strong authentication and a one-use code. |
| Password fallback | It works only locally. It is blocked when the Profile uses TOTP or the Server requires MFA. |
| Session rights | Every Jellyfin sign-in session is media-only. An Owner approval does not give the app Owner rights. |
| Viewer policy | Library, rating, schedule, download, transcoding, and remote rules still apply. |
| HLS playback | Child playlists and segments carry the item-bound playback session because some apps omit headers. Anonymous media access remains blocked. |
| Artwork | Some apps omit credentials on image requests. Kinosail permits only local anonymous item artwork. |
| Diagnostics | The authenticated bitrate test has a fixed 10 MB limit. Logs omit credentials, query data, and raw identifiers. |

The artwork exception does not allow anonymous browsing or media access. Public artwork requests return not found. Invalid presented credentials return unauthorized.

Jellyfin sign-in sessions cannot use Owner settings, API keys, or user-management routes. Kinosail records their channel and media-only privileges in activity.

## Fix common connection errors

### The app says it cannot connect

Confirm that the device is on the same network. Copy the complete trusted address from **Settings → Access**. Confirm that Jellyfin support is enabled.

Keep HTTPS and the displayed port. Confirm that trusted HTTPS reports ready after the restart.

### The app says the certificate is invalid

The device reached the wrong address or Kinosail has not completed trusted HTTPS setup. Use the displayed trusted address and check its status.

Do not disable HTTPS. Do not replace `https://` with `http://`.

### Password sign-in returns forbidden

The Profile uses TOTP, or the Owner requires MFA for every Profile. Use Quick Connect from a recently verified local or WireGuard session.

Public password sign-in is always blocked. Do not remove MFA to make a player work.

### The library loads but artwork is missing

Some Jellyfin apps request artwork without credentials. Kinosail allows that exception only on the local network or WireGuard.

Use a local or WireGuard Connect address for full artwork. Kinosail does not expose this exception on its public listener.

### Trusted HTTPS was saved but the certificate is not ready

Restart Kinosail. Reopen **Settings → Access** and read the trusted HTTPS status. Confirm the provider, hostname, LAN address, and token without sharing the token.

### The address works at home but not on cellular data

Trusted local HTTPS is still a LAN connection. Configure [remote access]({{ '/owner-guide/remote-access/' | relative_url }}) separately when a Viewer needs access away from home.

## Compatibility boundary

Kinosail tests common Jellyfin-compatible login, browse, playback, progress, download, and Quick Connect flows. A protocol test cannot certify every app version, codec, network, or physical device.

For current client behavior, see the [Jellyfin client list](https://jellyfin.org/docs/general/clients/) and [Jellyfin Quick Connect documentation](https://jellyfin.org/docs/general/server/quick-connect/).

## Source of truth

Sources: `internal/server/auth_remote_routes.go`, `internal/server/jellyfin.go`, `internal/server/jellyfin_bitrate.go`, `internal/server/quick_connect_jellyfin.go`, `internal/server/settings_jellyfin.go`, `internal/server/sessions.go`, and `packages/trustedhttps`.
