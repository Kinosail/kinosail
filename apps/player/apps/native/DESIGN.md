---
name: "Kinosail native Player"
description: "Electric media browsing with cinematic artwork, semantic Apple typography and system navigation."
colors:
  background: "#0b0d0b"
  surface: "#151914"
  raised: "#20271e"
  text: "#f6f8f2"
  muted: "#a0a79c"
  signal: "#c4ff47"
  signal-ink: "#142000"
  light-background: "#f4f8ef"
  light-surface: "#ffffff"
  light-raised: "#e9efe2"
  light-text: "#162011"
  light-muted: "#5b6852"
  light-signal: "#3c6100"
  light-signal-ink: "#ffffff"
  increased-dark-muted: "#dae2d3"
  increased-light-muted: "#39482e"
  increased-dark-signal: "#daff98"
  increased-light-signal: "#304e00"
typography:
  display:
    fontFamily: "SF Rounded"
    fontSize: "36pt"
    fontWeight: 700
  headline:
    fontFamily: "SF Pro"
    fontWeight: 700
  body:
    fontFamily: "SF Pro"
  label:
    fontFamily: "SF Pro"
    fontWeight: 500
rounded:
  artwork: "12pt"
  loading-line: "5pt"
spacing:
  content-ios: "20pt"
  content-tvos: "64pt"
  hero-wide-gap: "32pt"
  hero-stacked-gap: "12pt"
  hero-information: "12pt"
  action-gap: "12pt"
  shelf-gap: "18pt"
components:
  button-primary:
    backgroundColor: "{colors.signal}"
    textColor: "{colors.signal-ink}"
  button-secondary:
    backgroundColor: "{colors.raised}"
    textColor: "{colors.text}"
  artwork-card:
    rounded: "{rounded.artwork}"
  resume-row:
    textColor: "{colors.text}"
  watch-progress:
    backgroundColor: "{colors.signal}"
  navigation:
    textColor: "{colors.signal}"
  tab-editor-action:
    width: "44pt"
    height: "44pt"
---

# Design System: Kinosail native Player

## Overview

**Creative North Star: "Electric"**

Electric makes the media title and next action clear while letting artwork keep its own color. iOS Home places title, truthful progress and a green action beside contained 16:9 artwork on wide screens, followed by a landscape Up Next shelf. Compact screens keep feature information 12pt below the artwork. Apple TV Home starts with a landscape Continue watching rail and a Browse row of media destinations.

The user selected Electric option 1 and authorized replacing the previous coral system across iPhone, iPad and Apple TV. `Sources/Design/KinoTheme.swift`, `CinemaHero.swift`, `MediaViews.swift`, `WatchPosition.swift` and `Sources/App/PlayerTabs.swift` are the implementation authority. This workspace's `PRODUCT.md` and `AGENTS.md` preserve privacy, Direct First playback, existing server contracts and native accessibility. The active SwiftUI implementation is `Sources/`; legacy client sources do not define this system.

**Key Characteristics:**

- Untinted, contained artwork; dark iOS and Apple TV browsing surfaces share a static immersive sail backdrop.
- Electric actions with SF Pro text and rounded feature titles.
- Personal destinations and platform-owned navigation, touch and remote focus.

## Colors

### Primary

Electric green identifies the main action, selection and actual watch-progress track. Action ink is the explicit foreground on prominent filled actions. The light counterpart uses deep leaf green with white ink; Increased Contrast has separate action values in both appearances.

### Neutral

Background, surface and raised define three tonal levels. Off-white and sage-gray distinguish primary text and supporting metadata on the dark canvas. Pale green paper, white and quiet green-gray provide the light counterpart. Library counts, media metadata and watched labels use the adaptive muted role, including its Increased Contrast values. System forms, bars and semantic controls retain native materials where used.

**The Adaptive Role Rule.** Let KinoTheme resolve appearance and contrast traits; do not derive the shell palette from artwork or mistake the action color for proof of state.

## Typography

SF Pro supplies body text, metadata, controls and shelf headings. Rounded type is reserved for the feature title and Home greeting. Semantic styles preserve platform sizing: the greeting uses `.largeTitle.bold()`, shelf headings use `.title2.bold()`, media titles use `.headline`, supporting metadata uses `.caption`, and body copy uses `.body`. Time text uses monospaced digits.

The hero's rounded title has a 36pt iOS base and a 56pt tvOS base, scaled relative to `.largeTitle`; its frontmatter value is the base, not a fixed rendered size. Titles wrap vertically. At accessibility text sizes, media titles remove their line limit and hero plots remove the ordinary three-line limit. Native platform typography is an explicit part of the approved direction; it does not change the web Manrope contract.

## Layout

Home is a vertical scroll view with 32pt section spacing and theme content padding. iOS uses 20pt content padding; tvOS uses 64pt. Safe areas, back navigation, tab/sidebar adaptation and dismissal remain native.

iOS Home, title details and show pages open with `CinemaSpotlight` when a backdrop or episode still exists. The artwork bleeds edge to edge under the navigation bar and fades into the canvas through a bottom mask. Compact widths use an artwork height of 86% of the width; regular widths use 46% of the width, clamped to 380–640pt. The rounded title, metadata, progress and actions overlap the fade, capped at 600pt wide on regular widths. Home adds an Electric eyebrow (Watching, Listening or For you) and an icon-only My List action. Pulling down stretches the artwork and scrolling up moves it at 32% speed; Reduce Motion keeps it still. Episode rows on show pages show a thumbnail that fills the full row height, 124pt wide on compact widths and 176pt on regular widths. Show pages use the show backdrop, an Electric "Up next · S1 E1" eyebrow, the show title, year and genres, Play and Details for the next episode, and the show plot below. `SpotlightSkeleton` uses the same geometry while Home, details or show pages load. Without a backdrop, Home falls back to `CinemaHero`. The spotlight prefers the title or show backdrop over an episode still. On compact widths, landscape artwork is not cropped to the tall canvas: a sharp copy fills the width from the top of the screen at no more than 4:3, and a blurred copy of the same image fills the rest of the canvas behind the title.

Other iOS destinations share the cinema background and the spotlight type scale. Album pages show artwork, a rounded title, artist and track count in muted text, and an Electric Play capsule. Actor pages place the portrait beside a rounded name and title count. Collections and collection pages use large navigation titles over the cinema background. Secondary text uses `KinoTheme.muted`, not the system secondary color. Grouped lists such as Library, Downloads, Settings, preferences and More use `listCanvas()`, which hides the list background over the cinema background.

The reusable hero fills the available width. Regular iOS size classes and tvOS place contained artwork beside information with a 32pt gap. Compact and accessibility sizes put 16:9 artwork above information with a 12pt gap. Hero artwork stays untinted. Apple TV additionally uses a subdued full-screen copy of the backdrop behind browsing content. Missing backdrops use 2:3 poster artwork (square for audio) capped at 240pt. Information grows with content; actions try horizontal placement and fall back to vertical through `ViewThatFits`.

iOS Home labels a saved Watch feature Watching and places it above Continue watching. Continue watching shows up to 15 landscape cards with direct resume, metadata and true progress. Without a saved title, the recent feature is labeled For you. Listen mode labels its saved feature Listening and keeps compact Continue listening rows with iOS expansion controls. Apple TV Home opens with a spotlight: the rounded title, metadata and a two-line plot of the focused Continue watching title, or of the first saved or recent title before any card has focus. Episodes show the show name with season, episode and episode title. The spotlight is decorative text above up to 15 saved titles in one open Continue watching rail, including the first title, followed by Browse destinations for Movies, TV Shows, Music, Audiobooks and Photos. Browse uses native card buttons with a raised gradient platter and solid off-white and Electric green geometric category marks. A focused Browse card also shows a 4pt off-white outline. Focused media artwork lifts with a 6% scale and soft shadow and keeps a bright outline. Reduce Motion removes the lift animation. A final link with the Kinosail mark switches Watch and Listen Home modes. Watch then separates recently added movies and TV shows, adds unwatched shelves and up to four movie genre shelves, each titled by its genre, from the loaded catalog. Listen separates recently added music and audiobooks. At accessibility text sizes, card titles wrap and the shelf remains horizontally scrollable.

Media grids adapt from 96pt posters with 12pt column spacing or 280pt landscape cards with 20pt column spacing on iOS. tvOS uses 230pt posters or 360pt landscape cards with 20pt column spacing. All grids use 28pt row spacing; accessibility sizes use one flexible column. Shelves retain an 18pt gap and aligned scrolling; iOS poster/landscape widths scale from 164/260pt with 260/300pt caps, while tvOS uses 230/390pt widths.

## Elevation & Depth

Dark iOS and Apple TV Home, library, title details and episode browsing use one bundled CinemaSail image to establish cinematic depth. The sail stays fixed behind the scroll viewport and extends into safe areas. Its fill frame aligns to the trailing edge so portrait layouts retain the sail rather than cropping it out; the image is never stretched. A dark vertical fade protects text and settles into the shared canvas beneath the feature. Foreground artwork remains untinted; forms, playback and light appearance retain their existing surfaces. Increased Contrast and Reduce Transparency omit the decorative image entirely. The background is decorative only and does not receive focus, input or accessibility traversal.

Apple TV Home keeps the static sail background. Focus changes update only the spotlight text, and focusing a Continue watching card scrolls Home to the top so the spotlight stays visible. Movie details and show pages share the same full-bleed backdrop hero, Play action and fades.

Apple TV library content keeps compact navigation controls and the media grid without a focused-title header. Selection changes card focus only; it does not replace the shared sail or add a changing title above the grid. Native card focus has vertical breathing room and unclipped shelf edges.

Long title-sorted libraries show an A–Z index on the trailing edge. On Apple TV the index starts below the top bar and sizes each letter so every letter fits without scrolling. On iPhone and iPad a narrow scrubber lists every letter. It keeps a fixed place clear of the title bar and tab bar, so it does not move when the library scrolls. Dragging along it shows the current letter in a large bubble, ticks a selection haptic for each letter, and scrolls the grid to that letter. The full title list stays in place, so you can scroll back to earlier letters. VoiceOver reads it as an adjustable Title index.

On iPhone and iPad, pressing and holding a poster opens a menu with Play or Resume, Details, Go to show for episodes, and Add to My List. Continue watching posters also offer Remove from Continue watching. A success or error haptic confirms the change. Library search lists up to eight recent searches for the current Viewer Profile on this device when the field is empty, with Clear recent searches.

**The Native State Rule.** Let native controls own focus and interaction feedback. iOS media links press down to 96.5% with a spring and a slight dim; Reduce Motion keeps the dim only. tvOS adds a scale-and-shadow lift, a visible focus outline and retains focus sections. Preserve tvOS focus movement and platform Reduce Motion behavior rather than imposing the web no-lift rule.

## Shapes

Artwork and posters use the artwork radius; compact resume thumbnails use their smaller radius. Posters retain 2:3 proportions, landscape artwork 16:9 and audio artwork square proportions. Artwork is clipped at its own boundary. Video and show cards fill their frame instead of letterboxing. Music, audiobook and book covers stay contained. Unwatched titles show an Electric dot with a soft glow at the top-trailing corner: 15pt on iOS and 24pt on tvOS, with a dark rim so it reads on bright artwork.

The iOS setup form opens with the Kinosail mark, a rounded Kinosail Player title and one factual line above the native Server address field. Dark appearance places the form over the shared sail background.

Prominent actions use the system capsule border shape. Existing supporting bordered actions also retain native capsule treatment with the raised tint; this is a native control convention, not a rule to make every container a capsule. Native fields, lists, tab bars, sheets and card focus keep system geometry.

## Components

### Buttons and navigation

iOS Home's Play or Resume uses `.borderedProminent`, the signal tint and explicit action ink. Details and other secondary spotlight actions use the Liquid Glass button style on iOS. Fallback hero Details uses `.bordered`, the raised tint and normal text. Both request large controls and capsule border shapes. Routes still follow media kind. Apple TV retains AVKit playback controls; iPhone and iPad use the coordinated touch presentation below.

### Motion

Launch keeps the system launch screen: the Kinosail mark centered on black. `LaunchReveal` then dips the mark to 84% and grows it until the app shows through the sail, as a mask, in about 0.8 seconds. The app settles from 108% to its normal size underneath. Apple TV does not cut the sail out of the screen. The mark dips to 92%, then grows to 135% as it fades, while the black curtain fades and the app settles from 104% with a crossfade, in about 1.1 seconds. The black backdrop fills the whole screen during the reveal, so no edge bars show. Restore never waits on the network. Reduce Motion replaces the reveal with a 0.25-second fade.

iOS navigation from a video, show, album or collection card uses the native zoom transition from that card's artwork. Reduce Motion uses the standard push. The iOS tab bar collapses when scrolling down and expands as soon as scrolling turns upward, not only at the top. `TabBarCollapse` tracks the scroll direction and switches the minimize behavior. My List changes its symbol with a replace transition and plays a success haptic.

### Video playback

The Server's Playback settings decide autoplay and which markers skip automatically. By default autoplay is on and every marker type skips. During an intro, recap or commercial marker that is not set to skip automatically, video shows Skip Intro, Skip Recap or Skip. When the credits skip automatically and a next episode exists, the next episode starts at once with no countdown. Otherwise, from the credits marker or the last 30 seconds, an episode with a next episode shows Next Episode. iPhone and iPad draw it as a white capsule at the trailing edge above the timeline; Apple TV uses the native AVKit contextual action. When the episode ends, the next episode starts by itself, up to three in a row without viewer input.

On iPhone and iPad, status text such as Opening video…, Preparing video… or Buffering… floats above the playback buttons. The buttons keep their place when the status appears or clears.

`TouchPlaybackView` owns the immersive iPhone/iPad controls: one dismissal/title row, play and ten-second seeking, a native slider with elapsed and remaining time, and direct audio/subtitle, speed, volume and Picture in Picture actions. The played timeline track uses Electric green; the remaining track, thumb and controls stay neutral white over black, with safe-area placement, semantic type and at least 44pt targets. Compact layouts keep the audio/subtitle action as a labeled accessibility icon. Controls fade together after four seconds of uninterrupted playback, stay available with VoiceOver or Switch Control, and retain keyboard shortcuts while hidden. Reduce Motion removes the fade.

The Apple Watch opens on a compact Remote page with the current player, title, progress and playback controls. After the viewer opts in, a vertical swipe opens the Heart page; the Remote page also links to it. Electric green marks play and the movie-time heart graph, including the peak reading at its movie position. Heart tracking begins only when the viewer taps Start heart graph and grants Health access; missing readings or playback positions appear as gaps. The watch reads Health locally and does not send heart data to the iPhone, Apple TV or Server. The Remote page sits on a black container background with an Electric top glow that brightens while the selected player is playing. The title uses rounded type. Play and pause swap symbols with a replace transition, play a medium haptic and respond to the double-tap hand gesture.

`TouchVideoSurface` uses AVPlayerLayer with aspect-fit rendering and captions inside the picture. AVFoundation owns video rendering and system Picture in Picture. Loading, buffering and reconnection retain the player surface; terminal failures expose readable retry and dismissal. Speed applies immediately to the session, including downloads, while Server preference saving happens separately. External captions remain an in-app capability and do not claim system PiP support.

The iPhone landscape action uses opposing diagonal arrows in a dark 44pt circle. It requests landscape for the player's window with Apple's public scene geometry API. Its return action and leaving playback restore the previous interface orientation. iOS can decline a request; the player shows recoverable feedback and keeps playback available. This action does not change Portrait Orientation Lock or crop the video.

A downward swipe on the video background closes playback through the existing navigation callback. It requires 100pt of predominantly vertical movement and excludes header and timeline starts. Playback controls, sheets, volume and system Picture in Picture retain their own interactions. VoiceOver retains the visible Close player button. Closing pauses playback and saves position through the existing playback departure behavior.

Tabs default to Home, TV Shows, Movies and Search with persistent More. One to four destinations can be added, removed or reordered for the current Viewer Profile on this device. More always exposes the remaining destinations and Customize tabs. iPhone uses system tabs; iPad retains `.sidebarAdaptable`. Apple TV Home keeps Search and Settings in a fixed top bar above scrolling content and includes Library among the Browse destinations. Content scrolling under the top bar fades out over 32pt instead of ending in a hard line. Native Back navigation returns from those destinations to Home. Search opens library search; Home shows a toolbar Search action only when Search is absent from the pinned tabs. Settings remains reachable through More on iPhone and iPad.

### Personal tab editor

The editor uses native List sections for guidance, chosen destinations, available destinations and reset. Each chosen label sits above a row of move-earlier, move-later and remove controls. Their labels include explicit minimum 44×44pt hit regions and rectangular content shapes, with 8pt spacing. First/last movement, removal of the sole tab and adding beyond four are disabled by the actual rules.

### Cards / Containers

Media cards combine artwork, title, optional metadata and true Resume or Watched text. Decorative artwork is hidden from accessibility; the card combines its accessible children. The adaptive muted role keeps supporting metadata consistent with the page in both appearances. A missing image retains the media-kind placeholder rather than fabricated artwork.

### Inputs / Fields

### Server setup

Setup is one scrolling page on the cinema background. It shows, in order: servers from the viewer's other Apple devices, servers found on the network, and an address field. Each server is a card with a brand monogram, its name, its kind and host, and one short badge. Kinosail cards carry Best experience. Shared cards carry Ready · no sign-in needed. Placeholder cards pulse only while the network scan runs.

Brand monograms use colored letters, not third-party logos. Jellyfin, Emby and Plex logos are trademarks; add them only with permission.

Sign-in puts the server header first, then Who's watching (fixed columns, so one profile stays centered), then the password card. On Apple TV, the header and Quick Connect card sit on the left and the account column on the right. The Quick Connect code is large and tabular, with a QR code on Apple TV and iPad.

The ready screen fans up to five real posters from the new library, then Connected, You're all set, the server and counts, Start watching and Add another server. Short setup screens center vertically on iPad. Focused primary buttons on Apple TV use the TV primary fill with dark ink.

### Downloads

The Detail Download action shows live state. Before a download it reads Download. While preparing or transferring, an Electric progress ring and the status or percentage replace the arrow; selecting it opens Downloads. When the file is ready, it reads Downloaded with a filled Electric check and plays a success haptic. iOS cards for downloaded titles show a small Electric download badge at the top-leading corner.

Downloads opens with a storage summary: the space used by downloads, the free space on the device and an Electric share bar. In progress comes before Ready to watch. Rows use 16:9 artwork (square for audio) with a thin Electric progress line. A trailing ring pauses, resumes or retries; it stays at least 44pt. Ready rows play offline when selected. A download that stops after its automatic retries resumes by itself when the connection to the Server comes back. A download the viewer paused stays paused. Swipe actions and the context menu offer Pause, Resume, Play offline and Remove. Edit mode keeps multiple selection. The Download options sheet shows the title's artwork, lists each quality with a one-line explanation and pins Add to Downloads at the bottom.

Settings, setup, preparation and download screens retain native fields, lists and forms. Labels, disabled states, destructive roles and operation feedback stay attached to their controls. Fixed web input sizes and focus rings are not native tokens. On Apple TV, explanations in Settings are small muted section footers, not focusable rows. The Apple TV Now playing screen shows its playback options as a text Options button in the top bar, because the system draws an icon-only bar button too small to read.

Playback preferences group Boost Dialog and Normalize Loudness as native toggles with short plain-language descriptions. Changes save automatically on every platform; there is no Save button. The section states that the effects use a compatible Server stream. On iPhone and iPad it also states that existing downloads keep their encoded audio. The interface must not imply the effects work offline or without transcoding permission. On Apple TV, Speed, Audio language and Subtitles each open a choice list and show their current value on the same row. A title with its own preferences shows Use profile defaults. A title that follows the Viewer Profile shows no reset control and no status row.

Apple TV video uses the system scrub bar. There is no separate seek screen in the transport bar. The scrub bar shows keyframe thumbnails when the Server stream includes an I-frame playlist.

### Hero and watch position

iOS Home selects the first Continue watching item, otherwise the first Recently added item. Apple TV Home keeps the saved titles together in Continue watching. Their cards show true progress and time left instead of the unwatched mark. Details and episode browsing reuse the in-flow hero. On Apple TV show pages, Season buttons use the same capsule as other Apple TV actions, start at the Episodes heading edge and fill white when focused. The Episodes shelf runs to the bottom of the screen. Selecting an episode card opens its playback options at once; Play/Pause plays it directly; press and hold opens a Details menu for the episode page. `WatchPosition` renders a progress bar only when a valid duration produces a fraction; unavailable duration or a failed request retains the known saved position. Images stay still without an autoplay carousel.

### Loading and recovery

`LoadingState` uses destination-specific shelf, home, detail, grid, music, show, album and list layouts. iOS Home/detail reuse the hero's wide/stacked layout, artwork ratio and gap. Apple TV Home placeholders follow the spotlight, the Continue watching or Listening area, Browse links and a recent shelf. Their rails run past the right content edge, like the loaded rails. Show placeholders use the full-bleed hero geometry. Grids reuse `MediaGrid.columns` and eight artwork placeholders; shelves use four cards with the loaded spacing and width policy. These policies align the known layout, while real content, missing artwork and Dynamic Type can change its final geometry.

Artwork heals itself. A failed load keeps the quiet placeholder and retries after 1, 3, 8 and 20 seconds, then retries again when the app returns to the foreground or the connection comes back. Only a final failure shows the kind symbol. A card without a poster uses the title's backdrop.

`ResourceView` displays loading feedback only while initial content is pending. Native skeletons shimmer within their existing shapes and expose one accessible loading label; Reduce Motion keeps the placeholders still. Initial failure shows Try again and retries by itself after 2, 5, 15 and 30 seconds, and when the connection comes back. A missing item or a signed-out or expired session does not retry; failed refresh retains loaded content with truthful feedback. A new resource identity clears stale content. Empty, failed and settled content must not remain a skeleton. Determinate playback and download progress remains visible; actions without a replacement layout use plain status text.

Recovery and empty-state descriptions stay within a 560pt measure and wrap vertically. Recovery actions use large system controls. Connection banners use the same system sizing without adding a second minimum height inside their labels. Watch transport targets retain at least 44pt, and playback titles expand at accessibility text sizes.

### Asset provenance

Approved Player sail geometry is authored SVG under `assets/source/`, recolored to Electric green. `scripts/generate-brand-assets.sh` uses `sips` to rasterize and resize those sources, and `ffmpeg` to export the iOS app icon as RGB. Swift iOS and tvOS asset catalogs derive from these sources, including separate TV foreground/background layers and top-shelf images. Apple owns final icon masking. The decision comp is a critique reference, not production library artwork.

## Do's and Don'ts

### Do:

- Do resolve shared colors through KinoTheme and preserve native appearance and contrast traits.
- Do keep saved position, pending feedback and failure recovery truthful.
- Do preserve native navigation, Dynamic Type, accessibility labels and tvOS focus behavior.

### Don't:

- Don't tint foreground artwork. Apple TV background imagery alone receives the readability fade.
- Don't impose web geometry or the web no-lift rule on Apple system controls.
- Don't treat simulator captures or successful compilation as physical-device or accessibility certification.
