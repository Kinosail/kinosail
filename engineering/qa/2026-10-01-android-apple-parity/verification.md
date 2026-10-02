# Android and Apple parity verification

Failure paths identified before implementation:

- Navigation can lose access to unpinned destinations, share another Viewer's choices, lose Back behavior, or frame playback with library chrome.
- Home can mix listening and watching, count watched or dismissed titles as continuation, duplicate its feature, or omit media categories behind a single limited mixed query.
- Hero/episode artwork can crop the picture, use a poster as a landscape image, or load an external/unvalidated path.
- Cached launch metadata can disappear after a restart; unknown duration must not invent a completion percentage.
- Pending layouts can jump, remain after empty/error results, or hide retry and loaded cached content.
- TV focus can jump when network content arrives; D-pad focus must retain breathing room and Back must return to the previous page.
- Wear seek can target a different title after refresh, exceed duration, or issue requests while busy; heart readings remain local and opt-in.

Verification uses synthetic data and isolated Android emulator sessions. Build, local tests, emulator renders, hosted CI, and physical devices are separate evidence.
