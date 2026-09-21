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

Electric makes the media title and next action clear while letting artwork keep its own color. Home places title, truthful progress and a green action beside contained 16:9 artwork on wide screens, followed by compact Continue watching rows. Compact screens keep feature information 12pt below the artwork. Home-first navigation keeps Continue watching immediately available.

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

The reusable hero fills the available width. Regular iOS size classes and tvOS place contained artwork beside information with a 32pt gap. Compact and accessibility sizes put 16:9 artwork above information with a 12pt gap. Foreground artwork stays contained and untinted. Apple TV additionally uses a subdued full-screen copy of the backdrop behind browsing content. Missing backdrops use 2:3 poster artwork (square for audio) capped at 240pt. Information grows with content; actions try horizontal placement and fall back to vertical through `ViewThatFits`.

Home displays For you and My List above the feature, then up to four compact Continue watching rows. Each row combines a contained landscape backdrop or 2:3 poster fallback, title, metadata, true progress and play affordance; See all opens History. `ResumeRows` adapts from 420pt minimum columns on iOS and 640pt on tvOS. At accessibility text sizes, rows use one column, omit decorative thumbnails and let titles wrap without a line limit.

Media grids adapt from 144pt posters or 280pt landscape cards on iOS, and 230pt or 360pt on tvOS, with 20pt column and 28pt row spacing. Accessibility sizes use one flexible column. Shelves retain an 18pt gap and aligned scrolling; iOS poster/landscape widths scale from 164/260pt with 260/300pt caps, while tvOS uses 230/390pt widths.

## Elevation & Depth

Dark iOS and Apple TV Home, library, title details and episode browsing use one bundled CinemaSail image to establish cinematic depth. The sail stays fixed behind the scroll viewport and extends into safe areas. Its fill frame aligns to the trailing edge so portrait layouts retain the sail rather than cropping it out; the image is never stretched. A dark vertical fade protects text and settles into the shared canvas beneath the feature. Foreground artwork remains contained and untinted; forms, playback and light appearance retain their existing surfaces. Increased Contrast and Reduce Transparency omit the decorative image entirely. The background is decorative only and does not receive focus, input or accessibility traversal.

Apple TV library content keeps compact navigation controls and the media grid without a focused-title header. Selection changes card focus only; it does not replace the shared sail or add a changing title above the grid. Native card focus has vertical breathing room and unclipped shelf edges.

**The Native State Rule.** Let native controls own focus and interaction feedback. iOS media links use plain style; tvOS media links retain card style and focus sections. Preserve tvOS focus movement and platform Reduce Motion behavior rather than imposing the web no-lift rule.

## Shapes

Artwork and posters use the artwork radius; compact resume thumbnails use their smaller radius. Posters retain 2:3 proportions, landscape artwork 16:9 and audio artwork square proportions. Artwork is clipped at its own boundary.

Prominent actions use the system capsule border shape. Existing supporting bordered actions also retain native capsule treatment with the raised tint; this is a native control convention, not a rule to make every container a capsule. Native fields, lists, tab bars, sheets and card focus keep system geometry.

## Components

### Buttons and navigation

Home's Play or Resume uses `.borderedProminent`, the signal tint and explicit action ink. Details uses `.bordered`, the raised tint and normal text. Both request large controls and capsule border shapes. Routes still follow media kind. Apple TV retains AVKit playback controls; iPhone and iPad use the coordinated touch presentation below.

### Video playback

`TouchPlaybackView` owns the immersive iPhone/iPad controls: one dismissal/title row, play and ten-second seeking, a native slider with elapsed and remaining time, and direct audio/subtitle, speed, volume and Picture in Picture actions. Controls use neutral white over black, safe-area placement, semantic type and at least 44pt targets. Compact layouts keep the audio/subtitle action as a labeled accessibility icon. Controls fade together after four seconds of uninterrupted playback, stay available with VoiceOver or Switch Control, and retain keyboard shortcuts while hidden. Reduce Motion removes the fade.

`TouchVideoSurface` uses AVPlayerLayer with aspect-fit rendering and captions inside the picture. AVFoundation owns video rendering and system Picture in Picture. Loading, buffering and reconnection retain the player surface; terminal failures expose readable retry and dismissal. Speed applies immediately to the session, including downloads, while Server preference saving happens separately. External captions remain an in-app capability and do not claim system PiP support.

Tabs default to Home, TV Shows, Movies and Search with persistent More. One to four destinations can be added, removed or reordered for the current Viewer Profile on this device. More always exposes the remaining destinations and Customize tabs. iPhone uses system tabs; iPad retains `.sidebarAdaptable`; Apple TV retains system tab and remote focus behavior. Search opens library search; Home shows a toolbar Search action only when Search is absent from the pinned tabs. Settings remains reachable through More.

### Personal tab editor

The editor uses native List sections for guidance, chosen destinations, available destinations and reset. Each chosen label sits above a row of move-earlier, move-later and remove controls. Their labels include explicit minimum 44×44pt hit regions and rectangular content shapes, with 8pt spacing. First/last movement, removal of the sole tab and adding beyond four are disabled by the actual rules.

### Cards / Containers

Media cards combine artwork, title, optional metadata and true Resume or Watched text. Decorative artwork is hidden from accessibility; the card combines its accessible children. The adaptive muted role keeps supporting metadata consistent with the page in both appearances. A missing image retains the media-kind placeholder rather than fabricated artwork.

### Inputs / Fields

Settings, setup, preparation and download screens retain native fields, lists and forms. Labels, disabled states, destructive roles and operation feedback stay attached to their controls. Fixed web input sizes and focus rings are not native tokens.

### Hero and watch position

Home selects the first Continue watching item, otherwise the first Recently added item. Details and episode browsing reuse the in-flow hero. `WatchPosition` renders a progress bar only when a valid duration produces a fraction; unavailable duration or a failed request retains the known saved position. Images stay still without an autoplay carousel.

### Loading and recovery

`LoadingState` has shelf, home, detail and grid variants. Home/detail reuse the hero's wide/stacked layout, artwork ratio and gap. Home includes compact continuation row placeholders, then a poster shelf. Grids reuse `MediaGrid.columns` and eight poster placeholders; shelves use four posters with the loaded spacing and width policy. These policies align the known layout, while real content, missing artwork and Dynamic Type can change its final geometry.

`ResourceView` displays loading feedback only while initial content is pending. A labeled `ProgressView` supplies accessible status while decorative placeholders are hidden. Initial failure shows Try again; failed refresh retains loaded content with truthful feedback. A new resource identity clears stale content. Empty, failed and settled content must not remain a skeleton.

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
