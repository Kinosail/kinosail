---
title: Owner guide
description: Operate a Kinosail Server, control access, and maintain recoverable local state.
section: Own the Server
---

# Owner guide

Use this guide after first setup. Every task requires an Owner session unless this page says otherwise. Kinosail exposes the same application operations through its web interface and versioned API.

## Choose a task

- [Manage libraries]({{ '/owner-guide/libraries/' | relative_url }}) to add folders, scan media, and refresh metadata.
- [Configure playback]({{ '/owner-guide/playback/' | relative_url }}) to choose Direct, Automatic, Compatibility, subtitles, and hardware transcoding.
- [Secure accounts]({{ '/owner-guide/security/' | relative_url }}) to manage Owners, Viewers, passkeys, authenticators, sessions, and API keys.
- [Connect another browser]({{ '/getting-started/connect-devices/' | relative_url }}) to use secure local access or required trusted HTTPS for Jellyfin apps.
- [Configure remote access]({{ '/owner-guide/remote-access/' | relative_url }}) to configure public viewing or private Owner management.
- [Connect integrations]({{ '/owner-guide/integrations/' | relative_url }}) to configure metadata, subtitles, identity, webhooks, DLNA, and clients.
- [Back up and update]({{ '/owner-guide/backups-and-updates/' | relative_url }}) to protect state and recover from a host or storage failure.

## Use Settings safely

**Settings** is grouped into **General**, **Playback**, **Access**, **Migration**, **Library**, **System**, and **Appearance**. If a setting comes from YAML or the environment, Kinosail marks it **Read-only**. Change that external source and restart the Server.

Before a change, note whether it:

- signs out Profiles;
- needs a restart;
- changes which devices can connect;
- sends a request to an optional external service; or
- changes access to media or recovery material.

Use **Settings → System** for **Recent activity**, **Recent playback**, **Diagnostics**, and safe metrics. The activity journal records administration and security events without passwords, tokens, request bodies, or media paths.

## Keep the privacy boundary

Kinosail is local by default. Library Content, Profiles, credentials, and viewing activity stay on the owner-hosted Server. Optional metadata, subtitle, DNS, identity, notification, or Home Assistant connections receive only the requests required by the setting you enable. Kinosail does not operate a media relay, proxy, tunnel, or cache.

## Source of truth

Sources: `README.md`, `internal/server/settings_http.go`, `internal/server/system_http.go`, and `docs/research/documentation-information-architecture.md`.
