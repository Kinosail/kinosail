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
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performSemanticsAction
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextInput
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

    @Test fun decodedLightVideoKeepsItsTitleAndActionsReadable() = journey { json, title ->
        awaitLightFrame()
        val prefix = json.optString("capture", "phone")
        verifyLabels(capture("$prefix-loaded"), title)
        // Real View/decoder timers are outside Compose's test clock.
        Thread.sleep(4_000)
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

    @Test fun pendingAndFailedMediaKeepDismissalAndRetryUsable() = journey("pending") { json, title ->
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

    private fun journey(mode: String = "ready", check: (JSONObject, String) -> Unit) {
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
                waitForText("Library")
                compose.onNodeWithText("Library").performClick()
                waitForText("Search library")
                compose.onNodeWithText("Search library").performTextInput(title)
                compose.onNodeWithText("Search", useUnmergedTree = true).performClick()
                compose.waitUntil(30_000) {
                    compose.onAllNodesWithContentDescription(title).fetchSemanticsNodes().isNotEmpty()
                }
                compose.onNodeWithContentDescription(title).performClick()
                compose.waitUntil(30_000) {
                    compose.onAllNodesWithText("Play").fetchSemanticsNodes().isNotEmpty() ||
                        compose.onAllNodesWithText("Resume").fetchSemanticsNodes().isNotEmpty()
                }
                val action = if (compose.onAllNodesWithText("Resume").fetchSemanticsNodes().isEmpty()) "Play" else "Resume"
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
