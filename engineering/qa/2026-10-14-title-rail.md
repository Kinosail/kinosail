# Mobile title rail qualification

The accepted browse run at `22c0f5b7` shows the Movies title rail entering Search's lower edge at 390 × 844. This screenshot is an observation. A hosted geometry regression must qualify the defect before a production repair.

`browse-readability.spec.ts` now records the rail, header, Search, and dock rectangles at 320, 390, and 700 pixels wide. The pending phase holds the real navigation bundle. It preserves the server-rendered rail and measures its actual geometry without waiting for fonts blocked by that bundle. Loaded and scrolled phases wait for fonts and two animation frames, then retain screenshots.

The same document then changes to compact phone, desktop, TV, and tall phone sizes. A real title-filter request is aborted to check that the existing catalog and released busy state remain usable. An actual empty search checks that the rail disappears. This fault injection does not simulate a successful Server response.

The hosted browse selection requires exactly 23 cases, one Chromium worker, and no retries. The runner's isolated contract covers an E2E gap: a browser cannot prove that its runner rejected an omitted or extra case. Before the runner changed, the new contract failed twice against the old 22-case count. After the change, all 10 runner contracts passed.

No production behavior changes in this checkpoint. Local app and browser execution remains on hold. Syntax, source contracts, and line caps provide source evidence only. The hosted artifact must retain the exact revision, input hashes, results, geometry, screenshots, and owned fixture cleanup before acceptance.
