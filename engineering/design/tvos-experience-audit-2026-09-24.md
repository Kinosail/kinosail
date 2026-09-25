# tvOS experience audit — 2026-09-24

## Scope and evidence

This is a source-level review of every reachable tvOS screen in the native Player, against the approved Electric design in `apps/player/apps/native/DESIGN.md`. It covers the main path from connection through browsing and playback, plus secondary settings and recovery. The current build launched in an Apple TV simulator and displayed Setup. This host could not drive a populated simulator session or a physical Siri Remote, so focus traversal, visual balance, performance and VoiceOver observations below are hypotheses to validate on device, not measured results. Prior runtime observations in `apps/player/apps/native/VERIFICATION.md` predate this change.

The interaction target is: Home Play focused by default; one Select to details, Play/Pause to start a focused playable card; title-sorted long libraries reachable by letter; continued remote movement loads more results without a Load more detour; Back returns to browsing context. Keep visible controls discoverable and use native AVKit for video playback.

## Screen inventory

| Screen | What works in source | Finding and action | Priority |
| --- | --- | --- | --- |
| Setup and pairing code | Nearby Server discovery, manual address, QR and spoken six-digit code, Cancel and error states | Remote keyboard and QR handoff need a populated real-device timing and readability pass. No change without that evidence. | P1 validation |
| Home | Featured Play requests default focus; Continue watching links play directly; My List, recent shelf and browse links are visible | Recent cards required a detail visit to play. Play/Pause now opens playable media directly. Books are excluded from tvOS Home because the reader exists only on iPhone/iPad. Validate first focus and vertical shelf traversal on hardware. | P1 fixed; P1 validation |
| Library hub and More | Named media groups and configurable tabs keep destinations reachable | The hub linked to a reader placeholder. The tvOS book link is removed. More and tab customization still take several remote presses; measure real path counts before changing the shell. | P1 fixed; P2 validation |
| Movies, shows, audiobooks, photos, My List, history and All media | Shared adaptive grid, native card focus, sort and explicit empty/retry states | A–Z was behind a menu and continuation needed a Load more press. A visible title jump and focus-triggered next-page loading now serve large catalogs; Load more remains for recovery. Play/Pause on video/audio cards starts playback. Validate 1,000+ titles, fast repeated D-pad movement, return focus and non-Latin title buckets. | P1 fixed; P1 validation |
| Search | Dedicated Search destination and debounced loading; no Search field appears during ordinary tvOS browsing | Check text entry, result focus and return to query with real remote input. Keep search out of browse scroll. | P1 validation |
| Media links | Scoped links reject a changed Viewer Profile and route search, details or play by media type | Validate external link confirmation, correct profile error, and Back behavior after launch from a link. | P2 validation |
| Movie, episode, audio, photo and book details | Clear title/metadata, one main action, My List, bookmarks and cast; Play receives default focus where playable | Book details previously offered a Read action that opened an unsupported tvOS reader. They now explain that reading is on iPhone/iPad. Verify button focus and long synopsis at accessibility sizes. | P1 fixed; P1 validation |
| Show and season picker | Next episode Play requests default focus; season menu and landscape episode grid | Play/Pause on a focused episode now starts it. Verify Specials, long seasons, selected-season persistence, and movement from Play to picker to first episode. | P1 fixed; P1 validation |
| Actor credits | Separate movie and show groups with card focus and large artwork | Select reaches details; no direct Play/Pause shortcut here. Validate long names, portrait artwork and navigation back to the same card before extending shortcut behavior. | P2 |
| Collections and collection titles | Clear list, grid and empty state | Collection cards now support Play/Pause for video/audio. Validate focus return from a collection and large collection paging; collection APIs currently return the complete list. | P1 fixed; P2 validation |
| Albums and album tracks | Album browsing, first track default focus and queue play | Album tiles used phone-scale 160pt plain links; they now share tvOS grid sizing and native card focus. Track rows now get native card focus. Validate artwork crop and album-to-track focus on a populated library. | P1 fixed; P1 validation |
| Audio player and mini player | Main Play receives default focus, skip, track navigation, queue, shuffle/repeat, sleep timer and options | Verify Play/Pause remote command, long audiobook timing, queue continuity and options reachability. The mini player uses a plain link; assess its focus visibility when shown. | P1 validation |
| Video player | Full-screen native AVKit and loading/retry | Native controls preserve TV conventions. Validate first frame, seeking, captions and menu dismissal using physical remote. | P1 validation |
| Playback options, tracks and speed | Chapters, section skip, next episode, audio/subtitle tracks, speed, preferences and bookmarks | Verify the AVKit options item opens this sheet, active choices are spoken, and return from a nested preference restores focus. | P1 validation |
| Photo viewer | Full-screen photo, Play/Pause zoom, directional pan and visible zoom toolbar action | Directional commands previously intercepted focus even when fitted; pan now handles remote moves only while zoomed. Spoken hint explains zoom and pan. Validate image-edge bounds and exit focus on device. | P1 fixed; P2 validation |
| Bookmarks | Named saved positions, add/delete, retry and empty states | Validate long names and deletion focus. Reading bookmarks are iOS-only in ordinary tvOS paths. | P2 validation |
| Progress sync | Clear conflict choices, retry and empty states | Verify the conflict choice and error recovery with actual pending progress. | P2 validation |
| Settings | Native sections, profile privacy copy, Top Shelf toggle and sign-out confirmation | Verify focus order and text wrapping; check the Top Shelf privacy decision on a shared Apple TV. | P2 validation |
| Playback preferences | Descriptive audio toggles, explicit saves and profile/title overrides | Verify labels, unsaved edits and error recovery; effects require a compatible Server stream. | P2 validation |
| Tab editor | Up to four pinned tabs, reorder, add/remove and reset | Verify remote traversal and the cost of reaching favorite sections through More. | P2 validation |
| Supporter | Collection state and an owner visibility toggle | Assess visual hierarchy with real badges; the simple scroll layout is not yet visually verified. | P2 validation |
| Privacy link | Privacy policy is reachable from setup and Settings | Verify link opening and safe return to the previous screen. | P2 validation |
| Connect a TV approval | Code review, device identity, expiry and explicit approval | Verify remote code entry, matched-code review, expiry feedback and focus after success. | P2 validation |

Reader, Downloads, offline playback, Play on TV, and reader/download preferences are iOS-only routes in the ordinary tvOS navigation and were excluded from tvOS screen scoring. Top Shelf is a system surface, not an in-app screen; its privacy toggle remains in Settings and needs separate real Apple TV validation.

## Quality assessment

| Dimension | Source-level assessment | Device evidence still needed |
| --- | --- | --- |
| Navigation and focus | Major browse and show paths use explicit focus sections and default Play focus; direct Play/Pause reduces one details round trip. | Count D-pad moves and presses for Home→play, Search→play, letter→title, season→episode, Back→prior card. Confirm focus never traps or jumps. |
| Visual hierarchy | Approved Electric tokens, untinted art, 64pt TV content padding and native cards are consistent on core media screens; album sizing now matches. | Populated 1080p/4K screen captures in ordinary and increased contrast, with short/long titles and missing art. |
| Accessibility | Headers, combined card labels, explicit action labels and photo hints are present. | VoiceOver reading order, remote speech, contrast, Reduce Motion and accessibility text sizes. |
| Efficiency | Lazy grids, thumbnail dimensions and focus-based paging limit initial catalog cost; direct play does not alter server playback contracts. | Measure first focusable frame, page-load latency, dropped frames during rapid traversal, and memory with 1,000+ and 20,000+ titles. |
| Recovery | Explicit empty/retry states, playback failure retry, and manual Load more remain available. | Disconnect/reconnect during pagination and playback; failed artwork, empty seasons, expired pairing, and stale progress. |

## Next acceptance pass

Use a populated, authorized test library and physical Siri Remote. Record screen captures and click counts for the routes above, then fix observed focus, readability or timing problems. A design-award goal requires that complete visual and interaction pass, comparison across large and sparse libraries, and feedback from people using the app on a couch; a successful build or source audit cannot establish it.

## Visual polish follow-up

The next pass gave actor credits, collections, albums and the Supporter page in-flow headings and the same TV artwork and background rhythm as core browsing. Actor card text now has a bounded ordinary layout while accessibility sizes retain full text. Now Playing places artwork beside the transport on TV, exposes live progress and chapters when present, and responds to the remote Play/Pause command. The shared TV configuration panel no longer repeats generic remote instructions.

The current build was installed and visually checked on the Apple TV simulator's Setup screen; the form and revised configuration panel remained readable. Earlier populated screenshots were used only as incumbent visual references. This follow-up still needs current populated captures and remote traversal for the changed actor, collection, album, audio and Supporter screens. The setup capture contains a discovered Server address and stays outside the repository.

## Watch and Listen navigation follow-up

Apple TV now uses the iOS Watch and Listen modes with a direct toolbar switch, separate remembered tab layouts, and mode-specific Home and Search results. Search exposes a scope picker so Watch can search both Movies and Shows and Listen can search both Music and Audiobooks. The tab editor names the active mode, and the catalog warms the other mode for a quicker switch. The old watch-only tab bar and mixed Home made audio discovery depend on More or Library.

Both native simulator builds and the tvOS unit suite passed. The connected physical Apple TV initially returned a zero-sized-display error; a later capture showed a populated Home with the existing installed build and its Resume button visibly focused. The new mode switch, tab focus, and populated Listen Home still need remote traversal and a capture from the updated build before this path is considered device-verified.

The merged mode build reached the physical Apple TV at its exact main revision. Its populated Home capture showed the new Listen action isolated high above the page heading. The follow-up places that action in the “For you” row beside the existing navigation, retaining Resume as the default focus. The updated placement still needs a fresh physical capture and remote traversal.

The header follow-up was installed at its exact main revision. A populated physical Home capture confirmed the Listen action aligned with “For you” and Resume visibly focused. The remote path to Listen and its content remain unverified. For long title-sorted libraries, the tvOS title-jump grid is now available from the library toolbar as well as the first row, so reaching another letter does not require returning to the top of the catalog. A letter change clears stale titles while its new page loads. Physical remote reachability of the toolbar and the grid layout still need validation.
