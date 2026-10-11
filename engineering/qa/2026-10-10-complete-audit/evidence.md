# QA evidence and replay index

Private evidence root: `/Users/mikeo/Documents/Codex/complete-qa-20261010/`.
Raw diagnostics, session data, traces, device identifiers and result bundles remain private.
Each selected artifact below has a SHA-256 digest. The private full manifest covers all preserved files.

## Source and environment

Baseline: `2f62453888c3707587de0875709772290a728106`.
Native repair: `5e2443f8f1f6928306bc56fdc35f4a8b97bf44a9`.
Reconciled source: `6dfdf8dafff0f7f1d6c449efd375802ef03104a7`.
Quick Connect image: `740fbe99f39a4542f6e11775988d7175d2290741`.
The final theme test is included in the audit delivery commit; its source and run are preserved privately.
Environment versions are in `environment-final.json`; native media hashes are in `native-media.json`.
`source-equivalence.json` applies to the baseline and reconciliation, before the CSS repair.

## Replay

- Tooling: `make tooling-check`.
- Full package and static gates: `make packages-check`; `make quality-static`. See known failures in the report.
- Native: use the exact `xcodebuild` commands in `native-replay.json`. Create compatible disposable simulators and substitute their identifiers.
- Android client: `apps/player/apps/android/qa/server-contract/run.py`; exact commands, generated data and hashes are in its receipt.
- Android units: `./gradlew --no-daemon :app:testDebugUnitTest :watchcore:testDebugUnitTest :wear:testDebugUnitTest` from the Android directory.
- Hosted matrix: CI run `38099145590`, with coverage and scan diagnostics enabled. Retain attempt one and the unchanged Swift retry.
- Public browser: the private `public-player` receipts record exact commands, image IDs, source revisions, TLS pins, synthetic data and fixture corrections. Recreate disposable credentials; do not reuse session artifacts.
- Quick Connect regression: run `layout-audit-library.spec.ts`, `mobile-quick-connect.spec.ts`, and `quick-connect-scan.spec.ts` against the populated disposable Server.
- Download gap: use the exact opt-in commands in the download and expansion receipts. Do not replace their failing assertions with synthetic completion.
- Final local gates: `make max-loc`, `git diff --check`, and `make -C apps/player verify-changed` after committing app changes.

## Selected checksums

| Private artifact | SHA-256 |
| --- | --- |
| `tooling.log` | `a7b7e65497c561e21f49271eb10ccf85dd0afc8fa186ead25e6f0cb1d5e0ae8c` |
| `packages-check.log` | `8eaa38efc37647b9c43b21d9b50e317902c59a6de13654c3a229003a6aed0ff0` |
| `quality-static.log` | `9c6048e339faca1365bbe5ddad175a28fbfae4fa0d239323a63cd4393ae97d3c` |
| `ios-summary.json` | `47ffa299e5b7a13a4e22c7783ddaa71a1906670eb2da462ae4a030cdbe08f79c` |
| `ipad-contracts-summary.json` | `804b8c66ebff943186186dedf0d2a026a1aa7012f8a437cb3d82df8534c091f2` |
| `ipad-touch-summary.json` | `05269043c6c38479331992a68c9b66ad023c3a3119dd128582d14ef09441d9e0` |
| `iphone-touch-summary.json` | `2de5281fb92cd38b993b48159b4a88fd902d9daf26e583caf752712d9c1899c8` |
| `native-replay.json` | `a7fe249b2261c90e676210940cc84efd28fdabe6ba90179fa11c98c436170fd1` |
| `native-media.json` | `3ad14afaccb6c0a4db7725b528310f0d00008dad7251248c85843f5c02fe410c` |
| `android-server-contract/receipt.json` | `007ffe55ee3b2cd9244c1c0ad1133d226c5eb8e4b6ad420940b400b82ee9cbf6` |
| `android-reconciled-results.json` | `f22562ad5e210775937dfc83b5edf82f3c15f638318b084157f793e5f3ef2a27` |
| `deep-ci-attempt1.json` | `2915dabb2201f4b4adcaf2cc74cfe7cf9d4bdd9ac46482a2a49f13df67306e61` |
| `deep-ci-final.json` | `5d72d8ba9ff579af47baf1302eead3b4167500fdbf97d431c17e41a1e08c7bf9` |
| `hosted-ios-attempt1-summary.json` | `76a6e8fbd289ea88acd8e1f321a4d0f70e8d300d4ee4ea61f1f5762616ea1648` |
| `hosted-ios-attempt2-summary.json` | `2d9c64f0b528e8d378e6a63d05b8a6f0453cb0b27ca7a9c7559f9d3433f6c3d3` |
| `hosted-tvos-attempt2-summary.json` | `f78a5618b2d6b0f38d85bf1c5037e2a6d1886ad4906dbf34c8ffcddd1b664822` |
| `browser-batches.json` | `46e7517d51b21ae22cf3055dc1d000c9992693011154fd16a8478490f76134d9` |
| `bfcache-diagnostic.json` | `167b5d2e64de77daee28c2e137861253aae1c326aa5ece9a8a77b563af16122b` |
| `download-range-diagnostics.json` | `8cc05903a354f78f15a24d4880e6a2bd675d966042473dcebe8ed3ba3840d2cf` |
| `player-verify-css.json` | `1744d1a31fe1485eca325148fc8c74ca8b5101419d5746007cf23ce38add7f2e` |
| `public-player/source-build.json` | `d1dae055d3031005595f0175381b5d75791d6fd62beb98b7a550bdf76709ea08` |
| `public-player/compact-landscape-red2/receipt.json` | `ba4daf85e517c3d1dc73f761520305c32f77217b8b1d74380eccf6a3cc031f12` |
| `public-player/compact-landscape-red2/results-chromium.json` | `f9ed4eec047e268b28753f9e05796537b9a4c85ba345dab3675e2103162f671e` |
| `public-player/expanded-remainder/results-chromium.json` | `2fe3db211eac2ddd3683013ed95f271b27f091f528a02d2a15d0d6b2ad1eef1f` |
| `public-player/fixture-recovered/results-chromium.json` | `5ef1f523e6da71cf0b05d56d142396280c0c608c9422976cb035110b97f21502` |
| `public-player/fixed-build-stream.json` | `1b516bb77e1b1e481a305af4dbf61f5f387eafe330a4a6c704008f39bdb97c35` |
| `public-player/fixed-reconfigure.json` | `8675af87c0de298523738a06524cb407d06842cc4a5dae9756eda8a6f2b3cbbd` |
| `public-player/post-fix-remainder/results-chromium.json` | `e4145c82f613347ea139e0daaa68b1f8156519abb7a915f17fa7a55945cc24bd` |
| `public-player/expansion-focused-repeat/results-chromium.json` | `fb670f0365fca6274ed30f01a48734ed3f54f5463ac82a536e69023369f826f0` |
| `public-player/supporter-env-corrected/results-chromium.json` | `ce8579b0c42282af01fa650875abcc6256e1a62e03f30fa95eb457005cbd99fd` |
| `public-player/theme-public-verified/results-chromium.json` | `3c414ad3d5fbe50cc5fad74e7d26ed0e8bc58a8f2920bfb5f1db0ef27306611b` |
| `public-player/quick-connect-visual/measurements.json` | `990db941c1a7fa0ccdf666ef74504a6c841ded840c0e3743fd3df3746376d116` |
| `remaining-browser-cases-final.json` | `596d9378f485c240065cca93f0e9d3f12716408b593e56f0b742965bd5ae63a6` |
| `source-equivalence.json` | `b85455ee23f2aeade2d970402803d44369b279be031bb4deab0f69cbfd9e095d` |
| `local-resource-cleanup.json` | `2e5274c315cc4c449aa654ca9beca97bb4de675556f1e09d5e01bf422787e465` |

The full private manifest excludes its own digest. Build directories are omitted; native result bundles, media, logs, screenshots and traces are retained.
CI, container validation, publication, deployment, TLS and physical interaction remain separate evidence.
