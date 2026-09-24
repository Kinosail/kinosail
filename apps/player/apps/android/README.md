# Kinosail Android client

One application package contains separate phone/tablet and Android TV/Google TV launcher activities. Viewers can connect through Server Quick Connect, browse and search their library by media category, use For you and My List, open shows and seasons, and play video or audio with saved watch position and WebVTT captions. The [Android stack research](../../engineering/research/android-client-stack-2026-09-23.md) records the stack and remaining launch work.

The app uses Kotlin, Jetpack Compose, Compose for TV, and Kinosail's native Electric palette and authored artwork. The release application ID is `com.kinosail.player`; debug builds use `com.kinosail.player.dev` so they can coexist with other installed clients. Release signing and Play distribution are not configured.

Build with JDK 17 and Android SDK Platform 37 installed:

```sh
./gradlew :app:assembleDebug
./gradlew :app:testDebugUnitTest
```

The debug APK is `app/build/outputs/apk/debug/app-debug.apk`. Verify both launchers on Android phone and TV emulators. Server checks accept HTTPS addresses and local HTTP addresses using a private IP or localhost; redirects and unexpected health responses are rejected. Emulator builds and fixture playback do not prove physical-device codec support, live Server compatibility, or Play acceptance.
