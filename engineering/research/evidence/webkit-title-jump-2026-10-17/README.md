# WebKit pause around the Jump to title dialog

Date: October 17, 2026. Revision: `f90612019c97ddc8b8e9492a96a609a52ae96f38`.

## Question

In the CI layout harness, headless WebKit at 390×650 sometimes paused about 1 second when the Jump to title dialog opened. #682 made the harness sample long enough to settle. This note answers whether the pause is a product cost.

## Finding

The pause is a first-use cost of headless WebKit on Linux with an empty Mesa shader cache. It is not a cost of the dialog markup or script.

- With an empty shader cache, the first dialog open stalls 1.0–1.5 seconds. This happens with or without the dialog's `backdrop-filter: blur(5px)`.
- The second open on the same page has a largest frame gap of 21–43 ms.
- Once the machine's shader cache is warm, a new browser shows a largest gap of about 70–100 ms on first open.
- The blur adds about 0.4 seconds to the cold first open and about 10 ms to later opens.
- Chromium shows no comparable gap.

A fresh GitHub Actions runner always starts with an empty cache, so the first WebKit dialog open in a run pays this cost.

No product change was made. Removing the blur would shorten only the cold first open in software-rendered WebKit, and real Safari uses Metal.

## Results

Largest gap between animation frames during the 1.5 seconds after `showModal()`, in milliseconds. Each sample uses a new browser and a new empty `MESA_SHADER_CACHE_DIR`.

| Engine | Backdrop | First open | Second open |
|---|---|---|---|
| WebKit 26.6 | `blur(5px)` | 1468, 1471, 1440 | 27, 29, 28 |
| WebKit 26.6 | none | 1026, 1021, 1204 | 43, 22, 21 |
| Chromium 153.0.8010.12 | `blur(5px)` | 17, 17, 33 | 17, 17, 50 |
| Chromium 153.0.8010.12 | none | 17, 17, 17 | 17, 17, 17 |

Raw rows: [`results.json`](results.json), SHA-256 `f86acf6167a29abeda32821ebb2b44f56a0913be831e6a6b4ef73e6b36a8d4a4`.

## Method

- The test page is [`dialog-backdrop.html`](dialog-backdrop.html), SHA-256 `281161cdfb159c3bb7fa59152b01cce220753f62de89129ef587f85712ae09cb`. It has 60 generated poster images in a 3-column grid. Its dialog uses the same size, border, shadow, backdrop and 5-column letter grid as `packages/webassets/static/player-app.css`. `BACKDROP` is replaced with each variant.
- [`measure.mjs`](measure.mjs) uses Playwright 1.63.0 with its bundled WebKit (`webkit-2359`) and Chromium. It runs in a Linux container with no GPU (Mesa software rendering). The viewport is 390×650.
- Command: `node engineering/research/evidence/webkit-title-jump-2026-10-17/measure.mjs <directory containing @playwright/test 1.63.0>`.
- The first measurement records requestAnimationFrame gaps around the first `showModal()` and `close()`. The second measurement repeats them on the same page.

## Limits

- This uses an isolated page, not the populated Player Server. It reproduces the dialog's styling, not its letter-preview script.
- Not checked: Safari on a Mac, iPhone or iPad, or with GPU acceleration.
