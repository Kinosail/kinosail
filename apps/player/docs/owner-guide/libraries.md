---
title: Manage libraries
description: Configure Library folders, scanning, metadata, artwork, subtitles, and playback markers.
section: Own the Server
---

# Manage libraries

Manage library roots from **Settings → Library folders**. Kinosail reads each root from the read-only media mount and indexes supported media into the local catalog.

## Add or remove a root

1. Open **Settings** as an Owner.
2. In **Library folders**, confirm **Media mount**.
3. Enter an existing folder in **Folder inside media mount**. Use a relative path such as `Movies`.
4. Select **Add folder**.

Kinosail rejects absolute paths, missing folders, symlink-resolved paths outside the media mount, and overlapping roots. A root of `.` covers the whole media mount. Do not add a parent and child root together.

Select **Remove** beside a root to stop discovery from that root. This does not delete files. Move or delete host files with host tools, never through Kinosail.


## Run and schedule discovery

Open **Settings → Library discovery**. Select **Run library scan now** after you copy media. Kinosail also watches for changes where the host permits it.

The page reports **Watching for changes** or **Polling for changes**. Choose a **Safety scan** schedule and select **Save schedule**:

- **Environment default**;
- **Off**;
- **Every 5 minutes**;
- **Every 15 minutes**; or
- **Hourly**.

Safety scans repair missed file-system events on network mounts, containers, and supported desktop hosts. Wait for a copy to finish before you scan. A scan can run in the background while you browse.

## Refresh metadata and markers

Kinosail uses local NFO files, embedded tags, folder artwork, and media facts first. If TMDB is configured, select **Refresh missing metadata** in **Library discovery**. TMDB requests leave the Server and follow the provider's terms and attribution requirements.

Open **Settings → Playback segment analysis** and select **Analyze playback segments now** to detect intro, recap, commercial, outro, and credits markers. Enable the matching **Skip … automatically** choices in **Settings → Playback** only after you review the results.

## Diagnose an item that is missing or wrong

1. Confirm the file exists under the host media path.
2. Confirm its folder is a configured relative root.
3. Confirm the container user can read it.
4. Select **Run library scan now**.
5. Review the count and last scan in **Settings → System**.
6. If metadata is wrong, check local names and tags before enabling external enrichment.

Do not make `/media` writable. Do not add a second database or a sidecar scanner. Kinosail's one-container boundary keeps the web interface, API, background work, SQLite state, and media tools together.

## Source of truth

Sources: `internal/server/settings_library.go`, `internal/server/settings_http.go`, `internal/server/library.go`, `packages/library/scan.go`, and `README.md`.
