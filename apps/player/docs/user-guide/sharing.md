---
title: Share and watch together
description: Use direct Viewer access, Media Shares, downloads, and Watch Rooms safely.
section: Use Kinosail
---

# Share and watch together

Kinosail supports direct sharing and synchronized Watch Rooms. An Owner controls every share. Media travels directly between the recipient’s device and the owner-hosted Server.

## Use a Media Share

A Media Share is an expiring private link for selected Library Content. It is separate from a Viewer Profile and does not grant access to the rest of the Server.

An Owner creates a Media Share from **Settings → Media Shares**:

1. Select one or more Library Content items.
2. Choose an expiry of one hour or 24 hours.
3. Choose a device limit of one, two, or four devices.
4. Confirm that you have the right to share the selected items.
5. Select **Create secure Media Share**.
6. Copy the link immediately. Kinosail shows the claim link only after creation.

The recipient opens the link and sees **Opening shared media**. After the claim succeeds, the recipient sees **Shared with you** and can play the selected items. The claim is tied to a device session. The share expires at its configured time, and the session is capped at eight hours.

Do not post a Media Share link in a public channel. Anyone who has the unexpired link may claim an available device slot. An Owner can open **Settings → Media Shares** and select **Revoke**. Revocation removes active sessions for that share.


## Start a Watch Room

Watch Rooms synchronize authenticated Viewers who can all view the selected media. Photos are not supported.

1. Open a playable movie, episode, or audio item.
2. Open **Watch together**.
3. Select **Start Watch Together**.
4. Copy the invite link from the player.
5. Send the link to authenticated Viewers who have access to the item.

The person who starts the room is the leader. The leader controls play, pause, seek, and the current media. Other participants follow the leader. A participant cannot publish room state. The Server checks Profile access while the room is open and closes a connection when access is removed.

Rooms expire after a period without activity. A room does not bypass library, rating, viewing-hour, remote, or transcoding policy. Every participant must use an authenticated session.

## Download titles

Downloads are allowed only when the Owner enables **Downloads** for your Profile. Download titles in the Kinosail app on iPhone or iPad. The web app streams and does not save titles in the browser. See [Download and play offline]({{ '/user-guide/offline/' | relative_url }}).

A download uses the Server’s storage and, for converted quality, its transcoding capacity. Prepared files remain private to your Profile. They do not copy Library Content into another user’s Profile.

## Understand remote sharing

Remote access is off by default. Public HTTPS allows only remote-enabled Viewers with strong authentication. Owners and administrative routes remain local-network or WireGuard only. WireGuard is intended for Owner administration and paired managed devices.

Kinosail does not provide a proxy, relay, tunnel, or cache for media. Public HTTPS needs a reachable TCP 443 path, a correct TLS name, and router support. CGNAT or blocked inbound traffic must be solved with the internet provider or a network you control.

Source of truth: media share, Watch Room, download, and access handlers.
