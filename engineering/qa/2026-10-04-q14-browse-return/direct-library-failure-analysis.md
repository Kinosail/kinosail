# Q14 direct Library compatibility repair

The protected populated-browser run found a real public compatibility failure in the unchanged `happy-path-complete.ts` contract. After searching for Arrival, playing, saving progress and adding the title to My List, the journey clicks the exact Library link and expects Home's Resume and My List content. Q14 relabels the sole Back link to Back to search results for that valid query context. The direct Library action disappears. Root classified the failure at lines 105/66; this is not a strict-mode ambiguity, mocked response or source-only inference.

The original nine Q14 journeys require the unique `a.back` to return to the contextual browse URL. Replacing that action with a direct Home link would break the now-passing primary contract. Weakening either existing test would hide a real capability. Both actions must remain available when their destinations differ.

## Minimal presentation contract

Retain the context Back anchor and its canonical href validation. Add a separate Go-rendered direct Library anchor pointing to `/`, initially hidden, without the `back` class. Reveal it only after the existing bounded session record, current Viewer Profile, root browse URL and watch-destination checks pass and the parsed browse URL has a query. Keep it hidden for root Home, absent/corrupt state, another profile/tab and a mismatched Player destination. Hide it again before each Back update so a stale context cannot leave an extra action visible.

Search, Movies and Shows context labels stay unchanged. Other validated non-root contexts use the escaped Back to Library label rather than Library, so the direct Library action has a unique accessible name. That fourth ID is absent from the current Player catalogs. Append its English fallback to all 108 catalogs while preserving their entire existing content. This is fallback coverage, not translated locale or runtime proof.

The direct action uses existing button/quiet styles, normal native link behavior and the fixed root href. It does not acquire the context `back` selector or mark the saved record as returning. The root Home destination therefore uses its ordinary Resume/My List presentation. No shared CSS, account settings, Viewer Profile semantics, parser, player teardown, media startup or downloads change is needed. Context state and original restoration remain bounded and tab-owned.

## Test-first and verification boundary

The actual protected happy-path failure is the public runtime RED before this repair. Its assertions and all original nine, prepared Home and safety bodies remain unchanged. The Go response contract additionally requires the escaped fourth Back label, the separate initially hidden fixed-root anchor and the next effective main delivery token before production edits. Only the expected cache version changes; the exact canonical watch, no-referrer, Referer rejection, labels and Viewer Profile assertions remain intact.

Player's effective main token advances from 36 to 37 and Subtitles from 20 to 21 because the shared PWA behavior changes. The download token and content-derived Player playback asset stay untouched. Locale prefixes, all existing entries and all other source seams require exact static comparison.

Local Go, Node, browser, native and UI execution remain held. Responsive geometry, focus appearance, no-script/empty/corrupt/profile states, direct Library happy-path GREEN, contextual primary repeat, the remaining modes, both consumers and normal protected gates remain root-owned pending verification. A source fix and static identity checks do not establish those results. Keep every previous source/runtime evidence stamp intact.

## Source checkpoint

The test-first response expectations and this failure analysis were committed at `228c804f`, before any repair source edit. The source candidate adds only the fourth escaped label and the hidden fixed-root sibling in Player markup, the conditional visibility and distinct contextual fallback in `updateBack`, eight effective main-token replacements across the two consumers, and one English fallback entry in each Player catalog. The direct anchor does not enter the context Back click branch.

Offline comparisons verify the exact 112-file production ownership set. All 108 catalog arrays retain their 507 existing entries and byte prefixes, followed by the single new fallback. The unique `a.back`, original happy-path, nine journey bodies and helper, Home/safety specs, Go fixture, proof reporter, shared asset composition and Player progress/core bytes are preserved. No stylesheet or delivery driver changed. These checks establish source preservation only; no new public GREEN or rendered accessibility evidence has run.
