# App QA setup

Use `$app-qa full audit` for a journey audit or `$app-qa change audit <PR or diff>` for changed behavior. Read `SKILL.md`, then update only this configuration section when copying the skill to another app. Store account secrets in local environment variables or the app's existing test fixture, never in this skill. Use an absolute path for the isolated root because Playwright runs from its own directory.

## Configuration

| Field | This repository's Player web target |
| --- | --- |
| Start command | `cd apps/player && KINOSAIL_TEST_PROJECT=<unique-qa-project> KINOSAIL_TEST_ROOT=<absolute-isolated-root> ./scripts/test-instance.sh up` |
| Base URL | `cd apps/player && KINOSAIL_TEST_PROJECT=<same-project> KINOSAIL_TEST_ROOT=<same-absolute-root> ./scripts/test-instance.sh url` |
| Test accounts | Isolated test Owner from `test-instance.sh`; Playwright reads `KINOSAIL_E2E_OWNER_NAME`, `KINOSAIL_E2E_OWNER_PASSWORD`, and `KINOSAIL_TEST_TOTP_SECRET` from the test fixture or environment. Do not copy values into reports. |
| Critical flows | Owner sign-in; Viewer permissions; browse/search; video playback and subtitles; downloads and offline recovery; settings and navigation. Pick affected flows for a change audit. |
| Test-data reset | `cd apps/player && KINOSAIL_TEST_PROJECT=<same-project> KINOSAIL_TEST_ROOT=<same-absolute-root> ./scripts/test-instance.sh down --volumes`; then remove only the isolated root after evidence is saved. |

Player and Subtitles are the two web apps in this repository. Both use pinned Playwright packages and `@axe-core/playwright`. Run `pnpm --dir apps/<app>/e2e install --frozen-lockfile` if dependencies are missing. For Subtitles, change `apps/player` to `apps/subtitles` in the configuration above and use a separate project and root. Use `make -C apps/<app> test-instance-check` for its default isolated browser suite. For targeted tests against a running instance, export `KINOSAIL_TEST_INSTANCE=1`, `KINOSAIL_E2E_URL`, and fixture variables from that instance before `pnpm --dir apps/<app>/e2e test <spec>`.

The Swift iOS, tvOS, and watchOS clients live in `apps/player/apps/native`. Build with `./scripts/build-apple.sh ios`, `tvos`, or `watchos` from that directory. The Android phone, TV, and Wear clients live in `apps/player/apps/android`; its README lists the JDK 17, SDK, Gradle test, and build commands. Use emulator or simulator interaction and native tests for native QA; report physical device and remote input checks separately. Do not send purchases, email, or personal media through production during an audit.

Save each audit using `references/report-template.md`. Keep local screenshots, traces, and video beside the report or in the test runner's output directory, and link to the exact artifacts. Include the revision and commands so another engineer can replay the run.
