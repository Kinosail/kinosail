package com.kinosail.player.core

import android.app.Application
import android.os.Looper
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.ViewModelStore
import androidx.test.core.app.ApplicationProvider
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config
import org.robolectric.annotation.LooperMode

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
@LooperMode(LooperMode.Mode.PAUSED)
class PlaybackStartRequestsTest {
    private val item = CatalogItem("film-1", "video", "Fictional 1", "", "", "")

    @Test fun preferencesLoadWhileThePlanIsPending() = journey { model, fixture, _ ->
        fixture.playbackGate = CountDownLatch(1)
        model.start(item, fixture.viewer)
        await { fixture.playbackRequests.get() == 1 }
        await { fixture.requestPaths.any { it.endsWith("/playback-preferences") } }
        assertNull("Playback must wait for the plan", model.player)
        fixture.playbackGate.countDown()
        await { model.player != null }
    }

    @Test fun pendingProgressSyncsBeforeThePlanIsRequested() = journey { model, fixture, journal ->
        journal.record(item.id, WatchProgress(20.0, false, "device-position", 1), WatchProgress())
        val reply = AndroidRecoveryFixture.ProgressResponse(200)
        fixture.progressResponse = reply
        model.start(item, fixture.viewer)
        await { reply.entered.count == 0L }
        assertEquals("The plan must wait for the saved position to sync", 0, fixture.playbackRequests.get())
        assertFalse(fixture.requestPaths.any { it.endsWith("/playback-preferences") })
        reply.release.countDown()
        await { model.player != null }
        assertTrue(journal.pending().none { it.itemId == item.id })
    }

    private fun journey(check: (PlaybackModel, AndroidRecoveryFixture, ProgressJournal) -> Unit) {
        val application = ApplicationProvider.getApplicationContext<Application>()
        val models = ViewModelStore()
        AndroidRecoveryFixture(application).use { fixture ->
            val saved = requireNotNull(SessionStore(application).load())
            val journal = ProgressJournal.forViewer(application, saved.server, fixture.viewer)
            fixture.playbackFailure = false
            val model = ViewModelProvider(models,
                ViewModelProvider.AndroidViewModelFactory(application))[PlaybackModel::class.java]
            try { check(model, fixture, journal) }
            finally {
                fixture.progressResponse?.release?.countDown()
                fixture.playbackGate.countDown()
                models.clear()
                shadowOf(Looper.getMainLooper()).idle()
            }
        }
    }

    private fun await(condition: () -> Boolean) {
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(8)
        while (!condition()) {
            check(System.nanoTime() < deadline) { "Timed out waiting for playback start" }
            shadowOf(Looper.getMainLooper()).idle()
            Thread.sleep(10)
        }
    }
}
