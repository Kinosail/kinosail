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

## Remove hearing-impaired descriptions

A subtitle for deaf or hard-of-hearing viewers describes sounds and names speakers. You can remove those descriptions from one subtitle.

1. Open the video from the dashboard and select the subtitle language.
2. Open **Timing anchors and cleanup** and select **Remove hearing-impaired descriptions**.
3. Select **Preview changes**. The comparison shows each changed and removed cue.
4. Check the preview, then select **Save reviewed subtitle**.

Kinosail removes text in square brackets and parentheses, such as `[door slams]` and `(sighs)`. It also removes a speaker name in capital letters before a colon, such as `JOHN:`, and lines that contain only music notes. A cue with no other text is removed.

Ordinary dialogue in parentheses is also removed. Check the preview before you save. The option is off by default. To return to the previous file, select **Undo last replacement**.
