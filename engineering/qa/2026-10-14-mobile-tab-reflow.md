# Player mobile navigation at enlarged text sizes

## Confirmed finding

At 390 pixels wide and 200% root text size, personal tab labels break within words.
Firefox and WebKit render `Home` as `Hom` / `e` and `Search` as `Searc` / `h`.
Chromium also splits words in the same navigation bar.
All labels fit the viewport, so the existing horizontal-overflow check passes.
The label-readability check covers only the Subtitles dock.

These captures come from hosted layout run `38521537100` at
`8c249fd6ab990d2ff629e3dc7011ba521f9b526f`.
Relevant application assets and layout inputs match the later QA head `4ba40bdc`.

| Engine | Artifact | Archive SHA-256 |
| --- | --- | --- |
| Chromium | `11875071416` | `236e429e970e6348d911ff97b9fd02260771f5cae63ef7ad9a29370c2e4d1d46` |
| Firefox | `11873949967` | `04f1202179dcb2b8a9f3a742d8b41afde77acbb60627044ebb445878cd4a1530` |
| WebKit | `11874810481` | `4ac14d3c954340a9a5539485341ab3e899490da2bede1637deeeb3d6b5253109` |

The affected capture suffix is `player/390-text-200--settings-session-timeouts-settled.png`.
The source owner is the personal-navigation block in `apps/player/internal/server/static/home.css`.
It combines a fixed column count with arbitrary word breaking.

## Regression contract

Extend the existing real-server layout journey before changing production CSS.
Measure text ranges for each whitespace-delimited word in visible Player navigation controls.
Words must occupy one rendered line and remain inside their control.
Wrapping between words, such as `TV Shows`, remains valid.
The settled page must expose every expected tab and More, with nonempty text geometry.
Keep the existing Subtitles label check separate.

Exercise the default tabs and four saved long labels at 320, 390, and 844 pixels.
The saved set is Audiobooks, Collections, Unwatched, and Playlists, plus More.
Retain pending and settled snapshots, screenshots, exact revisions, source pins, and archive hashes.
Separate real-server flows check the last page link above the dock after keyboard traversal and end scrolling.
They also open More, focus Quick Connect, open Customize tabs, close it with Escape, and verify restored focus.
Finally, they follow Quick Connect and require its actual destination heading.

The existing disposable runner uses synthetic media and an isolated Owner account.
Run `gh workflow run layout-stability.yml --ref <regression-branch>` for the hosted baseline.
Local browser and application execution remain held.
Syntax, source-size, and whitespace checks alone do not establish a failing browser regression.

## Accepted failing baseline

Hosted run `38531698842` tests regression commit `597ff4e9982d208d1091b0fec2301e65b28754ed`.
All three engines record eight failed word-readability cases.
Chromium measures 97 Player snapshots; Firefox and WebKit each measure 42.
The settled controls contain the exact expected labels and initialized navigation state.
Individual words occupy multiple lines. The other recorded layout acceptance checks pass.
Each engine also records 11 existing Player flows.
Subtitles succeeds with 111 Chromium snapshots and 46 per other engine, plus 26 flows each.

| Engine | Artifact | Archive SHA-256 |
| --- | --- | --- |
| Chromium | `11879341610` | `7164c1d982425c454aac75891021e3325cf5dc47d1e4c8512b9b3cfbeb18afc6` |
| Firefox | `11878811014` | `5415c5c2a16f8753d1ccf37ff42682641283e3bb669c7ed5b5dbf5a2463c8255` |
| WebKit | `11879215097` | `294be95f4e5a311739538d2e362c52de87c14fcc759cdc06332c24bb80cc388e` |

All archive digests match GitHub metadata. All eight direct script pins match the clean recorded revision.
The later keyboard regression commit `6a42d34a` is separate from this word-readability baseline.
Its six default/saved viewport combinations still need hosted execution.
Commit `05b7b0c9` adds title-index and install-help reachability to those combinations before their production changes.
Those flows use a synthetic iPhone user agent to expose the web install action in each browser engine.
This does not establish physical iPhone behavior.

## Repair and current boundary

Personal tabs now use wrapping flex rows sized to their words, including More.
The existing navigation and editor styles move into `mobile-tabs.css` to keep source files within the size limit.
The app embeds those styles in the same stylesheet, immediately after `home.css`.
Text size, saved choices, and access to More and Customize tabs remain part of the acceptance contract.
The initial script measures the actual dock height and tracks changes with a resize observer.
That height reserves scrolling space and positions menus above the dock.
The title index and scrollable install help use the same height.
The title index retains its existing short-screen dialog fallback.
Keyboard traversal reveals page controls between the header and dock.

Independent source review found no remaining concrete blocker after the overlay and inline-index corrections.
The new stylesheet passes the Impeccable source detector. JavaScript syntax, Go formatting, and source-size checks pass.
CI at `2c3b919a` correctly rejected the stale Code Atlas after the two new embed declarations shifted source locations.
Hosted metadata run `38535168661` supplied canonical source data for the same clean revision.
Its Q09 groups passed 12 isolated, 47 server, and one phone-browser checks, with no retries.
Artifact `11879308556` has SHA-256 `6798c4e93c60f43de081d7fd56f473db93ebe151d05e95f10d5c6ad17dcdf1a8`.
After verifying the digest, source identity, and receipt, the existing reproduction command regenerated the Player Code Atlas.
Only its source counts and seven symbol line numbers changed. Subtitles and the surrounding HTML stayed unchanged.

WebKit at `2c3b919a` passes 42 Player snapshots and 29 flows, including all six combinations for each new flow.
It also passes 46 Subtitles snapshots and 26 flows.
Both apps record zero identified DOM layout shift, overflow, or terminal skeleton cases.
Artifact `11880305232` has SHA-256 `8dbfad20d8ce2566f7f85637a4f65306afd1fa55a31491fcc7017f4f1afb1236`.
The archive digest, clean revision, and eight direct source pins were verified.
Four selected WebKit screenshots show intact default and saved labels at enlarged text sizes, including short landscape.

Firefox passes all 42 Player snapshot checks, then rejects the inline title-index container at 844 pixels.
Artifact `11879164120` has SHA-256 `1b7c83737a1d1b898e7ce501800d0630c15251c2daa6289bba3de65ff8a5c017`.
Its trace shows all three controls ending at 261.333 pixels, above the dock at 268.667 pixels.
The focused control's six-pixel outline also fits. Only the container's padding extends beneath the dock.
Independent review confirms that the inline-container assertion is overstrict.
The correction retains whole-container bounds for fixed rails and checks every inline control and focus outline against the usable viewport.
It records the control rectangles and focus state. No production CSS change follows from this fixture error.
The corrected fixture and complete required CI still need a passing run.
Local application and browser execution remain held. Source checks do not establish browser acceptance.
These browser captures do not establish physical-device or assistive-technology acceptance.

## Install-help failure on the next revision

At `1f518666`, WebKit again passes all 42 Player snapshots, 29 Player flows, 46 Subtitles snapshots, and 26 Subtitles flows.
Its artifact `11880887581` has SHA-256 `58b5a041e8b35806412896f50704c4f1a0afa574e1f3e31f1b286cb11f4c3ef6`.
The eleven automated accessibility audits report no violations.

Firefox passes the corrected title-index assertion, then detects a clipped install-help button at 844 by 390 pixels with 200% root text.
Its artifact `11880488891` has SHA-256 `c8216423ef0e89a0b6e70c03957d606e9bad4f4557f08f0bedd541930883615e`.
The button ends at 260.400 pixels; the scrollable panel ends at 252.667 pixels. The dock begins at 268.667 pixels.
The screenshot confirms clipping by the panel. It does not establish whether focus scrolling has settled.
Firefox's 42 Player snapshots and all Subtitles checks pass, but its Player flow suite remains failed and incomplete.
Both archives verify their GitHub digest, clean revision, and eight direct script pins.

Commit `b184e6b9` records focus identity, scroll dimensions, computed overflow, and geometry across six animation frames.
It preserves the original geometry assertion and makes no production change.
These bounded observations distinguish delayed focus scrolling from a control that remains clipped.

At `5a8a0c80`, Firefox repeats the clipping with unchanged geometry across all three samples.
The button retains focus. The panel has 287 pixels of content and a 235-pixel client height, but its scroll offset stays zero.
Its computed overflow and scroll behavior are both `auto`.
WebKit scrolls the equivalent panel by nine pixels and exposes the complete button.
Firefox artifact `11882067135` has SHA-256 `f04a3379977825d9460285e25c51bade18ad32f71a3125ac68830b1ba6819538`.
The repair explicitly reveals focused install-help controls with nearest, immediate scrolling.
The original reachability assertion remains. A further keyboard check requires the complete focus outline inside the panel and retains a screenshot.

Required CI `38539329583` passes at `5a8a0c80`. Chromium and WebKit also pass their complete layout suites at that revision.
These results precede the focus repair, which still needs hosted verification.
The earlier `1f518666` Chromium run remains failed because the desktop account search field shifts during initial loading.
Its identified DOM layout shift is `0.003452506443437071`, above the unchanged `0.001` threshold.
Artifact `11881681724` has SHA-256 `2c787a67fd80086cb6d8c0097f235d25f2c47ef2d0c7d4512bbbd561e4c0d340`.
The later pass does not diagnose that intermittent shift.

## Hosted Xcode image transition

Required run `38573733320`, job `115886424823`, fails before Swift compilation at `874422be`.
Runner image `xcode-27-arm64 20261012.0260.1` no longer contains `Xcode_27.1_beta.app`.
Its [published manifest](https://github.com/actions/runner-images/blob/xcode-27-arm64/20261012.0260/images/macos/xcode-27-arm64-Readme.md) lists `Xcode_27.1_Release_Candidate.app` and the versioned alias `Xcode_27.1.app`.
The previous image also provides that alias.
Compilation, native contracts, focused probes, result inspection, and scheduled Swift scanning now use the alias.
This preserves the selected 27.1 toolchain across the bundle rename.

The existing CI contract failed before the workflow edit: two tests, one failure.
After the edit, all 192 CI contract tests pass, with five real-scanner checks deferred to the hosted secret gate.
Repository actionlint 1.17.0, workflow validation, source-size limits, and whitespace checks pass.
The system actionlint lacks native parallel syntax; its rejection is not accepted validation.
Private evidence records the original failure, manifests, commands, and log checksums.
Hosted compilation remains required. No local native runtime was started.
