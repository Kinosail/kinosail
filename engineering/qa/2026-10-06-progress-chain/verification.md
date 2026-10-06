# Native HLS source-offset verification

Base: `f42b72a183d667348ac727b43e7efa4529ac188d`. Production changes are limited to two Player JavaScript assets. Server storage, HLS encoder/cache, authentication, Nox and native applications are unchanged.

| Observable boundary | Before | Repaired |
|---|---|---|
| Unknown duration, saved 22 | Stream origin 0 | Offset recipe 22000ms; progress 22 |
| Replacement at 22, old decoder 22 | Progress 44; projected duration 92 for known 70 | Progress 22; known duration 70 |
| Replacement metadata arrives, local decoder 1.25 | Not reached in failing baseline | Position 23.25; later progress 23.25 |
| Superseded metadata cleanup | Initial repair rewound already-buffered frame 10.8 to 10.3 | Existing buffered-frame scenario retained 10.8 |
| Unknown-duration offset bounds | Valid saved 22 was dropped | 604800s accepted; nonfinite, negative and above 604800s rejected |

The baseline reproduces two client failures on unchanged e85544ebc. It queues an old decoder Pause. The final fixture also queues old seek completion and checks new metadata releases the requested-position hold. Browser fixtures use simulated native metadata and intercepted progress responses; they do not establish actual HLS decoding or persistent storage.

Focused acceptance:

- Chrome: 33/33 passed in 12.6s. Covers source windows, paused seeking, buffer starts, Apple launch, stale Play rejection, explicit Pause and preparation progress.
- Installed macOS Playwright WebKit 2359: 16/16 native/source seeking cases passed in 6.8s.
- Populated Go Server with generated 70-second H264 clip: 6/6 repeated public progress-chain cases passed in 25.3s. Real HTTP readback and decoded-frame callbacks prove accepted 22 reentry through Browser Back and Library. The web endpoint ignored an old revision with 204; the canonical API rejected it with 409. Accepted storage stayed unchanged.
- `make max-loc` and `git diff --check` passed. Independent working-source review found no remaining blocker; exact committed-head review and protected CI follow publication.

Repeat with one worker:

```sh
cd apps/player/e2e
KINOSAIL_E2E_VIDEO=off pnpm exec playwright test player-seeking.spec.ts player-native-seek-gap.spec.ts player-apple-launch.spec.ts player-apple-intent.spec.ts player-preparation-progress.spec.ts --project=chromium --workers=1
KINOSAIL_BROWSER_PROJECT=webkit KINOSAIL_E2E_VIDEO=off pnpm exec playwright test player-seeking.spec.ts player-native-seek-gap.spec.ts --project=webkit --workers=1
cd ../../..
GOMAXPROCS=2 python3 -B apps/player/scripts/test-player-checkpoint-local.py --phase candidate --project chromium --grep "progress chain"
```

Private artifact receipts preserve source fingerprints, exact commands and checksums for logs, screenshots and observable outcomes. State includes only disposable local data. Receipt checksums:

- `.verification/native-source-gaps/baseline-receipt.json`: `b14141d127e16693d18b8d4eafa4f96fe1401cf4b85f9df8d589f01024c91e8b`.
- `.verification/native-source-gaps/accepted-chromium-receipt.json`: `a19397a80fc17e61f4a3ab7766e2c77ecad45476370a612de3cc632fcb35799a`.
- `.verification/native-source-gaps/accepted-webkit-receipt.json`: `53748812e10917f5042df4d9f69ab1d6dfd2efad395233cbc575ca5cfd41c280`.
- `.verification/paused-seek-checkpoint/20261006T145003Z/receipt.json`: `268d1b8edec359193aa207740b6058e785a84fd50398426375ffd260f956e8cd`.

Production fingerprints:

- `apps/player/internal/server/static/player.js`: `a3d6f4534ee8e1df3abce255836efe5862486cd8bd26b01868f9aa613eb367d3`.
- `apps/player/internal/server/static/player-streaming-adaptive.js`: `fc3115417f2a5619ac327e416529529084bbfa1fccca833f2efc583f79f5d8e6`.

These are reliability checks, not startup latency comparisons. The original frozen 50 receipts remain unchanged. This repair does not prove the cause of the live zero starts, native fullscreen short timelines, stalled reentry, green/block images, audio quality, or physical Apple TV playback. No simulator, device install, server deployment or Library write occurred.
