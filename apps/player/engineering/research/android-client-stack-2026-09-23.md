# Android client stack and launch plan

**Research snapshot:** September 23, 2026. **Status:** proposed; no Android client is implemented by this note. Recheck library versions and Play requirements when the project is created.

## Decision for the first build

Build **one native Android app** for phones, tablets, Android TV, and Google TV. Use one package name and Android App Bundle, a shared Server client and application state, and separate touch and television presentation. Google's [TV app guide](https://developer.android.com/training/tv/get-started/create) recommends a single app for mobile and TV, requires a TV launcher activity, and explicitly advises a TV-specific interface. The [Play distribution guide](https://developer.android.com/training/tv/publishing/distribute) describes one bundle/listing with an Android TV opt-in and review. Google TV is included in Android TV distribution.

This is one codebase and product identity, **not one layout stretched across screens**. Phone and tablet navigation should follow touch, screen size, and accessibility needs. TV navigation must work entirely with a D-pad and Back from the couch, with clear focus, large targets, and playback controls designed for a remote. The manifest needs both mobile and `LEANBACK_LAUNCHER` entry points, `android.software.leanback` with `required="false"`, and touchscreen marked optional; validate the exact manifest during the first device build. [Source: TV app guide](https://developer.android.com/training/tv/get-started/create).

### Recommended stack

| Concern | Choice | Reason and evidence |
| --- | --- | --- |
| Language and build | Kotlin, Android Gradle Plugin (AGP) 9.4, Gradle 9.6, JDK 17; use AGP's built-in Kotlin support | Native APIs, modern tooling, and fewer plugin interactions. AGP 9.4 lists Gradle 9.6/JDK 17 and API 37 support. Kotlin 2.4.20 is the current language release, but confirm the compiler bundled by AGP and a clean build before pinning any standalone Kotlin plugin. [AGP 9.4](https://developer.android.com/build/releases/agp-9-4-0-release-notes), [built-in Kotlin](https://developer.android.com/build/migrate-to-built-in-kotlin), [Kotlin 2.4.20](https://kotlinlang.org/docs/whatsnew2420.html). |
| Touch UI | Jetpack Compose, stable Material 3 (1.4.0), Material 3 Adaptive (1.3.0) where a tablet layout benefits | Native touch UI and adaptive layouts without a second UI framework. These are the stable versions in the [AndroidX release index](https://developer.android.com/jetpack/androidx/versions). |
| TV UI | Compose for TV (`androidx.tv`, stable 1.1.0), TV-specific theme and screens | Google's [Compose for TV guide](https://developer.android.com/training/tv/playback/compose) recommends it for TV. Share state and domain operations with touch screens, while owning focus and remote behavior in TV UI. Version: [AndroidX release index](https://developer.android.com/jetpack/androidx/versions). |
| Navigation and state | Navigation 3 (stable 1.2.0), ViewModels, repositories, coroutines and Flow | Use the stable navigation API and Android's [architecture recommendations](https://developer.android.com/topic/architecture/recommendations); keep navigation graphs separate for touch and TV. Version: [AndroidX release index](https://developer.android.com/jetpack/androidx/versions). |
| Video and audio | Media3 / ExoPlayer (stable 1.11.1), `MediaSessionService` for background audio and system controls | The native Android playback stack supports streaming, tracks, sessions, and device media controls. [Media3 playback](https://developer.android.com/media/media3/exoplayer/hello-world), [background playback](https://developer.android.com/media/media3/session/background-playback), [version](https://developer.android.com/jetpack/androidx/versions). |
| Player controls | Begin with Media3 Compose controls; use `PlayerView` through `AndroidView` where needed | Google's [Media3 UI guide](https://developer.android.com/media/media3/ui/overview) says Compose UI has not reached parity with the View UI. Choose the control surface by tested subtitle, track, accessibility, and D-pad behavior, rather than appearance alone. |
| Local catalog | Room 3 (stable 3.0.3), with bounded per-Server/per-Viewer records | A durable offline catalog and progress journal need explicit ownership and migrations. [Room overview](https://developer.android.com/training/data-storage/room), [version](https://developer.android.com/jetpack/androidx/versions). |
| Artwork | Coil 3, with an authenticated fetcher and bounded, profile-scoped cache | Compose integration is straightforward, but Kinosail must supply its own authenticated, origin-bound loader and cache rules. [Coil documentation](https://coil-kt.github.io/coil/compose/). |
| Background transfers | User-initiated data transfer jobs for long user-started downloads on Android 14+; WorkManager for short or deferrable jobs | Android distinguishes these cases; choose a compatible path for older supported devices. Kinosail's resumable chunks, checksum, and authorization checks remain application logic. [Transfer options](https://developer.android.com/develop/background-work/background-tasks/data-transfer-options), [UIDT jobs](https://developer.android.com/develop/background-work/background-tasks/uidt). |

These are **stable defaults as of the snapshot**, not a mandate to add every dependency on day one. Use stable libraries for the initial app. Material 3 Expressive features that require experimental APIs should be evaluated separately, not made foundational to sign-in or playback; see [Material 3 release notes](https://developer.android.com/jetpack/androidx/releases/compose-material3). Avoid a React Native/Expo runtime and a Kotlin Multiplatform rewrite of the existing Swift client: the current Apple app is already native, while shared Server contracts provide the cross-platform seam.

## Fit with Kinosail

The [active Apple client](../../apps/native/IMPLEMENTATION.md) is the behavioral reference, not code to port mechanically. The [native README](../../apps/native/README.md) confirms that Android targets were removed with the former Expo app. The historical [shared Expo ADR](../adr/0008-share-one-native-client-source-across-mobile-and-tv.md) is superseded. Start Android from the Server's versioned API contract at `apps/player/internal/server/api_openapi.json` (path relative to the repository root), and preserve the existing Server operation semantics.

Keep one Android app module initially. Organize its source by `core` (bounded Server transport, session, model, persistence), `features` (shared screen state), `mobile` (touch UI), `tv` (remote UI), and `playback`. Extract Gradle modules only when build or ownership pressure justifies them. One shared playback coordinator should hold Media3 state; phone and TV controls present that state differently. This is a proposed code shape, not a claim that these directories exist.

Apply Kinosail's current security and playback contracts on Android: strictly bound and validate Server data before side effects; confine requests, redirects, artwork, HLS references, and saved files to the authorized Server and Viewer; keep credentials in protected storage such as [Android Keystore](https://developer.android.com/privacy-and-security/keystore); never put account tokens in media or artwork URLs. Attempt original playback first, then use a Server-approved compatible stream when the device rejects the format. Report capability from physical devices, not codec assumptions. The Apple client's [implementation record](../../apps/native/IMPLEMENTATION.md) describes these current behaviors.

## First build slice

1. Create the Android project, one mobile launcher and one TV launcher, shared API transport, and empty but usable touch/TV shells. Run both on an emulator immediately.
2. Connect to a real Server through manual address and Quick Connect; restore and revoke a Viewer-scoped session. Test malformed Server responses and rejected URLs without network or file side effects.
3. Browse a populated library, search, open details, and render authenticated artwork on phone, tablet, and TV. Verify D-pad focus order, Back behavior, TalkBack, and large text.
4. Play an original item with Media3, exercise the Server-approved HLS fallback, seek, choose tracks/captions, and sync progress. Test real files on a physical phone and a physical Android TV or Google TV device.
5. Only after that vertical path is reliable, add background music, durable cache/progress, mobile offline downloads, mobile readers, and deeper device integrations in separately verified slices.

The first release gate should require both launchers, a populated Server journey, remote-only TV navigation, profile isolation, direct and compatible playback, error recovery, and device-specific codec checks. Use [Macrobenchmark and Baseline Profiles](https://developer.android.com/topic/performance/baselineprofiles/overview) once real navigation and playback exist. Source tests, emulator journeys, physical playback, and Play approval are separate evidence.

## Distribution and platform boundaries

Play needs Android TV opt-in, TV screenshots/banner, and a TV quality review for discoverability on Android TV and Google TV. The [TV distribution guide](https://developer.android.com/training/tv/publishing/distribute) also states that, since August 1, 2026, TV apps must support 64-bit architectures and 16 KB page sizes. Validate those requirements and current target API rules at release time.

This app can be assessed for Fire TV as a **follow-on port** after validating Amazon's store, remote behavior, media capabilities, and services; it is not covered by Google Play distribution. Amazon's [Fire TV guide](https://developer.amazon.com/docs/fire-tv/get-started-with-fire-tv.html) describes the Android-based platform. Amazon [Vega](https://developer.amazon.com/docs/vega/0.21/app-submission.html) has its own app submission path, so do not assume the Android package covers it. Roku, Samsung Tizen, LG webOS, and Apple platforms remain distinct clients or delivery channels.

## Open choices to settle during bootstrap

- Pick the minimum Android API level from actual phone/TV device coverage and the behavior of Media3, Room 3, and transfer APIs; the latest SDK alone should not set it.
- Verify AGP 9.4, built-in Kotlin, Compose compiler, KSP/Room 3, and Navigation 3 in a clean Gradle sync/build before locking versions. The release index changes frequently.
- Prototype one representative direct file, one compatible HLS stream, embedded/external captions, and a remote-controlled player before fixing the player control implementation.
- Decide whether phone and TV can ship together. One package and codebase still allow staged releases and a dedicated TV track under the [Play distribution model](https://developer.android.com/training/tv/publishing/distribute).
