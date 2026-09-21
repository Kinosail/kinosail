---
title: Download and play offline
description: Prepare media, store it in this browser, resume transfers, and verify offline playback.
section: Use Kinosail
last_reviewed: 2026-09-20
---

# Download and play offline

Offline playback has two steps: Player prepares a file on the Server, then your browser saves and verifies its own copy. **Ready to download** means the first step has finished; it does not mean the media is on your device.

## Save a copy for a trip

1. While connected to your Server, open a movie, episode, song, or audiobook.
2. Open **Playback & downloads** and choose **Prepare** for the offered offline quality. Your Profile must allow downloads; preparation may also require transcoding permission and available Server capacity.
3. Choose **Manage downloads**. Wait for **Ready to download**. If preparation fails, read the error and use **Try again** after resolving it.
4. Select **Download to this device**. Keep this page open and allow enough free storage for the displayed file size.
5. Wait for **Saved and verified. Play to check compatibility.**, then select **Play offline**. Verification of the stored file and successful decoding in this browser are separate checks.
6. Open **Downloads on this device**, disconnect from the network, and try the saved title before leaving. Bookmark the Server's `/offline` page in this same browser.

Select the intended Kinosail Profile while still online, and stay signed in before leaving. Signing out clears the active offline identity; reconnect and choose the Profile again before using its saved downloads.

Use the same browser, browser profile, and Server address for the offline copy. A download stored in one browser is not automatically available in another browser or on another device.

## Resume an interrupted transfer

Reconnect to the Server, return to **Manage downloads**, and choose **Resume on this device**. Player retains resumable data when possible and checks it before continuing. If the prepared file no longer exists on the Server, prepare it again.

**This browser cannot store offline media safely** means required browser storage or locking capabilities are unavailable. **Offline playback needs an active service worker** means the offline worker is not controlling the page; use a trusted HTTPS connection, reload while online, and check the browser's site-storage permissions. Private browsing and restricted storage can prevent offline use.

## Original files and browser copies

**Download original** saves the source file through your browser's normal download handling. **Save file** on a prepared job saves the prepared file. These files are different from **Download to this device**, which creates a copy managed by Player's offline library. Use an appropriate local media player for files saved outside that library.

Original formats may not play in your browser. See [media compatibility]({{ '/reference/media-compatibility/' | relative_url }}) and [playback troubleshooting]({{ '/troubleshooting/playback/' | relative_url }}).

## Remove downloads and reclaim storage

Use **Remove download** on the download you no longer need. The online management page removes that prepared job and attempts to remove its local browser data; if it reports a removal failure, retry before assuming the space was reclaimed. Copies on other devices are separate.

Clearing browser site data removes local downloads. Browsers may also evict stored data under storage pressure. Offline copies are convenient travel copies, not backups of your media.

Source of truth: `internal/server/downloads_http.go`, `packages/webassets/static/downloads-ui.js`, and the shared offline storage/runtime scripts.
