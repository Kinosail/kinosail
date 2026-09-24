---
title: Move from Plex or Jellyfin to Kinosail
description: Reuse your media folders and import watched state and resume positions from Plex or Jellyfin, with a preview before anything changes.
section: Own the Server
last_reviewed: 2026-09-24
---

# Move from Plex or Jellyfin to Kinosail

You can try Kinosail with the media files you already use. [Install Kinosail]({{ '/quickstart/' | relative_url }}), point it at your existing media folder, and [scan the library]({{ '/getting-started/add-media/' | relative_url }}). Kinosail reads the original files through a read-only mount; it does not move or rewrite them.

After the scan, import watched state and resume positions from Plex or Jellyfin into one Kinosail Viewer Profile at a time. Preview the matches before applying them. Your source Server remains separate, so you can check playback in Kinosail before relying on it.

## What can move?

| From Plex or Jellyfin | In Kinosail |
| --- | --- |
| Existing media files | Scan the same folder; no media-file transfer is required. |
| Watched state and resume positions | Import for matched items into the selected Viewer Profile. |
| Jellyfin favorites | Import as **My List** entries. |
| Supported video playlists | Import during the initial move. |
| Source accounts, passwords, PINs, and permissions | Create and secure Kinosail Profiles separately; these do not transfer. |
| Plex Universal Watchlist | Not imported; the local Plex Server interface does not provide it. |

The import matches activity to media that Kinosail has already scanned. An unmatched title cannot receive its viewing activity. [Media compatibility]({{ '/reference/media-compatibility/' | relative_url }}) explains why scanning a file does not guarantee direct playback on every device.

## Preview before importing

1. Scan your media into Kinosail and create the destination Viewer Profile first.
2. As an Owner, open **Settings → Migration**. You can also import during first setup.
3. Select Plex or Jellyfin, enter the source Server URL and access token, and supply the source user ID where needed.
4. Select the destination Viewer Profile. Keep **Replace existing Kinosail watched and resume state** unchecked to preserve existing activity.
5. Select **Preview import**. Review matched, unchanged, conflicting, ambiguous, and unmatched items before applying anything.
6. Choose **Import once** for a one-time move.

Preview does not change your library. Resolve ambiguous metadata matches before trying again. An expired preview must be generated again.

## Keep importing automatically

From the preview, choose an interval of 15 minutes, one hour, six hours, or one day, then select **Import and sync automatically**. Kinosail retains the source credentials required to run this connection. This imports from the source into Kinosail; it is not a two-way synchronization promise.

The overwrite choice applies to the initial preview and import. Later scheduled pulls reconcile changed source viewing activity and report conflicts rather than continually replacing all Kinosail state. Favorites and playlists are imported initially; recurring runs synchronize watched state and resume progress only.

Return to Migration to inspect the connection and its latest result. Use **Sync now** to run it immediately. If a run fails, check source reachability, token validity, source-user access, and whether the media still matches. Do not paste source tokens into issue reports.

## Stop a recurring import

Choose **Remove** for the connection when you no longer need it. This removes the recurring connection and its retained source credentials; it does not undo activity already imported. Revoke the source token at Plex or Jellyfin as well if it is no longer needed elsewhere.

Repeat the workflow separately for each person. See [Profiles]({{ '/user-guide/profiles/' | relative_url }}) and [backups]({{ '/owner-guide/backups-and-updates/' | relative_url }}) before replacing a household's existing state.

Before retiring the old Server, compare a few matched titles, resume positions, and playlists, then play media on the devices your household uses. Client playback depends on the device, codec, and [playback policy]({{ '/owner-guide/playback/' | relative_url }}).

Source of truth: `internal/server/viewing_import_http.go`, the viewing-sync manager, and `internal/server/settings_http.go`.
