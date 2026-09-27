package com.kinosail.player.core

import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class AttributionTest {
    @Test fun packageIncludesDependencyNotices() {
        val notice = RuntimeEnvironment.getApplication().assets.open("THIRD_PARTY_NOTICES.md")
            .bufferedReader().use { it.readText() }
        assertTrue(notice.contains("Jetpack Compose"))
        assertTrue(notice.contains("Media3"))
        assertTrue(notice.contains("Apache-2.0.txt"))
        val license = RuntimeEnvironment.getApplication().assets.open("Apache-2.0.txt")
            .bufferedReader().use { it.readText() }
        assertTrue(license.contains("Apache License"))
    }
}
