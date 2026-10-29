# Android Player design

The Apple clients define the shared viewing experience. Follow
[`../native/DESIGN.md`](../native/DESIGN.md) and its active SwiftUI sources when
changing Android screens. Translate controls into Android conventions.

| Apple reference | Android surface | Shared pattern |
| --- | --- | --- |
| iPhone Home | Phone Home | Watching or Listening feature, then continuation and recent shelves |
| iPad navigation | Tablet navigation | Use available window space; Android uses a navigation rail |
| Apple TV Home | Android TV Home | Fixed Search and Settings, landscape continuation, Browse, recent shelves |
| Apple TV details and show pages | TvDetail and TvShowPage | Full-bleed backdrop hero, Season row, Episodes shelf |
| PlaybackPrompt | PlaybackPromptButton | Skip Intro, Skip Recap, Skip, and Next Episode at the trailing edge |
| Personal tabs | Phone and tablet tabs | Home, TV Shows, Movies, Search, and persistent More; save choices per Viewer Profile |
| iPhone Settings | Phone and tablet Settings | Server & Viewer Profile, Playback & library, Personalize, and About sections; Sign out asks first |
| Playback preferences | Playback preferences | Speed, Audio language, Subtitles, Boost Dialog, and Normalize Loudness save automatically |
| Connect a TV | Connect a TV | Enter the six-digit code, review the device, then approve it |
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
there is more than one track, otherwise a Captions on/off toggle.
The Media3 settings gear stays hidden because it would repeat Speed and Audio. The
Cast button appears only when Cast is ready; a failed Cast setup hides it and logs a
warning instead of showing a message.

Server markers drive one white prompt at the trailing edge, above the timeline. During
an intro, recap, or commercial it reads Skip Intro, Skip Recap, or Skip. From the credits,
or the last 30 seconds without markers, it reads Next Episode. On TV the prompt takes
focus. Marker types in the Viewer's automatic skip list skip without a prompt. When an
episode ends, the next one starts by itself up to three times in a row; any choice by the
Viewer resets that count.

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
TV Home opens with focus on the first Continue watching card. Back on Home moves
focus to Search, and Back on Search leaves the app. Back from details, a show, a
grid, Search, or Settings returns focus to the card or button that opened it.
TV library grids show only the title, title count, and Sort above the cards.
TV cards mark unwatched titles with an Electric dot at the top-trailing corner.

TV details and show pages use a full-bleed backdrop that fades to the canvas, with
the title, metadata, plot, and actions at the bottom-leading edge. They have no Back
button; Android Back closes them. Focus starts on Play or Resume. A show page adds a
row of Season buttons and an Episodes shelf of landscape cards.
Use bottom navigation on compact windows and a rail on wider, taller windows.
More keeps unpinned destinations and Customize tabs available. Settings is a
destination like the others, so a Viewer can pin it as a tab. Back from a
destination opened in More returns to More. Like iPhone tab labels, Android tab
labels stay on one line and grow only a little with large text. Settings rows move
the current value below the row title at large text sizes.

Phone title details use a filled Play action and a tonal My List button that reads
My List or In My List. A started or watched video offers Remove from Continue
watching. Phone Home ends with Browse library links for the current mode: Movies
and TV Shows, or Music and Audiobooks. Genre shelves have no extra heading. An empty
phone Home shows Your library is ready, or Nothing to listen to yet.

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
playback bookmarks, the Library hub, long-press card menus, Reader and Downloads
Settings, and Play on another device still need Android implementation. Wear Health Services and Apple Health remain separate platform integrations.
