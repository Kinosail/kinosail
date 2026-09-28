---
title: Configure languages and automation
description: Choose subtitle languages, playback choices, optional cleanup, and scanning behavior.
section: Own the Server
last_reviewed: 2026-09-27
---

# Configure languages and automation

In **Settings → Languages**, choose up to 20 preferred languages and put them in priority order. The first is the primary language. Choose a subtitle role: standard dialogue or SDH/captions. Use a catalog language tag such as `en` or `pt-br`. `KINOSAIL_SUBTITLE_LANGUAGE` sets the deployment-managed primary language.

The dashboard checks coverage in every selected language. Existing embedded text is considered before a provider request. A language-tagged sidecar covers that language; adding a preference does not translate an existing file.

## Choose playback tracks

**Playback subtitle choices** defaults to **Show every available track**. Select **Show preferred languages only** to show one regular track per selected language in the subtitle menu. **Off** remains available. If cleanup is confirmed with **Keep** for forced files, forced tracks remain in the limited menu only for selected languages. This setting hides choices; it does not rename or delete files.

## Remove unwanted subtitle files

Cleanup is optional and off by default. Back up your media and sidecars before using it. In **Settings → Cleanup**, enable cleanup, select at least one language to keep, and choose whether to keep or delete forced subtitle files in every language. Select **Preview files to delete** and review the count and file paths. The preview shows at most 100 paths; check the media tree when more files match. Confirm only when the selection is correct.

Cleanup can delete language-tagged `.srt` and `.vtt` sidecars beside scanned videos. Embedded tracks and files with uncertain language stay in place. Confirmation also saves the selected language order and limits playback choices. If files change after the preview, preview again. Deleted files are not placed in a recovery copy by cleanup.

To reduce a crowded subtitle menu without deleting files, use **Playback subtitle choices** alone.

## Schedule scans

Choose a safety scan schedule in setup. Filesystem events help detect completed copies, and scheduled scans catch changes they miss. Background provider maintenance runs in bounded cycles with a minimum 15-minute interval. Provider quotas and failures can leave items wanted for a later cycle.

Use a one-item fetch to validate your plan before relying on maintenance. See [subtitle management]({{ '/user-guide/' | relative_url }}) for upgrade and preservation rules.
