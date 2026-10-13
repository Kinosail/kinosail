package com.kinosail.player.mobile

import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Color
import android.content.pm.ActivityInfo
import android.os.ParcelFileDescriptor
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.junit4.v2.createEmptyComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onAllNodesWithContentDescription
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.hasText
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performSemanticsAction
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.performTouchInput
import androidx.compose.ui.test.click
import androidx.compose.ui.test.printToString
import androidx.compose.ui.text.TextLayoutResult
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import com.kinosail.player.core.CatalogApi
import com.kinosail.player.core.ProgressApi
import com.kinosail.player.core.ProgressJournal
import com.kinosail.player.core.SavedSession
import com.kinosail.player.core.ServerAddress
import com.kinosail.player.core.ServerApi
import com.kinosail.player.core.SessionStore
import com.kinosail.player.core.StrictJson
import com.kinosail.player.core.WatchProgress
import java.io.File
import java.net.HttpURLConnection
import java.net.URL
import java.net.URI
import java.util.UUID
import kotlin.math.pow
import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Assume.assumeTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

// Opt-in real Server journey. The runner copies a private, synthetic session seed
// into the isolated app. Playback uses the actual decoder and Server responses.
@RunWith(AndroidJUnit4::class)
class LivePlaybackOverlayTest {
    @get:Rule val compose = createEmptyComposeRule()

    @Test fun decodedLightVideoKeepsItsTitleAndActionsReadable() = journey("loaded") { json, title ->
        awaitLightFrame()
        val prefix = json.optString("capture", "phone")
        verifyLabels(capture("$prefix-loaded"), title)
        // Real View/decoder timers are outside Compose's test clock.
        Thread.sleep(5_000)
        compose.onRoot().performTouchInput { click(center) }
        waitForText("Speed 1×")
        verifyLabels(capture("$prefix-controller-settled"), title)
        val captions = captionsLabel()
        compose.onNodeWithText(captions).assertIsEnabled().performClick()
        waitForText(if (captions == "Captions on") "Captions off" else "Captions on")
        compose.onNodeWithText(captionsLabel()).performClick()
        waitForText(captions)
        compose.onNodeWithText(speedLabel()).assertIsEnabled().performClick()
        waitForText("Playback speed")
        capture("$prefix-speed-picker").recycle()
        compose.onAllNodesWithText("Normal", substring = true).let {
            it[0].performScrollTo().performClick()
        }
        waitForText("Speed 1×")
        verifyLabels(capture("$prefix-actions-used"), title)
    }

    @Test fun pendingAndFailedMediaKeepDismissalAndRetryUsable() = journey("recovery", "pending") { json, title ->
        compose.waitUntil(30_000) { progressIndicators() > 0 }
        val prefix = json.optString("capture", "phone")
        verifyLabels(capture("$prefix-pending"), title)
        setMedia(json, "failed")
        waitForText("Playback stopped. Check this title and your Server connection.")
        assertEquals("No pending placeholder after failure", 0, progressIndicators())
        verifyLabels(capture("$prefix-failed"), title)
        compose.onNodeWithText("Try again").assertIsEnabled()
        setMedia(json, "ready")
        compose.onNodeWithText("Try again").performClick()
        awaitLightFrame()
        compose.waitUntil(30_000) { progressIndicators() == 0 }
        verifyLabels(capture("$prefix-recovered"), title)
    }

    // Browser evidence cannot prove the native caption compositor respects Android system bars.
    // Use real decoded static media and on/off compositor frames, without private player internals.
    @Test fun captionsStayAboveTheSystemNavigationArea() = journey("captions") { json, _ ->
        awaitLightFrame()
        Thread.sleep(2_000) // The generated cue starts at one second.
        val prefix = json.optString("capture", "phone")
        val instrumentation = InstrumentationRegistry.getInstrumentation()
        val automation = instrumentation.uiAutomation
        fun accessibilitySnapshot(): List<android.view.accessibility.AccessibilityNodeInfo>? {
            val nodes = mutableListOf<android.view.accessibility.AccessibilityNodeInfo>()
            fun visit(node: android.view.accessibility.AccessibilityNodeInfo?): Boolean {
                if (node == null || nodes.size >= 250) return false
                nodes.add(node)
                for (index in 0 until node.childCount) if (!visit(node.getChild(index))) return false
                return true
            }
            return if (visit(automation.rootInActiveWindow)) nodes else null
        }
        fun visibleAction(nodes: List<android.view.accessibility.AccessibilityNodeInfo>, description: String) =
            nodes.firstOrNull { it.contentDescription?.toString() == description && it.isVisibleToUser }
        fun action(description: String) = accessibilitySnapshot()?.let { visibleAction(it, description) }
        fun completeSnapshot(): List<android.view.accessibility.AccessibilityNodeInfo> {
            var nodes: List<android.view.accessibility.AccessibilityNodeInfo>? = null
            compose.waitUntil(10_000) { nodes = accessibilitySnapshot(); nodes != null }
            return requireNotNull(nodes)
        }
        fun controllerHidden(): Boolean {
            val nodes = accessibilitySnapshot() ?: return false
            val visible = nodes.filter { it.isVisibleToUser }.map { it.contentDescription?.toString() }
            return "Show player controls" in visible &&
                "Play" !in visible && "Pause" !in visible && "Hide player controls" !in visible
        }
        val initialControls = completeSnapshot()
        if (visibleAction(initialControls, "Pause") == null && visibleAction(initialControls, "Play") == null) {
            compose.onRoot().performTouchInput { click(androidx.compose.ui.geometry.Offset(center.x, height * 0.65f)) }
        }
        compose.waitUntil(10_000) { action("Pause") != null || action("Play") != null }
        fun pausePlayback() {
            compose.waitUntil(10_000) {
                // A remote accessibility node can become stale between discovery and click.
                action("Pause")?.performAction(android.view.accessibility.AccessibilityNodeInfo.ACTION_CLICK) == true ||
                    action("Play") != null
            }
            compose.waitUntil(10_000) { action("Play") != null }
        }
        pausePlayback()
        val forceCaptionsOff = InstrumentationRegistry.getArguments().getString("captionOracleControl") == "true"
        fun captions(enabled: Boolean) {
            val target = if (enabled) "Captions on" else "Captions off"
            if (captionsLabel() != target) compose.onNodeWithText(captionsLabel()).performClick()
            waitForText(target)
            Thread.sleep(500) // Wait for the native decoder's next cue delivery.
        }
        fun difference(a: Int, b: Int) = maxOf(kotlin.math.abs(Color.red(a) - Color.red(b)),
            kotlin.math.abs(Color.green(a) - Color.green(b)), kotlin.math.abs(Color.blue(a) - Color.blue(b)))
        fun hasVisibleChange(reference: Bitmap, current: Bitmap): Boolean {
            if (reference.width != current.width || reference.height != current.height) return false
            var changed = 0
            for (y in reference.height / 2 until reference.height) for (x in 0 until reference.width) {
                if (difference(reference.getPixel(x, y), current.getPixel(x, y)) > 20 && ++changed > 100) return true
            }
            return false
        }
        fun captureCaptionFrame(name: String, reference: Bitmap? = null): Bitmap {
            // The native controller is painted above the captions. Dismiss it through the
            // video surface below its centered Play button so it cannot hide an overlap.
            val controls = completeSnapshot()
            assertNull("Playback must remain paused", visibleAction(controls, "Pause"))
            if (visibleAction(controls, "Play") != null) compose.onRoot().performTouchInput {
                click(androidx.compose.ui.geometry.Offset(center.x, height * 0.65f))
            }
            compose.waitUntil(10_000) { controllerHidden() }
            Thread.sleep(300) // Retain a frame after the controller's hide transition.
            assertTrue("A complete accessibility tree must confirm the controller is hidden", controllerHidden())
            if (reference != null) {
                // Track selection can finish before Media3 paints its cue. Observe actual
                // pixels with a bound, then retain the original visibility/overlap oracle.
                val deadline = android.os.SystemClock.elapsedRealtime() + 10_000
                while (true) {
                    val current = screenPixels()
                    val visible = try { hasVisibleChange(reference, current) } finally { current.recycle() }
                    if (visible || android.os.SystemClock.elapsedRealtime() >= deadline) break
                    Thread.sleep(250)
                }
            }
            assertTrue("The controller must stay hidden until capture", controllerHidden())
            val pixels = capture(name)
            try {
                assertTrue("The controller must remain hidden across capture", controllerHidden())
                val hiddenControls = completeSnapshot()
                assertNotNull("The captured player must expose its Show controls action",
                    visibleAction(hiddenControls, "Show player controls"))
                compose.onRoot().performTouchInput { click(androidx.compose.ui.geometry.Offset(center.x, height * 0.65f)) }
                compose.waitUntil(10_000) {
                    accessibilitySnapshot()?.let {
                        visibleAction(it, "Play") != null && visibleAction(it, "Pause") == null
                    } == true
                }
                compose.waitUntil(10_000) {
                    compose.onAllNodesWithText("Captions ", substring = true).fetchSemanticsNodes().isNotEmpty()
                }
                compose.onNodeWithText(captionsLabel()).assertIsDisplayed()
                waitForText("Done")
                return pixels
            } catch (error: Throwable) {
                pixels.recycle()
                throw error
            }
        }
        var bottomInset = 0
        instrumentation.runOnMainSync {
            val activity = androidx.test.runner.lifecycle.ActivityLifecycleMonitorRegistry.getInstance()
                .getActivitiesInStage(androidx.test.runner.lifecycle.Stage.RESUMED).single()
            bottomInset = requireNotNull(androidx.core.view.ViewCompat.getRootWindowInsets(activity.window.decorView))
                .getInsets(androidx.core.view.WindowInsetsCompat.Type.systemBars() or
                    androidx.core.view.WindowInsetsCompat.Type.displayCutout()).bottom
        }
        assertTrue("This mobile check needs a visible bottom system navigation area", bottomInset > 0)
        captions(false)
        val before = captureCaptionFrame("$prefix-captions-off-before")
        captions(!forceCaptionsOff)
        val shown = captureCaptionFrame("$prefix-captions-on", if (forceCaptionsOff) null else before)
        captions(false)
        val after = captureCaptionFrame("$prefix-captions-off-after")
        try {
            assertEquals(before.width, shown.width)
            assertEquals(before.height, shown.height)
            assertEquals(before.width, after.width)
            assertEquals(before.height, after.height)
            var captionPixels = 0
            var protectedPixels = 0
            var unstableProtectedPixels = 0
            var lowestCaptionPixel = -1
            val safeBottom = before.height - bottomInset
            for (y in before.height / 2 until before.height) for (x in 0 until before.width) {
                val a = before.getPixel(x, y)
                val b = after.getPixel(x, y)
                if (difference(a, b) > 3) {
                    if (y >= safeBottom) unstableProtectedPixels++
                    continue
                }
                val displayed = shown.getPixel(x, y)
                if (difference(a, displayed) > 20 && difference(b, displayed) > 20) {
                    captionPixels++
                    lowestCaptionPixel = maxOf(lowestCaptionPixel, y)
                    if (y >= safeBottom) protectedPixels++
                }
            }
            val evidence = JSONObject().put("bottomInset", bottomInset).put("safeBottom", safeBottom)
                .put("width", before.width).put("height", before.height)
                .put("captionPixels", captionPixels).put("protectedCaptionPixels", protectedPixels)
                .put("unstableProtectedPixels", unstableProtectedPixels).put("lowestCaptionPixel", lowestCaptionPixel)
                .put("forcedCaptionsOffControl", forceCaptionsOff)
            File(instrumentation.targetContext.filesDir, "video-overlay/$prefix-caption-safe-area.json")
                .writeText(evidence.toString(2))
            if (captionPixels <= 100 && !forceCaptionsOff) {
                // Diagnose delayed cue delivery without changing the retained failed pair.
                // This extra frame cannot make the original visibility assertion pass.
                runCatching {
                    val deadline = android.os.SystemClock.elapsedRealtime() + 15_000
                    fun awaitDiagnostic(condition: () -> Boolean) {
                        val remaining = deadline - android.os.SystemClock.elapsedRealtime()
                        check(remaining > 0)
                        compose.waitUntil(remaining, condition)
                    }
                    compose.onNodeWithText("Captions off").performClick()
                    awaitDiagnostic { captionsLabel() == "Captions on" }
                    awaitDiagnostic {
                        action("Play")?.performAction(android.view.accessibility.AccessibilityNodeInfo.ACTION_CLICK) == true ||
                            action("Pause") != null
                    }
                    awaitDiagnostic { action("Pause") != null }
                    check(deadline - android.os.SystemClock.elapsedRealtime() >= 2_000)
                    Thread.sleep(2_000)
                    awaitDiagnostic {
                        action("Pause")?.performAction(android.view.accessibility.AccessibilityNodeInfo.ACTION_CLICK) == true ||
                            action("Play") != null
                    }
                    awaitDiagnostic { action("Play") != null }
                    compose.onRoot().performTouchInput {
                        click(androidx.compose.ui.geometry.Offset(center.x, height * 0.65f))
                    }
                    awaitDiagnostic { controllerHidden() }
                    check(deadline - android.os.SystemClock.elapsedRealtime() >= 300)
                    Thread.sleep(300)
                    check(controllerHidden())
                    capture("$prefix-captions-after-resume").recycle()
                }
            }
            assertEquals("The system navigation area must be fully observed and stable", 0, unstableProtectedPixels)
            assertTrue("Captions must visibly render in the paired screenshots", captionPixels > 100)
            assertEquals("Caption pixels must not overlap the system navigation area", 0, protectedPixels)
        } finally {
            before.recycle(); shown.recycle(); after.recycle()
        }
    }

    private fun journey(evidenceName: String, mode: String = "ready", check: (JSONObject, String) -> Unit) {
        val instrumentation = InstrumentationRegistry.getInstrumentation()
        val context = instrumentation.targetContext
        val seed = File(context.filesDir, "video-overlay-seed.json")
        assumeTrue("Requires an isolated Server, generated white video and private session seed", seed.exists())
        require(seed.length() in 1..16384) { "Invalid video overlay seed." }
        val json = try { JSONObject(StrictJson.parse(seed.readText()).toString()) }
            catch (_: Exception) { throw IllegalArgumentException("Invalid video overlay seed.") }
        val fields = json.keys().asSequence().toSet()
        require(fields.containsAll(setOf("version", "server", "token", "title")) &&
            fields.all { it in setOf("version", "server", "token", "title", "capture", "landscape") } &&
            json.get("version") == 1 && listOf("server", "token", "title").all { json.get(it) is String } &&
            (!json.has("capture") || json.get("capture") is String) &&
            (!json.has("landscape") || json.get("landscape") is Boolean)) { "Invalid video overlay seed." }
        val address = try { URI(json.getString("server")) }
            catch (_: Exception) { throw IllegalArgumentException("Invalid video overlay seed.") }
        require(address.scheme == "http" && address.host == "10.0.2.2" && address.port == 39231 &&
            address.rawQuery == null && address.rawFragment == null && address.userInfo == null &&
            address.path.isNullOrEmpty() && json.getString("token").toByteArray().size in 1..512 &&
            json.getString("token").none(Char::isISOControl) &&
            json.getString("title") in setOf("Native portrait contrast 01a0f2ab", "Native landscape contrast 01a0f2ab") &&
            json.optString("capture", "phone").matches(Regex("[a-z0-9-]{1,80}"))) { "Invalid video overlay seed." }
        val server = ServerAddress(json.getString("server"))
        val token = json.getString("token")
        val title = json.getString("title")
        val viewer = ServerApi(server).viewer(token)
        val page = CatalogApi(server).list(token, viewer.id, rawQuery = title)
        assertEquals("Use the exact generated title", listOf(title), page.items.map { it.title })
        assertEquals("video", page.items.single().kind)
        val itemId = page.items.single().id
        val progress = ProgressApi(server)
        val journal = ProgressJournal.forViewer(context, server, viewer)
        for (entry in journal.pending().filter { it.itemId == itemId }) {
            val result = progress.sync(itemId, token, viewer.id, entry.progress, entry.expected)
            journal.apply(itemId, entry.progress, result)
            if (result.conflict) journal.resolve(itemId, false)
        }
        val reset = progress.sync(itemId, token, viewer.id, WatchProgress(session = UUID.randomUUID().toString(), revision = 1),
            progress.current(itemId, token, viewer.id))
        assertFalse("The generated title starts from known progress", reset.conflict)
        assertEquals(0.0, reset.progress.seconds, 0.0)
        val store = SessionStore(context)
        store.clear()
        store.save(SavedSession(server, token, viewer))
        try {
            setMedia(json, mode)
            ActivityScenario.launch(MobileActivity::class.java).use { scenario ->
                var stage = "search"
                try {
                    waitForText("Search")
                    compose.onNodeWithText("Search").performClick()
                    waitForText("Search library")
                    compose.onNodeWithText("Search library").performTextInput(title)
                    compose.onNode(hasText("Search") and SemanticsMatcher.expectValue(SemanticsProperties.Role, Role.Button)).performClick()
                    compose.waitUntil(30_000) {
                        compose.onAllNodesWithContentDescription(title).fetchSemanticsNodes().isNotEmpty()
                    }
                    stage = "detail"
                    compose.onNodeWithContentDescription(title).performClick()
                    compose.waitUntil(30_000) {
                        compose.onAllNodesWithText("Play").fetchSemanticsNodes().isNotEmpty() ||
                            compose.onAllNodesWithText("Resume").fetchSemanticsNodes().isNotEmpty()
                    }
                    val action = if (compose.onAllNodesWithText("Resume").fetchSemanticsNodes().isEmpty()) "Play" else "Resume"
                    stage = "playback"
                    compose.onNodeWithText(action).performClick()
                    if (json.optBoolean("landscape")) scenario.onActivity {
                        it.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_LANDSCAPE
                    }
                    compose.waitUntil(30_000) {
                        compose.onAllNodesWithText("Speed ", substring = true).fetchSemanticsNodes().isNotEmpty()
                    }
                    check(json, title)
                    compose.onNodeWithText("Done").assertIsEnabled().performClick()
                    waitForText("Back")
                } catch (error: Throwable) {
                    // Keep the visible isolated fixture before ActivityScenario closes it.
                    runCatching {
                        capture(json.optString("capture", "phone") + "-" + evidenceName + "-" + stage + "-failure").recycle()
                    }
                    throw error
                }
            }
        } finally {
            setMedia(json, "ready")
            store.clear()
        }
    }

    private fun setMedia(json: JSONObject, mode: String) {
        val connection = URL(json.getString("server") + "/__fixture/media").openConnection() as HttpURLConnection
        try {
            connection.connectTimeout = 5_000
            connection.readTimeout = 5_000
            connection.requestMethod = "POST"
            connection.setRequestProperty("X-Fixture-Control", "video-overlay")
            connection.doOutput = true
            connection.outputStream.use { it.write(JSONObject().put("mode", mode).toString().toByteArray()) }
            assertEquals("Operator-owned media fixture accepted", 204, connection.responseCode)
        } finally { connection.disconnect() }
    }

    private fun awaitLightFrame() {
        compose.waitUntil(30_000) {
            val pixels = screenPixels()
            val center = pixels.getPixel(pixels.width / 2, pixels.height / 2)
            pixels.recycle()
            Color.red(center) >= 220 && Color.green(center) >= 220 && Color.blue(center) >= 220
        }
    }

    // Use the same physical compositor capture path as adb in every configuration.
    private fun screenPixels(): Bitmap {
        val automation = InstrumentationRegistry.getInstrumentation().uiAutomation
        return ParcelFileDescriptor.AutoCloseInputStream(automation.executeShellCommand("screencap -p")).use {
            requireNotNull(BitmapFactory.decodeStream(it))
        }
    }

    private fun progressIndicators() = compose.onAllNodes(
        SemanticsMatcher.keyIsDefined(SemanticsProperties.ProgressBarRangeInfo)).fetchSemanticsNodes().size
    private fun speedLabel() = label("Speed ")
    private fun captionsLabel() = label("Captions ")
    private fun label(prefix: String) = compose.onAllNodesWithText(prefix, substring = true).fetchSemanticsNodes()
        .single().config[SemanticsProperties.Text].single().text

    private fun verifyLabels(pixels: Bitmap, title: String) {
        val actions = listOfNotNull(title, "Done",
            if (compose.onAllNodesWithText("Speed ", substring = true).fetchSemanticsNodes().isEmpty()) null else speedLabel(),
            if (compose.onAllNodesWithText("Captions ", substring = true).fetchSemanticsNodes().isEmpty()) null else captionsLabel())
        for (label in actions) {
            val node = compose.onNodeWithText(label).assertIsDisplayed()
            val bounds = node.fetchSemanticsNode().boundsInWindow
            assertTrue("$label fits the captured screen", bounds.left >= 0 && bounds.top >= 0 &&
                bounds.right <= pixels.width && bounds.bottom <= pixels.height)
            val layouts = mutableListOf<TextLayoutResult>()
            node.performSemanticsAction(SemanticsActions.GetTextLayoutResult) { assertTrue(it(layouts)) }
            val foreground = layouts.single().layoutInput.style.color.toArgb()
            assertEquals("Observe the resolved text color", 255, Color.alpha(foreground))
            val background = pixels.getPixel(bounds.left.toInt() + 1, bounds.bottom.toInt() + 1)
            val light = maxOf(luminance(foreground), luminance(background))
            val dark = minOf(luminance(foreground), luminance(background))
            val contrast = (light + .05) / (dark + .05)
            assertTrue("$label contrast over a decoded light frame was $contrast", contrast >= 4.5)
        }
        pixels.recycle()
    }

    private fun waitForText(text: String) {
        compose.waitUntil(30_000) { compose.onAllNodesWithText(text).fetchSemanticsNodes().isNotEmpty() }
        compose.onNodeWithText(text).assertIsDisplayed()
    }

    private fun capture(name: String): Bitmap {
        val instrumentation = InstrumentationRegistry.getInstrumentation()
        val pixels = screenPixels()
        val root = File(instrumentation.targetContext.filesDir, "video-overlay").apply { mkdirs() }
        File(root, "$name.png").outputStream().use { pixels.compress(Bitmap.CompressFormat.PNG, 100, it) }
        File(root, "$name-semantics.txt").writeText(compose.onRoot().printToString())
        return pixels
    }

    private fun luminance(color: Int): Double = listOf(Color.red(color), Color.green(color), Color.blue(color))
        .map { value -> (value / 255.0).let { if (it <= .04045) it / 12.92 else ((it + .055) / 1.055).pow(2.4) } }
        .zip(listOf(.2126, .7152, .0722)).sumOf { (channel, weight) -> channel * weight }
}
