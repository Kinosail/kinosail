---
title: Music, audiobooks, books, and photos
description: Listen to albums and audiobooks, read books and comics, and view photos in the web Player.
section: Use Kinosail
last_reviewed: 2026-09-20
---

# Music, audiobooks, books, and photos

Choose a media destination from the navigation. Your Owner decides which libraries your Profile can see; an empty destination can mean no matching media has been scanned or your Profile cannot access it.

## Music and albums

Open **Music**, choose an album or song, and start a track. Album and audio queues follow the available library order. Use the queue to choose another track; use [playlists]({{ '/user-guide/lists-and-collections/' | relative_url }}) when you want to curate your own order. Browser and operating-system media controls may remain available while audio plays.

## Audiobooks

Open **Audiobooks**, choose a title, and press play. Use **Playback speed** for 0.75× through 2× playback. Set **Sleep timer** to 15, 30, 45, or 60 minutes; choose **Off** to cancel it. The status beside the timer shows its current state. Use **Chapters** when the file supplies chapter information.

Playback progress belongs to your Profile. To listen without a connection, complete the [offline download workflow]({{ '/user-guide/offline/' | relative_url }}) in the browser you will take with you.

## Books and comics

Open **Books**, select a title, then choose **Read now**.

| Format | Browser reading experience |
| --- | --- |
| EPUB | Choose a chapter in **Reading order**. Selecting a chapter saves it for your Profile; reopening the book returns to that chapter. |
| PDF | The document opens in the browser's PDF viewer. Available page, zoom, and download controls depend on the browser. |
| CB7, CBT, CBZ | Comic archive images appear in reading order; scroll through the pages. |

The web EPUB reader saves chapter selection. Do not rely on exact scroll-position restoration for every format. An unsupported or empty archive cannot be opened by the reader. If **Download** is available for your Profile, you can use a compatible local reader instead.

If a book is missing, check its format and library access, then ask the Owner to [check the scan]({{ '/troubleshooting/scanning-and-metadata/' | relative_url }}).

## Photos

Open **Photos** and select an image to view it. Use **My List** to keep selected images easy to find. Photo rendering depends on the browser's support for the original format; see [media compatibility]({{ '/reference/media-compatibility/' | relative_url }}).

Source of truth: `internal/server/reader.go`, `internal/server/player.go`, and `packages/catalog/detail_http.go`.
