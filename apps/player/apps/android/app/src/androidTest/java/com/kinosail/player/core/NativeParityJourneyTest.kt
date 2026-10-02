package com.kinosail.player.core

import android.content.Intent
import android.graphics.Bitmap
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.Paint
import android.view.KeyEvent
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.v2.createEmptyComposeRule
import androidx.compose.ui.test.junit4.accessibility.enableAccessibilityChecks
import androidx.compose.ui.test.junit4.accessibility.disableAccessibilityChecks
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import com.kinosail.player.mobile.MobileActivity
import com.kinosail.player.tv.TvActivity
import java.io.ByteArrayOutputStream
import java.io.File
import java.net.ServerSocket
import java.net.URI
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicBoolean
import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class NativeParityJourneyTest {
    @get:Rule val compose = createEmptyComposeRule()
    private val instrumentation = InstrumentationRegistry.getInstrumentation()
    private val context get() = instrumentation.targetContext
    private val tv get() = InstrumentationRegistry.getArguments().getString("tv") == "true"

    @Test fun populatedNavigationDetailsAndMyList() = journey { fixture ->
        waitText(if (tv) "Continue watching" else "Watching")
        capture("home")
        if (tv) {
            instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_DPAD_RIGHT)
            instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_DPAD_DOWN)
            capture("home-focus")
            compose.onAllNodesWithText("Movies")[0].performScrollTo().performClick()
            waitText("New Film")
            compose.onNodeWithText("New Film").performClick()
        } else compose.onNodeWithText("Details").performScrollTo().performClick()
        waitText("Add to My List")
        capture("detail")
        compose.onNodeWithText("Add to My List").performScrollTo().performClick()
        waitText("Remove from My List")
        assertEquals(1, fixture.listWrites)
        instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
        waitText(if (tv) "Movies" else "Watching")
        if (tv) compose.onNodeWithText("Home").performClick()
        else compose.onNodeWithText("More").performClick()
        if (!tv) {
            compose.onNodeWithText("Customize tabs").performScrollTo().performClick()
            compose.onNodeWithText("Remove Movies").performScrollTo().performClick()
            compose.onNodeWithText("Add Listen").performScrollTo().performClick()
            compose.onNodeWithText("Done").performScrollTo().performClick()
            compose.onNodeWithText("Listen").performClick()
            waitText("Listening")
            capture("listen")
            compose.onAllNodes(hasScrollAction())[0].performScrollToIndex(2)
            waitText("Recently added music")
            capture("listen-shelf")
            assertEquals(listOf("home", "shows", "search", "listen"), PersonalTabs(context).load(fixture.viewer))
        }
    }

    @Test fun pendingFailureEmptyAndRecoveryKeepTheirOwnState() = journey("pending") { fixture ->
        compose.waitUntil(20_000) { compose.onAllNodesWithContentDescription("Loading library").fetchSemanticsNodes().isNotEmpty() }
        capture("pending")
        fixture.mode = "failed"
        waitText("Could not load your home. Try again.")
        assertEquals(0, compose.onAllNodesWithContentDescription("Loading library").fetchSemanticsNodes().size)
        capture("failed")
        fixture.mode = "empty"
        compose.onNodeWithText("Try again").performClick()
        waitText("Media added to your Server will appear here.")
        assertEquals(0, compose.onAllNodesWithContentDescription("Loading library").fetchSemanticsNodes().size)
        if (!tv) compose.onAllNodes(hasScrollAction())[0].performScrollToIndex(2)
        capture("empty")
        fixture.mode = "ready"
        if (tv) {
            compose.onNodeWithText("Movies").performScrollTo().performClick()
            compose.onNodeWithText("Home").performClick()
        } else {
            compose.onNodeWithText("Search").performClick()
            compose.onNodeWithText("Home").performClick()
        }
        waitText(if (tv) "Continue watching" else "Watching")
        capture("recovered")
    }

    @Test fun playbackChromeHidesTogetherAndReturnsWithInput() = journey { fixture ->
        waitText(if (tv) "Continue watching" else "Watching")
        if (tv) compose.onNodeWithText("Continuing Film").performClick()
        else compose.onNodeWithText("Resume").performScrollTo().performClick()
        waitText("Speed 1×")
        Thread.sleep(6000)
        capture("playback-before-hide-check")
        compose.waitUntil(10_000) { compose.onAllNodesWithText("Speed 1×").fetchSemanticsNodes().isEmpty() }
        assertEquals(0, compose.onAllNodesWithText("Continuing Film").fetchSemanticsNodes().size)
        capture("playback-hidden")
        if (tv) instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_DPAD_CENTER)
        else compose.onRoot().performTouchInput { click(center) }
        waitText("Speed 1×")
        capture("playback-controls")
        instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
        waitText(if (tv) "Continue watching" else "Watching")
    }

    @Test fun seasonsKeepPendingFailedEmptyAndLoadedLayoutsDistinct() = journey { fixture ->
        waitText(if (tv) "Continue watching" else "Watching")
        fixture.showMode = "pending"
        if (tv) compose.onNodeWithText("TV Shows").performScrollTo().performClick()
        else compose.onNodeWithText("TV Shows").performClick()
        waitText("Fixture Series")
        compose.onNodeWithText("Fixture Series").performClick()
        compose.waitUntil(20_000) { compose.onAllNodesWithContentDescription("Loading library").fetchSemanticsNodes().isNotEmpty() }
        capture("show-pending")
        fixture.showMode = "failed"
        waitText("Could not load this show. Try again.")
        capture("show-failed")
        fixture.showMode = "empty"
        compose.onNodeWithText("Try again").performClick()
        waitText("This show has no available episodes.")
        capture("show-empty")
        instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
        fixture.showMode = "ready"
        waitText("Fixture Series")
        compose.onNodeWithText("Fixture Series").performClick()
        waitText("Resume · S1 E1")
        capture("show-loaded")
        compose.onAllNodes(hasScrollAction())[0].performScrollToIndex(if (tv) 2 else 3)
        compose.onNodeWithText("Season 2").performClick()
        waitText("S2 E1 · Second episode")
        capture("show-season-two")
    }

    private fun journey(mode: String = "ready", check: (ParityFixture) -> Unit) {
        instrumentation.getUiAutomation(android.app.UiAutomation.FLAG_DONT_USE_ACCESSIBILITY)
        ParityFixture(mode).use { fixture ->
            context.getSharedPreferences("kinosail_session", 0).edit().clear().commit()
            context.getSharedPreferences("kinosail_tabs", 0).edit().clear().commit()
            SessionStore(context).save(SavedSession(ServerAddress("http://127.0.0.1:${fixture.port}"), "synthetic-token", fixture.viewer))
            val intent = Intent(context, if (tv) TvActivity::class.java else MobileActivity::class.java)
            ActivityScenario.launch<android.app.Activity>(intent).use { check(fixture) }
        }
    }
    private fun waitText(text: String) {
        compose.waitUntil(20_000) { compose.onAllNodesWithText(text).fetchSemanticsNodes().isNotEmpty() }
        compose.waitForIdle()
    }
    private fun capture(name: String) {
        compose.waitForIdle()
        instrumentation.waitForIdleSync()
        Thread.sleep(300)
        val prefix = InstrumentationRegistry.getArguments().getString("capture") ?: if (tv) "tv" else "phone"
        val directory = File(context.filesDir, "parity-evidence").apply { mkdirs() }
        File(directory, "$prefix-$name-semantics.txt").writeText(compose.onRoot().printToString())
        if (!tv && !name.startsWith("playback")) {
            compose.enableAccessibilityChecks()
            compose.onRoot().tryPerformAccessibilityChecks()
            compose.disableAccessibilityChecks()
        }
        instrumentation.getUiAutomation(android.app.UiAutomation.FLAG_DONT_USE_ACCESSIBILITY).takeScreenshot().let { bitmap ->
            File(directory, "$prefix-$name.png").outputStream().use { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }
            bitmap.recycle()
        }
    }
}

private class ParityFixture(@Volatile var mode: String) : AutoCloseable {
    val viewer = Viewer("Fixture library", "parity-fixture", "viewer", "Alex")
    private val server = ServerSocket(0)
    val port get() = server.localPort
    private val workers = Executors.newCachedThreadPool()
    private val running = AtomicBoolean(true)
    @Volatile var showMode = "ready"
    @Volatile var listWrites = 0
    private var listed = false
    private val movie = item("film", "Continuing Film", progress = 90)
    private val newMovie = item("new-film", "New Film")
    private val series = item("0123456789abcdef", "Fixture Series", kind = "show")
        .replace("\"genres\"", "\"showId\":\"0123456789abcdef\",\"genres\"")
    private val episodes = listOf(item("episode-one", "First episode", progress = 12), item("episode-two", "Second episode"))
        .mapIndexed { index, item -> item.replace("\"genres\"", "\"showId\":\"0123456789abcdef\",\"season\":${index+1},\"episode\":1,\"genres\"") }
    private val song = item("song", "Night Drive", kind = "music", progress = 45)
    init {
        workers.execute {
            while (running.get()) try {
                val socket = server.accept()
                workers.execute {
                    socket.use {
                        val input = it.getInputStream().bufferedReader()
                        val request = input.readLine() ?: return@use
                        var line = input.readLine()
                        var length = 0
                        while (!line.isNullOrEmpty()) {
                            if (line.startsWith("Content-Length:", true)) length = line.substringAfter(':').trim().toInt()
                            line = input.readLine()
                        }
                        val payload = CharArray(length)
                        var read = 0
                        while (read < length) { val n = input.read(payload, read, length - read); if (n < 0) break; read += n }
                        val requestBody = String(payload)
                        val parts = request.split(' ')
                        val uri = URI(parts[1])
                        val path = uri.path
                        var code = 200
                        var type = "application/json"
                        val body: ByteArray = when {
                            path == "/api/v1/me" -> """{"server":"Fixture library","serverId":"parity-fixture","viewer":{"id":"viewer","name":"Alex"}}""".toByteArray()
                            path == "/api/v1/library" -> {
                                while (mode == "pending" && running.get()) Thread.sleep(20)
                                if (mode == "failed") { code = 503; "{}".toByteArray() }
                                else {
                                    val query = uri.query.split('&').associate { val p = it.split('=', limit = 2); p[0] to p.getOrElse(1) { "" } }
                                    val view = query["view"] ?: "all"
                                    val items = if (mode == "empty") emptyList() else when (view) {
                                        "history" -> listOf(movie, song)
                                        "movies" -> listOf(newMovie, movie)
                                        "shows" -> listOf(series)
                                        "music" -> listOf(song)
                                        "list" -> if (listed) listOf(movie) else emptyList()
                                        "all" -> listOf(movie, newMovie, song)
                                        else -> emptyList()
                                    }
                                    """{"items":[${items.joinToString(",")}],"total":${items.size},"offset":0,"limit":${query["limit"] ?: "24"},"view":"$view","sort":"${query["sort"] ?: "title"}","query":"${query["q"] ?: ""}"}""".toByteArray()
                                }
                            }
                            path == "/api/v1/shows/0123456789abcdef" -> {
                                while (showMode == "pending" && running.get()) Thread.sleep(20)
                                if (showMode == "failed") { code = 503; "{}".toByteArray() }
                                else """{"id":"0123456789abcdef","title":"Fixture Series","year":"2026","plot":"A synthetic series.","episodes":[${if (showMode == "empty") "" else episodes.joinToString(",")}]}""".toByteArray()
                            }
                            path.startsWith("/api/v1/items/") && path.endsWith("/list") -> {
                                listed = JSONObject(requestBody).getBoolean("listed"); listWrites++
                                """{"listed":$listed}""".toByteArray()
                            }
                            path.endsWith("/playback-preferences") -> """{"playback":{"audioTrack":"","subtitleTrack":"","rate":1,"audioLanguage":"auto","subtitleLanguage":"off","nightMode":false,"dialogueBoost":false,"volumeBoost":1},"overridden":false}""".toByteArray()
                            path.endsWith("/progress/sync") -> JSONObject(requestBody).getJSONObject("progress").toString().toByteArray()
                            path.endsWith("/playback") -> """{"plan":{"allowed":true,"mode":"direct","reason":"direct-preferred"},"directAllowed":true,"direct":"/media/film","directType":"video/mp4","duration":20,"start":0}""".toByteArray()
                            path == "/media/film" -> {
                                type = "video/mp4"
                                InstrumentationRegistry.getInstrumentation().context.assets.open("parity-video.mp4").use { it.readBytes() }
                            }
                            path.startsWith("/api/v1/items/") -> {
                                val selected = if (path.endsWith("new-film")) newMovie else movie
                                """{"item":$selected,"listed":$listed,"profileId":"viewer"}""".toByteArray()
                            }
                            path.startsWith("/art/") || path.startsWith("/backdrop/") -> {
                                type = "image/png"
                                artwork(path.startsWith("/backdrop/"))
                            }
                            else -> { code = 404; "{}".toByteArray() }
                        }
                        it.getOutputStream().apply {
                            write("HTTP/1.1 $code OK\r\nContent-Type: $type\r\nContent-Length: ${body.size}\r\nConnection: close\r\n\r\n".toByteArray())
                            write(body); flush()
                        }
                    }
                }
            } catch (_: Exception) { }
        }
    }
    private fun item(id: String, title: String, kind: String = "video", progress: Int = 0) =
        """{"id":"$id","kind":"$kind","title":"$title","year":"2026","plot":"A synthetic title for a repeatable Android journey.","artwork":"/art/$id","backdrop":"/backdrop/$id","rating":"PG","genres":"Drama · Adventure","progress":{"seconds":$progress}}"""
    private fun artwork(landscape: Boolean): ByteArray {
        val bitmap = Bitmap.createBitmap(if (landscape) 640 else 240, if (landscape) 360 else 360, Bitmap.Config.ARGB_8888)
        val canvas = Canvas(bitmap)
        canvas.drawColor(Color.rgb(31, 58, 52))
        val paint = Paint().apply { color = Color.rgb(130, 173, 161) }
        canvas.drawCircle(bitmap.width * .7f, bitmap.height * .35f, bitmap.height * .22f, paint)
        return ByteArrayOutputStream().apply { bitmap.compress(Bitmap.CompressFormat.PNG, 100, this); bitmap.recycle() }.toByteArray()
    }
    override fun close() { running.set(false); server.close(); workers.shutdownNow() }
}
