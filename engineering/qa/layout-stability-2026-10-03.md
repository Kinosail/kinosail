# Responsive layout stability

## Failure analysis before implementation

The public interface is the rendered Player and Subtitles web UI. The reported failure is content changing position after an initial paint, without a user requesting a layout change.

Failure paths to measure:

- A late Manrope font changes text width, line wrapping, header height, and card placement.
- Deferred scripts reveal, move, or remove controls after HTML has painted.
- Settings initialization hides categories and inserts description text after first paint.
- Artwork, video metadata, and missing artwork change intrinsic geometry.
- HTMX pending state changes target minimum height, text width, grid sizing, or button width.
- Delayed, failed, aborted, and retried requests leave busy state or skeletons behind.
- Empty or error feedback appears above content without reserved geometry.
- Narrow, landscape, enlarged text, reduced motion, and keyboard flows expose wrapping and focus failures.

Measure unexpected LayoutShift entries separately from entries with recent input. Record element rectangles over time because recent-input filtering can conceal a visibly jerky pending state. Resize and explicit navigation are intentional layout changes; delayed changes after those settle are not.

Use native disposable Go servers on supported loopback HTTP, small generated media, and synthetic accounts. Delayed responses retain real server bodies. Injected error responses and emulated device APIs must be labeled. No production media, credentials, deployment, TLS bypass, or physical-device claims.

## Evidence

Baseline native Player measurements: `.verification/layout/20261003T191945Z/player/measurements.json`.
At 390px, aggregate unexpected shifts were 0.70048 for bookmarked Connections, 0.05269 for Movies, and 0.03206 for default Settings. At 1440px, bookmarked Settings measured 0.54017. These initial values are aggregate shifts, not session-window CLS.

The first implementation's CLS score concealed a wrong category at first paint. The final runner records initial/final visible sections, document-coordinate element boxes, supported timing APIs, and CLS session windows. Playwright screenshot font-readiness can itself force an optional-font substitution. Chromium pixels therefore use CDP capture, and measurements precede non-Chromium screenshot capture.

Independent review found and corrected template-definition matching, a library ID selector applied to Settings, a Subtitles control-mode regression, and native panel sizing. A real Subtitles run found the in-flow toolbar removed after the 2.2-second idle timer, shrinking the player 69.6px and moving primary actions. Normal inline toolbars now stay visible; theater/fullscreen overlay controls retain idle behavior.

Fonts use `font-display: optional`: slow cold loads retain the system fallback for that navigation. Warm loads still use Manrope. This changes typography availability, without delaying content rendering.

Repeatable frozen-source command:

```sh
KINOSAIL_LAYOUT_ENFORCE=1 KINOSAIL_LAYOUT_FLOWS=1 KINOSAIL_LAYOUT_VARIANTS=1 KINOSAIL_LAYOUT_APPLE_SHIM=1 python3 scripts/testing/test-layout-stability-local.py
```

Chrome is the default. Set `KINOSAIL_LAYOUT_BROWSER=webkit` or `firefox` for those engines. `KINOSAIL_LAYOUT_QUICK=1` selects 390/1440; omit it for 320/390/844/768/1024/1440/1920. Receipts record exact switches and source drift. The desktop-UA iPad capability shim is synthetic and does not establish physical-device or Safari proof.

Final matrix and artifact references will be appended after the frozen-source runs. No production origin, Nox, live playback, manual deployment, TLS bypass, or container/image operation is part of this verification.

## Transition failure analysis

An encoded fragment can select Connections during initial projection, then switch to Playback when deferred code reads the raw fragment. Malformed escapes must retain a safe default. A theater/fullscreen idle timer can leave the normal in-flow toolbar hidden after exit. A later pointer event then restores its height. Measure both transitions with real server pages before changing their behavior.
