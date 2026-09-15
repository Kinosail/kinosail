---
title: Every title, covered.
description: Install, configure, and operate Kinosail Subtitles.
hide_contribute: true
---

# Every title, covered.

Kinosail Subtitles is the subtitle automation Server in the Kinosail family. It scans movies and episodes you control, shows what is missing in your preferred language, and writes validated sidecar files beside the source video.

<div class="path-grid">
  <a href="{{ '/getting-started/' | relative_url }}"><strong>Set up a new Server</strong><span>Install one container, create the first Owner, and connect writable media folders.</span></a>
  <a href="{{ '/owner-guide/libraries/' | relative_url }}"><strong>Manage media folders</strong><span>Choose which local movie and show folders Kinosail may scan and update.</span></a>
  <a href="{{ '/reference/configuration/' | relative_url }}"><strong>Configure subtitle search</strong><span>Set the preferred language, provider credentials, scan frequency, and deployment paths.</span></a>
  <a href="{{ '/reference/api/' | relative_url }}"><strong>Automate with the API</strong><span>Read coverage, fetch one missing subtitle, or run a bounded wanted search.</span></a>
  <a href="{{ '/reference/architecture-and-privacy/' | relative_url }}"><strong>Understand privacy</strong><span>See exactly what stays local and what an enabled provider receives.</span></a>
  <a href="{{ '/troubleshooting/' | relative_url }}"><strong>Fix a problem</strong><span>Start from a visible symptom and follow focused checks.</span></a>
</div>

{% include screenshot.html title="Kinosail subtitle overview" alt="Future screenshot of the Kinosail Subtitles coverage dashboard." description="Add this image after the subtitle command center visual design is final." %}

## The operating model

- One Kinosail Subtitles container includes the web interface, versioned API, scanner, embedded state, and provider adapter.
- The media mount is writable because sidecar subtitles belong beside the source video.
- A provider is optional. Without credentials, the Server remains a useful local coverage ledger.
- Provider responses and downloads are untrusted until bounded and validated.
- A fetch never replaces an existing preferred-language sidecar.
- Local use needs no Kinosail-hosted account or media relay.

## About the inherited Kinosail material

This repository began from Kinosail Player so it deliberately retains the same security, deployment, API, testing, and design framework. Some deeper Player-oriented reference pages remain as engineering background while the subtitle-specific guide expands. The dashboard, setup copy, settings entrypoint, API additions, and deployment defaults documented here are the authoritative Kinosail Subtitles surface.

## Before you begin

You need a supported 64-bit Linux host, Docker Compose or Podman Compose, and an absolute path to movie and show folders you control. The container account must be allowed to create subtitle files in those folders.
