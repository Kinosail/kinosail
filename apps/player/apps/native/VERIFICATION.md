# Swift Apple verification

## SDK 27 readiness follow-up — 2026-09-19

Toolchain: Xcode 27.0, build 27A266a. Deployment minimums remain iOS/tvOS 26. Source includes authenticated foreground Search library / Play title / Continue watching App Intents, opt-in dynamic Top Shelf, per-Apple-TV-user storage, scoped media navigation and the app-only UserDefaults privacy declaration. The existing native design guidance kept these additions in system navigation and progressive settings rather than adding a new app shell.

Evidence from this follow-up:

- Both simulator targets and both unsigned device targets build with SDK 27 using the four documented `build-apple.sh` commands. The iOS bundle contains metadata for all three App Intents. These are compilation and packaging results, not executed intent results.
- A signed iOS Release build succeeds using the existing development profile. A signed tvOS Release build fails: Xcode reports `No Accounts` and missing profiles for `com.kinosail.player` and `com.kinosail.player.topshelf`. The TV app and extension now need App Group and User Management provisioning. No new physical TV installation is claimed.
- iPhone 18 Pro / iOS 27 and Apple TV 4K / tvOS 27 simulators install and launch to the live setup screen, including nearby Server discovery. Runtime inspection caught and corrected a missing environment around the Top Shelf modifier and a Keychain query option rejected by the TV simulator. The final TV setup screenshot has no secure-storage alert.
- URL-opening requests reach the system confirmation dialog. Device Hub computer-control requests repeatedly time out, so confirmation, populated browsing, playback, Shortcuts execution and Top Shelf activation were not completed through the UI in this follow-up. The historical intermittent TV black screen has no new stable reproduction or established root cause; it is not marked fixed.
- `make max-loc` and `git diff --check` pass. Added Swift regression sources cover malformed, duplicate, unknown, oversized and cross-profile links, invalid intent input before restoration, and rejected shelf data before filesystem effects. **No test suites were run** because `.gates-disabled` remains in force.

Local build logs: `/tmp/kinosail-27-{ios-build,tv-build,ios-device,tv-device,ios-signed,tv-signed}.log`. Local screenshots: `/tmp/kinosail-{ios27,tv27}-current.png`; the `*-link-rejected.png` filenames contain the system confirmation dialog, not proof of app-level rejection.

Remaining acceptance work: sign in to the developer account in Xcode and provision the TV capabilities; approve Icon Composer's first-use license before producing the layered iOS icon; run the enabled suites only after explicit authorization; exercise real-user switching, Top Shelf play/detail and privacy changes, authenticated Shortcuts, real media/HDR, interruptions, PiP, downloads, VoiceOver/Siri Remote and App Store distribution. Build success does not make this a 100% readiness certificate. Existing icons remain unchanged pending the Icon Composer decision.

## Historical implementation evidence — 2026-09-12

## Scope and source

This work resumes the interrupted `finish-swift-apps` checkout. Its three existing commits implement sessions/catalog, protected playback/progress, and verified downloads/reading. The final batch completes receiver contracts and controls, captions, photos, artwork request coordination, session cleanup and the native blue/neutral design direction. The active `Sources` tree has no unimplemented service or platform operations and no preview gallery. The previous JavaScript client was retained as reference material during this implementation and removed in the subsequent repository cleanup.

Only native Apple files were changed. Existing server APIs, web apps and `.gates-disabled` were preserved. iPhone and iPad share the iOS target; tvOS is independent. The user subsequently authorized signed-device installation while leaving tests disabled; both Info plists now permit the configured deployment helper.

## Builds

Using installed Xcode 26.6 (17F113), these commands succeeded during this run:

- `./scripts/build-apple.sh ios`
- `./scripts/build-apple.sh tvos`
- `./scripts/build-apple.sh ios device`
- `./scripts/build-apple.sh tvos device`

All four initial builds succeeded at `08aa9a01` after reconciliation with main. Subsequent verification-note edits do not change the built production sources. Simulator builds use ad-hoc signing for Keychain. Those initial device builds were unsigned and were not installations. Logs are retained locally under `.verification/swift-finish/`. The App Intents metadata tool reports that extraction was skipped because there is no AppIntents dependency; this is not an application compile failure.

## Manual simulator observations

The disposable loopback fixture is outside production sources and uses synthetic credentials and media. Its responses do not prove compatibility with a deployed Server.

- iPhone 17 Pro, iOS 26.5: restored the fixture Keychain session, loaded populated Home/artwork, opened Resume and played a real MP4 through the authenticated transport. Native AVKit exposed full screen, volume, skip, pause, seeking and playback speed. Timed external captions appeared. Playback progress requests reached the fixture.
- iPad, iOS 26.5: entered a Server address, displayed the QR/six-digit pairing screen, completed fixture Quick Connect, loaded populated Home, opened title details and download options, selected the supplied audio track and queued a compatible download. Request evidence includes download identity, preferences, preparation, manifest and media-file retrieval. A wrong fixture track route was corrected to the authoritative `/download-tracks` route; production code already used the correct route.
- Apple TV, tvOS 26.5: installed/launched, displayed setup, discovered nearby Servers, and opened the system text-entry UI with the remote Return action. Inspection exposed dark text on the dark setup surface; the final batch supplies explicit TV foreground roles and a readable native toolbar heading.

The Mac locked during interaction, and Computer Use reported that automatic unlock was paused after physical input. No attempt was made to bypass the lock. Final UI interaction and populated TV playback were therefore not completed. Final headless screenshots at build revision `08aa9a01` show the blue/neutral populated iPhone Home in dark appearance, populated iPad Home in light appearance, and the corrected readable TV setup heading and labels. These are visual evidence only; they do not establish remote focus or final interactive behavior.

## Follow-up fixes and device delivery

The follow-up uses the installed Impeccable skill and its Apple native guidance. The shared blue/neutral palette is retained, with increased-contrast roles, a horizontal Home feature when enough width is available, larger native audio/reader controls, adaptable audio options, contextual recovery headings and dark photo presentation. The final audio transport uses separate track navigation to avoid fitting five large controls into a narrow row. These changes compile on both targets; the final responsive and accessibility interaction sweep is blocked by the locked Mac.

Manual iPad inspection found a completed, hash-verified download stuck at “File verified, but local playback failed.” AVFoundation could play the MP4 but could not infer its type from the private `.media.part` name. Verification and playback now use the same bounded file-header identification and forbid external asset references. Unknown formats, playlists and nonlocal/symlink paths remain rejected. Focused negative regression sources were added without running them.

With the repaired simulator build, Resume completed verification and exposed Play offline. The fixture server was then stopped, and Play offline visibly played the saved video through native AVKit. A screenshot is retained at `.verification/apple-device-finish/ipad-offline-playback.png`. This proves this saved-file path, not long-running background recovery or every original media format.

Both simulator build commands succeeded again after the audio adjustment and reconciliation with the new icon assets. A signed iOS Release build succeeded using the already-configured development team. The configured device deployment helper published and installed native revision `8cabb9ef` as build 2464 on the paired iPhone 16 Pro Max. A separate `devicectl device process launch` succeeded. Final deployment receipts, including subsequent native changes, are retained locally in `~/Library/Caches/KinosailAppleDeploy/iphone.json` and `tv.json`; build/launch evidence is copied to `.verification/apple-device-finish/`. Installation is not a claim that every physical-device flow passed.

## Verification boundaries

No test or quality suites were enabled or run, as explicitly requested. Added Swift Testing sources are not passing results.

After the user unlocked the Mac, the follow-up below completed the listed simulator flows. Remaining interactive checks include split-view layouts, increased contrast, PiP, session interruptions and long-running background download recovery. Real Server pairing and the Nox flows listed below are now verified. Physical media/receiver behavior, VoiceOver/Siri Remote, HDR/codec coverage, battery/performance and store distribution remain unverified. Test suites remain excluded until the user separately enables them.

Google Cast has no native sender SDK and is not offered. External captions do not render in system PiP. Offline book resources/artwork and playback audio effects are not claimed. See IMPLEMENTATION.md for retained feature limits and persistence contracts.

## Unlocked runtime follow-up

Both simulator builds succeeded after these final production fixes:

- Audio previous/next controls use accessible symbols so large Dynamic Type does not break their labels across lines. Empty Up next sections are omitted, and playback failures expose retry.
- Audio/video screens respect preparation already owned by the player coordinator, preventing duplicate requests when advancing the queue.
- TV navigation headings use readable dark-appearance colors through the supported legacy navigation-bar API. Detail artwork leaves room for title actions.
- TV video hides app navigation and fills the screen. Playback options are available as an item in the native AVKit transport menu.

Manual loopback-fixture observations: iPhone Home adapted to accessibility text sizes; audio playback, next-track transition and sleep-timer selection worked. iPad EPUB chapter navigation, sepia preferences, reading bookmarks, native PDF rendering/scrolling, comic paging and photo zoom worked. Populated TV pairing, catalog focus, title details, full-screen video, captions and native pause/resume worked. The AVKit options item was visible; activating that item was not established by the available simulator input controls. Screenshots are retained under `.verification/apple-runtime-finish/` in the main checkout.

The Mac was kept awake with temporary display/system idle assertions. iPhone Mirroring could not find the phone during this review; physical installation and launch are tracked separately by the deployment receipts. These observations do not replace real Server, VoiceOver, physical Siri Remote or receiver checks. Tests and quality suites remain disabled.

## Real Nox follow-up — 2026-09-12

Computer control completed ordinary passkey sign-in and Quick Connect approval against the real Nox Server. Its advertised HTTPS origin passed normal certificate validation. Both iPad and tvOS simulators restored the resulting sessions after installing updated builds and loaded the populated library.

The live checks exposed and fixed these production failures:

- Chaptered movie responses include chapter indexes. The native contract now accepts sequential bounded indexes and rejects malformed or conflicting values. The real movie played on iPad and TV; native pause/resume, forward seeking, captions and persisted progress were observed.
- Expired Quick Connect requests now report an expired/cancelled code with connection recovery, instead of a missing-title error. The old misleading error was reproduced; the new 404 mapping has regression source but was not separately exercised after expiry.
- Compatible download preparation incorrectly passed encoder-only profile options to FFmpeg video copying. The exact Nox command failed with exit 234; removing those options produced a decodable file. This necessary server dependency fix is in `packages/downloads/transcoding.go` and was deployed to Nox.
- Resuming a failed native preparation now repeats the idempotent preparation request before requeuing the same job. The previously failed Compatible sample completed after Resume, passed local verification and visibly played through Play offline. The Original sample also completed and played. Network isolation was not applied during this real-Nox check; the earlier fixture check separately established playback with its server stopped.
- TV Change Server now uses an opaque full-screen cover. The whole approval code, QR, instructions and cancellation action remained visible; cancellation returned to the existing Nox session.
- A real audio sample had a saved position of 4,771 seconds despite a 30-second file. Bounded stale progress now restarts at zero. The sample played, advanced and paused at four seconds, and the detail view reflected its new saved position. Invalid numeric encodings and excessive positions remain rejected.

Real Nox EPUB reading, photo rendering and photo zoom also worked on iPad. Final iOS and tvOS simulator builds succeeded at production revision `dfb16dcc`. Signed Release builds installed and were confirmed by the deployment helper on both the paired iPhone and Apple TV as build 2475. The phone subsequently became unreachable to CoreDevice, and iPhone Mirroring reported iPhone Not Found; this does not establish physical playback. The iPhone simulator's tab bar could not be controlled because computer-control coordinate actions reported a missing window; its real-Nox pairing remains a separate boundary.

Evidence is retained locally under `.verification/nox-native-connect/` in the main checkout, with final installation receipts in `~/Library/Caches/KinosailAppleDeploy/`. Tests and quality suites remain disabled. Regression source additions are not passing test results, and these concrete live flows do not certify all media formats, background behavior, accessibility or release distribution.
