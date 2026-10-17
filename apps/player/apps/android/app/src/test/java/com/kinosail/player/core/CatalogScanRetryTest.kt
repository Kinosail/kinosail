package com.kinosail.player.core

import android.os.Looper
import java.time.Duration
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class CatalogScanRetryTest {
    @Test fun firstScanLoadsTheLibraryAfterTheServerDelay() = journey("2") { fixture, model ->
        assertEquals("A deferred read must not use quick retries", 1, fixture.catalogOffsets.size)
        assertEquals(SERVER_PREPARING, model.state.notice)
        assertTrue(model.state.items.isEmpty())
        fixture.catalogFailures.remove("")
        shadowOf(Looper.getMainLooper()).idleFor(Duration.ofMillis(1_500))
        assertEquals("The Server delay must pass before the next read", 1, fixture.catalogOffsets.size)
        shadowOf(Looper.getMainLooper()).idleFor(Duration.ofMillis(600))
        await { model.state.items.size == 24 }
        assertNull(model.state.notice)
        assertEquals(2, fixture.catalogOffsets.size)
    }

    @Test fun malformedRetryAfterKeepsTheUnavailableRecovery() = journey("soon") { fixture, model ->
        assertEquals(3, fixture.catalogOffsets.size)
        assertEquals("Could not load your library. Try again.", model.state.notice)
        fixture.catalogFailures.remove("")
        shadowOf(Looper.getMainLooper()).idleFor(Duration.ofSeconds(16))
        await { model.state.items.size == 24 }
    }

    @Test fun retryAfterIsBoundedToWholeSeconds() {
        assertEquals(5, retryAfterSeconds(503, "5"))
        assertEquals(1, retryAfterSeconds(503, "0"))
        assertEquals(60, retryAfterSeconds(503, "3600"))
        for (value in listOf(null, "", "-1", "1.5", " 5", "12345", "Sat, 17 Oct 2026 07:28:00 GMT")) {
            assertNull(value, retryAfterSeconds(503, value))
        }
        assertNull(retryAfterSeconds(429, "5"))
    }

    private fun journey(retryAfter: String, action: (AndroidRecoveryFixture, CatalogModel) -> Unit) {
        val application = RuntimeEnvironment.getApplication()
        AndroidRecoveryFixture(application).use { fixture ->
            fixture.catalogFailures[""] = 503
            fixture.catalogRetryAfter = retryAfter
            val model = CatalogModel(application)
            try {
                model.open(fixture.viewer)
                await { !model.state.loading && fixture.catalogOffsets.isNotEmpty() }
                action(fixture, model)
            } finally { model.reset() }
        }
    }

    private fun await(condition: () -> Boolean) {
        val deadline = System.nanoTime() + 8_000_000_000L
        while (!condition() && System.nanoTime() < deadline) {
            shadowOf(Looper.getMainLooper()).idle()
            Thread.sleep(10)
        }
        assertTrue("Production model did not reach the expected state", condition())
    }
}
