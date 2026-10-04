# R07: dialogue pairing

Public behavior: a preview compares transformed dialogue with its actual source cues, including removed and merged rows. Seek controls retain the correct side's timing. Imports or independently edited text must not imply verified correspondence.

Failure modes considered before production edits:

- Removing the first credit cue shifts every later comparison onto unrelated dialogue.
- Merging repeated cues shifts later rows and loses the original intervals being merged.
- Removing a credit between merged dialogue drops the removed cue from the review.
- Manual timing or automatic synchronization loses source identity.
- Repeated dialogue at different times matches the wrong occurrence.
- An installed file changes between the current review and conversion reads, making source indices refer to different bytes.
- An unrelated imported translation receives fabricated index correspondence.
- Findings filtering hides a warning in one side of a merged comparison.
- Comparison pagination drops removed, merged, or unpaired rows.
- Subtitle markup is interpreted as HTML rather than rendered safely as text.
- Preview creates or replaces a subtitle/recovery sidecar.

The API regression uses an installed synthetic SRT with edge credits, adjacent repeated dialogue, and later dialogue. It requests cleanup and a manual offset through `/api/v1/subtitle-library/{id}/preview`, verifies emitted source/proposed indices against independent expected dialogue, and verifies that the installed file is unchanged. It also imports an unrelated translation to require explicitly unpaired rows. Existing inspector tests verify preview/save/recovery and stale-edit rejection, but not before/after correspondence.

Browser verification must consume actual Server-rendered inspector HTML and actual API-generated preview JSON. A fixture browser journey is isolated transport/render proof, not populated-server E2E. It must inspect each paired row, removed/merged labels, safe text, seek targets, and responsive geometry.

Go provenance starts after the same normalization used by the installed-file review, and is tracked only for that source with automatic encoding and the same byte fingerprint. Imports, drafts, independently edited text, encoding overrides, and a changed source remain explicitly unpaired. This preserves each side without inventing correspondence. A concurrent external source rewrite is not yet covered by a controlled runtime test.

The native preview journey adds the transport/runtime gap: actual loopback Server responses, production request middleware, browser media loading and direct playback, and immutable installed data. It uses an authentication-disabled disposable installation and a read/preview-only fixture guard. It does not establish container, authenticated-session, hosted-browser, external-provider, or deployed-device behavior.

Independent review of exact `7015c885` identified a further rendering-budget failure. One comparison may contain 10,000 source indices. The original renderer creates a seek control and text for every index, despite its 40-comparison page budget. The reviewer measured 10,001 seek buttons and 30,010 created nodes in a lightweight Node VM against source SHA `e27a0b75f57c173ee230c52478606bfe1f9926e77253e7195b8dc47b29c86f54`. This is source/count evidence; actual browser performance was not measured.

The review repair starts with a public 10,000-cue preview regression and an isolated browser regression before production changes. The API must retain every cue's source index, dialogue and timing, its single merged proposed interval, immutable installed data, and no recovery copy. The browser requires a bounded initial DOM/control/text count and direct access to the final source cue and its own seek target.

The proposed presentation keeps normal two-source comparisons unchanged. Longer groups initially show a compact source sample, with one expanded group at a time and bounded source pages. Every source cue remains available, including through a page jump, without another write or fabricated match. Expansion, paging, collapse, main comparison paging, filters and new previews must retain truthful source labels and focus. No API cue limit changes or arbitrary cue removal are permitted.
