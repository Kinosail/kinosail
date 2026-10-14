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

The candidate still requires passing hosted geometry, keyboard, focus, responsive, and accessibility evidence.
Independent source review found no remaining concrete blocker after the overlay and inline-index corrections.
The new stylesheet passes the Impeccable source detector. JavaScript syntax, Go formatting, and source-size checks pass.
Local application and browser execution remain held. Source checks do not establish browser acceptance.
These browser captures do not establish physical-device or assistive-technology acceptance.
