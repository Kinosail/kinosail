# Subtitles documentation screenshots

The four PNGs in `apps/subtitles/docs/assets/images/subtitles-*.png` were captured on 2026-09-27 from the actual Subtitles test instance at source revision `d5b98a585b8576e6f2b00cad5e7d070d43674e8a`. It ran in a local Podman container with synthetic movie and episode files made by `apps/subtitles/scripts/generate-test-media.sh`. The Owner and Supporter badge were test fixtures. No personal media, purchase, or production account was used.

The repeatable run used `apps/subtitles/scripts/test-instance.sh up` and `verify`, `go test ./internal/server -run TestWriteUIStateFixtures`, and the targeted Playwright dashboard and Supporter tests in `apps/subtitles/e2e/`. The run passed. Its command, environment, data, result, and original full-page and viewport screenshots are recorded in `/tmp/kinosail-subtitles-shipping.2yxgoP/manifest.txt` on the capture host. The committed PNGs are copies of the viewport artifacts without editing.

| Screenshot | SHA-256 |
| --- | --- |
| `subtitles-dashboard-1200.png` | `773c7cbbaa2e2006ef90b420559dde5168abc6dd23be70269750a3dc005ce14e` |
| `subtitles-dashboard-1440.png` | `9c58a81a5f8d817619b304950e88bff8ab122643e7768a05d61325252f006812` |
| `subtitles-dashboard-mobile-390.png` | `37cc26b5f3255a42baaf9ec37b5fadb8d6bcd35e56ba5d8ac467cba206feda05` |
| `subtitles-supporter-1200.png` | `64f43cebdc6d62b1ca8436f33bcc933ee466f21ff38cffc3ed8275a445bcb190` |
