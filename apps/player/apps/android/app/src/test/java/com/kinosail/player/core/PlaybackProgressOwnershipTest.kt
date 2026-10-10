package com.kinosail.player.core

import android.app.Application
import android.os.Looper
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.ViewModelStore
import androidx.lifecycle.viewModelScope
import androidx.test.core.app.ApplicationProvider
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import kotlinx.coroutines.Job
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config
import org.robolectric.annotation.LooperMode

/**
 * Public playback/journal callers and a gated HTTP server reproduce late progress replies.
 * Existing browser and native journeys do not hold an Android progress reply across stop/restart.
 * Cover late conflict, late failure, and a restart of the same title; keep active conflict feedback.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
@LooperMode(LooperMode.Mode.PAUSED)
class PlaybackProgressOwnershipTest {
    private val item = CatalogItem("film-1", "video", "Fictional 1", "", "", "")
    private val local = WatchProgress(20.0, false, "device-position", 1)
    private val remote = WatchProgress(55.0, false, "other-position", 2)
    private val changedAgain = WatchProgress(70.0, false, "another-position", 3)

    @Test fun activeResolutionStillReportsTheNewConflict() = journey(409) { model, _, journal, reply, resolution ->
        reply.release.countDown()
        await { resolution.isCompleted && journal.pending().singleOrNull()?.conflict == changedAgain &&
            model.progressNotice == "Watch position changed again on another device." }
        assertFalse("Resolution must complete without cancellation", resolution.isCancelled)
        assertTrue(model.progressConflict)
    }

    @Test fun stoppedResolutionCannotRestoreConflict() = journey(409) { model, _, journal, reply, resolution ->
        model.stop()
        reply.release.countDown()
        assertProgressStaysClear(model, resolution)
        assertEquals("Stopped playback must still synchronize its journal", changedAgain, journal.pending().single().conflict)
        assertNull(model.player)
    }

    @Test fun stoppingWithAQueuedProgressReplyCannotRestoreConflict() = journey(409) { model, _, journal, reply, resolution ->
        reply.release.countDown()
        // This I/O job only creates a child when dispatching its UI commit to Main.
        // Leave Main paused until that child exists, then stop before it can run.
        awaitWithoutMain { journal.pending().singleOrNull()?.conflict == changedAgain && resolution.children.any() }
        model.stop()
        assertProgressStaysClear(model, resolution)
        assertEquals(changedAgain, journal.pending().single().conflict)
    }

    @Test fun stoppedResolutionCannotRestoreFailureNotice() = journey(503) { model, _, journal, reply, resolution ->
        model.stop()
        reply.release.countDown()
        assertTrue("The delayed HTTP failure must be delivered", reply.written.await(3, TimeUnit.SECONDS))
        assertProgressStaysClear(model, resolution)
        assertEquals("Failed sync must retain the saved position", local, journal.pending().single().progress)
        assertNull(journal.pending().single().conflict)
    }

    @Test fun sameTitleRestartCannotRestoreOldConflictWhileLoading() = journey(409) { model, fixture, journal, reply, resolution ->
        fixture.playbackGate = CountDownLatch(1)
        model.start(item, fixture.viewer)
        assertTrue(model.loading)
        reply.release.countDown()
        await { journal.pending().singleOrNull()?.conflict == changedAgain && fixture.playbackRequests.get() == 2 }
        assertProgressStaysClear(model, resolution)
        assertTrue("The restarted title is still loading", model.loading)
        fixture.playbackGate.countDown()
        await { model.player != null }
        assertTrue("The new session must load its current durable conflict", model.progressConflict)
        assertEquals("Watch position changed on another device.", model.progressNotice)
    }

    private fun journey(status: Int, check: (PlaybackModel, AndroidRecoveryFixture,
        ProgressJournal, AndroidRecoveryFixture.ProgressResponse, Job) -> Unit) {
        val application = ApplicationProvider.getApplicationContext<Application>()
        val models = ViewModelStore()
        AndroidRecoveryFixture(application).use { fixture ->
            val saved = requireNotNull(SessionStore(application).load())
            val journal = ProgressJournal.forViewer(application, saved.server, fixture.viewer)
            journal.record(item.id, local, WatchProgress())
            journal.apply(item.id, local, ProgressResult(remote, true))
            fixture.playbackFailure = false
            val model = ViewModelProvider(models,
                ViewModelProvider.AndroidViewModelFactory(application))[PlaybackModel::class.java]
            val reply = AndroidRecoveryFixture.ProgressResponse(status, changedAgain)
            try {
                model.start(item, fixture.viewer)
                await { model.player != null }
                assertTrue("Start must expose the saved conflict", model.progressConflict)
                fixture.progressResponse = reply
                val scope = requireNotNull(model.viewModelScope.coroutineContext[Job])
                val existingJobs = scope.children.toSet()
                model.resolveProgress(true)
                await { reply.entered.count == 0L }
                val resolution = scope.children.single { it !in existingJobs }
                assertNull("Resolution must clear the old durable conflict before sync", journal.pending().single().conflict)
                check(model, fixture, journal, reply, resolution)
            } finally {
                reply.release.countDown()
                fixture.playbackGate.countDown()
                models.clear()
                shadowOf(Looper.getMainLooper()).idle()
            }
        }
    }

    private fun await(condition: () -> Boolean) {
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(8)
        while (System.nanoTime() < deadline) {
            shadowOf(Looper.getMainLooper()).idle()
            if (condition()) return
            Thread.sleep(10)
        }
        fail("Timed out waiting for the public playback/HTTP state")
    }

    private fun assertProgressStaysClear(model: PlaybackModel, resolution: Job) {
        // Wait for the public lifecycle job, so cleanup cannot cancel a harmful reply after the assertion.
        await {
            assertFalse("A previous playback session must not restore a conflict", model.progressConflict)
            assertNull("A previous playback session must not restore a notice", model.progressNotice)
            resolution.isCompleted
        }
        assertFalse("Resolution must complete without cancellation", resolution.isCancelled)
    }

    private fun awaitWithoutMain(condition: () -> Boolean) {
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(8)
        while (System.nanoTime() < deadline) {
            if (condition()) return
            Thread.sleep(10)
        }
        fail("Timed out waiting for the queued progress reply")
    }
}
