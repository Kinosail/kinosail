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
