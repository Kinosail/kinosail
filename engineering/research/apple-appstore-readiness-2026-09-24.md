# Apple App Store readiness — 2026-09-24

Scope: the `Kinosail-iOS` and `Kinosail-tvOS` targets at `69a7a895` plus the privacy changes in this task. The iOS target includes iPhone and iPad; the TV app embeds `KinosailTopShelf`. This is a submission audit, not an Apple approval prediction.

## Decision

**Not ready to submit.** Source tests and unsigned Release archives pass. A reviewer-access path, distribution signing, final metadata, and physical interaction evidence are still required. Apple makes the final review decision; no local test can guarantee acceptance.

## Findings and actions

| Priority | Finding | Evidence and action |
| --- | --- | --- |
| P0 | Reviewers need a reachable, populated Server and a working sign-in path. | Both apps begin with a Server address and five-minute Quick Connect request. Public viewing requires approval from an already signed-in trusted device; public password login is disabled. Provide a dedicated HTTPS review Server with licensed sample media and a repeatable approval plan, or obtain Apple's prior approval for a full-feature demo mode. Test the exact instructions from outside the household. [Apple 2.1](https://developer.apple.com/app-store/review/guidelines/#performance) requires demo access and a running backend. |
| P0 | App Store distribution credentials are not configured on this machine. | Both Release archives are arm64 and contain their expected bundles, but they were built with `CODE_SIGNING_ALLOWED=NO`. The local keychain has an Apple Development identity, no Apple Distribution identity; no local provisioning profiles were found. Configure the intended Apple Developer team and App Group `group.com.kinosail.player` for the TV app and Top Shelf extension. Produce signed archives, export for App Store Connect, and validate the uploaded builds. |
| P1 | Native Supporter badges invite an external purchase path. | `SupporterScreen` tells users to activate a key in the Server web app; iOS also displays “Support Kinosail.” Purchased digital badges appear in the app without an in-app purchase path. Remove this native UI for the store build or implement a compliant StoreKit path after business review. [Apple 3.1.1 and 3.1.3](https://developer.apple.com/app-store/review/guidelines/#business) govern digital unlocks and external purchase calls to action. |
| P1 | Privacy policy must be public and configured per platform. | This task adds `/privacy/` and in-app access from setup and Settings. Verify the public HTTPS page after Pages deployment, review its factual/legal wording, enter its URL for iOS and policy text for tvOS in App Store Connect, and answer data-use questions for both platforms. [Apple 5.1.1](https://developer.apple.com/app-store/review/guidelines/#privacy) requires in-app and metadata access; [App Store Connect](https://developer.apple.com/help/app-store-connect/manage-app-information/manage-app-privacy) specifies the platform fields. |
| P1 | Final device and reviewer journeys are unproven. | The Mac was locked for direct UI control. iPhone and Apple TV simulators launched to the current Connect screen and displayed the new privacy entry. A test scheme and setup screenshot do not cover TV remote traversal, populated browsing, cold/warm playback, downloads, PiP, accessibility, physical media, or a submitted build. Exercise those flows on signed builds and physical devices. |
| P1 | Store listing assets and declarations remain unverified. | Check App Store Connect records, support URL, categories, age ratings, content rights, export-compliance answers, privacy responses, version/build numbers, and screenshots showing populated app use. [Apple 2.3](https://developer.apple.com/app-store/review/guidelines/#performance) requires accurate metadata and actual app screenshots. iOS and tvOS versions submit separately. |

## Source and package evidence

- Xcode 27.0, build 27A266a. iOS simulator test scheme: **181 tests in 32 suites passed** after the privacy change. tvOS simulator test scheme: **175 tests in 31 suites passed** after the privacy change. Test output still contains Swift macro actor-isolation warnings in existing test code; no test failed.
- Both unsigned Release `archive` commands completed. iOS archive contains `com.kinosail.player`, version `0.2.0` build `1`, arm64. tvOS archive contains the same app identity and embedded `KinosailTopShelf.appex`; both app and extension contain privacy manifests. Neither archive has a signing team or identity.
- iOS app icon is a 1024×1024 RGB PNG. tvOS layered icon and Top Shelf assets are present in the asset catalog. Asset presence is not an App Store Connect upload validation.
- Documentation build and link/search checker passed: 52 HTML pages and 50 searchable Player pages; generated canonical policy URL is `https://kinosail.com/privacy/`. This is a local artifact, not proof of live publication.
- `make max-loc` and `BASE=origin/main KINOSAIL_VERIFY_WORKTREE=1 make -C apps/player verify-changed` passed. Documentation Python tests and all five Node search tests passed after installing the declared Node dependencies.

## Reviewer notes to complete in App Store Connect

Use separate iOS and tvOS notes. Enter the real review Server address and reviewer credentials only in App Store Connect's private App Review information fields, never in repository files or screenshots. Explain that Kinosail Player is a free native client for a household-operated Server and that its media comes from that Server. Include exact steps for Connect, Quick Connect approval, browsing a populated title, starting playback, and the TV remote. Supply a reliable contact who can resolve access during the review window. Use licensed fictional or owned sample media and fictional account details in screenshots.

Do not submit until those instructions work on a clean external device without developer intervention, and the signed uploaded build has been exercised through the same path.
