---
version: 1
slug: "packages-webassets-static-last-light-css"
primary_target: "packages/webassets/static/last-light.css"
related_targets: ["apps/player/internal/server/static/home.css","apps/dashboard/internal/server/web/static/dashboard.css","apps/subtitles/internal/server/static/subtitle-dashboard.css","apps/player/apps/native/Sources/Design/KinoTheme.swift"]
---

# Kinosail Electric

Scope: Player web, iPhone, iPad and Apple TV; Subtitles and Dashboard web. User selected Electric (option 1) on September 12 and explicitly requested application across all apps.

## Direction contract

THESIS: A personal media library with Robinhood-inspired clarity: one unmistakable next action, expressive type, and useful open rows.

OWN-WORLD: Black-green #0b0d0b, electric #c4ff47, off-white #f6f8f2 and muted #a0a79c. Manrope on web and rounded system type on Apple. Flat surfaces, 12pt artwork corners and generous primary capsules.

STORY: Recognize the title, resume directly, then find the next item. Operators recognize service or subtitle state and act beside it.

FIRST VIEWPORT: Phone Home has Kinosail navigation, Your evening., For you/My List, wide artwork above a large title, truthful progress, a full-width green playback action, and compact Continue watching rows. Desktop and tablet use a wider artwork/copy split. Other apps inherit clear headings, open rows and restrained green controls.

FORM: User-pinned Robinhood inspiration and approved Electric comp at .impeccable/mocks/decision/robinhood-mobile-20260912/electric.png. This explicit selection supersedes seed 70041d3c.

SIGNATURE: The green action and progress line guide the eye; static artwork remains untinted. Pending placeholders preserve the final geometry. Native navigation, Dynamic Type and reduced motion remain platform-native.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Invariants and boundaries

Preserve API compatibility, Direct First playback, permissions, account/session isolation, offline downloads, direct Dashboard links and real subtitle state. Real library artwork replaces the comp's illustrative media. Unknown runtimes show saved position rather than invented percentages. Accessible light and increased-contrast appearances remain available. Root .gates-disabled excludes test suites and automated design gates; builds, manual rendering and independent design review supply separate evidence.

## Confirmed navigation update

The user additionally requested customizable bottom navigation, starting with Movies and TV Shows. Apple clients now default to these two tabs plus persistent More. Viewers can add, remove and reorder one to four destinations; all others stay reachable in More. Preferences are scoped to the Viewer Profile and device. Player mobile web uses the same defaults and a browser-scoped per-profile tab editor; desktop retains its existing navigation controls. Native tabs, sidebar and TV focus remain system controls.
