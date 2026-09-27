package com.kinosail.player.wear

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
        assertTrue(notice.contains("Wear Compose"))
        assertTrue(notice.contains("Apache License 2.0"))
    }
}
