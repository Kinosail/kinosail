---
title: Sign-in and access problems
description: Recover from certificate, passkey, authenticator, profile policy, remote, and session problems.
section: Fix a problem
---

# Sign-in and access problems

Use this page when a Profile cannot sign in, a passkey or authenticator fails, a page is denied, or a remote link stops working.

## Password sign-in fails on the local network

Check the Profile name, password, and address. Profile names are not case-sensitive, but the password is. Do not retry a password that you may have exposed.

Use the local HTTPS address that matches the configured authentication origin. A certificate warning can occur with a locally generated certificate. Confirm the host name before trusting it.

Remote public HTTPS does not accept password-only sign-in. Use a verified passkey or Quick Connect from the local network or WireGuard.

## A passkey does not work

Use the exact configured HTTPS origin. Passkeys are bound to their origin, so a different host name, port, or scheme can fail even when the Server is healthy.

Try another enrolled passkey from **Account**. If no passkey works, use the Profile’s TOTP authenticator or recovery process from the local network. Do not delete all passkeys until another strong sign-in method works.

If the passkey list shows **Usage not recorded**, the credential still works; Kinosail could not record usage metadata for that event. Review the account page and Server logs without sharing credential data.

## TOTP or a recovery code fails

Ensure the device clock is correct and enter the current six-digit code. A recovery code works once. Store unused recovery codes privately.

If every code fails, stop repeated attempts and ask an existing Owner to review the Profile locally. Do not send a TOTP secret or recovery code to support.

## A Viewer cannot see a library or title

Ask an Owner to review the Profile policy. The Owner may have granted no libraries, selected libraries, a content-rating ceiling, or a viewing schedule. The Server applies these rules to every relevant operation.

If the title appears but playback is denied, check the Profile’s transcoding permission and the item’s playback mode. If downloads are missing, check the Downloads permission.

## A Viewer cannot use remote access

Remote access must be enabled for both the Server path and the Viewer Profile. Public HTTPS is off by default and allows only remote-enabled Viewers. Owners use the local network or WireGuard for administration.

Check the public host name, TLS certificate, router TCP 443 forward, firewall, and external network one at a time. CGNAT can block inbound access. Kinosail does not relay media around that limit.

If an Owner disabled remote access or used the public kill switch, existing public sessions are revoked. Re-enable the approved mode from a local or WireGuard session, then create a new strong Viewer sign-in.

## A Jellyfin app cannot connect

Open **Settings → Access** as an Owner. Confirm that trusted HTTPS reports ready and **Allow compatible Jellyfin apps to connect** is enabled. Copy the complete **Connect address**.

If the app reports an invalid certificate, use the displayed trusted address. Complete the **Devices** wizard and restart Kinosail if trusted HTTPS is not ready. Do not switch to HTTP or disable certificate checks.

If playback closes immediately, retry after the Server update completes. Kinosail binds each HLS child request to the short-lived playback session. It does not require anonymous media access.

Follow [Connect phones, TVs, and Jellyfin apps]({{ '/getting-started/connect-devices/' | relative_url }}) for the complete Jellyfin and trusted HTTPS steps.

## Quick Connect does not complete

Create the request on the limited-input device. Approve it from a strongly authenticated local or WireGuard session. Use it once before it expires. A Quick Connect request cannot create Owner access or bypass Viewer policy.

If the approval page asks for sign-in, complete passkey or TOTP verification. Retry within ten minutes.

If the app reports forbidden during password sign-in, use Quick Connect. TOTP and Server-wide MFA intentionally block Jellyfin password fallback.

If it is rejected, create a new request. Confirm that the active Profile has the intended access. Never approve an unexpected request.

For a public client, the Profile must also allow remote access. Owners cannot approve public Quick Connect for themselves.

## A Watch Room closes or does not show the item

Every participant must be authenticated and allowed to view the selected movie, episode, or audio item. Photos are not supported. A viewing-hour, library, rating, remote, or transcoding restriction can remove a participant while the room is open.

Create a new room after access changes. Rooms expire after inactivity. The leader controls playback; other participants cannot publish room state.

## A Media Share link is unavailable

Open the link before its expiry and use a device slot that the Owner allowed. A Media Share can expire, be revoked, reach its device limit, or refer to media that is no longer indexed.

Ask the Owner to create a new share rather than forwarding the old claim link. Do not include the claim token in a report. A Media Share is not a general Viewer account and cannot open other library items.

## A session ends unexpectedly

An Owner can change inactive or absolute session timeouts, require multi-factor authentication, remove a Profile, revoke a device, or sign out other devices. Remote public sessions have their own expiry. Sign in again with a strong method and review **Account** sessions.

If a session ends after a Profile policy change, ask an Owner to confirm the change. The Server revokes access when policy requires it.

## Collect safe access evidence

Record the address class, Profile role, device, app version, exact message, and time. An Owner can review **Settings → System** and recent activity.

Successful Jellyfin sign-in activity includes the channel and `media-only` privileges. Unknown compatible paths use a normalized `compatibility_path` value.

The activity journal records `settings.trusted-https.updated` and `settings.jellyfin.updated`. Each entry includes success or failure, HTTP status, request ID, and safe before-and-after state. Provider tokens never appear.

Kinosail omits query data, credentials, and raw identifiers from request logs. Still review a short excerpt before sharing it:

```sh
podman compose logs --tail 100 kinosail
```

Source of truth: authentication, Profile policy, remote access, Watch Room, and Media Share handlers.
