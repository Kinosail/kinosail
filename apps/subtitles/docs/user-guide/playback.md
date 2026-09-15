---
title: Check subtitle matching and timing
description: Confirm a subtitle fits the video before relying on it.
section: Use Subtitles
last_reviewed: 2026-09-15
---

# Check subtitle matching and timing

Subtitles prepares files for a player. Open the video in Kinosail Player or another player that supports sidecars to check language, episode, accessibility role, and timing near the start and later in the video.

Providers may carry subtitles for different edits of the same film or episode. Kinosail ranks stable IDs, release information, language, and available hashes; weak release matches can require local speech alignment. A title match alone does not prove timing compatibility.

If a result is wrong, retain the original `.kinosail.bak` recovery copy, inspect the matching information, and follow [subtitle troubleshooting]({{ '/troubleshooting/playback/' | relative_url }}). Do not repeatedly fetch the same mismatch without changing the cause.

SubSource files remain unchanged on disk. Optional locally generated drafts require review; they do not replace subtitles automatically.
