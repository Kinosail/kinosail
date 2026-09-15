---
title: Lists and collections
description: Organize media with My List, playlists, queues, smart playlists, and collections.
section: Use Kinosail
---

# Lists and collections

Use organization tools to keep media for later, make a playback order, or build a shared curation.

## Choose the right tool

| Tool | Scope | Best use |
| --- | --- | --- |
| **My List** | Personal to your Profile | Save titles for later. |
| **Playlist** | Personal to your Profile | Keep an ordered set of playable media. |
| **Smart playlist** | Personal to your Profile | Build a changing result from a rule. |
| **Collection** | Owner-curated library view | Group titles into a shared theme or set. |
| **Queue** | Current playback session | Play selected audio or video items in sequence. |

## Use My List

Open a title and select **Add to My List**. Select the same action again to remove it. Open **My List** from the home page to see your saved items.

My List uses the current Profile. It does not grant access to an item that the Owner has not granted to that Profile.

## Create a playlist

1. Open **Playlists**.
2. Choose the control to create a playlist.
3. Enter a unique name.
4. Open a title and select **Add to playlist or collection**.
5. Choose the playlist.
6. Open the playlist to review its order and play it.

Playlist membership and order belong to your Profile. A playlist can contain visible items only. If an item later becomes unavailable, it will not be playable from the current Profile.

You can export a regular playlist from its page. The export is a Kinosail playlist document. Keep exports private if they contain private titles or identifiers. You can import a Kinosail playlist document from **Playlists**. Import accepts a bounded JSON document and rejects invalid or unknown content.

## Create a smart playlist

Open **Playlists**, expand **New smart playlist**, and provide a name, media kind, query, and sort choice. A smart playlist evaluates its rule against media that your Profile can view. Its contents can change when the library or your visibility changes.

## Curate a collection

Collections are Owner-managed. Open **Collections**, create a collection, search for a visible title, and choose **Add to**. Open a collection to remove an item or delete the collection. Deleting a collection does not delete Library Content.

Viewers can open collections that contain items they can view. A Viewer cannot add to, remove from, or delete a collection.


## Build a queue

Use the player’s queue or shuffle action when available. Queues are temporary playback order. A queue does not change playlist membership or My List.

If a list is empty, add an item from its detail page. If an item is missing from a smart playlist or collection, first confirm that your Profile can view its library and rating.

Source of truth: `internal/server/lists.go`, playlist, and collection handlers.
