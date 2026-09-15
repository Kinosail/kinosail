---
title: Scanning and metadata problems
description: Diagnose missing media, wrong matches, artwork, permissions, and external metadata failures.
section: Fix a problem
---

# Scanning and metadata problems

Use this page when media is missing, grouped incorrectly, matched to the wrong title, or missing artwork or metadata.

## A new file does not appear

1. Confirm that the file is below a configured Library root.
2. Confirm that the host can read the file and that it is not a symbolic link.
3. Confirm that the extension is supported.
4. In **Settings → System**, check **Library discovery** and choose **Run library scan now**.
5. Refresh the library page.

Kinosail watches for filesystem changes when the platform provides events. It also runs a safety scan according to its schedule. The schedule can be **Environment default**, **Off**, **Every 5 minutes**, **Every 15 minutes**, or **Hourly**. An Owner can change it unless deployment configuration manages the setting.

Supported media includes common video extensions such as `.mkv`, `.mp4`, `.mov`, `.webm`, and `.ts`; audio such as `.flac`, `.m4a`, `.mp3`, `.ogg`, and `.wav`; `.m4b` audiobooks; photos such as `.avif`, `.gif`, `.jpeg`, `.jpg`, `.png`, and `.webp`; and books such as `.cb7`, `.cbt`, `.cbz`, `.epub`, and `.pdf`. Other extensions are ignored. A supported extension does not guarantee direct playback on every device.

The scanner does not follow symbolic links. Mount the real directory into the container and point the Library root at that path.

## The scan status shows an error

Open **Settings → System** and record the scan status and time. Run one scan after correcting the reported path or permission. If the error repeats, inspect a short log excerpt:

```sh
podman compose logs --tail 100 kinosail
```

Use `docker compose logs --tail 100 kinosail` for Docker. Do not publish a full log without reviewing paths, names, and integration details.

## A Show or season is grouped incorrectly

Use stable folder and file names. Kinosail derives Show and episode identity from the scanned path and metadata. Keep episodes for one Show under a common Show folder. Avoid two unrelated Shows with the same title and year in one ambiguous folder.

If local metadata contains a provider identifier, use the supported NFO `uniqueid` types `tmdb`, `tvdb`, or `imdb`. Values for other types are ignored. After changing the file, run **Run library scan now**.

## The title or year is wrong

Place a valid NFO sidecar beside the media file, or use the Owner **Manage media → Edit metadata** form on the item. An NFO can provide title, sort title, year, plot, rating, content rating, tagline, genres, director, studio, and music tags.

Owner metadata edits are persisted by Kinosail. Do not edit Library Content from inside the container. If TMDB enrichment is enabled, an Owner can choose **Refresh from TMDB** when the action is available. Review the match before keeping it.

## Artwork is missing or appears on the wrong item

Keep artwork beside the media with a supported image extension. Recognized names include a media basename image and common names such as `poster`, `fanart`, `backdrop`, `thumb`, `folder`, `clearlogo`, and `logo`. A sidecar artwork file is not indexed as a separate photo.

After copying artwork, run a library scan. If the artwork is still stale, check whether the item has an Owner-managed artwork or metadata record and use the item’s management controls.

## Subtitles are missing

Kinosail indexes `.srt` and `.vtt` sidecar subtitles when their basename matches the media. For example, `Film.en.srt` belongs beside `Film.mp4`. An orphan subtitle without matching media is ignored.

If an Owner configured a subtitle provider, use **Find** in **Playback & downloads**. The provider may be unavailable or return no match. Kinosail accepts only trusted download links and stores the resulting subtitle in its cache.

## Metadata refresh does not work

Check that the metadata provider is configured and that its credential is valid. Optional provider requests leave the Server. A provider outage does not prevent local NFO and embedded metadata from being used.

If a YAML or environment setting controls the provider, edit that deployment source and restart Kinosail. If the provider is configured in Owner Settings, review its status there. Never include a provider token in logs or reports.

Source of truth: `packages/library`, library settings, and metadata handlers.
