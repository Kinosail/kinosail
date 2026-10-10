# Android Player design

The Apple clients define the shared viewing experience. Follow
[`../native/DESIGN.md`](../native/DESIGN.md) and its active SwiftUI sources when
changing Android screens. Translate controls into Android conventions.

| Apple reference | Android surface | Shared pattern |
| --- | --- | --- |
| iPhone Home | Phone Home | Watching or Listening feature, then continuation and recent shelves |
| iPad navigation | Tablet navigation | Use available window space; Android uses a navigation rail |
| Apple TV Home | Android TV Home | Fixed Search and Settings, landscape continuation, Browse, recent shelves |
| Personal tabs | Phone and tablet tabs | Home, TV Shows, Movies, Search, and persistent More; save choices per Viewer Profile |
| CinemaHero and MediaCard | MediaHero and HomeShelf | Contained artwork, separate title and metadata, truthful saved position |
| LoadingState | LibraryLoading | Pending placeholders follow the loaded artwork sizes, spacing, and shelf structure |
| Apple Watch Remote | Wear OS Remote | Current player, title, playback position, seek, and playback controls |
| Apple Watch Heart | Wear OS Heart | Explicit opt-in, local readings, movie-time graph, and gaps for missing readings |

## Presentation

Use the native Electric palette and bundled CinemaSail artwork. Android phone,
tablet, and TV browsing currently remain dark. Keep Material typography in `sp`,
Material controls on touch screens, and Compose for TV focus on television.
Do not copy Apple controls, SF Symbols, or fixed Apple text sizes.

Landscape continuation cards keep a 16:9 artwork box even with portrait-only
artwork. Fit the image inside that box. Poster shelves use 2:3 artwork; music
and audiobooks use square artwork. Place Home shelf titles, metadata, and saved
positions below the image. Keep Home cards open against the shared backdrop.

Use the same card widths in pending and loaded Home shelves. Keep an 18dp shelf
gap and enough padding for TV focus. Large text can wrap card titles. Compact
Home and title details stack artwork above information; wider windows place
them beside each other. Large text restores the stacked arrangement.

## Navigation and state

Keep TV Search and Settings above the scrolling Home content. Both remain
reachable after scrolling; Android Back returns from each destination to Home.
Use bottom navigation on compact windows and a rail on wider, taller windows.
More keeps unpinned destinations and Customize tabs available.

Show placeholders only while initial content is pending. Empty and failed
responses remove them. Failure exposes Try again; a later-page failure retains
loaded cards and offers Retry loading more. Loaded continuation uses real saved
positions. Do not infer a percentage when duration is unavailable.

## Verification and limits

Compare pending, loaded, empty, and failed screens on phone, tablet, and TV.
Check enlarged text, Android Back, and TV D-pad focus. Device-side synthetic
HTTP journeys verify Android presentation and transport together. They do not
prove compatibility with a populated Go Server, physical input, or codecs.

This design mapping does not establish complete feature parity. Collections,
offline downloads, and EPUB reading still need Android implementation. Wear
Health Services and Apple Health remain separate platform integrations.
