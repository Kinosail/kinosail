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

## Repair constraints and current boundary

Production remains unchanged while the regression baseline is pending.
The repair must preserve text size, saved choices, and access to More and Customize tabs.
If navigation gains rows, verify its actual height against page padding and menu placement.
Check bottom-content visibility, keyboard reachability, Escape, and focus restoration.
Check short landscape viewports as well as portrait phones.

An independent source review supports this contract and identifies dock-height accounting as a required check.
Implementation and hosted red-green evidence are still pending.
These browser captures do not establish physical-device or assistive-technology acceptance.
