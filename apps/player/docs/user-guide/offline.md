---
title: Download and play offline
description: Download titles in the Kinosail app for iPhone and iPad, and know what the web app does without a connection.
section: Use Kinosail
last_reviewed: 2026-10-24
---

# Download and play offline

Use the Kinosail app on iPhone or iPad to watch without a connection. The web app streams from your Server and does not save media in the browser.

## Download a title on iPhone or iPad

1. While connected to your Server, open a movie, episode, show, or season in the Kinosail app.
2. Select **Download**. Your Viewer Profile must allow downloads. Converted qualities can also need transcoding permission and free Server capacity.
3. Open **Downloads** to follow progress. The app continues in the background and resumes after a dropped connection.
4. Wait for **Downloaded**, then play the title once before you travel.

If a download shows **Download needs attention**, reconnect to your Server and try it again. If it fails again, see [playback troubleshooting]({{ '/troubleshooting/playback/' | relative_url }}).

## Remove downloads

Select **Remove download** on a title, or **Remove downloads** in **Downloads** to clear them all. Downloads are travel copies, not backups of your media.

## Use the web app without a connection

The web app keeps its own interface files so it can open while offline. Without a connection it shows **You’re offline**. Reconnect to your Server and select **Try again** to continue watching.

On a book page, **Download** saves the book file through your browser's normal download handling when your Profile allows downloads.

Earlier versions of the web app could save titles in the browser. The current web app removes that browser storage the first time it opens.

Source of truth: `internal/server/downloads_http.go`, `internal/server/static/service-worker.js`, and the native app's download library.
