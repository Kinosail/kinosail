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
Hosted baseline execution is pending. The recorder remains unchanged for that baseline.
Local application and browser execution remain held.
