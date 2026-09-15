---
title: Subtitle file compatibility
description: Understand sidecars, embedded text, and matching limits.
section: Reference
last_reviewed: 2026-09-15
---

# Subtitle file compatibility

Subtitles scans movie and episode videos and recognizes untagged or language-tagged SRT and WebVTT sidecars. It considers embedded text before requesting provider downloads. Bitmap subtitles and speech-based drafts require their specific optional processing paths and review; they are not equivalent to a ready text sidecar.

Use matching basenames: `Film.mkv`, `Film.en.srt`, or `Film.pt-br.vtt`. An exact untagged sidecar can count as a default subtitle; a language-tagged file covers that language. A subtitle must also fit the episode and release timing.

Provider coverage and your player's support are separate. Verify a saved sidecar in the player that will display it. Kinosail Player has its own media/playback compatibility reference.

See [matching and timing]({{ '/user-guide/playback/' | relative_url }}).
