# Kinosail UI/UX direction: Signal

Research snapshot: 2026-08-23.

This note proposes a distinctive interface direction for Kinosail's server-rendered Go + HTMX web app. It is a design brief, not an implementation specification. It preserves familiar media terminology, progressive enhancement, direct playback, and the existing one-server product boundary.

## Product calibration: the Signal system

The rendered editorial **Afterglow** exploration was rejected as dated. The final direction is **Signal**: a media-first product interface that translates the strongest patterns shared by Robinhood, Linear, Mercury, Stripe, Ramp, Vercel, Superhuman, Notion Calendar, Attio, and Framer without copying any one product.

- black, warm neutrals, and one electric yellow-green signal color;
- crisp system sans typography with compact, deliberate density;
- a lightweight desktop top shell and thumb-reachable mobile utility dock;
- modular surfaces, fine separators, restrained radii, and precise states;
- generous macro spacing paired with information-dense controls; and
- a chart-like visual grammar—lines, rings, progress, and data hierarchy—applied to a private media library.

This calibration tracks Robinhood's own description of its current identity: simplified iconography, a neutral palette with purposeful neon, modular layouts, and a “less is more” product experience. Its earlier design account also emphasizes content-centric design, typography, familiar mobile conventions, and simple, usable interactions. ([Robinhood visual identity](https://robinhood.com/us/en/newsroom/a-new-visual-identity/); [Robinhood design story](https://robinhood.com/us/en/newsroom/the-top-secret-robinhood-design-story/))

Two additional implementation checks came directly from current product guidance. Linear's 2026 refresh argues that navigation should recede, unequal elements should not receive equal visual weight, and structure should be “felt not seen” through softer borders; Vercel's Geist system separates page, component, border, and text color roles and treats typography as explicit size/weight/spacing combinations. Kinosail applies those ideas through a quiet top shell, thin low-contrast separators, compact controls, and a strict two-surface hierarchy. ([Linear design refresh](https://linear.app/now/behind-the-latest-design-refresh); [Vercel Geist colors](https://vercel.com/geist/colors); [Vercel Geist typography](https://vercel.com/geist/typography))

## Original research foundation

Build **Signal**: a quiet, precision-made media workspace that makes a personal library feel fast, legible, and alive—not another streaming-service clone and not a generic dark SaaS dashboard.

The signature is deliberately small and repeatable:

- an ink-black, warm-neutral shell with one electric yellow-green **signal** color;
- title artwork used as illumination, never as unreadable wallpaper: a constrained, low-chroma color sampled from the selected artwork may tint only decorative regions;
- a thin horizontal **lightline** that anchors the active section and progress states;
- exceptionally crisp sans typography throughout; and
- sharp hierarchy, generous negative space, and almost no ornamental containers.

This balances what Lavie and Tractinsky found to be two distinct dimensions of perceived web aesthetics: clean, clear “classical” aesthetics and creative, original “expressive” aesthetics. Kinosail should earn clarity from its system and expression from a single ownable motif, not from a pile of effects. [Lavie and Tractinsky, *Assessing dimensions of perceived visual aesthetics of web sites*](https://doi.org/10.1016/j.ijhcs.2003.09.002)

The governing test is: **unfamiliar in emotion, familiar in operation**. Tuch et al.'s two experiments found that low visual complexity and high prototypicality produced the strongest first-impression ratings, with effects observable after very brief exposure. Kinosail should therefore look unmistakably like a media library at first glance while feeling unlike its competitors in art direction. [Tuch et al., *The role of visual complexity and prototypicality regarding first impression of websites*](https://research.google/pubs/the-role-of-visual-complexity-and-prototypicality-regarding-first-impression-of-websites-working-towards-understanding-aesthetic-judgments/)

## Evidence translated into rules

| Evidence | Kinosail rule |
| --- | --- |
| People can form stable visual-appeal judgments after extremely brief exposure; first-frame composition matters. [Lindgaard et al., 2006](https://doi.org/10.1080/01449290500330448) | The first viewport gets one obvious destination, one dominant content region, and no admin forms or decorative competition. |
| Enclosing elements in a common region can override proximity and similarity and can support hierarchical grouping. [Palmer, 1992](https://pubmed.ncbi.nlm.nih.gov/1516361/) | Let spacing and a shared tonal field define shelves and action groups. Do not draw a rounded box around every object. |
| Excess or disorganized display items can impair recognition, segmentation, and visual search. [Rosenholtz, Li, and Nakano, 2007](https://pubmed.ncbi.nlm.nih.gov/18217832/) | Remove low-value chrome, reduce simultaneous button styles, and reveal secondary actions on focus or in the detail view. |
| In an eight-menu experiment, weakening the semantic match between labels and goals reduced success from 89% to 11% and increased first-click time from 12 to 31 seconds. [Tselios, Katsanos, and Avouris, 2009](https://dl.ifip.org/db/conf/interact/interact2009-2/TseliosKA09.pdf) | Every shelf name, filter, and card subtitle must explain what lies beyond it. Preserve plain labels such as **Movies** and **Continue watching**; keep nautical language in the visual brand, not the information architecture. |
| Means–ends problem solving consumes processing capacity that is then unavailable for learning schemas. [Sweller, 1988](https://doi.org/10.1207/s15516709cog1202_4) | This is an inference, not a direct UI finding: keep browse, resume, search, and play paths conventional so people spend attention choosing a title rather than learning the interface. |
| Coherent, meaningfully named rows help viewers decide whether to explore or skip a group; device-aware row formatting matters. [Netflix, *Learning a Personalized Homepage*](https://netflixtechblog.com/learning-a-personalized-homepage-aa8ec670359a) | Retain shelves, but rank utility first, give every shelf a useful reason, deduplicate titles, and end each shelf with **See all**. Do not imitate Netflix's volume or opaque personalization. |
| Multi-carousel recommenders have two-dimensional positional bias: item and row position both affect attention. [Bendada et al., 2022](https://pmc.ncbi.nlm.nih.gov/articles/PMC9218726/) | Never bury **Continue watching**, **My list**, downloads, or server warnings below promotional material. Row order must be stable and user-serving. |

## Information architecture and core flows

### Global navigation

Use one adaptable navigation spine with the same destinations and order everywhere:

1. Home
2. Movies
3. Shows
4. Music
5. More: Books, Photos, Live TV

Search, viewer profile, and settings are utilities, visually separated from content destinations. On wide screens the spine is a slim left column; on narrow touch screens the primary destinations become a bottom bar plus **More**; on ten-foot/keyboard layouts it becomes a focusable edge rail. Do not rename destinations to “Decks,” “Voyages,” or other branded metaphors—the label is the information scent.

Move playlist/collection creation and sort forms out of the home feed and into contextual browse tools. The current home mixes the main library with two creation forms and sort controls ([current template](../../internal/server/server.go)); that makes the opening page serve incompatible jobs at once.

### Home: resume, orient, discover

The first viewport follows intent, not promotion:

- **Continue watching** is first when it exists. Use landscape resume cards with episode/title, remaining time, and a persistent progress line. The explicit play control resumes; the adjacent title/details target opens the detail view. These must be sibling controls, never nested interactive elements.
- Otherwise lead with **Recently added**, not a random cinematic hero.
- Then **My list**, **Recently added**, and at most two context shelves such as “Unwatched movies” or “Next episodes.” Provide **See all** rather than rendering the entire catalog into the home page.
- A featured canvas is allowed only when it represents a defensible user intent (for example, the single most recently paused title). It is static until the user acts: no rotating hero, autoplay trailer, or surprise audio.

### Browse and search: scan, narrow, compare

Browse pages should be true catalog tools: a compact sticky title/filter row, visible result count, chips only for active filters, and a responsive poster grid. Preserve the scroll position and filters through back navigation. Offer a density choice only if real testing shows a need; do not add one preemptively.

Keep search visible on desktop and one tap away elsewhere. HTMX can continue returning server-rendered results, but results should be grouped by type and put exact title matches first. A search result needs title, year, type, and one discriminating cue such as show/season, artist/album, or match reason. Empty states should say what was searched and offer a single recovery action.

### Detail: decide without hunting

Use a two-column editorial composition on wide screens and a single reading flow on mobile:

- poster or restrained backdrop crop;
- title, year, runtime, rating, genres, and synopsis;
- one signal-green **Play/Resume** action;
- quiet secondary actions: My list, Download, Watch together;
- playback options disclosed immediately below the primary action; and
- episodes/chapters as scannable rows with thumbnails, duration, progress, and watched state.

Never put essential text directly on uncontrolled artwork. If artwork sits behind text, use a guaranteed opaque ink scrim and test the final pixels; artwork-derived colors remain decorative and must not determine text or focus colors.

## Visual system

### Palette

| Token | sRGB fallback | Role | Tested WCAG contrast |
| --- | --- | --- | --- |
| Ink | `#070B10` | page canvas, text on signal | — |
| Hull | `#111820` | navigation and quiet raised regions | — |
| Plate | `#18212B` | controls and selected metadata panel | — |
| Screen | `#F4F1E8` | primary text, like warm projection light | 17.47:1 on Ink; 15.82:1 on Hull |
| Mist | `#A9B3BD` | secondary text | 9.27:1 on Ink; 8.40:1 on Hull |
| Signal | `#FF684D` | one primary action, active lightline, progress | 6.90:1 against Ink in either direction |
| Focus | `#76D7FF` | keyboard/remote focus only | 12.15:1 on Ink; 11.00:1 on Hull |
| Ready | `#56D68A` | ready/healthy status, always with text/icon | 10.69:1 on Ink |
| Attention | `#FFC857` | warning status, always with text/icon | 12.83:1 on Ink |

These ratios use the WCAG 2 relative-luminance algorithm. WCAG 2.2 requires at least 4.5:1 for normal text, 3:1 for large text, and 3:1 for essential non-text UI cues; color alone cannot carry meaning. [WCAG 2.2 §§1.4.1, 1.4.3, 1.4.11](https://www.w3.org/TR/WCAG22/)

Author tokens in OkLCh with sRGB fallbacks so tonal steps can be adjusted perceptually while hue remains stable. CSS Color 4 describes Oklab as more perceptually uniform than CIE Lab and shows stronger hue constancy in OkLCh. [CSS Color 4 §9.2](https://www.w3.org/TR/css-color-4/#ok-lab)

Do not use violet/cyan gradients as ambient decoration. They are already the dominant language of generic AI-product interfaces. Coral is a sparse navigational signal; the collection's artwork supplies the chromatic variety.

### Type, shape, and spacing

- Use one locally hosted variable serif such as **Newsreader** only for media-title display and one neutral sans for UI/body. If font cost or licensing is not acceptable, use a system-serif display stack; never block the first render on a third-party font host.
- Establish five intentional type roles: display title, page title, section title, body, metadata. Keep card titles at a readable size and two lines where ambiguity would otherwise be created; do not truncate every title to one line.
- Use an 8 px spacing rhythm with 4 px optical corrections. Section separation should be materially larger than card gaps so proximity performs the grouping.
- Use restrained 6–12 px radii. Reserve pills for compact status/filter tokens. Poster artwork keeps its native 2:3 ratio; resume cards use 16:9 or a purposeful landscape crop.
- Borders are state or separation tools, not decoration. Prefer negative space and tonal changes over nested translucent panels.

### Components

- **Navigation spine:** opaque, quiet, fixed destination order; text labels remain visible on desktop and ten-foot layouts.
- **Shelf:** semantic heading, optional one-line rationale, horizontal list, visible previous/next controls where applicable, exposed partial next item, and **See all**. No automatic movement. W3C notes carousel content can be hard to discover and requires keyboard operation, understandable focus management, announcements, and user control over movement. [WAI Carousels Tutorial](https://www.w3.org/WAI/tutorials/carousels/)
- **Poster tile:** artwork, title, one useful metadata line, and distinct default/hover/focus/pressed/selected states. Hover may reveal details but may not be the only way to access them.
- **Resume tile:** landscape artwork, persistent progress lightline, explicit resume action, remaining time, and episode context.
- **Focus state:** a two-color ring—dark separation under a 3 px Focus outline—plus modest surface lift, so it survives both light and dark artwork. On TV/keyboard, a 1.025–1.05 scale may reinforce it if layout reserves the space; focus never equals activation. Apple and Android both treat focus as the essential remote/keyboard state and require it to remain visually clear. [Apple focus guidance](https://developer.apple.com/design/human-interface-guidelines/focus-and-selection/), [Android TV focus system](https://developer.android.com/design/ui/tv/guides/styles/focus-system)
- **System state:** human explanation plus stable icon and color: scanning, direct, transcoding, waiting, offline, ready. Do not expose implementation jargon as the main message.

## Responsive and ten-foot behavior

Use content-driven modes, not a desktop layout squeezed through viewport breakpoints:

| Mode | Behavior |
| --- | --- |
| Compact touch | Bottom primary navigation; two-column poster grid; 44 px minimum product target; edge-to-edge detail artwork; filters in a labeled sheet; no hover dependency. |
| Medium pointer | Collapsible navigation spine; 3–5 grid columns; shelves with visible arrow controls; detail panel beside artwork when space permits. |
| Wide pointer | Persistent text navigation; max reading widths; larger gutters rather than more chrome; selected-item context may occupy a calm side panel. |
| Ten-foot / keyboard | 56 px controls, large type, 5–6 visible shelf items, directional focus, vertical axis between shelves and horizontal axis within a shelf, predictable Back behavior. |

Android TV explicitly recommends assigning the vertical axis to categories and the horizontal axis to their contents while avoiding nested, hard-to-reach focus paths. Apple notes viewers can be eight feet or more from a TV and recommends edge-to-edge artwork that remains clear and legible. [Android TV navigation](https://developer.android.com/training/tv/get-started/navigation), [Apple tvOS guidance](https://developer.apple.com/design/human-interface-guidelines/designing-for-tvos/)

Small direct TV studies reinforce the need to validate this rather than treating focus polish as decoration: older viewers were substantially slower and often misunderstood edge-of-screen color-key cues in an eye-tracking study, while a 20-participant remote-control prototype identified recognizing active focus as a major problem before iteration. Put explicit, high-contrast action cues beside the selected content and test them from a couch with age-diverse viewers. [Obrist et al., 2007](https://link.springer.com/chapter/10.1007/978-3-540-72559-6_8), [Fernandes et al., 2018](https://doi.org/10.1145/3233824.3233851)

Use CSS container queries for reusable tiles/shelves so a component responds to the space it actually receives; the W3C Containment Level 3 draft defines inline-size containers and `@container` conditions for this purpose. [CSS Containment 3 §4](https://www.w3.org/TR/css-contain-3/#container-queries)

## Motion, accessibility, and control

- Motion has a job: preserve spatial continuity, confirm focus, or communicate a state transition. It never fills dead air. Use opacity/color for frequent states and short transform transitions only for infrequent spatial changes.
- Aim for roughly 120–180 ms for focus/press feedback and 180–240 ms for panel transitions, then tune on real devices. These are product hypotheses, not standards. Interaction must remain available while motion runs.
- Under `prefers-reduced-motion: reduce`, remove parallax, scaling, smooth scrolling, and spatial transitions; use immediate state changes or brief opacity changes. W3C specifically identifies unnecessary interaction animation and parallax as potential vestibular triggers. [WCAG animation guidance](https://www.w3.org/WAI/WCAG22/Understanding/animation-from-interactions)
- No autoplaying hero, preview, or audio. If future moving content begins automatically and persists, it needs pause/stop/hide controls; the better default is user initiation.
- Expose every available caption/subtitle, audio-description, and audio-language track through labeled keyboard/remote-operable controls. Do not treat captions as a player afterthought. [WCAG time-based media](https://www.w3.org/TR/WCAG22/#time-based-media)
- Set a 44×44 CSS px product target baseline even though WCAG 2.2 AA's normative minimum is 24×24 px with exceptions. [WCAG target-size guidance](https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum.html)
- Test 200% zoom and reflow down to 320 CSS px without lost content or page-level two-dimensional scrolling, complete keyboard operation, logical focus order, screen-reader names/states, forced-colors mode, and color-vision simulations. Status and progress need text or an icon in addition to hue. [WCAG reflow guidance](https://www.w3.org/WAI/WCAG22/Understanding/reflow.html)

## Performance is part of the aesthetic

Keep server-rendered HTML as the complete experience and let HTMX enhance search, filtering, shelves, and detail transitions. Do not introduce a client-side design-system runtime merely to animate the shell.

Set measurable budgets at the 75th percentile on mobile and desktop: LCP ≤2.5 s, INP ≤200 ms, and CLS ≤0.1—the current “good” Core Web Vitals thresholds. [Google, Core Web Vitals thresholds](https://web.dev/articles/defining-core-web-vitals-thresholds)

- Emit explicit image dimensions/aspect ratios to prevent layout shift.
- Eagerly load only the first meaningful artwork/LCP candidate; lazy-load offscreen posters. Field analysis found that eager in-viewport images plus liberal below-fold lazy loading can improve both bytes and LCP compared with lazy-loading everything. [web.dev, *The performance effects of too much lazy loading*](https://web.dev/articles/lcp-lazy-loading)
- Serve poster, resume, and backdrop sizes appropriate to their rendered slots with `srcset`/`sizes`; never download backdrop-scale art for a phone poster.
- Keep the initial DOM bounded: render the first useful shelves, then progressively request lower shelves. Consider `content-visibility:auto` only after measuring rendering cost and accessibility behavior.
- Preserve focus and scroll position across HTMX swaps; use `aria-live` only for concise result/status changes, not the entire moving catalog.

## Explicit anti-patterns

Reject these even if they photograph well in a mockup:

- purple/blue aurora gradients, glassmorphism, glowing orbs, and indiscriminate blur;
- every region as a floating rounded card, every label as a pill, and every icon inside a circle;
- a giant marketing hero that pushes resume/search below the fold;
- autoplay previews, rotating banners, sound on focus, or animated backgrounds;
- text or controls laid over arbitrary artwork without a guaranteed contrast layer;
- poster walls with identical hierarchy, repeated titles, or one-line ellipsis everywhere;
- hidden icon-only navigation, mystery-meat nautical labels, hover-only actions, or focus that moves unexpectedly;
- dozens of micro-animations, springy scaling on every tile, and motion that blocks input;
- client-rendered skeleton screens for HTML the server can return immediately;
- fabricated “Because you watched” explanations or opaque recommendation theater; and
- hiding server health, direct/transcode state, or failures behind vague colored dots.

## First design slice and validation gate

Prototype only three surfaces first: Home, one media detail page, and responsive navigation. Use representative libraries with missing art, long titles, mixed media, an active resume item, no resume item, loading, empty, and server-offline states.

The direction earns implementation only if moderated tasks show that new and returning viewers can:

1. resume the last title without scanning promotional content;
2. find a known movie through browse and search;
3. identify what will happen before pressing Play, Download, or Delete;
4. traverse every target by keyboard/remote without losing focus; and
5. understand loading, empty, direct/transcode, and failure states without relying on color.

Measure time to first play, wrong turns, search reformulations, focus losses, and subjective ratings for both **clear/ordered** and **original/distinctive**. That last pairing directly tests the classical/expressive tension rather than treating “award-caliber” as a purely visual judgment.
