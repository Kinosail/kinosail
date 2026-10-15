# Mobile title rail qualification

The accepted browse run at `22c0f5b7` shows the Movies title rail entering Search's lower edge at 390 × 844. This screenshot is an observation. A hosted geometry regression must qualify the defect before a production repair.

`browse-readability.spec.ts` now records the rail, header, Search, and dock rectangles at 320, 390, and 700 pixels wide. The pending phase holds the real navigation bundle. It preserves the server-rendered rail and measures its actual geometry without waiting for fonts blocked by that bundle. Loaded and scrolled phases wait for fonts and two animation frames, then retain screenshots.

The same document then changes to compact phone, desktop, TV, and tall phone sizes. A real title-filter request is aborted to check that the existing catalog and released busy state remain usable. An actual empty search checks that the rail disappears. This fault injection does not simulate a successful Server response.

The hosted browse selection requires exactly 23 cases, one Chromium worker, and no retries. The runner's isolated contract covers an E2E gap: a browser cannot prove that its runner rejected an omitted or extra case. Before the runner changed, the new contract failed twice against the old 22-case count. After the change, all 10 runner contracts passed.

The first baseline at `77fa7aa4` confirmed pending overlap but exposed a test cleanup race. Removing the active route could continue it before the held handler did. The test now releases the request, joins the handler, then removes the route. It retains explicit cleanup outcomes.

The corrected baseline at `29cb3a87` completed every phase. At 320px and 390px, the rail started at 104px while Search ended at 111.1875px and the header ended at 122.578125px. Sixteen assertions failed across pending, loaded, scrolled, resized, and failed-filter states. All three held-request cleanups passed. Compact phone, 700px, desktop, TV-size, and empty-state controls passed. Run `38647950675`, artifact `11933705356`, has SHA-256 `d3d0228a5bf99b8c39127bfdc30ee37a69bff0cd9f69f65bfe326d3f6e218115`.

The repair reuses the early mobile navigation measurement and observes both the header and dock. Player places the fixed rail below the measured header, while preserving its existing minimum top offset. Both measurements are removed outside mobile layouts. This changes presentation only; it adds no API or Server failure path.

Local app and browser execution remains on hold. Syntax, source contracts, and line caps provide source evidence only. Repaired hosted acceptance is pending. The artifact must retain the exact revision, input hashes, results, geometry, screenshots, and owned fixture cleanup before acceptance.
