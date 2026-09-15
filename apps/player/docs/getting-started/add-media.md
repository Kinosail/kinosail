---
title: Add your media
description: Organize folders, mount Library Content read-only, and run the first scan.
section: Start here
---

# Add your media

Kinosail reads media from the configured media mount. Add folders inside that mount in **Settings → Library folders**, then run a scan and check the result in Library.

## Prepare the media path

Create one or more folders on the host. Use clear folder names, such as `Movies`, `Shows`, `Music`, `Books`, or `Photos`. The host path must exist before you run the installer or start Compose.

The release Compose file mounts `${KINOSAIL_MEDIA_PATH}` at `/media:ro`. Kinosail can read Library Content but cannot change, rename, or delete files through this mount. Copy and organize files on the host, then wait for the copy to finish before scanning.

## Add a Library folder

1. Sign in as an Owner.
2. Open **Settings**.
3. Find **Library folders** under **Library**.
4. Confirm the **Media mount** path.
5. Enter a folder relative to the media mount in **Folder inside media mount**. For example, enter `Movies`, not `/media/Movies`.
6. Select **Add folder**.

Kinosail accepts an existing folder inside the media mount. It rejects absolute paths, missing folders, symlink-resolved paths outside the mount, and overlapping folders. Adding `.` means the complete media mount. Do not add both `.` and `Movies`, or add a parent and child folder together.

{% include screenshot.html title="Library folders" alt="Future screenshot of Settings showing the media mount, existing folders, and the Folder inside media mount field." description="Show a safe local path such as Movies. Do not show a real host username or private folder name." %}

## Run the first scan

In **Settings → Library discovery**, select **Run library scan now**. Kinosail reports the number of discovered items and the last scan time. New media is detected automatically after copying finishes. Safety scans repair missed file-system events on Windows, macOS, Linux, containers, and network mounts.

For ongoing checks, choose **Safety scan**:

- **Environment default**;
- **Off**;
- **Every 5 minutes**;
- **Every 15 minutes**; or
- **Hourly**.

Select **Save schedule**. Kinosail can also watch for changes. The page shows **Watching for changes** or **Polling for changes**.

If a metadata provider is configured, select **Refresh missing metadata** after the scan. Metadata enrichment is optional. Local NFO files, embedded tags, and folder artwork remain local inputs.

## Check the result

Open **Library** and verify one item from each folder. Check the title, media type, artwork, year, seasons or tracks, and playback action. Use a small file first so you can isolate path and permission errors.

If the item is missing:

1. Confirm the host file exists inside the configured host media path.
2. Confirm the folder is listed relative to `/media`.
3. Confirm the container user can read the host path.
4. Run **Run library scan now** again.
5. Open **Settings → System** and inspect the scan status.

Do not make the mount writable to solve a scan problem. Kinosail requires a read-only Library Content boundary.

## Remove or reorganize a folder

In **Settings → Library folders**, select **Remove** beside the folder. Removing a configured root stops future discovery from that root. It does not delete host files. Move files on the host, update the folder list, and run a new scan.

When a value is managed by YAML or the environment, the page marks it **Read-only**. Change the external value and restart Kinosail instead.

## Source of truth

Sources: `internal/server/settings_library.go`, `internal/server/settings_http.go`, `internal/server/library.go`, `compose.release.yaml`, and `README.md`.
