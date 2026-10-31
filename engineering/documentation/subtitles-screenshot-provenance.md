# Subtitles documentation screenshots

The Subtitles dashboard images in the docs and README are screenshots of the
real app running in an isolated container. They were refreshed on 2026-10-31
from main commit `552bee541350b81830461a00022c7f672ce601f4`, using the Nox
release image `localhost/kinosail-subtitles:nox-552bee541350` built by
`bin/nox-deploy.sh` from `apps/subtitles/Containerfile` at that commit. The
image passed its Trivy HIGH/CRITICAL scan before use.

The container ran with a read-only root, a `/tmp` tmpfs, TLS off, no published
port, and separate disposable config, cache, and media directories. Its
`/healthz` returned `{"status":"ok"}`. A demo-only Owner with TOTP signed in.
No personal media, production state, provider keys, purchase, or personal
account data appears in the images.

The media directory used the layout from
[Add movie and episode folders](../../apps/subtitles/docs/getting-started/add-media.md),
with the default folder list `.`:

- `Movies/`: The Last Observatory (2024), The Quiet Coast (2023), After the
  Rain (2022), and A Place Between (2025).
- `Shows/Signal House/Season 01/`: two episodes, with a `tvshow.nfo` and a
  `poster.jpg` in the Show folder.

Each file is a generated six-second H.264 MP4 with a generated gradient
poster and a local NFO. One English SRT sidecar covers The Last Observatory;
the other five files need English subtitles. The generator script is
`make-media.sh` in the capture workspace; it uses only `ffmpeg` test sources.

Playwright 1.56.1 Chromium captured the Overview in the dark theme with a 2x
device scale, after one reload so the Manrope font came from the browser
cache (the stylesheet uses `font-display: optional`):

- Desktop: 1440 by 900 viewport, full page, 1624 CSS pixels tall.
- Phone: 390 by 1240 viewport, viewport only, so the fixed tab bar stays at
  the bottom.
- Social preview: 1200 by 630 viewport.

Every capture showed five wanted files and one ready file, with no horizontal
overflow, no broken images, and no page errors after sign-in. axe-core 4
(`@axe-core/playwright`, tags `wcag2a`, `wcag2aa`, `wcag21a`, `wcag21aa`,
`wcag22aa`) found no violations at the desktop and phone sizes.

The WebP images were encoded from the captures with `ffmpeg -c:v libwebp
-quality 88 -compression_level 6`. The PNG copies at the older public paths
were downsampled from the same captures with the Lanczos filter. Encoding and
resizing did not add or remove UI.

| Public image | Size | SHA-256 |
| --- | --- | --- |
| `subtitles-dashboard-desktop.webp` | 2880×3248 | `141b01a3c2556a26ec4d4d79692313709e5cf064f85204c7c1c70981f8da2733` |
| `subtitles-dashboard-mobile.webp` | 780×2480 | `2fb440bf7dbf3933c9f7f193f0bf59f0fb25e67489474b1f1c6d2176c3d5ca41` |
| `subtitles-dashboard-1440.png` | 1440×1624 | `df0899696d08441126ce156db7431cf21eac7e3f0b3202207bedcfda900cbf60` |
| `subtitles-dashboard-mobile-390.png` | 390×1240 | `4eedd96d857fde574cbdd504544be43e1b4c7297d4b974ae8a73ffb6b5b5c728` |
| `subtitles-dashboard-1200.png` | 1200×630 | `f5497c7801b3b39d742cb076c8f6bc6e6b0b0c873a724bb0cc30109ed8ea784d` |

The Supporter gallery (`subtitles-supporter-gallery.webp`,
`subtitles-supporter-1200.png`) was not recaptured. It still comes from the
2026-09-27 Podman capture of commit `f4043cf5eb88218d00f42820a3b2d3533ce1de5f`.

| Unchanged image | SHA-256 |
| --- | --- |
| `subtitles-supporter-gallery.webp` | `5c018006b2113f41182ea2de394f88b07fd5a24a4907a3bb65a8e2da163edc52` |
| `subtitles-supporter-1200.png` | `5cc6ee242f9df2ef2a492c451513a5a5a7f1401c061a3bbf1c0574dd74f5511d` |
