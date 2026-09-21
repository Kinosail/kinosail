---
title: Import and sync viewing history
description: Preview Plex or Jellyfin history and optionally keep importing it on a schedule.
section: Own the Server
last_reviewed: 2026-09-20
---

# Import and sync viewing history

Move watched state and resume positions from Plex or Jellyfin into one Kinosail Viewer Profile at a time. Jellyfin favorites become **My List** entries; supported video playlists can also be imported. Plex Universal Watchlist is not available through the local Server interface and is not imported.

## Preview before importing

1. Scan your media into Kinosail and create the destination Viewer Profile first.
2. As an Owner, open **Settings → Migration**. You can also import during first setup.
3. Select Plex or Jellyfin, enter the source Server URL and access token, and supply the source user ID where needed.
4. Select the destination Viewer Profile. Keep **Replace existing Kinosail watched and resume state** unchecked to preserve existing activity.
5. Select **Preview import**. Review matched, unchanged, conflicting, ambiguous, and unmatched items before applying anything.
6. Choose **Import once** for a one-time move.

Preview does not change your library. Resolve ambiguous metadata matches before trying again; unmatched titles cannot receive activity. An expired preview must be generated again. Source passwords, PINs, permissions, and account identities do not transfer.

## Keep importing automatically

From the preview, choose an interval of 15 minutes, one hour, six hours, or one day, then select **Import and sync automatically**. Kinosail retains the source credentials required to run this connection. This imports from the source into Kinosail; it is not a two-way synchronization promise.

The overwrite choice applies to the initial preview and import. Later scheduled pulls reconcile changed source viewing activity and report conflicts rather than continually replacing all Kinosail state. Favorites and playlists are imported initially; recurring runs synchronize watched state and resume progress only.

Return to Migration to inspect the connection and its latest result. Use **Sync now** to run it immediately. If a run fails, check source reachability, token validity, source-user access, and whether the media still matches. Do not paste source tokens into issue reports.

## Stop a recurring import

Choose **Remove** for the connection when you no longer need it. This removes the recurring connection and its retained source credentials; it does not undo activity already imported. Revoke the source token at Plex or Jellyfin as well if it is no longer needed elsewhere.

Repeat the workflow separately for each person. See [Profiles]({{ '/user-guide/profiles/' | relative_url }}) and [backups]({{ '/owner-guide/backups-and-updates/' | relative_url }}) before replacing a household's existing state.

Source of truth: `internal/server/viewing_import_http.go`, the viewing-sync manager, and `internal/server/settings_http.go`.
