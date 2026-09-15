---
title: Browse and search
description: Find media with shelves, library views, filters, title jumps, and search.
section: Use Kinosail
---

# Browse and search

Use the home page to open a media area, filter the result, and move through a large library.

## Open a media area

The home page shows destinations that contain visible items. Depending on the Server library, you can see:

- **Movies** for feature films.
- **Shows** for series and episodes.
- **Music** for albums and tracks.
- **Audiobooks** for long-form audio.
- **Books** for books and comics.
- **Photos** for photos.
- **My List**, **Collections**, and **Playlists** for organization.

The home page can also show **Continue Watching**, **Recently added**, and **Recently played**. These shelves use your Profile state. Another Profile does not change your progress or history.

## Search the visible library

Enter a word or phrase in the search field. Search can match a title, Show name, year, plot, genre, director, studio, artist, album, cast member, or cast role. Search ignores letter case, accents, and punctuation for matching.

Search results are ranked by exact title, title prefix, word match, and then other matches. Search does not expose items that your Profile cannot view.

To clear a search, use the clear control or remove the query. Search is separate from a letter jump and from sorting.


## Filter and sort

Use the library view to choose **All**, **My List**, **Unwatched**, **History**, **Movies**, **Shows**, **Collections**, **Playlists**, **Music**, **Audiobooks**, **Books**, or **Photos**. A view is available only when its data exists.

Use the sort control to choose **Title**, **Added**, or **Year**. Title is the default. **Added** shows newest additions first. **Year** shows newer years first. Search ranking remains the first ordering when a search is active.

When the view is sorted by title without a search, use the letter navigation to jump to a title group. A letter jump is not available with a search or with another sort order.

## Move through a long library

Kinosail loads a bounded page of results. Use **Next** and **Previous** controls, or the page loading control when it appears. The result count describes the current view after Profile policy, search, and filters.

If an item is missing, check the [scanning and metadata troubleshooting guide]({{ '/troubleshooting/scanning-and-metadata/' | relative_url }}). If the item is present but cannot play, use [playback troubleshooting]({{ '/troubleshooting/playback/' | relative_url }}).

Source of truth: `internal/server/browse.go` and `internal/server/home.go`.
