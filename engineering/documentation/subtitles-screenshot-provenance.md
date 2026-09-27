# Subtitles documentation screenshots

The three dashboard PNGs in `apps/subtitles/docs/assets/images/` were recaptured on 2026-09-27 from this commit's Subtitles app with the Player-style navigation. The isolated Podman test instance used synthetic movies and episodes from `apps/subtitles/scripts/generate-test-media.sh`, a test Owner, and no production account or personal media. The Supporter PNG retains its earlier synthetic capture at revision `d5b98a585b8576e6f2b00cad5e7d070d43674e8a`.

The dashboard capture used `KINOSAIL_TEST_PROJECT=kinosail-nav-test KINOSAIL_PORT=38138 ./scripts/test-instance.sh up`, followed by the `Subtitles navigation follows the Player shell` Playwright test with `KINOSAIL_TEST_INSTANCE=1`, `KINOSAIL_E2E_URL=https://127.0.0.1:38138`, and the test instance TOTP secret read from its local fixture file. The focused capture passed (1 test). The complete populated dashboard suite passed (22 tests). The committed PNGs are unchanged copies of the viewport artifacts. The earlier Supporter capture remains documented in `/tmp/kinosail-subtitles-shipping.2yxgoP/manifest.txt` on the capture host.

| Screenshot | SHA-256 |
| --- | --- |
| `subtitles-dashboard-1200.png` | `443d1ffc2264ebb1ee178b872b8908bb418caca80243004597476fc03f43ca64` |
| `subtitles-dashboard-1440.png` | `d6a4af5b0c919560ca5159e1e469410ac908ba7c0d87465ca1df5e021d2f0457` |
| `subtitles-dashboard-mobile-390.png` | `5866e40d029b30434afa42fcda45775b67fbf7d01b97401a8c5ee71a8b4e8063` |
| `subtitles-supporter-1200.png` | `64f43cebdc6d62b1ca8436f33bcc933ee466f21ff38cffc3ed8275a445bcb190` |
