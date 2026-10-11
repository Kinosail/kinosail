# tvOS audit evidence

The private evidence archive is stored locally at
`/Users/mikeo/.codex/artifacts/tvos-navigation-20261011`.
Raw simulator diagnostics and full result bundles stay in that archive.
The repository contains the findings, reproduction instructions, and checksums.

## Reproduce the accepted navigation suite

Use Xcode 27.0 (27A266a), the tvOS 27.0 runtime, and a disposable Apple TV
simulator. The audit used macOS 27.0.1 on arm64, Python 3.15.0, and Pillow 12.3.0.
`environment.json` records those versions and the generated media metadata.

1. Check out the accepted source revision recorded below.
2. Boot one disposable simulator. Do not use a physical Apple TV or shared data.
3. Use the archived `movie.mp4`. It contains 30 seconds of synthetic test video
   and a generated tone. It contains no library media.
4. Run this command from `apps/player/apps/native`, substituting the simulator ID
   and an unused evidence directory:

```sh
python scripts/run-tvos-navigation-qa.py \
  --device <disposable-simulator-UUID> \
  --media <archive>/movie.mp4 \
  --evidence <new-evidence-directory> \
  --mode loaded
```

The Python environment must contain Pillow. The runner starts its own loopback
fixture on port 38359, creates a fresh synthetic identity, seeds the app, and
runs the native remote journeys serially. It records every Xcode command,
source input hash, fixture health check, exit code, timeout, and child cleanup.
The synthetic catalog has 160 movies, six index letters, four genres, a show
with two seasons, one music track, an audiobook, a photo, and a book.

The native tree captured by the combined run is
`4c6c51493b634eea511cf831e152975c9d73560a`. The combined 4K run records source
commit `ca03f21e7e95f9d53663fff1e34e78d9649a54db`. Merge commit
`9c1d3f9937b8e4c0c4cc97bc3f34dcc2aa50fa22` has the same native tree.
`reconciliation.json` records that comparison. The final production commit is
`2707f1f5`; reconciliation commit `f52619fd1fc70977cda717ac18393a9fe52e3e60`
has the same production source tree, `13763cef78cc7e7c99ba4b82538c3234ff936ed0`.
Its only production differences from the combined run opt into initial Retry
focus in the detail and audio error views. The final focused recovery runs
record all source hashes and verify those paths at 1080p and 4K.
Commit `5f09978e36a8330698be017476e2836182fbd6ed` then removes two trailing
blank lines. Its production source tree is `cdf07b0201866aa9717f363f73a784941f8f7c7a`.
The source diff ignoring blank lines is empty; no behavior changed.

## State and focused journeys

The archive's `run-focused.py` records the complete seed and remote commands in
each run's `source-and-run.json`. State tests require explicit fixture flags.
The artwork cases use `KINOSAIL_TV_ARTWORK_STATE_CASE` with `pending`, `retry`,
`empty`, `404`, or `offline`. Audio state cases require
`KINOSAIL_TV_AUDIO_STATE_CASE=1`. Xcode receives these names with the
`TEST_RUNNER_` prefix. Each accepted state run starts with a fresh identity.

Full Cast traversal uses `KINOSAIL_TV_ARTWORK_CAST_FIXTURE=1`. The ordinary
loaded suite uses the fixture's shorter cast row. The expanded Cast result is
recorded separately in `aligned-controls-1080`.

Home state runs use the retained runner with `--mode pending`, `--mode empty`,
or `--mode failed`. A timed-out run is rejected even if Xcode later exits zero.
The 100-return stress case has a 900-second allowance; other loaded cases keep
their 600-second default. The runner imposes a separate wall-clock limit.
The measured 4K run exhausted the old 30-minute wall limit during Settings.
The retained runner now permits 40 minutes; timeout rejection and child cleanup
remain enforced. The original timed-out bundle is not an aggregate pass.

## Verification boundaries

These are native UI journeys against an isolated HTTP fixture. They exercise
the real tvOS focus engine, rendering, playback UI, and remote button events.
They do not establish production-server behavior or physical remote behavior.
The fixture does not persist movie progress or implement audio progress sync.
The resulting audio sync notice is expected fixture behavior.

No physical Apple TV, Siri Remote, VoiceOver, accessibility text-size variant,
reduced-motion variant, production deployment, or live-library playback was
verified. The simulator rejected the appearance and increased-contrast commands.
The app's configured appearance is dark.

## Result and checksum index

An aggregate failure remains a failure even when an individual case passed.
The findings report identifies those individual cases explicitly.

| Local run | Passed / failed / skipped | Result bundle tree SHA-256 |
| --- | --- | --- |
| `accepted-loaded-4k` | 22 / 2 / 0 | `fb6a659945639faf7f736377e1d155b3c2ad5adeb98100fa24ece769a5af5b85` |
| `accepted-detail-pending-4k` | 4 / 0 / 0 | `46747523ccb94d8cf42c5cc7beadec406ec9509798e12176cdb4eaf7901a3295` |
| `accepted-states-1080` | 2 / 1 / 0 | `aeeb1b2fbf9178c434ff74dec4fd8dce4cfc093f2a0b44ced58b378382d17ff8` |
| `accepted-audio-retry-1080` | 1 / 1 / 0 | `0e66ae4f910e9844405799383da71d985533393ce6c96885fc447cef3c6e0272` |
| `final-audio-retry-1080` | 1 / 0 / 0 | `4f9812ad18dfc733512042e352d3e651b9c7c36ae60a12a12a177377a9026e2c` |
| `final-outage-4k` | 1 / 0 / 0 | `5d2ba3cddf334f088f5c702f6f51868c0efe4b80c16a62b65971da4c3646121b` |
| `final-audio-retry-4k` | 1 / 0 / 0 | `2ca2fbf633be3c2eb5b91b54f86dddbad9586abc53dad86237b201eff586b52b` |
| `aligned-controls-1080` | 3 / 0 / 0 | `21845ef1ce219c7f6599dface373d6acad5950ff834f0885c3b38dadf9cd9896` |
| `detail-pending-aligned-1080` | 1 / 0 / 0 | `e0e4648d9d5d2c0ee859a81e2c8e73abfd35d417336435c585e5f48b28ed08c9` |
| `detail-retry-1080` | 1 / 0 / 0 | `46d09bc36565a9176c0dcccf848d02200d2c2ac2ebc18e443aa87cff7186c863` |
| `detail-empty-1080` | 1 / 0 / 0 | `bb98ac0aa5af81488c6907e37c7bef975108c1c8e5ea7af8510c026562f62065` |
| `detail-404-1080` | 1 / 0 / 0 | `b93ee7d76943c8ec268d6203ba5cb684ca804c1d8714496db98fd6af93044fe3` |
| `home-pending-4k` | 1 / 0 / 0 | `0c25af930cc2090b0c1491f465200e9e1c0b6026d3d2f91f6e0d8b71f2df1d11` |
| `home-empty-4k` | 1 / 0 / 0 | `48ece6e67e480ce1712c502788278ded670e7fa293129ce1dd08aef29961a589` |
| `home-failed-4k-retry` | 1 / 0 / 0 | `4c9d5dc1a1fd5da1065423b68df15d67d321b3f7d664c7a23fe66a35ae4b60bd` |
| `sustained-outage-detail-4k` | 1 / 0 / 0 | `9f9c9a201e695e56f47773b8c2d67d3c0bbd47b7565c9813fb1cc8c4cbc77fb3` |
| `offline-audio-aligned-4k` | 1 / 1 / 0 | `7685558b77110dbdb3f945ee9ab46481f2e9187a9fcf8f7921bb64eb58b34514` |
| `recovery-geometry-red-4k` | 3 / 1 / 0 | `f7455e4a8be63078fb3db0c85605059ff8794826d9c861d9bdff4adbee734883` |
| `final-loaded-1080` | 23 / 2 / 0 | `41c92ac3489472085fecd7368009a7c94b86160325292cc3061036af4e620186` |
| `disconnected-detail-red-4k` | 0 / 1 / 0 | `471360d9a457b6122ce5a2d26f438c5db1f305ff61e32d796e4857006a27fe0e` |
| `pending-retry-red-1080-retry` | 1 / 1 / 0 | `953f3cde750fd43002eb55adca6ae2e77aca10e31ace960f91d34acb34e5c94b` |

| Local evidence file | SHA-256 |
| --- | --- |
| `movie.mp4` | `71771b8981828d2663d8ecf3d2b69f6ae8b6492c6f45a90cc855799e0c9323ea` |
| `environment.json` | `955763948d91eb9d52b89ccd97a33ef02a78f3b43f52361ad44530a0de468a1a` |
| `case-index.json` | `9ac6754dff73aa0f417d55c665832a3b07ab3e82a6b957194feb0974a45ff416` |
| `reconciliation.json` | `b1f9b142e2596ccbe127596ce18e0bda92df68181e3b566455aad028dc121a07` |
| `accepted-loaded-4k/source.json` | `6682910c97f380660e5c4ee40b4080ab7a123afd89054b83c6aa9e21853be211` |
| `accepted-loaded-4k/run.json` | `fadfc49893bbe6e025d084b14a67ae17843450afb8fee7ebe7d383b85cc77295` |
| `accepted-loaded-4k/remote.log` | `b9f95102082c5d9afbdad186060a121394cf3919c3ab74fae398c2dc01a4f569` |
| `final-verify-changed.log` | `c488a17266f0270c6984a67836f5181c24e9ad08f77dca2a0bc19a9fed8bbe6b` |
| `SHA256SUMS` | `f6cfa04f83d1487e9955a00b6724fadb858dc20ae451f9e6166259d49e2d23bd` |

A result tree checksum is SHA-256 over sorted lines of each file’s SHA-256,
two spaces, its path relative to the result bundle, and a newline.
`index-evidence.py` retains the exact algorithm. `SHA256SUMS` records every
archived evidence file separately, including screenshots and logs.
Verify those files from the archive directory with `shasum -a 256 -c SHA256SUMS`.
