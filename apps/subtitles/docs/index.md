---
title: Kinosail Subtitles
description: Install and operate subtitle automation for your movies and episodes.
section: Start here
last_reviewed: 2026-09-15
---

# Kinosail Subtitles

Kinosail Subtitles scans local movies and episodes, finds missing subtitles, and writes validated sidecar files beside the videos. It runs independently from Kinosail Player.

- [Install and secure the Server]({{ '/getting-started/' | relative_url }})
- [Complete first setup]({{ '/getting-started/first-setup/' | relative_url }})
- [Connect a subtitle provider]({{ '/owner-guide/integrations/' | relative_url }})
- [Find and manage subtitles]({{ '/user-guide/' | relative_url }})
- [Back up and update]({{ '/owner-guide/backups-and-updates/' | relative_url }})
- [Use the HTTP API]({{ '/reference/api/' | relative_url }})
- [Fix a problem]({{ '/troubleshooting/' | relative_url }})

Media stays on your Server. Providers receive search metadata, such as title, episode, language, release identifiers, and sometimes a locally computed file hash; they do not receive your media bytes. Existing local sidecars and embedded text tracks are considered before provider downloads.

Use a writable media mount for Subtitles and a read-only mount for Player. Keep separate app state and backups. Subtitles can safely upgrade managed files and preserves eligible unknown originals as `.kinosail.bak` recovery copies. SubSource downloads stay unchanged under its terms.
