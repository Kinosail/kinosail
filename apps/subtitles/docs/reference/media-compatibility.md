---
title: Subtitle file compatibility
description: Understand sidecars, embedded text, and matching limits.
section: Reference
last_reviewed: 2026-09-15
---

# Subtitle file compatibility

Subtitles scans movie and episode videos and recognizes untagged or language-tagged SRT and WebVTT sidecars. It considers embedded text before requesting provider downloads. When a selected language has only image subtitles, such as PGS or VobSub, automatic maintenance reads them with Tesseract OCR and saves a clean SRT sidecar. The history shows these files as read from image subtitles. OCR text can contain recognition errors. A provider subtitle with a much better match score can later replace it under the normal upgrade rules.

The container includes OCR data for 19 common languages. For about 90 more languages, Kinosail downloads the matching Debian `tesseract-ocr-<language>` package from `deb.debian.org` the first time a file needs it. It installs the data only when the package size and SHA-256 match values pinned from the signed Debian 13 package index. A failed download is retried after one hour. Speech-based drafts still need review.

Use matching basenames: `Film.mkv`, `Film.en.srt`, or `Film.pt-br.vtt`. An exact untagged sidecar can count as a default subtitle; a language-tagged file covers that language. A subtitle must also fit the episode and release timing.

Provider coverage and your player's support are separate. Verify a saved sidecar in the player that will display it. Kinosail Player has its own media/playback compatibility reference.

See [matching and timing]({{ '/user-guide/playback/' | relative_url }}).
