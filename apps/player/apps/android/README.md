# Kinosail Android client

One application package contains separate phone/tablet and Android TV/Google TV launcher activities. The signed-out screens can check a manually entered Server through its public `/healthz` endpoint. This does not authenticate a Viewer; Quick Connect, library, and playback remain future slices. The [Android stack research](../../engineering/research/android-client-stack-2026-09-23.md) defines the next steps.

The app uses Kotlin, Jetpack Compose, Compose for TV, and Kinosail's native Electric palette and authored artwork. The release application ID is `com.kinosail.player`; debug builds use `com.kinosail.player.dev` so they can coexist with other installed clients. Release signing and Play distribution are not configured.

Build with JDK 17 and Android SDK Platform 37 installed:

```sh
./gradlew :app:assembleDebug
./gradlew :app:testDebugUnitTest
```

The debug APK is `app/build/outputs/apk/debug/app-debug.apk`. Verify both launchers on Android phone and TV emulators. Server checks accept HTTPS addresses and local HTTP addresses using a private IP or localhost; redirects and unexpected health responses are rejected. A successful check proves reachability only, not account access, media playback, physical-device behavior, or Play acceptance.
