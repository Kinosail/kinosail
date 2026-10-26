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
| CinemaSpotlight and CinemaHero | MediaSpotlight and MediaHero | Full-bleed backdrop on compact phones, contained artwork on wider windows |
| MediaCard | HomeShelf | Separate title and metadata, truthful saved position |
| LoadingState | LibraryLoading | Pending placeholders follow the loaded artwork sizes, spacing, and shelf structure |
| Collections | Collections | Named collections open a title grid; returning from details preserves the collection |
| EPUB reader | Phone and tablet reader | Chapter contents, readable text width, appearance controls, and recoverable saved position |
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

Compact phone Home, title details, and show pages open with `MediaSpotlight` when a
backdrop or episode image exists. The image spans the full width, runs under the top
bar, and fades into the canvas. An Electric eyebrow (Watching, Listening, For you, or
Up next), the bold title, metadata, and actions overlap the fade. Wider windows and
large text keep `MediaHero`.

The video player header follows iPhone: a round Close player button on phones (Back
closes on TV), the bold title with the year or season and episode, and one row of pill
actions over a top fade. Each action appears once: Speed, then Audio & captions when
there is more than one track, otherwise a Captions on/off toggle, then Next episode.
The Media3 settings gear stays hidden because it would repeat Speed and Audio. The
Cast button appears only when Cast is ready; a failed Cast setup hides it and logs a
warning instead of showing a message.

Speed and Audio & captions open one list each. Phones use a bottom sheet and TV uses a
right-side panel. Each row has a check on the current choice, and choosing a row applies
it and closes the list. On TV, focus starts on the current choice and returns to the
button that opened the list.

Artwork retries after 1, 3, 8, and 20 seconds when a load fails, and again when the app
resumes. A card without a poster uses the backdrop. Every failure message uses the same
filled Try again button. A failed screen also retries by itself after 2, 5, 15 and 30
seconds, then every minute, and again when the app returns to the foreground.

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

This design mapping does not establish complete feature parity. Offline downloads,
playback bookmarks, and some advanced Settings capabilities still need Android
implementation. Wear Health Services and Apple Health remain separate platform integrations.
