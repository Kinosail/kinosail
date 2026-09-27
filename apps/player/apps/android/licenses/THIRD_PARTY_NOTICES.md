# Third-party notices for Kinosail Android and Wear OS

Thank you to the maintainers of AndroidX, Jetpack Compose, Wear Compose, Media3,
Kotlin coroutines and serialization, Jackson, and the other libraries that help
these clients connect to and play from a household Server.

The Android and Wear OS packages use components published by Google under the
Android SDK and Google APIs terms, and libraries published under Apache License
2.0. The Apache license text is included in `Apache-2.0.txt` in each package.
The exact versions and direct dependency coordinates are recorded in
`app/build.gradle.kts` and `wear/build.gradle.kts` at the release revision.
These projects and their own notices remain authoritative:

- AndroidX and Jetpack Compose: https://developer.android.com/jetpack/androidx
- Wear Compose and Health Services: https://developer.android.com/health-and-fitness/guides/health-services
- Media3: https://github.com/androidx/media
- Kotlin coroutines and serialization: https://github.com/Kotlin/kotlinx.coroutines and https://github.com/Kotlin/kotlinx.serialization
- Jackson Core: https://github.com/FasterXML/jackson-core
- Google Play services: https://developers.google.com/android/guides/overview

The TMDB logo was created by Travis Bell and converted to PNG for these apps.
The source and credit are at https://commons.wikimedia.org/wiki/File:Tmdb.new.logo.svg.
It is available under https://creativecommons.org/licenses/by-sa/4.0/ and is
used subject to https://developer.themoviedb.org/docs/faq.

The Server's separate FFmpeg, browser, font, model, and Go dependency notices
are in `apps/player/THIRD_PARTY_NOTICES.md` in the Kinosail source repository
and `/licenses` in the Server image. FFmpeg runs on the Server, not in either
Android package.
