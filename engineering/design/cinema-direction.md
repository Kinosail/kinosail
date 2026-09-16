# Kinosail Electric

Scope: Player web and Swift iPhone/iPad/tvOS, Subtitles web and Dashboard web. The user selected Electric option 1 on September 12, 2026 and authorized replacing the previous coral direction across these active apps. Kinosail names and the approved app-symbol family remain.

## Direction contract

THESIS: A personal media library with Robinhood-inspired clarity: one unmistakable next action, expressive type and useful open rows.

OWN-WORLD: Black-green canvas, Electric green action color, off-white primary text and sage-gray metadata. Web uses self-hosted Manrope; Apple uses rounded semantic system type. Flat surfaces, modest artwork corners and strong primary capsules keep content and action legible. Light appearance has its own pale green canvas, dark text and deeper action green.

STORY: Recognize the title, resume directly, then find the next item. Operators recognize real subtitle or service state and act beside it.

FIRST VIEWPORT: Web Home places the greeting and For you/My List navigation before untinted wide artwork, separate title/progress and a prominent green playback action; continuation rows follow. Wide screens place artwork beside copy. Subtitles places the next task beside actual file state; Dashboard makes direct application rows easy to scan. Native defaults now open Home, reflecting the user's later navigation choice; Home remains reachable in More or as a chosen tab.

FORM: The user-pinned Robinhood inspiration and selected Electric decision comp at `.impeccable/mocks/decision/robinhood-mobile-20260912/electric.png` establish the chosen direction. This is a code-led implementation; the comp is a critique reference, not a literal content fixture or an image to reproduce pixel-for-pixel. No separate Electric QUALITY BAR card was supplied. The approved selection supersedes the earlier seed and coral direction.

SIGNATURE: The green action and truthful progress line guide the eye. Artwork stays untinted and in flow; title text does not depend on a scrim. Open continuation and operational rows avoid enclosing every item in a card. Pending placeholders use loaded geometry only while real work is pending. Web hover stays stable; native Apple materials, Dynamic Type, Reduce Motion and tvOS card focus remain platform-owned.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Source authority

The machine-readable primitives live in the YAML frontmatter of each active `DESIGN.md`; `.impeccable/design.json` contains schema-2 extensions, component previews and narrative without a duplicate primitive-token registry. Source remains the extraction authority:

- `packages/webassets/static/last-light.css`: shared dark/light semantic colors, Manrope, controls, focus, motion and detail/setup composition.
- `apps/player/internal/server/static/home.css`: in-flow Home feature, open resume rows, responsive shelves and browser tab editor.
- `apps/subtitles/internal/server/static/subtitle-dashboard.css`: task overview, compact coverage, file disclosures and mobile navigation.
- `apps/dashboard/internal/server/web/static/dashboard.css`: application rows, direct-link affordances, service context and matching pending rows.
- `apps/player/apps/native/Sources/Design/KinoTheme.swift`: adaptive dark/light and Increased Contrast colors.
- Native `CinemaHero.swift`, `MediaViews.swift`, `WatchPosition.swift`, `HomeScreen.swift`, `PlayerTabs.swift` and `TabPreferencesScreen.swift`: native composition, progress, loading, focus and personal navigation.

## Personal navigation

Player mobile web and Apple clients start with Home, TV Shows, Movies and Search plus persistent More. Viewers can add, remove and reorder one to four destinations; unpinned destinations remain accessible in More. Web preferences are scoped to the Viewer Profile in the browser; native preferences are scoped to the Viewer Profile on the device. Desktop web keeps its existing navigation. iPhone tabs, adaptable iPad tabs/sidebar and Apple TV focus remain system navigation.

The native editor places controls below each destination label and gives each reorder/remove icon an explicit 44×44pt minimum hit region. Web reorder uses the existing authored back icon rotated into direction, with the existing 44px controls, accessible action labels and focus restoration. These are implemented source details; native interaction remains a separate verification boundary.

## Product and visual invariants

Preserve existing API compatibility, account/session isolation, permissions, Direct First playback, offline downloads, direct Dashboard application links and truthful subtitle/service state. Buffering alone does not authorize transcoding. Unknown runtimes retain known saved position rather than invented percentages. Pending placeholders disappear when the underlying operation settles or fails.

Electric is the user's selected visual identity, not a claim that a color causes a universal psychological response. Relative contrast, readable hierarchy and proximity keep actions associated with the right object. State labels continue to carry meaning without relying only on color.

## Asset provenance

The Kinosail sail, caption and panel symbols retain the previously approved authored SVG geometry and now use Electric green. App authority lives in `apps/player/internal/server/static/icon.svg`, `apps/subtitles/internal/server/static/icon.svg` and `apps/dashboard/internal/server/web/static/icon.svg`; Player/Subtitles keep adjacent maskable variants. Header marks and documentation copies follow the same family. Web PNG icons are raster exports of their corresponding SVG sources, not separately painted artwork.

Player native authority lives under `apps/player/apps/native/assets/source/`: `icon.svg`, `tv-foreground.svg`, `tv-background.svg`, `top-shelf.svg` and `top-shelf-wide.svg`. `apps/player/apps/native/scripts/generate-brand-assets.sh` rasterizes and resizes with `sips`, exports the iOS app icon as RGB through `ffmpeg`, and populates the active Swift iOS/tvOS asset catalogs directly. TV foreground/background layers stay separate and Apple owns final icon masking.

The Electric decision raster is a design-selection artifact. Runtime media artwork comes from actual library responses and is not replaced by the comp's illustrative media. No new generative raster artwork is a production requirement of this direction.

## Evidence boundaries

The review packet is `.impeccable/review/electric/evidence.md`, the declared viewport/native captures and `finish-review.md`. Reported builds succeeded for the three Go web apps and iOS/tvOS simulators. Web observations cover populated Home at 430×932 and 1440×1000, no document horizontal overflow, browser tab add/reorder/reload/remove, subtitle disclosure and Dashboard filtering. They do not constitute a complete browser or accessibility audit.

Native captures show populated Movies on iPhone, iPad and tvOS. The Mac locked during the task, so final Home navigation, tab editing, touch, Siri Remote, VoiceOver and Dynamic Type interactions were not exercised. The independent reviewer confirmed the corrected native light metadata at 5.50:1 and the SVG web reorder icons. Native tab-editor controls now have the explicit 44pt source targets and stacked layout; narrow and large-text editor interaction remains unavailable because the Mac is locked. Final review details belong in the review record. The initial review also noted a web screenshot/source canvas-color discrepancy; a source declaration alone does not resolve rendered palette fidelity.

No tests, lint, quality suites, detector or design gates ran while `.gates-disabled` exists. Added regression tests are source-only until explicitly enabled and executed. Build success, source publication, deployed revision, container health, TLS, media playback, downloads, receivers and physical-device proof remain distinct. These design documents record the implementation and its durable visual decisions; they do not certify every product flow.
