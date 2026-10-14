# UI polish across the active apps

This pass preserves the Electric design system and each platform's navigation. It refines presentation without changing API behavior.

- Player web reserves two lines for poster titles and uses tabular saved-position values.
- Shared web empty states use readable copy widths and clearer action spacing.
- Subtitles reduces overview spacing, wraps closed file titles, and enlarges narrow status labels and history targets.
- Apple clients constrain recovery descriptions and use system control sizing. Watch titles expand at accessibility sizes.
- Android aligns media titles and completes the Electric semantic color roles. Wear separates playback from supporting actions.

## Evidence

The production build revision is `cd82bf03`. The Android TV test correction is `df0c3807`; it scrolls to Browse before selecting Movies. Later documentation changes do not change the application sources.

| Check | Result |
| --- | --- |
| `make max-loc`; source diff whitespace | Passed |
| Subtitles `make test-instance-check` | 66 Chromium tests passed |
| Player focused Chromium UI checks | 23 tests passed on a disposable loopback HTTP Server |
| Player `make test-instance-check` | 5 passed; 82 failed because the disposable HTTPS certificate was not trusted |
| iOS, tvOS, watchOS simulator builds | Passed |
| Android app/Wear packages and instrumentation packages | Built |
| Android phone and tablet fixture journeys | Three tests passed on each device |
| Android TV fixture journeys | Two passed initially; the corrected catalog journey then passed |
| Wear accessibility fixture checks | Two tests passed |
| Apple TV fixture journeys | Failed/retry, empty, and pending/loaded journeys passed; no fixture health failures |
| Impeccable detector | 184 advisories from the existing type, radius, and color declarations; no blocking findings |
| Both apps' `make verify-changed` | Stopped on 95 shared-package lint findings in unchanged Go files |
| Android app lint | 43 existing Media3 opt-in errors in two unchanged playback files |
| Wear lint | Passed |

The retained screenshots and their SHA-256 values are listed in `manifest.json`. The manifest also identifies log checksums and Apple TV process-join receipts. Full logs, result bundles, and browser reports are retained privately with the task artifacts.

## Repeat the checks

Run each app's `make test-instance-check` from its directory. The script creates generated CC0 media and an isolated Owner. Player's default HTTPS fixture needs a trusted local certificate.

For the focused Player run, create a disposable instance with `scripts/test-instance.sh up`. Use its loopback HTTP address for this UI check. Set `KINOSAIL_TEST_INSTANCE=1`, `KINOSAIL_E2E_URL`, `KINOSAIL_TEST_TOTP_SECRET`, and `KINOSAIL_UI_FIXTURE_DIR` from that instance, then run:

```sh
pnpm --dir e2e test polish-shell.spec.ts home-stack-layout.spec.ts \
  skeleton-layout.spec.ts masthead-spacing.spec.ts conditional-states.spec.ts --workers=1
```

The conditional-state and skeleton checks use rendered fixtures. They provide isolated layout and accessibility evidence. The shell and masthead journeys use the populated Server.

From `apps/player/apps/native`, build with `scripts/build-apple.sh ios`, `tvos`, and `watchos`. Run `scripts/run-tvos-navigation-qa.py` serially with `--mode failed`, `empty`, and `pending`, a QA simulator ID, and a generated MP4. Use the bundled Python runtime with Pillow installed.

From `apps/player/apps/android`, build the debug app, Wear app, and instrumentation packages. Run `NativeParityJourneyTest` methods `homeMatchesAppleLandscapeShelvesAndPersistentTvNavigation`, `pendingFailureEmptyAndRecoveryKeepTheirOwnState`, and `catalogGridKeepsPendingFailedEmptyAndLoadedStatesDistinct`. Use `-e tv true` on TV. Run `WearAccessibilityTest` on Wear.

## Proof boundaries

Native fixture checks do not establish live Server compatibility or physical-device behavior. iPhone and iPad captures show cached synthetic content with the connection banner; they do not prove every library state. Apple Watch was captured connecting, and Wear was checked unpaired. Active paired-watch transport remains unverified.

No physical-device, live deployment, performance, or race run is claimed. Hosted checks and container publication remain separate delivery evidence. No API or diagnostic failure path changed.

The primary checkout's native project and localization edits were preserved. Older inactive worktrees were reported without changing their files or leases.
