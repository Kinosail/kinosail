package com.kinosail.player.wear

import android.graphics.Bitmap
import android.os.Build
import java.io.File
import androidx.activity.ComponentActivity
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.junit4.accessibility.enableAccessibilityChecks
import androidx.compose.ui.test.junit4.v2.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.printToString
import androidx.compose.ui.test.tryPerformAccessibilityChecks
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class WearAccessibilityTest {
    @get:Rule val compose = createAndroidComposeRule<ComponentActivity>()

    @Test fun changingRemoteStatusCanBeAnnounced() = retainingEvidence("heart-permission") {
        val remote = WearRemoteSession(InstrumentationRegistry.getInstrumentation().targetContext)
        compose.setContent {
            WatchApp(remote, null, "Allow heart rate access to make a graph.", {}, {}, {})
        }
        compose.onNodeWithText("Allow heart rate access to make a graph.")
            .assert(SemanticsMatcher.expectValue(SemanticsProperties.LiveRegion, LiveRegionMode.Polite))
        if (Build.VERSION.SDK_INT >= 34) {
            compose.enableAccessibilityChecks()
            compose.onRoot().tryPerformAccessibilityChecks()
        }
    }

    @Test fun disconnectedStateCanBeAnnounced() = retainingEvidence("unpaired") {
        val remote = WearRemoteSession(InstrumentationRegistry.getInstrumentation().targetContext)
        compose.setContent { WatchApp(remote, null, null, {}, {}, {}) }
        compose.waitUntil(10_000) { compose.onAllNodesWithText("Connect phone").fetchSemanticsNodes().isNotEmpty() }
        compose.onNodeWithText("Connect phone")
            .assert(SemanticsMatcher.expectValue(SemanticsProperties.LiveRegion, LiveRegionMode.Polite))
    }

    private fun retainingEvidence(name: String, action: () -> Unit) {
        try {
            action()
            capture(name)
        } catch (failure: Throwable) {
            runCatching {
                val directory = evidenceDirectory()
                File(directory, "wear-$name-semantics.txt").writeText("captureFailed=true\n")
                val image = requireNotNull(InstrumentationRegistry.getInstrumentation().uiAutomation.takeScreenshot())
                try {
                    File(directory, "wear-$name.png").outputStream().use {
                        check(image.compress(Bitmap.CompressFormat.PNG, 100, it))
                    }
                } finally { image.recycle() }
            }.onFailure(failure::addSuppressed)
            throw failure
        }
    }

    private fun evidenceDirectory(): File =
        File(InstrumentationRegistry.getInstrumentation().targetContext.filesDir, "parity-evidence")
            .apply { mkdirs() }

    private fun capture(name: String) {
        val directory = evidenceDirectory()
        compose.waitForIdle()
        val instrumentation = InstrumentationRegistry.getInstrumentation()
        instrumentation.waitForIdleSync()
        Thread.sleep(300)
        File(directory, "wear-$name-semantics.txt").writeText(compose.onRoot().printToString())
        val bitmap = compose.onRoot().captureToImage().asAndroidBitmap()
        File(directory, "wear-$name.png").outputStream().use { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }
        bitmap.recycle()
    }
}
