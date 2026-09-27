# Subtitles documentation screenshots

The Subtitles dashboard, Supporter gallery, README, and social preview use
screenshots of the real app running in an isolated Podman container. The image
was built from commit `f4043cf5eb88218d00f42820a3b2d3533ce1de5f` with
`podman build --file apps/subtitles/Containerfile --tag localhost/kinosail-subtitles:docs-screenshots-20260927 --build-arg REVISION=f4043cf5eb88218d00f42820a3b2d3533ce1de5f .`.
The container used a loopback-only port, read-only root, and separate disposable
config, cache, backup, and media volumes. Its `/healthz` returned `{"status":"ok"}`.

The media volume contained six generated six-second MP4s: The Last Observatory,
The Quiet Coast, After the Rain, A Place Between, and two Signal House episodes.
The files had local NFO metadata and generated gradient posters. One English
SRT sidecar covered The Last Observatory; the other five files needed English
subtitles. A demo-only Owner with TOTP signed in. No personal media, production
state, provider keys, purchase, or personal account data appears in the images.

Playwright Chromium captured the dashboard at 1440 by 900 with a full-page
height of 1522, the phone at 390 by 1240, the social preview at 1200 by 630,
and the Supporter gallery at 1200 by 900. All captures used a 2x device scale.
The social and gallery shots are genuine scrolled viewport captures. Each
dashboard view showed five wanted files and one ready file, with no broken
images, page errors, horizontal overflow, or WCAG A/AA findings. The Supporter
gallery had the same clean browser and accessibility checks.

The public pages use WebP copies encoded with `cwebp -q 88 -m 6` from the
captures. PNG copies at the older public paths were downsampled from the same
captures to preserve existing links. The social PNG is 1200 by 630, matching
its Open Graph dimensions. Encoding and resizing did not add or remove UI.

| Public image | SHA-256 |
| --- | --- |
| `subtitles-dashboard-desktop.webp` | `127a40e279715031c5f24dd69450b4b85e3de4a16bb57c6afe477d2357833e85` |
| `subtitles-dashboard-mobile.webp` | `82d2d58d987c0764ba781fa5831a8f96b51c0f223973d715e8479331636f6e70` |
| `subtitles-supporter-gallery.webp` | `5c018006b2113f41182ea2de394f88b07fd5a24a4907a3bb65a8e2da163edc52` |
| `subtitles-dashboard-1200.png` | `6324ae2b1d02221720c21795db8e9312e368ede3b376af8ffd44098f1fae6ef8` |
| `subtitles-dashboard-1440.png` | `507f648a2e7ebf87d95e53423c7deb986b0a7c38ff1b1a81ac9601dfc0fa25a0` |
| `subtitles-dashboard-mobile-390.png` | `956cc76cc694a2601684cc1fad2c99a61c18893a02c2b4a699b074e92c0b483a` |
| `subtitles-supporter-1200.png` | `5cc6ee242f9df2ef2a492c451513a5a5a7f1401c061a3bbf1c0574dd74f5511d` |
