---
title: Browse coverage and wanted items
description: Find a video and understand its local subtitle coverage.
section: Use Subtitles
last_reviewed: 2026-10-30
---

# Browse coverage and wanted items

Use the subtitle dashboard to inspect preferred-language coverage, wanted items, and the scanned library. Search by a recognizable title and verify the episode/release before starting a fetch.

A missing item usually means the media mount, selected relative folder, or scan needs attention. A covered item may use an existing language sidecar or embedded text track. Changing the preferred language changes the coverage question; it does not translate existing files.

## What needs attention

The Overview shows what is happening now and what needs you:

- **Working now** lists the automatic check and any subtitle task that is running, such as **Finding a subtitle: Arrival**.
- **Timing not checked** counts installed subtitles whose timing Kinosail could not check against the audio. Select it to see the files. Open a file and select **Check timing** to check it in the subtitle editor. Select **Keep this version** to hide a file from this list.
- **Last automatic check** shows the result of the last automatic check and when the next one runs.

The same counts are in `/api/v1/subtitle-library` as `timingUnchecked` and `activity`.

Run a scan after changing folders or copying media. Wait for copying to finish. If the item still does not appear, follow [scanning troubleshooting]({{ '/troubleshooting/scanning-and-metadata/' | relative_url }}).

Continue with [fetching and maintenance]({{ '/user-guide/' | relative_url }}).
