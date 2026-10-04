# Public-flow E2E checks

These tests use `tester-army/e2e` against real Player and Subtitles processes.
The runner uses exact browser and API actions. It needs no model credentials.
Telemetry is disabled. Dependency versions are pinned in the lockfile.

Requirements: the repository Go toolchain, Node.js, pnpm, FFmpeg with H.264/AAC,
and the Chromium version required by `@e2e-dev/web`.

1. Run `pnpm --dir scripts/e2e install --frozen-lockfile`.
2. Install Chromium with the project's Playwright CLI if it is not cached.
3. Run `scripts/e2e/run.sh` for both apps, or append `player` or `subtitles`.
4. Read `scripts/e2e/.e2e/runs/<run>/runner/summary.md` and the context manifest.

The script builds selected app binaries and removes them when it exits.
Each process uses a separate loopback port and temporary media, data, cache,
and backup directories. The fixture generates a moving eight-second movie,
AAC audio, and two English subtitle cues. It removes its directories on exit.
No container, shared deployment, or personal media directory is used.

The context manifest records the command, revision, environment, and result.
`inputs.sha256` records this runner's fixture, tests, config, and lockfile.
Runner JSON, Markdown, JUnit, and screenshots record individual assertions.
Traces are disabled to bound storage use. A failed command remains a failure.

The existing hosted Chromium job runs this suite for each affected app.
Its artifacts include the checksummed context and runner results.
Player skips subtitle editing because that operation belongs to Subtitles.

This suite adds process and persistence coverage to the existing tests.
It does not prove all media formats, browsers, external providers, or devices.
See `engineering/qa/2026-10-04-public-flows/README.md` for the full inventory
and the current verification boundaries.
