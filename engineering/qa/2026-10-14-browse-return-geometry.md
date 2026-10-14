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

At `46557737e7ad6e4c510d8fbab7d9ba82221373c1`, hosted run `38590788569` passes all three cases without retries.
The Movies card records top 409.28125 and bottom 497.28125 before and after navigation.
Both Continue watching controls retain their original bounds. All three cases have zero vertical scroll drift.
Artifact `11906679259` has SHA-256 `c733f71fb5aebd84f214a502c330b84cfcdf7b1c011bfbd28e8a124b7aaa0e04`.
The tracked tree, 160 source pins, three child hashes, final source identity, and owned-process shutdown verify.
Independent source review finds no concrete blocker.

The broader Movies-return run `38591287593` stops during dependency preparation with an HTTP/2 stream error.
No journey executes in that attempt. The failed job is retried without changing its source, deadlines, or assertions.
The unchanged failed-job replay passes both primary Movies cases twice, without browser retries.

Reconciliation with merged PR #602 produces `55d3da01094769048ad4839b86a332cf0f1f864d` with the same tracked tree as `46557737`.
No application or test source changes during that reconciliation.
The existing first-link fallback remains ambiguous when the requested destination is not focused.
The verified repair concerns the matching focused action.
Local application and browser execution remain held.


## Wider verification and delivery

All seven affected suites pass: 21 distinct cases across 23 executions.
Home and primary use repair commit `46557737`; all 160 selected inputs and the Player subtree match `7fce6b94`.
The remaining suites run directly at `7fce6b94c6bfe4dbad8686b1b14d7c5de280b983`.
Each receipt verifies its archive digest, 160 source inputs, three child hashes, final source identity, and process cleanup.

| Suite | Run | Executions | Archive SHA-256 |
| --- | --- | ---: | --- |
| home | `38590788569` | 3 | `c733f71fb5aebd84f214a502c330b84cfcdf7b1c011bfbd28e8a124b7aaa0e04` |
| primary | `38591287593` | 4 | `edf06a8ff1199c4d7c158736453d42915fb63dd69bb00f241200632b3025ff2c` |
| cold | `38592618709` | 2 | `5925838d50b69f48ec73ad1e0ce28bd27c6b828980347a0bd76e2250f9242545` |
| htmx | `38594145034` | 1 | `bd61f70e031217611c331f8eba5ce85465874c6b0f4ff8c0e8db49b09e293acf` |
| shows | `38594588817` | 2 | `639c645d22c43aaa2bee4fab6ed3f109eee3a004d85219e3f2d0048ef8175638` |
| search | `38595072862` | 1 | `100d1889472d1ae5d7eaad62a44256b3b46c2949ea9631a095f0bbc2ce11d57d` |
| safety | `38595587712` | 10 | `f5a5c967c006cbc3dc6e38f21f9d2c2f48cf50bdb88aa3f3e934f5ad614feb3b` |

The required Chromium artifact passes 534 executions, with 63 skipped executions and no retries or flaky results.
Artifact `11913165010` has SHA-256 `1270c410f8a6e3cbf431ef607d93ed12ccac059da1bbe2088945d25f65e36c32`.
Its ten source pins, 434 evidence hashes, and 435 checksum entries verify.
The tested PR merge `b686e77236e6613dfd819c43b6fc839cf53b4bbc` has the same tracked tree as the reviewed head.
These counts include isolated fixtures.

Required run `38592522246`, attempt two, passes all four gates.
Its first Go attempt receives HTTP 503 in `TestCommandLineCanDisableTLS`, after 1.87 seconds with empty child output.
The unchanged failed-job replay passes. No source, assertion, or deadline changes during that retry.
The original responding process and cause remain unproved.
The failed log has SHA-256 `972b4be2e61029eae1e5eba9231fa1716d9d12e86c3c35fe841ff8704ece4d67`.
The passing replay log has SHA-256 `3936f227fd81e7ffa55fcc06bd50a1c369f91be27dd847595339cf97fa234147`.

PR #603 merges as `1a811974d4a05022557602eade45b0bfedc40352`.
Fetched ancestry proves the reviewed commit is included in remote main.
Exact-main CI and the selected Player image publication remain pending at this checkpoint.
The unsupported BFCache path, physical devices, and deployment remain separate boundaries.
Q14 uploads schema-admitted receipt observations; original raw attachment bytes are not independently inspected.
