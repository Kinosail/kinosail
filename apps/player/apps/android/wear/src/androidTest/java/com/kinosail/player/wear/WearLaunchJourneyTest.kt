package com.kinosail.player.wear

import android.content.pm.PackageManager
import android.graphics.Bitmap
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
import java.io.File
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
        fun capture(stage: String) {
            val directory = File(context.filesDir, "parity-evidence").apply { mkdirs() }
            val failures = mutableListOf<Throwable>()
            runCatching {
                File(directory, "wear-$stage-semantics.txt").writeText(compose.onRoot().printToString())
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
        var stage = "launch-unpaired"
        fun <T> retainingFailure(action: () -> T): T = try { action() } catch (failure: Throwable) {
            runCatching { capture(stage) }.onFailure(failure::addSuppressed)
            throw failure
        }
        retainingFailure { ActivityScenario.launch(WearActivity::class.java) }.use { scenario ->
            retainingFailure {
                compose.enableAccessibilityChecks()
                fun verifyAndCapture() {
                    compose.waitUntil(10_000) {
                        compose.onAllNodesWithText("Connect phone").fetchSemanticsNodes().isNotEmpty()
                    }
                    compose.onNodeWithText("Connect phone").assertIsDisplayed()
                        .assert(SemanticsMatcher.expectValue(SemanticsProperties.LiveRegion, LiveRegionMode.Polite))
                    compose.onNodeWithText("Pair Android phone.").assertIsDisplayed()
                    compose.onNodeWithContentDescription("Play").assertDoesNotExist()
                    compose.onNodeWithContentDescription("Pause").assertDoesNotExist()
                    compose.onRoot().tryPerformAccessibilityChecks()
                    compose.waitForIdle()
                    instrumentation.waitForIdleSync()
                    capture(stage)
                }
                verifyAndCapture()
                stage = "recreated-unpaired"
                scenario.recreate()
                verifyAndCapture()
            }
        }
    }
}
