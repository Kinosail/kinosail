---
title: Complete first setup
description: Create your account, pick your media folders, and share Kinosail with your home devices.
section: Start here
---

# Complete first setup

The first time you open Kinosail, a setup guide starts. It has 5 steps: your account, your media, your devices, your household, and your viewing history. You can skip every step except the first.

## Step 1: Create your account

Open the local address from [Download Kinosail]({{ '/download/' | relative_url }}) or [Install Kinosail]({{ '/getting-started/install/' | relative_url }}). On **Welcome to Kinosail.**:

1. Enter your name in **Name**.
2. Enter a unique password of at least 12 characters in **Password**. Keep it in a password manager.
3. Select **Create account & continue**.

This account is the Owner. The Owner can change Server settings, manage Profiles, and recover the account.

### Protect your account (recommended)

Next, Kinosail asks you to add a second way to prove it is you:

- **Use Touch ID, Windows Hello, or my phone** creates a passkey.
- **Use an authenticator app instead** sets up a time-based one-time password (TOTP).
- **Skip for now** continues without one.

If you skip, Settings shows a **Protect your account** reminder. Kinosail asks for a passkey or authenticator before you create API keys, turn on remote or management access, export backups, or change advanced configuration.

## Step 2: Choose your movie and show folders

On **Where are your movies and shows?**, pick the folders Kinosail should read:

1. Select **Add** next to a folder in **Suggested folders**. Kinosail shows how many videos each folder has.
2. To find another folder, use **Browse folders**. Select a folder name to open it, or **Up one folder** to go back. Select **Add this folder** when you are in the right place.
3. Select **Continue**. Select **Skip for now** to add folders later in **Settings → Library folders**.

Kinosail only reads these folders. It never moves, renames, or deletes your files. See [Add your media]({{ '/getting-started/add-media/' | relative_url }}) for more detail.

## Step 3: Watch on your TV and phone

On **Watch on every screen in your home.**, keep **Share Kinosail with devices on my home network** selected. Then select **Continue**. Your TVs, phones, and tablets on the same home network can then open Kinosail.

If Kinosail must restart to share, it restarts by itself when it runs as a service. Otherwise, the page asks you to restart it. After sharing, the page lists the addresses to open on your other devices.

When Docker or another installer sets `KINOSAIL_LISTEN`, the switch is hidden. The page says that your installation controls this setting.

Kinosail uses HTTPS with a private certificate. Browsers on other devices show a certificate warning until they trust it. To remove the warning, or to use Jellyfin apps, open **Advanced options**. It contains **Trusted HTTPS**, **Jellyfin apps**, **Home Assistant**, movie details from TMDB, and update choices. Trusted HTTPS publishes your private home-network address in public DNS. It does not open a router port or relay media. Follow [Connect browsers and Jellyfin apps]({{ '/getting-started/connect-devices/' | relative_url }}) for details.

Select **Skip the remaining steps** to go straight to your Library.

## Step 4: Add household Profiles

On **Set up Viewer Profiles.**, choose one of these options:

- **Parent or guardian** creates a Viewer with all ratings and every Library, without Owner controls.
- **Child or teen** creates a Viewer with a selected boundary: **Through PG / TV-PG**, **Through PG-13 / TV-14**, or **All ratings**.

Enter a name and a password with at least 12 characters. Select **Add Parent profile** or **Add Child profile**. Each Profile has its own watch progress, My List, and recommendations. Both options create standard Viewer Profiles. Later, change library access, schedules, remote access, transcoding, and downloads in **Settings → Profiles**.

Select **Continue to optional viewing history** or **Finish and open Library**.

## Step 5: Move viewing activity (optional)

On **Import your viewing history.**, choose **Plex** or **Jellyfin**. Enter the source **Server URL** and **Access token**. Then select a **Destination Viewer Profile**. For Jellyfin, you can also enter a **Source user ID**.

Select **Preview import**. This preview makes no changes and expires. Review watched items, resume positions, Jellyfin favorites, playlists, conflicts, unclear matches, and unmatched items. Select **Import once** only after you review the preview.

Kinosail does not transfer passwords, permissions, PINs, or source credentials. It does not change the source Server. Kinosail does not import Plex Universal Watchlist because Plex does not provide it through its local Server interface. To import more history later, use **Settings → Migration**.


## Confirm setup

When you are ready, select **Finish and open Library**. Check that:

- the Owner can sign in with the password and, if you added one, the passkey or authenticator;
- a Viewer sees only its assigned content;
- a second device on your home network can open Kinosail; and
- Kinosail opens the Library after a scan.

You can reopen the wizard from **Settings → Setup guide → Open setup guide**. To share Kinosail with your home network later, open the **Devices** step again.

## Source of truth

Sources: `internal/server/profiles_http.go`, `internal/server/passkeys.go`, `internal/server/onboarding_media_http.go`, `internal/server/onboarding_devices_http.go`, `internal/server/settings_http.go`, `internal/server/viewing_import_http.go`, `README.md`, and `docs/research/documentation-information-architecture.md`.
