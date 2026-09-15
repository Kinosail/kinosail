---
title: Manage library folders
description: Control which media directories subtitle automation may update.
section: Own the Server
last_reviewed: 2026-09-15
---

# Manage library folders

Choose folders below the media mount in setup or Settings. Use relative folder names, such as `Movies` and `Shows`, rather than host paths. `.` selects the whole mount. Deployment-managed lists are read-only until changed at their source.

Grant the container account only the filesystem access it needs. Subtitles writes next to videos, so read-only mounts are unsuitable. Inspect host/NAS permissions and container file sharing if a scan works but a fetch cannot save.

Scan after folder changes. Removing a folder from the app's scope is different from deleting its files; make any filesystem cleanup separately and deliberately. Keep backups of original sidecars and media.

See [media layout]({{ '/getting-started/add-media/' | relative_url }}) and [scanning troubleshooting]({{ '/troubleshooting/scanning-and-metadata/' | relative_url }}).
