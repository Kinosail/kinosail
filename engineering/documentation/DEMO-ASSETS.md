# Public site screenshot provenance

The public homepage, comparison pages, docs landing page, playback guide, and
GitHub README use actual Kinosail screenshots. They were captured on
2026-10-26 from a source-built, loopback-only Podman Server with a disposable
fictional library. No personal library, production state, account details,
hostnames, or credentials appear in the captures.

## Fictional library

The library is produced by `engineering/documentation/demo-library/`:

- `catalog.py` defines 12 fictional films and two fictional shows (Harbor
  Lights, two seasons; Field Notes, one season) with invented titles, plots,
  directors, studios, ratings, and caption lines.
- `scenes.py` draws each poster and backdrop as an original SVG scene.
- `render-art.cjs` renders the SVG files to PNG with Playwright Chromium and
  fits each title to the poster.
- `build.py` writes the media tree: posters, fanart, NFO metadata, English
  subtitle files, and H.264 videos built from a still frame and silent audio.

To rebuild it outside the source checkout:

```sh
python3 engineering/documentation/demo-library/build.py \
  --output /path/outside/checkout/demo-media \
  --playwright apps/player/e2e/node_modules/@playwright/test
```

## Capture

1. Start Player in its own Podman Compose project with separate data, cache,
   backup, and read-only media volumes, TLS off, and a loopback-only port. The Server image was built from commit
   `19cfa1cdd184876e7ceebf0cc567dd13521a3c44`.
2. Create a demo-only Owner, add the `Movies` and `Shows` libraries, and set a
   few resume positions and My List entries through `/api/v1`.
3. Capture the web Player with Playwright Chromium at 1440 by 900 and 390 by
   844, both at 2x device scale, in dark mode. Wait for visible artwork, fonts,
   and network idle. The film page is a 2880 by 1800 crop of a full-page
   capture.
4. Capture the Android app Home on an API 36 phone emulator connected to the
   same Server. The status bar is cropped.
5. Encode with `cwebp -q 86 -m 6 -metadata none`. Resizing and cropping did not
   change any controls or product behavior.

Each page had loaded artwork, no pending skeletons, no horizontal overflow, and
no browser errors. Android TV, iPhone, and Apple TV captures were not taken for
this set.

## GitHub README images

| Image | SHA-256 |
| --- | --- |
| `player-home-desktop.webp` | `afc9c37436e57ac18fd801710343d8f30f00580ba068060ed7380185fc4b8194` |
| `player-home-mobile.webp` | `17133e1d41b67c7cc258163ad28dfea47d856c2f4fc66e51f97b98c321d12037` |
| `player-film-detail.webp` | `2cd66526d84ba078e00c6c9491ac06072f507cefa10af35de7f4da63d505d5e8` |
| `player-movies-desktop.webp` | `8c1930d2ee9f98043033e4e7e7b49c49cf61fd16a0744eecafb267443851edb9` |
| `player-watch-desktop.webp` | `310111228ff24da3803722c8040cc21813113dcdcd82a852c00d2569c68ffe83` |
| `player-show-desktop.webp` | `9b8c5d4ddb3bc8c0e92fec8b1b51fa4707d989a24e4c2ae8c4552c0ebc7a185a` |
| `android-phone-home.webp` | `46231217deebfac318900df9c1651b072266313d0c489830d2cf9346dce9de17` |

## Website images

| Image | SHA-256 |
| --- | --- |
| `player-library.webp` | `884a395d016301cd0c85c63193de90716a6effa41b7216d5a2226b2aaa777afe` |
| `player-movies.webp` | `1fe9b8635033c095ee3504c95bed6dd687c3c2e5785df5924c1102e69f00dca7` |
| `player-watch.webp` | `461bf397c66067a42e2a2a44395151b85272f5f97dc4a20c839481c37b97759a` |
| `player-show.webp` | `9f67ce6cb651e36e02859a93c7350360d75b5a625d67b13f59595280a2e6f5da` |
| `player-detail.webp` | `e44351918364bcd227e675996328290566956ce88b9ba2bc986bbe542afcd5af` |
| `player-home-mobile.webp` | `4856f1c5fa406e214f96ebfd9fa1c1ee960725d18fc65f87df8dbe88c2219b2c` |
| `player-watch-mobile.webp` | `b659d3c60485d1502703468f2b99a1e864fd0ff21d3733ea66a6997bc48cc15d` |
| `android-phone-home.webp` | `20bffafd1cffec8a78b7cef0bb6a3875a4d9068206f24b2c6ddf9a9cfd44d55c` |

The 688, 1024, and 2880 pixel variants are resized from the same captures.

The Why page adapts the creator's supplied marketing thread
01a0a67b-3ebe-7093-a062-115aa222ee55, especially its two stated project goals and
explicit AI-engineering disclosure. It makes no claims about unreleased apps.
