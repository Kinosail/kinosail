# Progress notice visibility verification

At widths700px and below, the hidden progress notice matched a later important grid-display rule. The actual Go markup rendered an87.390625px empty row with Retry at360px, despite `hidden=true`. Two pre-production repeated journeys failed this computed-style assertion on base e85544ebce14aab0454ea11d1fb01415dfbedd4f.

The repair limits that mobile display rule to `.primary-player-actions:not([hidden])`. The matching old/new lines in the embedded Subtitles stylesheet patch also change, preserving its existing layout difference and keeping asset initialization valid. No checkpoint, stream, authentication or recovery behavior changes.

## Repeatable proof

Command: `GOMAXPROCS=2 python3 -B scripts/testing/test-player-progress-local.py` from repository root.

Private failing run: `.verification/r03-progress/20261006T135042Z/receipt.json`. Private passing run: `.verification/r03-progress/20261006T140619Z/receipt.json`. Each receipt includes source revision, working diff hash, environment, commands and artifact checksums. The passing production/test diff reviewed independently is `84ceb98bea6b473bd72561b4a60fd43bdaaeed5e24a7dedbc721119ec4b11cd3`.

The disposable native Go Server uses loopback HTTP, a synthetic Owner/TOTP and generated12-second H264 video without audio. It uses build-p1, GOMAXPROCS2 and one headless Chromium worker. Original run state and private logs remain local.

- Candidate:4/4 executions passed; two journeys repeated twice.
- Real Server rejects an invalid public progress request without changing stored state; the page shows the access recovery status.
- Controlled503, held retry, real204 and public persisted-state readback verify failed/pending/saved behavior. The injected503 is an isolated transport failure within a populated journey, not a naturally failing Server.
- Idle/empty and saved notices have hidden=true, computed display:none and zero height at360,390,700,701,1440 and1920px.
- Failed and pending notices remain visible, Retry is at least44px, and no responsive overflow occurs. Every pending capture verifies busy state, Saving progress copy and disabled Retry.
- Failed-save axe checks pass at390,1440 and1920px. Populated idle390, failed390, pending1920 and saved700 captures were visually inspected.
- Historical progress-asset replay:2 rejection executions passed and2 unrelated recovery executions skipped. This historical phase is separate from the CSS failure baseline.

The initial repaired run140443Z failed during embedded stylesheet initialization, before any browser journey. The required companion Subtitles patch repairs that concrete dependency; the failed receipt is retained. Earlier pre-production runs134606Z/134828Z injected503 before idle and are not valid idle proof.

The manual Impeccable detector reported existing advisory styling findings outside the changed selector. No visual redesign is included.

## Boundaries

This proof is Chromium with a real local Server. It does not establish Safari/native HLS playback, audio, physical iPhone/AppleTV, TLS, container deployment or a repair for the frozen cohort zero-position restarts. Library is on hold and no Nox/device deployment occurred. Protected CI must cover the affected shared packages, Player and Subtitles.

## Selected artifact SHA-256

| Artifact relative to passing run | SHA-256 |
|---|---|
| `receipt.json` | `1ddd0a1facea8198a0cbd2f48452a3957a886ecada3757a9a7f0e5cb8ab58cc5` |
| `candidate-browser.log` | `f8dc0b52d31549147aca3106d82fc509b528873b28a5812197a2fcea4b8ba4c2` |
| `candidate-artifacts/test-instance-progress-pop-5de17-d-renders-accessible-states-chromium/idle-390.png` | `3bda70235320e9400fc73c51409a4b18b29b301b4bedc0f4c258da018473881e` |
| `candidate-artifacts/test-instance-progress-pop-5de17-d-renders-accessible-states-chromium/failed-390.png` | `8ea1d4d9a80065e4a5a9b60e647841ac7b88578a55d4b433376657e59767c215` |
| `candidate-artifacts/test-instance-progress-pop-5de17-d-renders-accessible-states-chromium/pending-1920.png` | `fd2c2de02aea1e68024ed4416644ff7c002a0d150f1e3ae5b9074be7bdc2cb8e` |
| `candidate-artifacts/test-instance-progress-pop-5de17-d-renders-accessible-states-chromium/saved-700.png` | `0293c73cbc024153fe67cb79b433d9443c977fd4313bd964f82218f66a663af4` |
