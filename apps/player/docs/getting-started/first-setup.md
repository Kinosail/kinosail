---
title: Complete first setup
description: Create the first Owner, secure the account, and choose initial Server options.
section: Start here
---

# Complete first setup

The first-launch wizard has four steps. Create the Owner first. Then choose the trusted device address, add household Profiles, and optionally move viewing activity from Plex or Jellyfin.

## Step 1: Create your Owner

Open the local address from [Install Kinosail]({{ '/getting-started/install/' | relative_url }}). On **Set up Kinosail**, enter:

1. A name in **Name**.
2. A unique password of at least 12 characters in **Password**.
3. Keep **MFA - Add extra sign-in protection now** selected to configure a time-based one-time password (TOTP) during setup. Kinosail requires extra sign-in protection for every Viewer Profile by default.
4. Select **Create Owner & continue**.

The Owner can change Server settings, manage Profiles, and perform recovery. Kinosail keeps the media catalog, Profiles, and viewing activity on this Server. Remote access is a separate choice.

If you do not enable TOTP in this step, sign in and add a passkey from your Profile page. A passkey needs the configured Server address and trusted HTTPS. Keep your password in a password manager.


## Step 2: Choose how devices connect

Kinosail keeps HTTPS on and stays on your network unless you enable remote access later. Each connection choice is optional.

Choose any options your household needs:

- Keep **Secure local access**, which is on by default. Devices can trust the Kinosail local CA certificate at `/api/v1/agent-connections/certificate`.
- Enable **Jellyfin apps** only when the household uses a compatible Jellyfin client. This option does not disable HTTPS.
- Configure **Trusted HTTPS** for phones, TVs, and Jellyfin apps. Jellyfin apps require this step because many clients reject private certificates. Choose DuckDNS for the easiest setup. Choose deSEC for a narrow token and more privacy. Enter the hostname, provider token, and Kinosail LAN address. Accept the Let's Encrypt subscriber agreement, then select **Save trusted HTTPS**.

Trusted HTTPS publishes the private LAN address in public DNS and puts the hostname in certificate-transparency logs. It does not open a router port and does not relay media. Restart Kinosail when the page says **Restart required**. Sign in again at the trusted hostname and add your passkeys again because the authentication origin changed.

Follow [Connect phones, TVs, and Jellyfin apps]({{ '/getting-started/connect-devices/' | relative_url }}) for the complete device, Jellyfin, and DNS-provider procedures.

Select **Continue to household setup** or **Skip optional setup**.

## Step 3: Add household Profiles

On **Set up your household**, choose a setup shortcut:

- **Parent or guardian** creates a Viewer with all ratings and every Library, without Owner controls.
- **Child or teen** creates a Viewer with a selected boundary: **Through PG / TV-PG**, **Through PG-13 / TV-14**, or **All ratings**.

Enter a name and a password of at least 12 characters. Select **Add Parent profile** or **Add Child profile**. Each Profile keeps its own watch progress, My List, and recommendations. Both shortcuts create regular Viewer Profiles. Refine library access, schedules, remote access, transcoding, and downloads later in **Settings → Profiles**.

Select **Continue to arrival** or **Finish later**.

## Step 4: Move viewing activity (optional)

On **Bring your history home**, choose **Plex** or **Jellyfin**, enter the source **Server URL** and **Access token**, then select a **Destination Viewer Profile**. For Jellyfin, you can enter an optional **Source user ID**.

Select **Preview import**. The preview makes no changes and expires. Review watched items, resume positions, Jellyfin favorites, playlists, conflicts, ambiguous matches, and unmatched items. Select **Import once** only after you approve the preview.

Kinosail does not transfer passwords, permissions, PINs, or source credentials. It does not write to the source. Plex Universal Watchlist is not imported because Plex's documented local Server interface does not expose it. The onboarding path does not offer recurring pulls; configure those later in **Settings → Migration**.


## Confirm setup

Select **Finish and open Library** when you are ready. Confirm that:

- the Owner can sign in with the password and configured authenticator;
- a Viewer sees only its assigned content;
- the trusted address works on a second device, when configured; and
- Kinosail opens the Library after a scan.

You can reopen the wizard from **Settings → Setup guide → Open setup guide**. To expose the Server on the LAN after local setup, rerun the installer with `--lan`.

## Source of truth

Sources: `internal/server/profiles_http.go`, `internal/server/settings_http.go`, `internal/server/viewing_import_http.go`, `README.md`, and `docs/research/documentation-information-architecture.md`.
