# Fast navigation in long media libraries

Research snapshot: 2026-08-24.

## Decision

Use a **layered known-title path**, not one replacement for scrolling:

1. Keep instant search as the primary path when someone knows any meaningful part of a title: typing `gri` should put **The Grinch** first.
2. Add a locale-aware alphabetic jump index to the title-sorted **All Movies** and equivalent long-library views. It is the fast, low-input fallback when the person knows the filing letter: choose **G**, then scan the G section.
3. Keep stable title grouping and lightweight filters for browsing or disambiguation. Do not make facets a prerequisite for a known-title lookup.
4. Keep bounded server pages and progressive loading for performance, but make every letter directly addressable. Infinite scrolling and virtualization are delivery techniques, not findability features.

This is a hybrid recommendation because the evidence distinguishes tasks. Search is the simpler interface for a precisely known item; alphabetic indexing materially outperforms ordinary mobile scrolling in directly relevant name-finding studies; facets help more when the target is only partly known or the task is exploratory. No study located directly compared search with an A–Z index in a movie catalog, so Kinosail should validate the combined design with its own device-level usability test.

## What the measured evidence says

These are empirical results, not general design rules.

### Known titles are a search-shaped task

- Four transaction-log studies covering more than 4.2 million library-search sessions found that 38–57% of sampled queries were known-item queries. Titles or title fragments were the most frequent query elements, and sessions were generally short. The collection was bibliographic rather than cinematic, but the task closely matches “I know it is Grinch.” [Schultheiß et al., 2020](https://doi.org/10.1016/j.acalib.2020.102202)
- A set of user studies comparing interfaces for known-item and exploratory tasks found simple interfaces more effective for known-item search, while richer interfaces better supported exploratory work. [Diriye, 2012](https://discovery.ucl.ac.uk/id/eprint/1343928/)
- In a small recipe-site study, users preferred direct search when they knew precisely what they wanted and a browsing interface for a browsing task. In a later 19-person Flamenco study of a 40,000-image collection, 16 of 19 participants preferred the powerful faceted interface overall, but the authors still concluded it had more functionality than a direct search needed and that a simpler interface seemed more efficient for an exact target. [Hearst et al., 2002](https://bailando.berkeley.edu/papers/cacm02-final.html)

Implication: Kinosail's first result after a short title fragment matters more than exposing every possible refinement. Rank normalized exact title, title prefix, title token/prefix, then title substring before matches found only in people, plot, studio, or genre. Show the complete title and year so remakes can be distinguished. Typo tolerance can follow, but it should not make exact results unstable.

### An alphabet index is substantially better than long scrolling on touch

- A mobile-phone name-location experiment compared ordinary sliding, a scrollbar, and an alphabetic index. The alphabetic index was fastest regardless of target location; its subjective ratings improved over the session while sliding's declined. This is directly relevant to locating a known title in an ordered list, although the content was contact names rather than posters. [Li et al., 2023](https://doi.org/10.1080/10447318.2022.2041883)
- A tablet study compared seven list-navigation techniques on lists of different lengths. Several techniques performed statistically similarly, but participants preferred inertial direct manipulation with an alphabetically labeled index, which also performed well. It supports combining normal scrolling with an index rather than replacing scrolling. [Breuninger, Popova-Dlugosch, and Bengler, 2013](https://doi.org/10.3182/20130811-5-US-2037.00064)

Implication: A–Z is not dated ornament. It is an efficient secondary accelerator for an ordered, bounded catalog. It should coexist with ordinary scrolling and search.

### Autocomplete helps only when its ordering is good

- Two known-item studies of thesaurus term completion found autocomplete improved keyword quality. The best organization depended on the collection: grouped/composite suggestions were faster for place names, while alphabetical suggestions worked better for WordNet. [Amin et al., 2009](https://ir.cwi.nl/pub/13282)
- A controlled eye-tracking study found a strong position bias in query-autocomplete examination and use; changing ranking quality also affected task-completion behavior. [Hofmann et al., 2014](https://www.microsoft.com/en-us/research/publication/an-eye-tracking-study-of-user-interactions-with-query-auto-completion/)
- A mobile text-entry experiment found that more assertive suggestions reduced keyboard actions and were preferred, but the attention and selection costs worsened average completion time. That is a warning against an elaborate suggestion UI merely because suggestions look modern. [Quinn and Zhai, 2016](https://research.google/pubs/a-costbenefit-study-of-text-entry-suggestion-interaction/)

Implication: Kinosail can initially let the existing live result grid serve as type-ahead instead of adding a second, competing suggestion popup. If a compact suggestion popup is later tested, keep it to a few excellent title results and implement the standard combobox interaction completely.

### Facets and dynamic filters are valuable, but solve another problem

- Eighteen participants completed real-estate tasks significantly faster with dynamic graphical queries than with a natural-language system or paper listings. [Williamson and Shneiderman, 1992](https://doi.org/10.1145/133160.133216)
- In the 19-participant Flamenco study, participants used hierarchical facet drilling more than search-within, rarely restarted, and reported a strong sense of control. The study centered on open-ended image browsing, not exact title lookup. [Hearst et al., 2002](https://bailando.berkeley.edu/papers/cacm02-final.html)

Implication: year, genre, watched state, collection, and perhaps runtime are useful for “find a Christmas movie” or “which Grinch version?” They should refine a result set and display result counts, not stand between the user and a title they already know.

## Platform and accessibility guidance

The following sources are platform guidance or standards. They are not comparative performance measurements.

### Alphabetic index and locale

Apple explicitly documents the trailing alphabet index as a control that jumps to the corresponding list section. It also warns that a trailing index can conflict with controls placed at the trailing edge of each row. [Apple Lists and tables](https://developer.apple.com/design/human-interface-guidelines/lists-and-tables)

Do not treat A–Z as universal. Unicode's collation model and ICU's `AlphabeticIndex` exist because correct sort order and section labels vary by locale and script; labels can be sequences rather than single Latin letters. [Unicode CLDR collation indexes](https://www.unicode.org/reports/tr35/tr35-collation.html#Collation_Indexes), [ICU collation guide](https://unicode-org.github.io/icu/userguide/collation/)

Kinosail should therefore:

- derive both ordering and bucket assignment from the same active-locale collation;
- use a `#`/other bucket for numbers and symbols only where that is appropriate for the locale;
- file on a normalized **sort title**, while preserving the display title; prefer the explicit Kodi-compatible NFO `<sorttitle>` value, which exists specifically to change sorting without changing the displayed movie title. [Kodi movie NFO](https://kodi.wiki/view/NFO_files/Movies)
- when no explicit sort title exists, apply only a documented language-aware initial-article policy. Library of Congress MARC likewise represents initial articles as nonfiling characters that are disregarded for sorting, but blindly removing strings such as `A` or `The` is unsafe when the title language is unknown. [MARC 21 title statement](https://www.loc.gov/marc/bibliographic/bd245.html), [MARC nonfiling-character analysis](https://www.loc.gov/marc/marbi/dp/dp102.html)
- keep search independent of filing rules, so `grinch` matches **The Grinch** even if the active sort is Newest; and
- fall back to search plus ordinary pagination when trustworthy index labels are unavailable, rather than showing a misleading hard-coded Latin index.

The repo already depends on `golang.org/x/text`, so locale-aware collation can reuse a vetted Go-native dependency. Locale-specific alphabetic label generation still needs a deliberate design; it must not be guessed from the first byte or rune.

### Keyboard and screen reader

WCAG 2.2 requires keyboard operation and a meaningful focus order. Its AA target-size criterion requires at least 24 by 24 CSS pixels or sufficient spacing, with defined exceptions. [WCAG 2.2](https://www.w3.org/TR/WCAG22/), [Focus order](https://www.w3.org/WAI/WCAG22/Understanding/focus-order.html), [Target size](https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum.html)

For the web adapter:

- expose the alphabet as a labeled navigation region, “Jump to title”; use real links so URLs, browser Back, open-in-new-tab, and no-JavaScript operation work;
- keep all visual bucket positions stable; empty buckets may be visibly unavailable but must not create dead keyboard stops;
- after an intentional jump, focus the letter heading or first result and announce a concise result summary, such as “G, 42 movies”; do not announce every card;
- preserve the search-field focus while live results update, support `Escape` to clear/dismiss where appropriate, and never intercept normal text-editing keys;
- use semantic headings for each letter so screen-reader heading navigation is another fast path;
- retain a visible focus state over variable artwork and preserve focus/scroll when returning from details; and
- if a popup autocomplete is added, follow the WAI-ARIA combobox contract for arrow keys, `Enter`, `Escape`, names, active descendant, and result state. [WAI-ARIA combobox pattern](https://www.w3.org/WAI/ARIA/apg/patterns/combobox/)

A poster wall should remain a semantic collection of links unless Kinosail commits to the full ARIA grid keyboard model. The WAI grid pattern can shorten the tab sequence and adds arrow, Home, End, Page Up, and Page Down navigation, but it also requires application-managed focus. It is useful for a TV-grade spatial grid, not a role to add only for styling. [WAI-ARIA grid pattern](https://www.w3.org/WAI/ARIA/apg/patterns/grid/)

### Mobile

On a tall native list, an edge index is established platform behavior. On the web, 26 tiny stacked letters can violate target-size and spacing requirements. Use responsive presentations of the same operation:

- wide/tall touch: a sticky edge index or scrubber with a large floating current-letter preview;
- compact touch: one 44-pixel-or-larger **Jump to…** button opening a 5-by-6 locale-label grid, rather than 26 undersized tap targets; and
- all touch sizes: keep the search entry one tap away, start filtering as text is entered, and make normal inertial scrolling continue to work.

Apple recommends giving important search a primary position and providing suggestions/completions to reduce typing. [Apple Searching](https://developer.apple.com/design/human-interface-guidelines/searching)

### TV and remote

Android TV says users frequently have specific content in mind and that a search interface can reach it faster than browsing a large catalog; its standard TV search supports voice input. TV navigation otherwise advances one D-pad interaction at a time and requires predictable, visible focus. [Android TV in-app search](https://developer.android.com/training/tv/discovery/in-app-search), [Android TV navigation](https://developer.android.com/training/tv/get-started/navigation), [Android TV focus system](https://developer.android.com/design/ui/tv/guides/styles/focus-system)

Apple likewise separates focus from activation on tvOS and requires every onscreen element to be reachable with directional input. [Apple Focus and selection](https://developer.apple.com/design/human-interface-guidelines/focus-and-selection/)

For TV:

- make Search/voice the first known-title route;
- expose **Jump to…** as a coarse 5-by-6 focus grid so reaching **G** takes a few D-pad moves, not dozens of vertical presses;
- use the vertical axis between sections and the horizontal axis within a poster row;
- when a letter is activated, put focus on the first matching poster and scroll it fully into view;
- keep focus distinct from activation and keep Back predictable: close the letter chooser first, then return to the prior screen/position; and
- never auto-move focus because more results loaded.

## Sorting, grouping, and poster presentation

Keep **Title**, **Newest**, and **Year** as explicit sort choices. Show the alphabet index only when Title sort is active; choosing a letter may explicitly switch to Title sort, but the UI must say so and the URL must record it. Under Title sort, render sticky letter headings and enough whitespace to make section changes visible.

Keep the poster grid. Apple recommends text-oriented lists for scanability but collections for large numbers of images; movies benefit from both recognizable artwork and always-visible titles. [Apple Lists and tables](https://developer.apple.com/design/human-interface-guidelines/lists-and-tables) Do not add a list/grid density toggle before testing shows that it solves a real problem.

Keep user-intent shelves—Continue Watching, My List, Recently Added—separate from the stable All Movies catalog. They answer “what is likely next,” while title search and the alphabet answer “where is the thing I named.”

## Pagination, infinite loading, and virtualization

Virtualization improves rendering cost; it does not tell a user where **G** is. Chrome documents that larger DOMs increase initial-render and update work. The CSS Containment specification identifies `content-visibility: auto` as a possible alternative to complicated virtual-list techniques for long offscreen content, while retaining semantic relevance for search and assistive technologies. [web.dev DOM size and interactivity](https://web.dev/articles/dom-size-and-interactivity), [CSS Containment Level 2](https://www.w3.org/TR/css-contain-2/)

Kinosail should keep bounded server pages and load only the selected bucket/page. Do not sequentially fetch A through F before answering a G jump. If only part of a logical set exists in the DOM, expose its position and total correctly where the chosen ARIA pattern requires `aria-posinset` and `aria-setsize`. [WAI-ARIA 1.2](https://www.w3.org/TR/wai-aria/#aria-posinset)

Prefer server paging plus measured `content-visibility: auto` over a JavaScript-only virtual scroller for the first implementation. It preserves real links, browser find, no-JavaScript use, and simpler focus restoration. Introduce windowing only if traces from low-powered supported devices show that bounded pages plus containment cannot meet the interaction budget.

## Kinosail implementation implications

Kinosail already has most of the foundation:

- [server.go](../../apps/player/internal/server/server.go) contains an always-visible search field that updates the main results after 250 ms;
- [browse.go](../../apps/player/internal/server/browse.go) centralizes strict web/API query parsing, filtering, title/newest/year sorting, offsets, and 100-item default pages;
- [pwa.js](../../packages/webassets/static/pwa.js) progressively fetches the next page while retaining a real **Load more** link fallback; and
- [performance_benchmark_test.go](../../apps/player/internal/server/performance_benchmark_test.go) already exercises a 10,000-item library.

The smallest coherent capability slice would be:

1. Extend the shared browse operation and `GET /api/v1/library` with one strictly validated, locale-aware bucket key; return available bucket labels/counts and active bucket metadata. The web adapter must call that same operation, consistent with [ADR 0005](../../apps/player/engineering/adr/0005-expose-complete-api-from-one-server-container.md).
2. Ingest explicit NFO `<sorttitle>` metadata, then make title ordering locale-aware and deterministic with ID as the final tie-breaker. Define and test any fallback article rule separately from display titles.
3. Rank text searches for known titles instead of returning every metadata substring in raw title order. Bound query work and response cardinality as the current browse contract does.
4. Render the alphabet navigation, section heading, result count, and deep-linkable URL on the web. Add responsive compact and TV presentations without changing the application operation.
5. Preserve current progressive paging inside the selected bucket; abort stale live-search and bucket requests, and never append results from a prior query state.

Strict negative/no-side-effect coverage is required for unknown/duplicate/oversized/malformed bucket values, bucket/sort conflicts, unsupported locale labels, unauthorized Viewer content, and offsets outside the selected bucket. API and web tests should prove identical visible item sets and bucket counts for the same Viewer policy.

## Validation plan

Research transfer is not proof of Kinosail usability. Run a small within-subject study on a populated 1,000-plus-title fixture:

- tasks: exact title known, partial title known, first letter known, misspelled title, remake disambiguation, and exploratory genre/year lookup;
- conditions: current scrolling, search, alphabet jump, and the combined interface;
- devices: keyboard/pointer, compact touch, screen reader plus keyboard, and TV D-pad/voice where supported;
- measures: success, time to first correct activation, keystrokes/taps/D-pad presses, wrong openings, query reformulations, focus loss, and single-ease rating; and
- retention: return from details and repeat the lookup to verify stable position and Back behavior.

Also benchmark the current and proposed operation at 100, 1,000, and 10,000 visible titles. Record server latency and allocations, response bytes, DOM nodes, INP/presentation delay, image requests, and letter-jump latency on the slowest supported class of device. Treat targets such as “first correct known-title result in under five seconds on touch/keyboard” as product hypotheses until measured, not as research-derived thresholds.

## Bottom line

For “I know it is Grinch,” the best Kinosail experience is: tap Search and type `gri`, or tap **Jump to… → G** when typing is inconvenient. Both routes should be direct, stable, deep-linkable, locale-aware, keyboard/remote operable, and independent of how many earlier posters have loaded. Filters remain available for “which Grinch?” and exploratory browsing, while bounded paging keeps the interface fast.
