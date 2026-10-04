package com.kinosail.player.core

import android.os.Looper
import java.time.Duration
import java.util.concurrent.CountDownLatch
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config
import org.robolectric.shadows.ShadowLog

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class CatalogRecoveryTest {
    @Test fun manualRetryKeepsLoadedRowsFirstRun() = manualRetryJourney()
    @Test fun manualRetryKeepsLoadedRowsSecondRun() = manualRetryJourney()

    private fun manualRetryJourney() = journey(400) { fixture, model, loaded ->
        fixture.failingOffset = null
        model.retry()
        await { !model.state.loading }
        println("R09 manual retry: loaded=48 rows=${model.state.items.size} requestedOffset=${fixture.catalogOffsets.last()}")
        assertEquals("Retry must append the failed page", 72, model.state.items.size)
        assertEquals(loaded, model.state.items.take(48))
        assertEquals(48, fixture.catalogOffsets.last())
        assertNull(model.state.notice)
    }

    @Test fun automaticRetryKeepsLoadedRowsFirstRun() = automaticRetryJourney()
    @Test fun automaticRetryKeepsLoadedRowsSecondRun() = automaticRetryJourney()

    private fun automaticRetryJourney() = journey(503) { fixture, model, loaded ->
        fixture.failingOffset = null
        shadowOf(Looper.getMainLooper()).idleFor(Duration.ofSeconds(16))
        await { model.state.items.size != 48 }
        println("R09 delayed retry: loaded=48 rows=${model.state.items.size} requestedOffset=${fixture.catalogOffsets.last()}")
        assertEquals("Delayed retry must append the failed page", 72, model.state.items.size)
        assertEquals(loaded, model.state.items.take(48))
        assertEquals(48, fixture.catalogOffsets.last())
    }

    @Test fun supersededSearchDoesNotReplayTheFailedPage() = journey(503) { fixture, model, _ ->
        fixture.failingOffset = null
        model.searchInput = "new"
        model.search()
        await { !model.state.loading && fixture.catalogQueries.last() == "new" }
        val requests = fixture.catalogOffsets.size
        shadowOf(Looper.getMainLooper()).idleFor(Duration.ofSeconds(16))
        assertEquals(requests, fixture.catalogOffsets.size)
        assertEquals(24, model.state.items.size)
    }

    @Test fun duplicatePageDoesNotReplaceOrDuplicateLoadedRows() = journey(400) { fixture, model, loaded ->
        fixture.failingOffset = null
        fixture.duplicatePage = true
        ShadowLog.clear()
        model.retry()
        await { !model.state.loading }
        assertEquals(loaded, model.state.items)
        assertNotNull("The failed page needs visible recovery feedback", model.state.notice)
        val diagnostic = ShadowLog.getLogsForTag("KinosailCatalog").single()
        assertEquals(android.util.Log.WARN, diagnostic.type)
        assertTrue(diagnostic.msg.contains("offset=48"))
        assertTrue(diagnostic.msg.contains("failure=duplicate_items"))
        assertTrue(diagnostic.msg.contains("outcome=loaded-rows-retained"))
        assertFalse(diagnostic.msg.contains("recovery-fixture") || diagnostic.msg.contains("127.0.0.1"))
        val requests = fixture.catalogOffsets.size
        shadowOf(Looper.getMainLooper()).idleFor(Duration.ofSeconds(16))
        assertEquals("Invalid pages must not trigger automatic retries", requests, fixture.catalogOffsets.size)
    }

    @Test fun repeatedManualRetryDoesNotOpenConcurrentRequests() = journey(400) { fixture, model, _ ->
        val requests = fixture.catalogOffsets.size
        fixture.catalogGate = CountDownLatch(1)
        try {
            model.retry(); model.retry(); model.retry()
            await { fixture.catalogOffsets.size > requests }
            shadowOf(Looper.getMainLooper()).idle()
            Thread.sleep(100)
            assertEquals(requests + 1, fixture.catalogOffsets.size)
        } finally { fixture.catalogGate.countDown() }
    }

    @Test fun deniedPageKeepsRecoveryFeedbackWithoutAutomaticRetry() = journey(403) { fixture, model, loaded ->
        assertTrue(model.state.notice.orEmpty().contains("expired"))
        val requests = fixture.catalogOffsets.size
        shadowOf(Looper.getMainLooper()).idleFor(Duration.ofSeconds(16))
        assertEquals(requests, fixture.catalogOffsets.size)
        assertEquals(loaded, model.state.items)
    }

    private fun journey(status: Int, action: (AndroidRecoveryFixture, CatalogModel, List<CatalogItem>) -> Unit) {
        val application = RuntimeEnvironment.getApplication()
        AndroidRecoveryFixture(application).use { fixture ->
            val model = CatalogModel(application)
            try {
                model.open(fixture.viewer)
                await { !model.state.loading && model.state.items.size == 24 }
                model.loadMore()
                await { !model.state.loading && model.state.items.size == 48 }
                val loaded = model.state.items
                fixture.catalogFailure = status
                fixture.failingOffset = 48
                model.loadMore()
                await { !model.state.loading }
                assertEquals(loaded, model.state.items)
                action(fixture, model, loaded)
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
