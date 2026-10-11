package com.kinosail.player.core

import android.content.Intent
import android.graphics.Bitmap
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.Paint
import android.view.KeyEvent
import androidx.compose.ui.semantics.SemanticsActions
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

    // Only a native journey can interleave a rendered chapter change with the device's durable journal.
    @Test fun epubKeepsLatestPositionWhilePreviousSaveFinishes() = journey { fixture ->
        org.junit.Assume.assumeFalse(tv)
        waitText("Watching")
        compose.onNodeWithText("More").activate()
        compose.onNodeWithText("Books").performScrollTo().activate()
        waitText("A quiet crossing")
        compose.onNodeWithText("A quiet crossing").activate()
        waitText("Read book")
        compose.onNodeWithText("Read book").performScrollTo().activate()
        waitChapter("Departure")
        fixture.holdReaderSave = true
        compose.onNodeWithText("Next").activate()
        waitChapter("Harbor lights")
        compose.waitUntil(20_000) { fixture.readerSaveInFlight }
        compose.onNodeWithText("Previous").activate()
        waitChapter("Departure")
        fixture.holdReaderSave = false
        compose.waitUntil(20_000) { fixture.readerWrites >= 2 && fixture.readerPage == 1 }
        capture("epub-latest-position")
        compose.onNodeWithText("Done").activate()
        compose.onNodeWithText("Read book").performScrollTo().activate()
        waitChapter("Departure")
        waitText("Chapter 1 of 2")
    }

    // Native WebView rendering, script isolation, and chapter navigation have no browser E2E owner.
    // Exercise malformed manifests, failed chapters, escaped resources, and persisted progress.
    @Test fun epubReadsChaptersSafelyAndRestoresSavedPosition() = journey { fixture ->
        org.junit.Assume.assumeFalse(tv)
        waitText("Watching")
        compose.onNodeWithText("More").activate()
        compose.onNodeWithText("Books").performScrollTo().activate()
        waitText("A quiet crossing")
        compose.onNodeWithText("A quiet crossing").activate()
        waitText("Read book")
        fixture.bookMode = "invalid"
        compose.onNodeWithText("Read book").performScrollTo().activate()
        waitText("Could not open this book. Check your Server connection and try again.")
        assertEquals(0, fixture.chapterReads)
        fixture.bookMode = "empty"
        compose.onNodeWithText("Try again").activate()
        waitText("Could not open this book. Check your Server connection and try again.")
        assertEquals(0, fixture.chapterReads)
        capture("epub-empty")
        fixture.bookMode = "pending"
        compose.onNodeWithText("Try again").activate()
        compose.waitUntil(20_000) { fixture.chapterReads > 0 }
        compose.onNodeWithContentDescription("Opening your book").assertExists()
        capture("epub-pending")
        fixture.bookMode = "failed"
        waitText("Could not load this chapter. Try again.")
        capture("epub-failed")
        fixture.bookMode = "ready"
        compose.onNodeWithText("Try again").activate()
        waitText("Chapter 1 of 2")
        waitChapter("Departure")
        capture("epub-loaded")
        instrumentation.runOnMainSync {
            val activity = androidx.test.runner.lifecycle.ActivityLifecycleMonitorRegistry.getInstance()
                .getActivitiesInStage(androidx.test.runner.lifecycle.Stage.RESUMED).first()
            fun find(view: android.view.View): android.webkit.WebView? {
                if (view is android.webkit.WebView) return view
                if (view is android.view.ViewGroup) for (index in 0 until view.childCount) {
                    find(view.getChildAt(index))?.let { return it }
                }
                return null
            }
            val web = requireNotNull(find(activity.window.decorView))
            assertTrue(requireNotNull(web.webViewRenderProcess).terminate())
        }
        waitText("Could not load this chapter. Try again.")
        capture("epub-renderer-stopped")
        compose.onNodeWithText("Try again").activate()
        waitChapter("Departure")
        compose.onNodeWithText("Next").activate()
        waitText("Chapter 2 of 2")
        waitChapter("Harbor lights")
        compose.waitUntil(20_000) { fixture.readerPage == 2 }
        capture("epub-next")
        compose.onNodeWithText("Done").activate()
        compose.onNodeWithText("Read book").performScrollTo().activate()
        waitText("Chapter 2 of 2")
        waitChapter("Harbor lights")
        assertEquals(0, fixture.escapedResourceReads)
        capture("epub-restored")
        fixture.failReaderSaves = true
        compose.onNodeWithText("Previous").activate()
        waitText("Reading position was not saved.")
        compose.onNodeWithText("Done").activate()
        compose.onNodeWithText("Read book").performScrollTo().activate()
        waitText("Chapter 1 of 2")
        waitChapter("Departure")
        waitText("Reading position was not saved.")
        fixture.failReaderSaves = false
        compose.onNodeWithText("Retry sync").activate()
        compose.waitUntil(20_000) { fixture.readerPage == 1 }
        fixture.bookMode = "failed"
        compose.onNodeWithText("Next").activate()
        waitText("Could not load this chapter. Try again.")
        compose.onNodeWithText("Done").activate()
        assertEquals("A failed chapter must not advance saved progress", 1, fixture.readerPage)
    }

    private fun waitChapter(text: String) {
        compose.waitUntil(20_000) { compose.onAllNodesWithContentDescription("Opening your book").fetchSemanticsNodes().isEmpty() }
        val automation = instrumentation.getUiAutomation()
        fun contains(node: android.view.accessibility.AccessibilityNodeInfo?): Boolean {
            if (node == null) return false
            if (node.text?.toString() == text) return true
            return (0 until node.childCount).any { contains(node.getChild(it)) }
        }
        compose.waitUntil(20_000) { contains(automation.rootInActiveWindow) }
    }

    // Native navigation, D-pad focus, and stale-detail recovery are not covered by web E2E.
    // The fixture also rejects unsafe collection names before any detail request can occur.
    @Test fun collectionsRecoverAndReturnFromDetailsWithoutLosingTheirPlace() = journey { fixture ->
        waitText(if (tv) "Continue watching" else "Watching")
        fixture.collectionMode = "pending"
        if (!tv) compose.onNodeWithText("More").activate()
        compose.onNodeWithText("Collections").performScrollTo().activate()
        compose.waitUntil(20_000) { compose.onAllNodesWithContentDescription("Loading collections").fetchSemanticsNodes().isNotEmpty() }
        capture("collections-pending")
        fixture.collectionMode = "failed"
        waitText("Could not load collections. Try again.")
        capture("collections-failed")
        fixture.collectionMode = "invalid"
        compose.onNodeWithText("Try again").activate()
        waitText("Could not load collections. Try again.")
        assertEquals(0, fixture.collectionDetailReads)
        fixture.collectionMode = "empty"
        compose.onNodeWithText("Try again").activate()
        waitText("No collections yet")
        capture("collections-empty")
        fixture.collectionMode = "ready"
        compose.onNodeWithText("Refresh").activate()
        waitText("Family & friends")
        capture("collections-loaded")
        if (tv) {
            compose.onNodeWithText("Family & friends").performSemanticsAction(SemanticsActions.RequestFocus) { it() }
            capture("collections-focused")
        }
        compose.onNodeWithText("Family & friends").activate()
        waitText("New Film")
        capture("collection-titles")
        compose.onNodeWithText("New Film").activate()
        waitText("Add to My List")
        instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
        compose.waitUntil(20_000) { compose.onAllNodes(hasText("New Film") and hasClickAction()).fetchSemanticsNodes().isNotEmpty() }
        assertEquals(1, fixture.collectionDetailReads)
        instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
        waitText("Family & friends")
    }

    // The populated Go Server browser journeys cannot inspect Compose geometry or remote focus.
    // Keep this device journey for portrait-only continuation artwork and fixed TV navigation.
    @Test fun homeMatchesAppleLandscapeShelvesAndPersistentTvNavigation() = journey {
        waitText(if (tv) "Continue watching" else "Watching")
        compose.onNodeWithText("Continue watching").performScrollTo()
        val card = compose.onNodeWithContentDescription("Next Film")
        card.performScrollTo().assertIsDisplayed()
        val bounds = card.fetchSemanticsNode().boundsInRoot
        assertTrue("Landscape fallback must keep a wide card: $bounds", bounds.height < bounds.width)
        compose.onNode(hasText("PG · Drama · Adventure") and hasAnyAncestor(hasContentDescription("Next Film")),
            useUnmergedTree = true).assertExists()
        compose.onNode(hasText("Resume at 0:30") and hasAnyAncestor(hasContentDescription("Next Film")),
            useUnmergedTree = true).assertExists()
        capture("home-continuation")
        if (tv) {
            compose.onAllNodes(hasScrollAction())[0].performScrollToNode(hasText("Unwatched movies"))
            compose.onNodeWithText("Search").assertIsDisplayed().activate()
            waitText("Search library")
            instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
            waitText("Continue watching")
            compose.onAllNodes(hasScrollAction())[0].performScrollToNode(hasText("Unwatched movies"))
            compose.onNodeWithText("Settings").assertIsDisplayed().activate()
            waitText("Sign out")
            instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
            waitText("Continue watching")
            capture("home-navigation-return")
        }
    }

    @Test fun populatedNavigationDetailsAndMyList() = journey { fixture ->
        waitText(if (tv) "Continue watching" else "Watching")
        capture("home")
        if (tv) {
            instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_DPAD_RIGHT)
            instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_DPAD_DOWN)
            capture("home-focus")
            compose.onAllNodesWithText("Movies")[0].performScrollTo().activate()
            waitText("New Film")
            compose.onNodeWithText("New Film").activate()
        } else compose.onNodeWithText("Details").performScrollTo().activate()
        waitText("Add to My List")
        capture("detail")
        compose.onNodeWithText("Add to My List").performScrollTo().activate()
        waitText("Remove from My List")
        assertEquals(1, fixture.listWrites)
        instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
        waitText(if (tv) "Movies" else "Watching")
        if (tv) compose.onNodeWithText("Home").activate()
        else compose.onNodeWithText("More").activate()
        if (!tv) {
            compose.onNodeWithText("Customize tabs").performScrollTo().activate()
            compose.onNodeWithText("Remove Movies").performScrollTo().activate()
            compose.onNodeWithText("Add Listen").performScrollTo().activate()
            compose.onNodeWithText("Done").performScrollTo().activate()
            compose.onNodeWithText("Listen").activate()
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
        compose.onNodeWithText("Try again").activate()
        waitText("Media added to your Server will appear here.")
        assertEquals(0, compose.onAllNodesWithContentDescription("Loading library").fetchSemanticsNodes().size)
        if (!tv) compose.onAllNodes(hasScrollAction())[0].performScrollToIndex(2)
        capture("empty")
        fixture.mode = "ready"
        if (tv) {
            compose.onNodeWithText("Movies").performScrollTo().activate()
            compose.onNodeWithText("Home").activate()
        } else {
            compose.onNodeWithText("Search").activate()
            compose.onNodeWithText("Home").activate()
        }
        waitText(if (tv) "Continue watching" else "Watching")
        capture("recovered")
    }

    @Test fun playbackChromeHidesTogetherAndReturnsWithInput() = journey { fixture ->
        waitText(if (tv) "Continue watching" else "Watching")
        if (tv) compose.onAllNodesWithContentDescription("Continuing Film")[0].activate()
        else compose.onNodeWithText("Resume").performScrollTo().activate()
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
        if (tv) compose.onNodeWithText("TV Shows").performScrollTo().activate()
        else compose.onNodeWithText("TV Shows").activate()
        waitText("Fixture Series")
        compose.onNodeWithText("Fixture Series").activate()
        compose.waitUntil(20_000) { compose.onAllNodesWithContentDescription("Loading library").fetchSemanticsNodes().isNotEmpty() }
        capture("show-pending")
        fixture.showMode = "failed"
        waitText("Could not load this show. Try again.")
        capture("show-failed")
        fixture.showMode = "empty"
        compose.onNodeWithText("Try again").activate()
        waitText("This show has no available episodes.")
        capture("show-empty")
        instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
        fixture.showMode = "ready"
        waitText("Fixture Series")
        compose.onNodeWithText("Fixture Series").activate()
        waitText("Resume · S1 E1")
        capture("show-loaded")
        compose.onAllNodes(hasScrollAction())[0].performScrollToIndex(if (tv) 2 else 3)
        compose.onNodeWithText("Season 2").activate()
        waitText("S2 E1 · Second episode")
        capture("show-season-two")
    }

    @Test fun catalogGridKeepsPendingFailedEmptyAndLoadedStatesDistinct() = journey { fixture ->
        waitText(if (tv) "Continue watching" else "Watching")
        fixture.mode = "pending"
        if (tv) compose.onNodeWithText("Movies").performScrollTo().activate()
        else compose.onNodeWithText("Movies").activate()
        compose.waitUntil(20_000) { compose.onAllNodesWithContentDescription("Loading library").fetchSemanticsNodes().isNotEmpty() }
        capture("catalog-pending")
        if (!tv) assertReadableHeading(compose.onNode(hasText("Movies") and !hasClickAction()))
        fixture.mode = "failed"
        waitText("Could not load your library. Try again.")
        capture("catalog-failed")
        fixture.mode = "empty"
        compose.onNodeWithText("Try again").activate()
        waitText("Nothing in your library yet.")
        capture("catalog-empty")
        fixture.mode = "ready"
        compose.onNodeWithText("Home").activate()
        waitText(if (tv) "Continue watching" else "Watching")
        if (tv) compose.onNodeWithText("Movies").performScrollTo().activate()
        else compose.onNodeWithText("Movies").activate()
        waitText("New Film")
        capture("catalog-loaded")
    }

    private fun SemanticsNodeInteraction.activate() {
        if (tv) {
            performSemanticsAction(SemanticsActions.RequestFocus) { it() }
            compose.waitForIdle()
            assertIsFocused()
            instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_DPAD_CENTER)
            compose.waitForIdle()
        } else performClick()
    }

    private fun journey(mode: String = "ready", check: (ParityFixture) -> Unit) {
        instrumentation.getUiAutomation()
        ParityFixture(mode).use { fixture ->
            context.getSharedPreferences("kinosail_session", 0).edit().clear().commit()
            context.getSharedPreferences("kinosail_tabs", 0).edit().clear().commit()
            context.getSharedPreferences("kinosail_reading_pending", 0).edit().clear().commit()
            SessionStore(context).save(SavedSession(ServerAddress("http://127.0.0.1:${fixture.port}"), "synthetic-token", fixture.viewer))
            val intent = Intent(context, if (tv) TvActivity::class.java else MobileActivity::class.java)
            try { ActivityScenario.launch<android.app.Activity>(intent).use {
                try { check(fixture) } catch (error: Throwable) {
                    runCatching { capture("diagnostic") }
                    throw error
                }
            } }
            finally { SessionStore(context).clear() }
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
        instrumentation.getUiAutomation().takeScreenshot().let { bitmap ->
            File(directory, "$prefix-$name.png").outputStream().use { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }
            if (name == "epub-loaded") {
                val done = compose.onNodeWithText("Done").fetchSemanticsNode().boundsInRoot
                var foreground = 0
                for (y in done.top.toInt() until done.bottom.toInt()) for (x in done.left.toInt() until done.right.toInt()) {
                    if (Color.green(bitmap.getPixel(x, y)) > 150) foreground++
                }
                assertTrue("The rendered chapter must not cover Done", foreground > 10)
            }
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
    @Volatile var collectionMode = "ready"
    @Volatile var collectionDetailReads = 0
    @Volatile var bookMode = "ready"
    @Volatile var chapterReads = 0
    @Volatile var escapedResourceReads = 0
    @Volatile var readerPage = 1
    @Volatile var failReaderSaves = false
    @Volatile var holdReaderSave = false
    @Volatile var readerSaveInFlight = false
    @Volatile var readerWrites = 0
    @Volatile var listWrites = 0
    private var listed = false
    private val movie = item("film", "Continuing Film", progress = 90)
    private val newMovie = item("new-film", "New Film")
    private val nextMovie = item("next-film", "Next Film", progress = 30)
        .replace("\"backdrop\":\"/backdrop/next-film\"", "\"backdrop\":\"\"")
    private val series = item("0123456789abcdef", "Fixture Series", kind = "show")
        .replace("\"genres\"", "\"showId\":\"0123456789abcdef\",\"genres\"")
    private val episodes = listOf(item("episode-one", "First episode", progress = 12), item("episode-two", "Second episode"))
        .mapIndexed { index, item -> item.replace("\"genres\"", "\"showId\":\"0123456789abcdef\",\"season\":${index+1},\"episode\":1,\"genres\"") }
    private val song = item("song", "Night Drive", kind = "music", progress = 45)
    private val book = item("book", "A quiet crossing", kind = "book")
    init {
        workers.execute {
            while (running.get()) try {
                val socket = server.accept()
                workers.execute {
                    try { socket.use {
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
                            path == "/api/v1/collections" -> {
                                while (collectionMode == "pending" && running.get()) Thread.sleep(20)
                                when (collectionMode) {
                                    "failed" -> { code = 503; "{}" }
                                    "invalid" -> """{"collections":["../private"]}"""
                                    "empty" -> """{"collections":[]}"""
                                    else -> """{"collections":["Family & friends"]}"""
                                }.toByteArray()
                            }
                            path.startsWith("/api/v1/collections/") -> {
                                collectionDetailReads++
                                assertEquals("/api/v1/collections/Family%20%26%20friends", uri.rawPath)
                                """{"name":"Family & friends","items":[$newMovie,$movie]}""".toByteArray()
                            }
                            path == "/api/v1/me" -> """{"server":"Fixture library","serverId":"parity-fixture","viewer":{"id":"viewer","name":"Alex"}}""".toByteArray()
                            path == "/api/v1/library" -> {
                                while (mode == "pending" && running.get()) Thread.sleep(20)
                                if (mode == "failed") { code = 503; "{}".toByteArray() }
                                else {
                                    val query = uri.query.split('&').associate { val p = it.split('=', limit = 2); p[0] to p.getOrElse(1) { "" } }
                                    val view = query["view"] ?: "all"
                                    val items = if (mode == "empty") emptyList() else when (view) {
                                        "history" -> listOf(movie, nextMovie, song)
                                        "movies" -> listOf(newMovie, movie)
                                        "shows" -> listOf(series)
                                        "music" -> listOf(song)
                                        "books" -> listOf(book)
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
                            path == "/api/v1/books/book/reader" -> {
                                val first = if (bookMode == "invalid") "/read/book/asset/../../private" else "/read/book/asset/first.xhtml"
                                if (bookMode == "empty") """{"id":"book","title":"A quiet crossing","type":"epub","pages":[]}""".toByteArray()
                                else """{"id":"book","title":"A quiet crossing","type":"epub","pages":[{"number":1,"title":"Departure","url":"$first"},{"number":2,"title":"Arrival","url":"/read/book/asset/second.xhtml"}]}""".toByteArray()
                            }
                            path == "/api/v1/books/book/reader/progress" -> {
                                if (parts[0] == "PUT") {
                                    readerSaveInFlight = true
                                    while (holdReaderSave && running.get()) Thread.sleep(20)
                                }
                                if (parts[0] == "PUT" && failReaderSaves) { code = 503; "{}".toByteArray() }
                                else {
                                    if (parts[0] == "PUT") { readerPage = JSONObject(requestBody).getInt("page"); readerWrites++ }
                                    """{"page":$readerPage,"total":2,"offset":0}""".toByteArray()
                                }
                            }
                            path.startsWith("/read/book/asset/") -> {
                                chapterReads++
                                while (bookMode == "pending" && running.get()) Thread.sleep(20)
                                type = "application/xhtml+xml"
                                if (bookMode == "failed") { code = 503; "unavailable".toByteArray() }
                                else ("<html><body><h1>" + if (path.endsWith("first.xhtml")) "Departure" else "Harbor lights")
                                    .let { heading -> (heading + "</h1><p>A synthetic chapter for a private reading journey.</p>" +
                                        "<script>document.body.innerHTML='Script ran';fetch('/escape')</script>" +
                                        "<img src='/read/other/asset/private.png'><img src='http://127.0.0.1:$port/escape'>" +
                                        "</body></html>").toByteArray() }
                            }
                            path == "/escape" || path.startsWith("/read/other/") -> {
                                escapedResourceReads++; "blocked".toByteArray()
                            }
                            path.startsWith("/api/v1/items/") && path.endsWith("/list") -> {
                                listed = JSONObject(requestBody).getBoolean("listed"); listWrites++
                                """{"listed":$listed}""".toByteArray()
                            }
                            path.endsWith("/playback-preferences") -> """{"playback":{"audioTrack":"","subtitleTrack":"","rate":1,"audioLanguage":"auto","subtitleLanguage":"off","nightMode":false,"dialogueBoost":false,"volumeBoost":1},"overridden":false}""".toByteArray()
                            path.endsWith("/progress/sync") -> JSONObject(requestBody).getJSONObject("progress").toString().toByteArray()
                            path.endsWith("/playback") -> """{"policy":"automatic","plan":{"allowed":true,"mode":"direct","reason":"direct-preferred"},"directAllowed":true,"direct":"/media/film","directType":"video/mp4","duration":20,"start":0}""".toByteArray()
                            path == "/media/film" -> {
                                type = "video/mp4"
                                InstrumentationRegistry.getInstrumentation().context.assets.open("parity-video.mp4").use { it.readBytes() }
                            }
                            path.startsWith("/api/v1/items/") -> {
                                val selected = if (path.endsWith("new-film")) newMovie else if (path.endsWith("/book")) book else movie
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
                    } } catch (_: InterruptedException) {
                        Thread.currentThread().interrupt()
                    } catch (_: java.io.IOException) {
                        // A cancelled request or fixture shutdown can close the client socket.
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
