---
title: Kinosail Player and Subtitles
description: Compare Kinosail Player and Kinosail Subtitles by purpose, media access, Docker setup, costs, and privacy. Run either app alone or both together.
section: Project
last_reviewed: 2026-09-30
---

# Choose a Kinosail app

Kinosail makes free, source-available apps for personal media on your own hardware.
**Kinosail Player** browses and plays your library. **Kinosail Subtitles** finds
missing subtitles and saves validated files beside your videos. Each app runs
independently with its own container, settings, and application data.

## Which app do I need?

| Your goal | Choose |
| --- | --- |
| Browse and play your library | [Kinosail Player]({{ '/' | relative_url }}) |
| Find and maintain subtitles | [Kinosail Subtitles]({{ '/subtitles/' | relative_url }}) |
| Play your library and fill subtitle gaps | Both apps |

Player supports movies, shows, music, audiobooks, books, comics, and photos.
Subtitles works with local movies and episodes in your preferred languages.

For `Film.mkv`, an English subtitle sidecar can be named `Film.en.srt`.
A sidecar is a separate file that a compatible player reads beside the video.
Subtitles can prepare these files without Player. Player can use existing
subtitles without the Subtitles app.

## Can both apps use the same media folder?

Yes. Player reads your existing media folders through a read-only mount.
Subtitles needs a writable mount to save SRT or WebVTT files beside videos.
Give each container the access it needs. Keep their application data separate.

## How do I install them?

Use [the Player Docker quickstart]({{ '/quickstart/' | relative_url }}) or
[the Subtitles Docker guide]({{ '/subtitles/getting-started/install/' | relative_url }}).
The [NAS and Proxmox guide]({{ '/getting-started/platforms/' | relative_url }})
provides prepared Compose files for either app or both together.
Keep each app's persistent data separate and complete first setup for each Server.

Player provides a web interface for playback and library management. Subtitles
provides a web interface for coverage, matching, languages, providers, and history.
For browser playback and optional Jellyfin app connections, read
[supported devices and limits]({{ '/getting-started/connect-devices/' | relative_url }}).

## What does Kinosail cost?

Both Servers are free to run, including their web interfaces and core features.
Optional Supporter badges do not unlock core features. You provide the hardware
and storage. External subtitle providers have their own accounts, terms, and quotas;
Kinosail does not include a paid provider subscription.
Read [Subtitles provider setup]({{ '/subtitles/owner-guide/integrations/' | relative_url }})
before choosing a provider.

Kinosail is **source-available**, not open source. Its PolyForm Perimeter license
limits commercial competition. Read [the licensing guide](https://github.com/Kinosail/kinosail/blob/main/LICENSING.md)
for the exact permissions and limits.

## Where does my media go?

Your media stays on your Server. Player reads source files without changing them.
Subtitles saves sidecar files locally and does not upload your videos to providers.
Provider requests can include search metadata and a locally calculated file hash.
Local use does not require a Kinosail-hosted account; create the Owner on your Server.

Read [Player privacy]({{ '/reference/architecture-and-privacy/' | relative_url }})
and [Subtitles privacy]({{ '/subtitles/reference/architecture-and-privacy/' | relative_url }})
for optional integrations and remote-access boundaries.

## Can I connect an AI assistant?

Both apps provide authenticated Model Context Protocol (MCP) interfaces for
supported operations. MCP lets a compatible assistant use the Server's tools
with the access you approve. An assistant connection is optional.
Read [Player MCP]({{ '/developer-guide/mcp/' | relative_url }}) or
[Subtitles MCP]({{ '/subtitles/developer-guide/mcp/' | relative_url }})
for connection methods, permissions, and limits.

## Can I try Player with an existing library?

Yes. Player reads your existing folders without moving the original files.
The [Plex and Jellyfin migration guide]({{ '/owner-guide/migration/' | relative_url }})
explains supported history imports, preview steps, and what does not transfer.
For project context and how AI tools help development, read
[why Kinosail exists]({{ '/why/' | relative_url }}).
