# Kinosail Android client

One application package contains separate phone/tablet and Android TV/Google TV launcher activities. A companion Wear OS package controls playback on the paired Android phone and Server-connected TV players. Viewers can connect through Server Quick Connect, browse and search by media category, use For you and My List, open shows and seasons, and play video or audio with saved watch position, track selection, WebVTT captions, and speed control. Phone video supports Picture-in-Picture; music and audiobooks continue through a media session. Phone and tablet can open original photos, PDFs, and comic archives. EPUB and offline downloads remain launch work. The [Android stack research](../../engineering/research/android-client-stack-2026-09-23.md) records the stack and verification boundaries.

The apps use Kotlin, Jetpack Compose, Compose for TV, Compose for Wear OS, and Kinosail's native Electric palette and authored artwork. Both release packages use the application ID `com.kinosail.player`; debug builds use `com.kinosail.player.dev` so the phone and watch can pair through the Wear Data Layer. The watch's optional heart graph uses passive Health Services readings and stores them only on the watch, mapped to movie time where playback position is known. Opting in starts a foreground tracking notification; missing readings or a lost phone connection leave gaps. Release signing and Play distribution are not configured.

Build with JDK 17 and Android SDK Platform 37 installed:

```sh
./gradlew :app:testDebugUnitTest :watchcore:testDebugUnitTest :wear:testDebugUnitTest
./gradlew :app:assembleDebug :wear:assembleDebug :app:bundleRelease :wear:bundleRelease :app:lintDebug :wear:lintDebug
```

The debug APK is `app/build/outputs/apk/debug/app-debug.apk`; the release bundle is `app/build/outputs/bundle/release/app-release.aab`. Verify both launchers on Android phone and TV emulators. Server checks accept HTTPS addresses and local HTTP addresses using a private IP or localhost; redirects and unexpected health responses are rejected. Populated API 36 phone, tablet, and TV emulator journeys use a temporary local fixture. They do not prove physical-device codec support, live Server compatibility, or Play acceptance.
