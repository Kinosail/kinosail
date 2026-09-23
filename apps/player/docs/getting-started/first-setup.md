---
title: Complete first setup
description: Create the first Owner, secure the account, and choose initial Server options.
section: Start here
---

# Complete first setup

The first-launch wizard has four steps. First, create the Owner. Then choose a trusted address for devices. Add household Profiles. You can also import viewing activity from Plex or Jellyfin.

## Step 1: Create your Owner

Open the local address from [Install Kinosail]({{ '/getting-started/install/' | relative_url }}). On the **Set up Kinosail** page, enter:

1. A name in **Name**.
2. A unique password of at least 12 characters in **Password**.
3. Keep **MFA - Add extra sign-in protection now** selected to set up a time-based one-time password (TOTP). By default, Kinosail requires extra sign-in protection for each Viewer Profile.
4. Select **Create Owner & continue**.

The Owner can change Server settings, manage Profiles, and recover the account. Kinosail keeps the media catalog, Profiles, and viewing activity on this Server. You can set up remote access later.

If you skip TOTP, sign in and add a passkey from your Profile page. A passkey needs the Server address that you set up and trusted HTTPS. Keep your password in a password manager.


## Step 2: Choose how devices connect

Kinosail uses HTTPS and stays on your home network unless you enable remote access. Both connection options are optional.

Choose any options your household needs:

- Keep **Secure local access**, which is on by default. Devices can download the Kinosail local CA certificate from `/api/v1/agent-connections/certificate`.
- Set up **Trusted HTTPS** for browsers on your home devices. DuckDNS is the easiest option. deSEC lets you use a more limited token. Enter the hostname, provider token, and Kinosail home-network address. Accept the Let's Encrypt agreement. Then select **Save trusted HTTPS**.

Trusted HTTPS publishes your private home-network address in public DNS. It also adds the hostname to public certificate records. This does not open a router port or relay media. Restart Kinosail when the page says **Restart required**. Then sign in again at the trusted hostname. Add your passkeys again because the sign-in address changed.

Follow [Connect browsers and Jellyfin apps]({{ '/getting-started/connect-devices/' | relative_url }}) for LAN access, certificate choices, and optional compatible-client setup.

Select **Continue to household setup** or **Skip optional setup**.

## Step 3: Add household Profiles

On **Set up your household**, choose one of these options:

- **Parent or guardian** creates a Viewer with all ratings and every Library, without Owner controls.
- **Child or teen** creates a Viewer with a selected boundary: **Through PG / TV-PG**, **Through PG-13 / TV-14**, or **All ratings**.

Enter a name and a password with at least 12 characters. Select **Add Parent profile** or **Add Child profile**. Each Profile has its own watch progress, My List, and recommendations. Both options create standard Viewer Profiles. Later, change library access, schedules, remote access, transcoding, and downloads in **Settings → Profiles**.

Select **Continue to arrival** or **Finish later**.

## Step 4: Move viewing activity (optional)

On **Bring your history home**, choose **Plex** or **Jellyfin**. Enter the source **Server URL** and **Access token**. Then select a **Destination Viewer Profile**. For Jellyfin, you can also enter a **Source user ID**.

Select **Preview import**. This preview makes no changes and expires. Review watched items, resume positions, Jellyfin favorites, playlists, conflicts, unclear matches, and unmatched items. Select **Import once** only after you review the preview.

Kinosail does not transfer passwords, permissions, PINs, or source credentials. It does not change the source Server. Kinosail does not import Plex Universal Watchlist because Plex does not provide it through its local Server interface. To import more history later, use **Settings → Migration**.


## Confirm setup

When you are ready, select **Finish and open Library**. Check that:

- the Owner can sign in with the password and configured authenticator;
- a Viewer sees only its assigned content;
- the trusted address works on a second device, when configured; and
- Kinosail opens the Library after a scan.

You can reopen the wizard from **Settings → Setup guide → Open setup guide**. To expose the Server on the LAN after local setup, rerun the installer with `--lan`.

## Source of truth

Sources: `internal/server/profiles_http.go`, `internal/server/settings_http.go`, `internal/server/viewing_import_http.go`, `README.md`, and `docs/research/documentation-information-architecture.md`.
