# Subtitles documentation screenshots

The four PNGs in `apps/subtitles/docs/assets/images/subtitles-*.png` were captured on 2026-09-27 from the actual Subtitles test instance at source revision `8207b32109fa979ef625da4a5c9b94e91a49ac4e`. It ran in a local Podman container with synthetic movie and episode files made by `apps/subtitles/scripts/generate-test-media.sh`. The Owner and Supporter badge were test fixtures. No personal media, purchase, or production account was used.

The repeatable run used `apps/subtitles/scripts/test-instance.sh up` and `verify`, `go test ./internal/server -run TestWriteUIStateFixtures`, and the targeted Playwright dashboard and Supporter tests in `apps/subtitles/e2e/`. The run passed. Its command, environment, data, result, and original full-page and viewport screenshots are recorded in `/tmp/kinosail-subtitles-shipping.wnKpzr/manifest.txt` on the capture host. The committed PNGs are copies of the viewport artifacts without editing.

| Screenshot | SHA-256 |
| --- | --- |
| `subtitles-dashboard-1200.png` | `e84d7a8b8f22d15802edc24748d26b27e66d003cd0ffb131fa8ec8a8af9cd30d` |
| `subtitles-dashboard-1440.png` | `4f1b19aef9107e677948c827b161f0da4165188130919fa5c492b92ef83d5f6b` |
| `subtitles-dashboard-mobile-390.png` | `f8f52e6d0b9f56227ad3584a7bf15bf9b14cee78046721b81218c272a8c6d603` |
| `subtitles-supporter-1200.png` | `64f43cebdc6d62b1ca8436f33bcc933ee466f21ff38cffc3ed8275a445bcb190` |
