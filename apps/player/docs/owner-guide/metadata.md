---
title: Edit metadata and skip markers
description: Correct a title and control where intros, recaps, commercials, outros, and credits are skipped.
section: Own the Server
last_reviewed: 2026-09-20
---

# Edit metadata and skip markers

Sign in as an Owner and open the affected media item. Viewer Profiles do not have these management controls.

## Correct an individual title

1. Open **Manage media → Edit metadata**.
2. Correct the title, year, rating, tagline, genres, or plot.
3. Select **Save metadata**, then check the displayed title and details.

Use **Refresh from TMDB** when offered to request fresh provider metadata. TMDB requires Owner configuration and an eligible item; a refresh is not guaranteed to identify an ambiguous filename correctly. Review the result afterward. For library-wide refreshes, local NFO files, artwork, and scanning behavior, see [Manage libraries]({{ '/owner-guide/libraries/' | relative_url }}).

## Add or correct a skip marker

For a video, open **Manage media → Edit skip markers**. Choose **Add skip marker**, select **Intro**, **Recap**, **Commercial**, **Outro**, or **Credits**, and enter the start and end in seconds. For example, a segment from 1:30 to 2:05 uses start `90` and end `125`.

Select **Save marker**. Play just before the segment to verify that the skip action lands at the intended point. The end must follow the start; keep both positions within the actual video. Use the existing marker's form to correct its range, or **Remove … marker** to remove it.

Automatic segment analysis and manual markers are separate ways to find ranges. Check the result before relying on automatic skipping, especially around credits that may contain an extra scene. [Configure playback]({{ '/owner-guide/playback/' | relative_url }}) explains Server playback and skip behavior.

If changes do not appear, reload the item and confirm you edited the correct library entry. See [Scanning and metadata troubleshooting]({{ '/troubleshooting/scanning-and-metadata/' | relative_url }}).

Source of truth: `internal/server/player.go` and the metadata/marker handlers.
