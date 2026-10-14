# Browse-return selection geometry

## Evidence boundary

The Home-return journey separately verifies the focused action and restored scroll position.
Its geometry recorder selects the first document link with the requested `href`.
Duplicate navigation links can precede the focused destination card.

At `3b3e2ecd5cfe17bfa79f68a6a1aae33eaef5d408`, hosted run `38587212196` passes all three Home-return cases.
The Movies destination receipt records top and bottom as zero, before and after navigation.
The same case verifies focus on `.destination-card` and restores vertical scroll to 9,065 pixels.
Those passing assertions do not establish that the recorded rectangle belongs to the focused card.

Artifact `11906610067` has SHA-256 `1bb9efb7f742ab83c411035cf9c9c00b9320a5026b391200104605b10a0d6a46`.
The archive digest, tracked tree, 160 source pins, three child hashes, and process cleanup verify.
This is a QA evidence defect; the retained result does not establish broken product focus or scrolling.

## Regression before repair

The existing disposable Go Server journey now compares recorded bounds with the actual selected card.
The actual card must have nonzero height, and both recorded edges must match within half a pixel.
The check runs before leaving the Movies destination and after every Home return.
It retains the existing focus, scroll, cold-document, and request assertions.

No application response, document, scrolling behavior, or lifecycle is mocked.
The recorder remains unchanged at baseline commit `b98c5450e65e332f00ea5aa1cfa23a31448f91d8`.
Hosted run `38590117987` passes both Continue watching cases and rejects the Movies destination's recorded top edge.
The failure maps to `browse-return-home.spec.ts:66`, after the actual card's nonzero-height check.
The retained selection has zero top and bottom, while focus identifies the Movies destination.

Artifact `11907686622` has SHA-256 `739fef2c78e103b094bca71a3e7c35fd949f330652f3e24ca4fbc187e912d0bb`.
Its tracked tree, 160 input pins, three child hashes, and owned-process shutdown verify.
The runner classifies this geometry prerequisite failure as `prerequisite-blocked`.
It exits before its final source-unchanged check, so that additional boundary is not claimed.

## Repair

The recorder now prefers the focused link when its destination matches the requested selection.
It retains the existing fallback for observations without a matching focused link.
Focus and geometry now come from the same selected action in these Home journeys.
The application code and acceptance checks remain unchanged.

Hosted verification of the repaired recorder is pending.
Local application and browser execution remain held.
