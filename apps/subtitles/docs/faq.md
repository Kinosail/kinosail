---
title: Frequently asked questions
description: Concise answers about Kinosail Subtitles, privacy, providers, files, and deployment.
section: Project
last_reviewed: 2026-08-30
---

# Frequently asked questions

## What is Kinosail Subtitles?

Kinosail Subtitles is a private, self-hosted subtitle manager for movies and episodes you control. It is the subtitle app in the Kinosail family and shares Kinosail Player's setup, security, visual language, container, API, and verification framework.

## Is this a replacement for Bazarr?

Kinosail now covers the core Bazarr workflow without Sonarr, Radarr, a database sidecar, or provider scrapers. It scans media directly, tracks wanted subtitles, searches configured free providers, ranks candidates, validates downloads, writes safe sidecars, synchronizes weak matches, upgrades managed files, and exposes the same operations through its versioned API.

## Does it require a hosted account or subscription?

No. Local use requires neither. An optional subtitle provider can have its own account or limits.

## What leaves my Server?

Nothing leaves for local inventory and coverage. When the Owner enables a provider and starts a search, the provider receives the title, requested language, media type, and episode identity needed for that search. Media bytes are not uploaded or relayed.

## Why is the media mount writable?

The app creates subtitle sidecars such as `Arrival.en.srt` beside the corresponding video. The target comes from an already scanned video path and a validated language code; callers cannot choose an arbitrary filesystem path.

## Will it overwrite my subtitles?

No. A preferred-language or default sidecar counts as covered, and final file creation fails if another file already owns the target name—even when two requests race.

## Which subtitle files count as coverage?

An untagged sidecar such as `Arrival.srt` counts as the default subtitle. A tagged sidecar such as `Arrival.en.srt` covers that language. SRT and WebVTT files found by the scanner are included.

## Which providers are supported?

Kinosail supports SubDL, OpenSubtitles.com, and SubSource. One provider is enough. It searches every configured provider and selects one result automatically. SubSource requires personal-use acceptance and its downloaded bytes stay unchanged.

## How many containers do I need?

One. The supported deployment includes the Go Server and embedded SQLite state without a database sidecar.

## Where are configuration and backups kept?

Application state uses `/config`, caches use `/cache`, writable media and subtitle files use `/media`, and backups use `/backups` by default. Environment variables override YAML, which overrides Owner settings.

## Where is the API contract?

Open `/api/v1/openapi.json` on the running Server. Subtitle inventory and fetch operations live under `/api/v1/subtitle-library`.

## Is Kinosail Subtitles open source?

Kinosail Subtitles is source-available under the PolyForm Perimeter License 1.0.1. Read the repository's licensing terms before redistribution, rebranding, resale, hosting, or competing use.
