# Public site screenshot provenance

The public homepage, docs landing page, and playback guide use the actual
Player screenshots documented below. They were captured on 2026-09-27 from a
source-built, loopback-only Podman Server with a disposable fictional library.
The desktop captures are 2880 by 1800 pixels; the phone capture is 780 by 1688.
The web page serves smaller desktop WebP copies when appropriate and selects
the phone Home screenshot for narrow screens. Image conversion and resizing did
not change any controls or product behavior.

The four film titles, plots, years, ratings, and video files were created for
this demo: The Last Observatory, The Quiet Coast, After the Rain, and A Place
Between. Their poster and backdrop artwork was generated for these fictional
films. The public pages label the demo library. No personal library, production
state, account details, hostnames, or credentials appear in the captures.

To reproduce: run Player with entirely separate data, cache, backup, and media
directories; create synthetic MP4 files and matching basename NFO/PNG sidecars;
complete setup using a demo-only Owner; capture the home and film detail views
without browser chrome. Inspect all visible text and images before publication.
Never reuse real media, real Owner details, or an existing Server's state.

The Why page adapts the creator's supplied marketing thread
01a0a67b-3ebe-7093-a062-115aa222ee55, especially its two stated project goals and
explicit AI-engineering disclosure. It makes no claims about unreleased apps.

## Source capture and GitHub README screenshots (2026-09-27)

The three images in `.github/assets/player-*.webp` are fresh Chromium captures
of the Player built from commit `0a8bcafccdad4847f18de42cc3f537ed78fd52f5`
with `podman build --file apps/player/Containerfile --tag localhost/kinosail:github-screenshots .`.
A separate, loopback-only Podman container ran with disposable config, cache,
backup, and read-only media volumes. Its `/healthz` returned `{"status":"ok"}`.

The disposable library contained four synthetic 12-second MP4 files with local
NFO metadata and original generated artwork: The Last Observatory, The Quiet
Coast, After the Rain, and A Place Between. The browser signed in as a demo-only
Owner. No real media, account details, hostnames, or credentials appear in the
screenshots. Playwright Chromium captured Home at 1440 by 900 and 390 by 844,
and film detail at 1440 by 900, all at 2x device scale. Each page had loaded
artwork, no pending skeletons, no horizontal overflow, and no browser errors.
The PNG captures were encoded with `cwebp -q 88 -m 6` without changing the UI.

| Image | SHA-256 |
| --- | --- |
| `player-home-desktop.webp` | `528bee78914c9fa5d22fce6f3a6250849a1cd9ca90275e19605338ba5b508e18` |
| `player-home-mobile.webp` | `cdeee91fc479a2f2ca9f6b85264ca4a7e38f4a901a748581745f7ddaa86d796e` |
| `player-film-detail.webp` | `84e6777ee99f0e360c153e3f9d553741723937e4937b4f1132305be4bfdb2a5d` |
