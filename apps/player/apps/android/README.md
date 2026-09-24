# Kinosail Android client

This is the first buildable Android client slice. One application package contains separate phone/tablet and Android TV/Google TV launcher activities. The current screens show the signed-out visual shell; Server connection, library, and playback are not implemented yet. The [Android stack research](../../engineering/research/android-client-stack-2026-09-23.md) defines the next slices.

The app uses Kotlin, Jetpack Compose, Compose for TV, and Kinosail's native Electric palette and authored artwork. The release application ID is `com.kinosail.player`; debug builds use `com.kinosail.player.dev` so they can coexist with other installed clients. Release signing and Play distribution are not configured.

Build with JDK 17 and Android SDK Platform 37 installed:

```sh
./gradlew :app:assembleDebug
```

The debug APK is `app/build/outputs/apk/debug/app-debug.apk`. Verify both launchers on Android phone and TV emulators. A successful build or install is not evidence of Server connection, media playback, physical-device behavior, or Play acceptance.
