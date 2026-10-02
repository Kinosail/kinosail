# Android playback header contrast

A real decoded light video made the Android playback title almost invisible. The existing header now has an opaque black backdrop. The repair adds one background modifier. It changes no inputs, playback policy, dependencies, focus logic or production test seams.

The black backdrop covers the existing title and control area. The video viewport and normal Media3 controls retain their layout. The shared header also serves audio and Android TV playback.

## Regression and environment

Baseline source: `2f8f8e1401718a543a5ccd58ad6bae29c42390b5`. The regression was written before production changed. Two unchanged phone runs failed for title contrast of 1.0085:1 over a real decoded light frame. The expanded test also failed for that reason. Its pending, failure and recovery journey passed on the baseline.

The actual app used its encrypted SessionStore, catalog API, playback API and Media3 decoder. An isolated actual Go Server served generated 120-second white AVC video and captions. The API 23 ARM64 emulator runs the app's minimum supported Android version. A loopback HTTP bridge verifies the isolated Server's TLS certificate upstream. This does not prove native HTTPS trust on a device.

Each instrumentation context records revision, pending production diff hash, APK hashes, command, device and result. The screenshot and semantics files contain generated fixture content. Credentials and private session files are excluded.

## Repeat the journey

1. Start an isolated Player Server on HTTPS port 39227. Keep its CA at `apps/player/.kinosail-test/host/data/tls-ca.pem`.
2. Copy the generated files in `fixtures/` into its Movies folder. Trigger an Owner library scan. The saved bootstrap source records the public scan and catalog checks.
3. Copy `native-media-bridge.py.txt` to `apps/player/.kinosail-test/video-overlay/native-media-bridge.py`. Run it with Python. It binds only host loopback port 39231.
4. Create a private seed for a synthetic Viewer native credential. Use version 1, server `http://10.0.2.2:39231`, token, exact generated title, and capture prefix. Keep the file private. The test loads the current Viewer through the public API.
5. Build the app and instrumentation APKs with JDK 17 and the configured Android SDK. Install both on a task-owned emulator.
6. Copy the seed to the app's private `files/video-overlay-seed.json` with `adb push` and `run-as`.
7. Run `adb -s emulator-5564 shell am instrument -w -r -e class com.kinosail.player.mobile.LivePlaybackOverlayTest com.kinosail.player.dev.test/androidx.test.runner.AndroidJUnitRunner`.
8. Require `OK (2 tests)` in the result. Instrumentation can exit zero when assertions fail. Extract the PNG and semantics files from private `files/video-overlay/`.

Each journey resets only its generated title through the public progress API. Pending local progress for that title is reconciled through the same public API. The fixture's bounded operator endpoint holds media pending or returns HTTP 404. Catalog, plan, captions, progress and preference responses remain actual Server responses. The tests release pending work, exercise terminal failure, retry, decoded video, caption toggling, speed selection and dismissal.

## Final results

The final test source failed on the byte-identical baseline owner at 1.0085:1 title contrast. The repaired source passed all eight journeys across phone portrait/landscape and tablet portrait/landscape. These use a native 1080×1920 pixel display: phone density 480, tablet density 208, and actual activity rotation for landscape. They are layout configurations on one phone AVD, not separate tablet hardware.

Loaded title contrast is 21:1 over black. Enabled action labels meet the test's 4.5:1 minimum and fit the compositor capture. Pending indicators disappear after terminal failure and recovered decoding. Caption toggles, the speed picker, Normal selection, retry and Done work. Six representative loaded/pending/failed captures were visually inspected together.

[Open the capture gallery](gallery.html). The exact APK/source hashes, commands and results are retained. The unchanged empty library has no playback header. Both existing content-error accessibility tests passed on the API 23 emulator, including unavailable TV playback content. This does not establish loaded TV playback.

## Verification notes

The first wide-layout attempt changed logical dimensions without rotating the legacy emulator. It timed out during catalog navigation and did not reach playback. Its log and context are retained. The corrected harness rotates the actual activity and keeps native display dimensions. The test uses the public Activity orientation API; playback remains the actual client path. A tablet landscape screenshot check later timed out despite an adb capture showing decoded video. Using the same physical `screencap` path in all configurations passed that journey. Failed diagnostics are retained; no decoder repair is claimed.

The full Gradle Android check passed 120 app tests. The 6 watchcore and 5 Wear results were accepted as up-to-date. App, instrumentation and Wear debug builds passed. The hosted workflow builds Android production APKs and scans Kotlin. It does not run this opt-in live emulator journey.

## Remaining boundaries

API 37, physical phone/tablet/TV, Cast devices, API 26+ Picture in Picture, TalkBack service behavior, signed store builds, long playback and deployment remain separate checks. The failed wide-layout run establishes no catalog defect. The TV activity attempt on the phone emulator stopped at Home navigation before playback. The shipped regression is limited to MobileActivity. No loaded TV runtime result is claimed. The unchanged empty library has no playback header; the existing unsupported-content accessibility test covers unavailable playback. Hosted CI and protected-main delivery are recorded separately after they finish.

## Reconciled source

The branch incorporates main `2d00b19a6b9502b6fd02149c8c13dd1f80e96410`. Upstream changed Swift validation and research evidence; the Android tree is unchanged. Source hashes still match all runtime evidence. The post-commit Player changed-app gate passed. Artifact secrets scans found no leaks. Protected hosted checks must finish before merge.

## Fixture input guard

Before the guard, an unsafe capture prefix was accepted and generated-title progress changed. The saved negative control failed. The test now parses strict JSON and rejects missing, unknown, malformed, duplicate, oversized and conflicting seed inputs before any application action. It accepts only the isolated emulator bridge, the two generated titles and a bounded filename prefix. Eleven negative controls pass with authoritative generated-title progress unchanged. All eight valid mobile journeys pass again with this exact test source. The app production code is unchanged by this fixture guard.
