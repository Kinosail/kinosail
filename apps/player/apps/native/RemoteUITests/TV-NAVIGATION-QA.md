# Disposable Apple TV navigation verification

These are isolated native UI journeys. They protect Home Back, spatial focus,
detail and player returns, season selection, and pending/empty/error navigation.
The populated-server browser journeys do not exercise the tvOS focus engine.

The catalog contains 160 synthetic movies, four genre shelves, a two-season show,
and other media categories. Every video route serves the same generated 30-second
MP4. The fixture acknowledges progress without persisting it. These runs do not
establish real-library playback, saved-position reopening, codec diversity, or a
50-video sample. They also do not measure physical remote latency, HDR or audio
output quality.

Use an existing disposable tvOS simulator and run one native build/test owner at
a time. Never use a simulator with a real saved session. The opt-in seed refuses
to replace a nonfixture session. It runs only on a tvOS simulator, keeps Viewer
Profile `qa` fixed, and assigns a fresh synthetic Server identity to isolate each
state's catalog cache. It preserves existing cache files and simulator data.

Prerequisites: Xcode with an installed tvOS runtime, FFmpeg, and a Python
interpreter with Pillow. Confirm capacity before building. Generate media in a
task-owned directory; do not pass original library media:

```sh
ffmpeg -n -f lavfi -i testsrc2=size=640x360:rate=24 \
  -f lavfi -i sine=frequency=440:sample_rate=48000 -t 30 \
  -c:v libx264 -pix_fmt yuv420p -c:a aac -movflags +faststart /path/to/qa/movie.mp4
```

From the native directory, substitute the disposable simulator UUID and fresh
output directories. The helper owns the loopback listener on port 38359, seeds
the fixture session, then runs XCTest remote inputs serially. It bounds the
process duration, checks fixture health, joins its processes, and retains source
hashes, exact commands, logs and `.xcresult` bundles:

```sh
python3 scripts/test-tvos-navigation-fixture.py
python3 scripts/test-tvos-navigation-runner.py
python3 scripts/run-tvos-navigation-qa.py --device SIMULATOR-UUID \
  --media /path/to/qa/movie.mp4 --evidence /path/to/evidence/loaded --mode loaded
```

Repeat the `run-tvos-navigation-qa.py` command with modes `pending`, `empty`, and `failed`, each using
a new evidence directory. The loaded mode runs 100 detail-return trials, held
direction reversal, layered Back, direct Select/Play-Pause returns, season
changes, and existing directional/category/player-options journeys. It records
the ordinary Up path's movement count and the one-Back Home shortcut.

The pending run checks the skeleton, usable top navigation, and removal of the
skeleton after real pending work completes. Empty and failed runs verify Back
to Search; the failed run also selects Try again and observes recovery. Those
cases require their matching seeded fixture mode. New fixture cases skip unless
`KINOSAIL_TV_FIXTURE_QA=1` is present in the test runner.

Export screenshots through the supported result tool:

```sh
xcrun xcresulttool export attachments --path /path/to/evidence/loaded/remote.xcresult \
  --output-path /path/to/evidence/loaded/attachments
```

Inspect `source.json`, `run.json`, the complete result bundles, and the rendered
attachments together. Exit zero alone is insufficient: confirm the expected
tests actually ran. A failed, skipped, interrupted or timed-out run is not
passing coverage. Check that no task-owned runner or diagnostic process remains
before another owner uses the simulator. XCTest's elapsed time includes test
automation overhead and is not app input latency.
