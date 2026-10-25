# Apple App Store readiness — 2026-10-24

Scope: `Kinosail-iOS` (iPhone and iPad, with the embedded `KinosailWatch` app) and `Kinosail-tvOS` (with `KinosailTopShelf`), version 0.2.0 build 1. This audit replaces the [2026-09-24 audit](apple-appstore-readiness-2026-09-24.md). Apple makes the final review decision; no local test can guarantee approval.

## Decision

**Ready except for one account step.** The source, archives, privacy files, and a reviewer sign-in path are ready. The Apple Developer account is not: Xcode is signed in only with a free Personal Team, which cannot create App Store profiles or upload builds.

## Fixed in this change

| Finding | Effect before the fix | Fix |
| --- | --- | --- |
| The Watch app was installed twice in the iOS archive. | Xcode produced a Generic Xcode Archive, which cannot be distributed to the App Store. | `SKIP_INSTALL = YES` for `KinosailWatch`. The archive is now an iOS App archive with `ApplicationPath = Applications/KinosailPlayer.app`. |
| The Watch app reads `UserDefaults` but had no privacy manifest. | App Store Connect reports ITMS-91053 (missing required-reason API declaration). | `Watch/PrivacyInfo.xcprivacy` declares `UserDefaults` with reason `CA92.1` and no collected data. |
| No export-compliance key. | Every upload asks the encryption question, and a build cannot be tested or submitted until someone answers it. | `ITSAppUsesNonExemptEncryption = false` in the iOS, tvOS, and watchOS Info.plists. The apps use only HTTPS from the OS and CryptoKit hashing. |
| The privacy policy described only Kinosail Servers on iPhone, iPad, and Apple TV. | Apple requires the policy to cover all data the app handles, including HealthKit data. | `/privacy/` now covers Jellyfin, Emby, and Plex servers, the Plex sign-in, and the Apple Watch heart rate graph. |
| No support page (`https://kinosail.com/support/` returned 404). | App Store Connect requires a Support URL with a way to contact the developer. | New `/support/` page with help steps and a contact link. |

## Checks that pass

- iOS and tvOS Release archives build and sign with the development team. The iOS archive contains the embedded Watch app and both privacy manifests. The tvOS archive contains the Top Shelf extension.
- App icons are 1024 × 1024 without transparency (iOS and watchOS). The tvOS layered icon and Top Shelf images are in the asset catalog.
- `NSLocalNetworkUsageDescription`, `NSHealthShareUsageDescription`, and `UIBackgroundModes = audio` are present and match real features. App Transport Security allows only local networking.
- iPad supports all four orientations, so multitasking requirements are met.
- The apps have no in-app account creation, so the account-deletion rule (5.1.1(v)) does not apply.
- Plex sign-in accesses the viewer's own Plex content. This is the client exception to Sign in with Apple (4.8).
- No advertising, analytics, or tracking SDKs. TMDB attribution is shown in Settings and on the Watch.
- Reviewer path, iOS simulator, 2026-10-24: Connect to `https://demo.jellyfin.org/stable`, choose `demo`, select Sign in with an empty password, select Start watching. Home showed public-domain movies, and The Boy in the Plastic Bubble played.

## Required before upload (account owner)

1. Enroll in the Apple Developer Program, or join a team as Admin or App Manager. A Personal Team cannot distribute apps.
2. In Xcode, select that team for `Kinosail-iOS`, `KinosailWatch`, `Kinosail-tvOS`, and `KinosailTopShelf`. Register the App Group `group.com.kinosail.player` and the HealthKit capability for `com.kinosail.player.watchkitapp`.
3. In App Store Connect, create one app record with bundle ID `com.kinosail.player` for iOS and tvOS (universal purchase).
4. Archive each scheme in Xcode (Product > Archive), then select Distribute App > App Store Connect > Upload.
5. Install the TestFlight build on a physical iPhone, Apple Watch, and Apple TV. Repeat the reviewer path below on each before you submit.

## App Store Connect answers

- **Category:** Entertainment.
- **Age rating:** Answer "None" to every content question. The app shows only media from servers that the user connects to. Answer Yes to "Unrestricted web access" only if the app can open arbitrary web pages. It cannot, so answer No.
- **Privacy policy URL:** `https://kinosail.com/privacy/`. For tvOS, also paste the policy text.
- **Support URL:** `https://kinosail.com/support/` (live after this change deploys).
- **App Privacy:** "Data Not Collected." The developer does not collect data. Data goes only to servers the user chooses, and heart rate data stays on the Watch.
- **Content rights:** The app does not contain third-party content. Media comes from the user's own server.
- **Encryption:** Answered by `ITSAppUsesNonExemptEncryption = false`.
- **Price:** Free. No in-app purchases.
- **Screenshots:** Use real app screens with licensed or public-domain media, such as the Jellyfin demo. Required sets: 6.9-inch iPhone, 13-inch iPad, Apple TV, and Apple Watch.

## App Review notes (paste into App Review Information)

Turn on "Sign-in required". Enter user name `demo`. The demo account has no password; if the form needs one, enter `none` and keep the note below, which says to leave the password empty. Paste this text into Notes.

> Kinosail Player is a free client for media servers that people run at home (Kinosail, Jellyfin, Emby, and Plex). It has no content of its own, no accounts, and no purchases.
>
> To review with a public demo server:
> 1. On the first screen, under "Or enter an address", enter https://demo.jellyfin.org/stable and tap the arrow.
> 2. On "Who's watching?", tap "demo". Leave the password empty and tap "Sign in".
> 3. Tap "Start watching". Home shows public-domain movies from the Jellyfin project's demo server.
> 4. Tap a movie, then Play or Resume.
>
> Apple TV: use the same address and steps with the Siri Remote. Apple Watch: open the Watch app while a video plays on the iPhone to use the remote controls. The heart rate graph is optional and asks for read-only Health access only when the user starts it.
>
> The app uses local network access to find servers on the home network. It plays audio in the background for music and audiobooks.

Before you submit, check that `https://demo.jellyfin.org/stable` is up. If it is down, give App Review a Jellyfin server you run with a demo user and public-domain media instead.

## Remaining risks

- The Jellyfin demo server is run by the Jellyfin project. If it is down during review, the review fails with Guideline 2.1. A reviewer server you control removes this risk.
- Supporter badges appear only when a Kinosail Server has badge records. Reviewers using the Jellyfin demo will not see them. If Apple asks, explain that badges are display-only and unlock nothing (Guideline 3.1.1).
- Not verified here: a signed distribution upload, App Store Connect processing, TestFlight on physical devices, and Apple TV and Apple Watch review paths.
