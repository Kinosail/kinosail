package com.kinosail.player.wear

import android.content.pm.PackageManager
import android.graphics.Bitmap
import android.os.Handler
import android.os.Looper
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.accessibility.enableAccessibilityChecks
import androidx.compose.ui.test.junit4.v2.createEmptyComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.printToString
import androidx.compose.ui.test.tryPerformAccessibilityChecks
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import androidx.test.runner.lifecycle.ActivityLifecycleMonitorRegistry
import androidx.test.runner.lifecycle.Stage
import java.io.File
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicReference
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class WearLaunchJourneyTest {
    @get:Rule val compose = createEmptyComposeRule()

    // Rendered WatchApp fixtures miss the real Activity entrypoint, lifecycle, and platform Data Layer.
    @Test fun unpairedWatchLaunchesAndSurvivesRecreation() {
        val instrumentation = InstrumentationRegistry.getInstrumentation()
        val context = instrumentation.targetContext
        assertTrue("Wear launch requires a watch", context.packageManager.hasSystemFeature(PackageManager.FEATURE_WATCH))
        assertTrue("Hosted watch profile must be round", context.resources.configuration.isScreenRound)
        fun foregroundIsApp(): Boolean {
            instrumentation.uiAutomation.clearCache()
            return instrumentation.uiAutomation.rootInActiveWindow?.packageName?.toString() == context.packageName
        }
        var stage = "launch-unpaired"
        var phase = "launch"
        var activityBefore = "unavailable"
        fun activityObservation(): String {
            val observation = AtomicReference("unavailable")
            val done = CountDownLatch(1)
            Handler(Looper.getMainLooper()).post {
                try {
                    val monitor = ActivityLifecycleMonitorRegistry.getInstance()
                    val rows = Stage.values().flatMap { lifecycle ->
                        monitor.getActivitiesInStage(lifecycle).filterIsInstance<WearActivity>().take(2).map {
                            "${lifecycle.name}:focus=${it.hasWindowFocus()},finishing=${it.isFinishing}," +
                                "destroyed=${it.isDestroyed},changing=${it.isChangingConfigurations}"
                        }
                    }
                    observation.set(rows.joinToString("|").ifEmpty { "none" })
                } catch (_: Exception) { observation.set("failed") }
                finally { done.countDown() }
            }
            return if (done.await(500, TimeUnit.MILLISECONDS)) observation.get() else "timeout"
        }
        fun capture(stage: String) {
            val directory = File(context.filesDir, "parity-evidence").apply { mkdirs() }
            val failures = mutableListOf<Throwable>()
            runCatching {
                val semantics = File(directory, "wear-$stage-semantics.txt")
                val observations = "phase=$phase\nactivityBefore=$activityBefore\nactivityAtCapture=${activityObservation()}\n"
                semantics.writeText(observations)
                val foreground = foregroundIsApp()
                semantics.writeText("activeWindowIsApp=$foreground\n$observations")
                semantics.appendText(compose.onRoot().printToString())
                assertTrue("The captured semantics must belong to the active app", foreground)
            }.onFailure { failures.add(it) }
            runCatching {
                val image = requireNotNull(instrumentation.uiAutomation.takeScreenshot())
                try {
                    File(directory, "wear-$stage.png").outputStream().use {
                        assertTrue("Compositor image must encode", image.compress(Bitmap.CompressFormat.PNG, 100, it))
                    }
                } finally { image.recycle() }
            }.onFailure { failures.add(it) }
            if (failures.isNotEmpty()) {
                failures.drop(1).forEach(failures.first()::addSuppressed)
                throw failures.first()
            }
        }
        fun <T> retainingFailure(action: () -> T): T = try { action() } catch (failure: Throwable) {
            runCatching { capture(stage) }.onFailure(failure::addSuppressed)
            throw failure
        }
        retainingFailure { ActivityScenario.launch(WearActivity::class.java) }.use { scenario ->
            retainingFailure {
                activityBefore = activityObservation()
                phase = "accessibility-setup"
                compose.enableAccessibilityChecks()
                fun verifyAndCapture() {
                    // Compose visibility alone can pass behind a system welcome screen.
                    phase = "foreground"
                    compose.waitUntil(10_000) { foregroundIsApp() }
                    phase = "content"
                    compose.waitUntil(10_000) {
                        compose.onAllNodesWithText("Connect phone").fetchSemanticsNodes().isNotEmpty()
                    }
                    compose.onNodeWithText("Connect phone").assertIsDisplayed()
                        .assert(SemanticsMatcher.expectValue(SemanticsProperties.LiveRegion, LiveRegionMode.Polite))
                    compose.onNodeWithText("Pair Android phone.").assertIsDisplayed()
                    compose.onNodeWithContentDescription("Play").assertDoesNotExist()
                    compose.onNodeWithContentDescription("Pause").assertDoesNotExist()
                    phase = "accessibility"
                    compose.onRoot().tryPerformAccessibilityChecks()
                    compose.waitForIdle()
                    instrumentation.waitForIdleSync()
                    assertTrue("The app must own the active window before capture", foregroundIsApp())
                    phase = "capture"
                    capture(stage)
                }
                verifyAndCapture()
                stage = "recreated-unpaired"
                phase = "recreate"
                scenario.recreate()
                verifyAndCapture()
            }
        }
    }
}
