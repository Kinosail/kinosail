---
title: Frequently asked questions
description: Get concise answers about Kinosail requirements, privacy, clients, playback, remote access, and licensing.
section: Project
last_reviewed: 2026-09-24
---

# Frequently asked questions

## What is Kinosail Player?

Kinosail Player is a free, self-hosted media Server and web player that runs on your own hardware. It brings movies, shows, music, audiobooks, books, comics, and photos from folders you choose into one library. Your original files stay on a read-only mount, and local use needs no Kinosail account. [Start with Docker]({{ '/quickstart/' | relative_url }}) or [explore the features]({{ '/features/' | relative_url }}).

## Can I try Kinosail with the media I already have?

Yes. [Install Kinosail with Docker]({{ '/quickstart/' | relative_url }}), mount your existing media folder read-only, and [add a Library]({{ '/getting-started/add-media/' | relative_url }}). Kinosail scans the files in place and keeps its catalog and playback cache outside the media folder. You can explore the web Player without moving your original files or creating a hosted account. Check [media compatibility]({{ '/reference/media-compatibility/' | relative_url }}) for supported file types and device-dependent playback.

## Does Kinosail require a hosted account or subscription?

No. Local use does not require a hosted account or subscription. Optional metadata, subtitle, identity, DNS, notification, supporter, and agent integrations make their own network requests only when enabled.

## Does Kinosail upload my media?

No Kinosail-operated service relays or stores your media. The Server scans a read-only local mount and serves direct media from the owner-hosted Server. Optional providers receive only the requests needed for the features that you enable.

## How many containers do I need?

The supported self-hosted deployment uses one Kinosail Server container. It includes embedded SQLite and does not require a database sidecar.

## What files can Kinosail scan?

It scans common video, audio, audiobook, photo, and book extensions. See the complete [media compatibility reference]({{ '/reference/media-compatibility/' | relative_url }}). Scanner ingest does not guarantee direct playback on every device.

## What is the difference between Direct, Automatic, and Compatibility playback?

Direct uses the source representation. Automatic starts Direct and can fall back to an adaptive compatible representation. Compatibility requests Server-generated HLS. The internal API value is `compatible`. The Viewer policy must permit playback and, for fallback, transcoding.

## Why is an item scanned but not playing directly?

The source container, codec, resolution, bitrate, HDR format, subtitle mode, client capabilities, or policy may require remuxing or transcoding. Check `GET /api/v1/items/{id}/playback` for the selected mode and reason.

## Which apps do these docs cover?

These docs cover the web Player, Server administration, the HTTP API, MCP, and supported Jellyfin-compatible app flows. See [Connect browsers and Jellyfin apps]({{ "/getting-started/connect-devices/" | relative_url }}) for setup and compatibility limits.

## Can I access Kinosail from the internet?

Yes, if the Owner configures WireGuard or public HTTPS and the network has public reachability. Public HTTPS is off by default, uses a restricted Viewer boundary, and does not provide a media relay. CGNAT or blocked inbound traffic still requires a solution from the internet provider.

## Can remote users sign in with a password?

No. Public password login is disabled. Remote Viewers use a passkey or a short-lived Quick Connect request approved from a strongly authenticated local path.

## Where does Kinosail keep configuration and backups?

Configuration and application state use `/config`, the playback and analysis cache uses `/cache`, Library Content uses read-only `/media`, and backups use `/backups` by default. See [Configuration]({{ '/reference/configuration/' | relative_url }}) for source precedence and mounted paths.

## How do I configure Kinosail without the web interface?

Use `kinosail.yaml` for deployment-owned values and environment variables for container-owned values. Environment variables override YAML, which overrides Owner settings. Secret `_FILE` forms keep supported secrets out of environment text.

## Is Kinosail open source?

Kinosail Server is source-available under the PolyForm Perimeter License 1.0.1. It is not an Open Source Initiative-approved open-source license. Read the repository's [licensing terms](https://github.com/Kinosail/kinosail/blob/main/apps/player/LICENSING.md) before redistributing, rebranding, reselling, hosting, or providing a competing product.

## How do I report a security problem?

Do not publish exploit details, credentials, tokens, private paths, or media information in an issue. Use GitHub's private **Security → Advisories → Report a vulnerability** flow when available. See the [security policy](https://github.com/Kinosail/kinosail/blob/main/apps/player/SECURITY.md).

## Where is the API contract?

Open `/api/v1/openapi.json` on the running Server. It is the live OpenAPI 3.1 document for versioned API routes. See the [API reference]({{ '/reference/api/' | relative_url }}).
