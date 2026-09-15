---
title: Find and manage subtitles
description: Read coverage and fetch or upgrade subtitle sidecars.
section: Use Subtitles
last_reviewed: 2026-09-15
---

# Find and manage subtitles

The dashboard shows local coverage for your preferred language, wanted items, and scanned videos. Start with [first setup]({{ '/getting-started/first-setup/' | relative_url }}) if no library or provider is configured.

## Fetch one subtitle

1. Locate a movie or episode in the library/wanted view.
2. Review the video identity and preferred language.
3. Fetch a subtitle and read the operation result.
4. Check the saved sidecar beside the video and verify it in your player.

A wanted batch from the dashboard handles up to ten items. The HTTP API accepts bounded batches up to 50. A batch can have mixed outcomes; retry only the items that still need work after resolving the cause.

## Automatic maintenance

After scans, maintenance fills missing subtitles and considers safer upgrades, with a minimum of 15 minutes between bounded automatic cycles. Existing embedded text and sidecars are considered before a provider search. Provider requests still depend on account quotas and network availability.

Managed sidecars require a score improvement of at least ten points for the conservative automatic upgrade. Unknown sidecars require an exact OpenSubtitles file-hash match; the replaced original is kept as a `.kinosail.bak` recovery copy. Keep that original until the replacement is checked. SubSource sidecars are preserved unchanged under its terms.

## Review uncertain results

Check language, episode, release, and timing when a subtitle is wrong. Optional OCR/transcription creates drafts for review and does not automatically replace sidecars. Review changes before accepting them. See [matching and timing]({{ '/user-guide/playback/' | relative_url }}) and [recovery]({{ '/owner-guide/backups-and-updates/' | relative_url }}).

## Keep or restore a sidecar

Use the item's replacement control to freeze a subtitle that should be kept. When a preserved original is available, use restore for the intended language and check the file again in your player. Review the inspector's source, match evidence, and warnings before previewing or applying edits. The API exposes the same [replacement, restore, and review operations]({{ '/reference/api/' | relative_url }}).
