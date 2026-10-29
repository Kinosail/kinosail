package com.kinosail.player.core

import android.content.Intent
import android.graphics.Bitmap
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.Paint
import android.os.Build
import android.view.KeyEvent
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.semantics.SemanticsProperties
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
    private var automationFlags = 0

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
        if (InstrumentationRegistry.getArguments().getString("reader-light") == "true") {
            compose.onNodeWithText("Appearance").activate()
            compose.onNodeWithText("Light page").activate()
        }
        fixture.bookMode = "empty"
        compose.onNodeWithText("Try again").activate()
        waitText("Could not open this book. Check your Server connection and try again.")
        assertEquals(0, fixture.chapterReads)
        compose.onNodeWithContentDescription("Opening your book").assertDoesNotExist()
        capture("epub-empty")
        fixture.bookMode = "pending"
        compose.onNodeWithText("Try again").activate()
        compose.waitUntil(20_000) { fixture.chapterReads > 0 }
        val pendingColumn = compose.onNodeWithContentDescription("Opening your book").fetchSemanticsNode().boundsInRoot
        capture("epub-pending")
        fixture.bookMode = "failed"
        waitText("Could not load this chapter. Try again.")
        compose.onNodeWithContentDescription("Opening your book").assertDoesNotExist()
        capture("epub-failed")
        fixture.bookMode = "ready"
        compose.onNodeWithText("Try again").activate()
        waitText("Chapter 1 of 2")
        waitChapter("Departure")
        capture("epub-loaded")
        fun chapterBounds(node: android.view.accessibility.AccessibilityNodeInfo?): android.graphics.Rect? {
            if (node == null) return null
            if (node.text?.toString() == "Departure") return android.graphics.Rect().also(node::getBoundsInScreen)
            for (index in 0 until node.childCount) chapterBounds(node.getChild(index))?.let { return it }
            return null
        }
        val loadedColumn = requireNotNull(chapterBounds(uiAutomation().rootInActiveWindow))
        val width = compose.onRoot().fetchSemanticsNode().boundsInRoot.width
        val density = context.resources.displayMetrics.density
        val geometry = JSONObject().put("pendingLeft", pendingColumn.left).put("pendingRight", pendingColumn.right)
            .put("loadedLeft", loadedColumn.left).put("windowWidth", width).put("density", density)
            .put("fontScale", context.resources.configuration.fontScale)
        var horizontalOverflow = false
        instrumentation.runOnMainSync {
            val web = chapterWebView()
            horizontalOverflow = web.canScrollHorizontally(1) || web.canScrollHorizontally(-1)
            geometry.put("webTextZoom", web.settings.textZoom).put("horizontalOverflow", horizontalOverflow)
        }
        File(context.filesDir, "parity-evidence/epub-column-geometry.json").writeText(geometry.toString())
        assertFalse("Large-text EPUB chapters must reflow inside the reading viewport", horizontalOverflow)
        assertEquals("Pending EPUB lines must reserve the loaded reading column's left edge", loadedColumn.left.toFloat(), pendingColumn.left, density * 2)
        assertEquals("Pending EPUB lines must reserve the loaded reading column's right edge", width - loadedColumn.left, pendingColumn.right, density * 2)
        compose.onNodeWithContentDescription("Opening your book").assertDoesNotExist()
        if (Build.VERSION.SDK_INT >= 29) {
            instrumentation.runOnMainSync {
                val web = chapterWebView()
                assertTrue(requireNotNull(web.webViewRenderProcess).terminate())
            }
            waitText("Could not load this chapter. Try again.")
            capture("epub-renderer-stopped")
            compose.onNodeWithText("Try again").activate()
            waitChapter("Departure")
        }
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
        fixture.bookMode = "failed"
        compose.onNodeWithText("Next").activate()
        waitText("Could not load this chapter. Try again.")
        waitText("Reading position was not saved.")
        fixture.failReaderSaves = false
        compose.onNodeWithText("Retry sync").activate()
        compose.waitUntil(20_000) { fixture.readerPage == 1 }
        compose.onNodeWithText("Done").activate()
        assertEquals("A failed chapter must not advance saved progress", 1, fixture.readerPage)
    }

    private fun chapterWebView(): android.webkit.WebView {
        val activity = androidx.test.runner.lifecycle.ActivityLifecycleMonitorRegistry.getInstance()
            .getActivitiesInStage(androidx.test.runner.lifecycle.Stage.RESUMED).first()
        fun find(view: android.view.View): android.webkit.WebView? {
            if (view is android.webkit.WebView) return view
            if (view is android.view.ViewGroup) for (index in 0 until view.childCount) find(view.getChildAt(index))?.let { return it }
            return null
        }
        return requireNotNull(find(activity.window.decorView))
    }

    private fun waitChapter(text: String) {
        compose.waitUntil(20_000) { compose.onAllNodesWithContentDescription("Opening your book").fetchSemanticsNodes().isEmpty() }
        val automation = instrumentation.getUiAutomation()
        fun contains(node: android.view.accessibility.AccessibilityNodeInfo?): Boolean {
            if (node == null) return false
            if (node.text?.toString() == text) return true
            return (0 until node.childCount).any { contains(node.getChild(it)) }
        }
        try { compose.waitUntil(20_000) { contains(automation.rootInActiveWindow) } }
        catch (failure: Throwable) {
            runCatching { capture("epub-chapter-missing") }
            throw failure
        }
    }

    @Test fun collectionsRecoverAndReturnFromDetailsWithoutLosingTheirPlace() = journey { fixture ->
        waitText(if (tv) "Continue watching" else "Watching")
        fixture.collectionMode = "pending"
        if (!tv) compose.onNodeWithText("More").activate()
        if (tv) {
            compose.onNodeWithText("Browse").performScrollTo()
            compose.waitForIdle()
            compose.onNode(hasScrollToNodeAction() and hasAnyChild(hasContentDescription("Movies")))
                .performScrollToIndex(6)
            compose.waitForIdle()
        }
        compose.onNodeWithText("Collections").performScrollTo().activate()
        compose.waitUntil(20_000) { compose.onAllNodesWithContentDescription("Loading collections").fetchSemanticsNodes().isNotEmpty() }
        val pendingCollections = compose.onNodeWithContentDescription("Loading collections").fetchSemanticsNode().boundsInRoot
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
            val loadedCollection = compose.onNodeWithText("Family & friends").fetchSemanticsNode().boundsInRoot
            assertEquals("Pending collections must reserve the loaded row's left edge", pendingCollections.left, loadedCollection.left, 1f)
            assertEquals("Pending collections must reserve the loaded row's top edge", pendingCollections.top, loadedCollection.top, 1f)
            assertEquals("Pending collections must reserve the loaded row's width", pendingCollections.width, loadedCollection.width, 1f)
            compose.onNodeWithText("Family & friends").performSemanticsAction(SemanticsActions.RequestFocus) { it() }
            capture("collections-focused")
        }
        compose.onNodeWithText("Family & friends").activate()
        waitText("New Film")
        capture("collection-titles")
        compose.onNodeWithText("New Film").activate()
        waitFor(myList(false))
        instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
        compose.waitUntil(20_000) { compose.onAllNodes(hasText("New Film") and hasClickAction()).fetchSemanticsNodes().isNotEmpty() }
        assertEquals(1, fixture.collectionDetailReads)
        instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
        waitText("Family & friends")
    }

    @Test fun homeMatchesAppleLandscapeShelvesAndPersistentTvNavigation() = journey {
        waitText(if (tv) "Continue watching" else "Watching")
        if (!tv) {
            val width = context.resources.displayMetrics.widthPixels
            val title = compose.onNodeWithText("Kinosail").fetchSemanticsNode().boundsInRoot
            assertEquals("Home title must be centered, as on iPhone", width / 2f, title.center.x, 2f)
            compose.onNode(hasText("Listen") and hasClickAction()).assertIsDisplayed()
            compose.onNode(hasText("My List") and hasClickAction()).assertDoesNotExist()
            compose.onNode(hasText("PG · Drama · Adventure") and !hasAnyAncestor(hasContentDescription("Next Film")),
                useUnmergedTree = true).assertExists()
            compose.onNode(hasText("Resume") and hasClickAction()).assertIsDisplayed()
            compose.onAllNodes(hasScrollAction())[0].performScrollToNode(hasText("Browse library"))
            compose.onAllNodes(hasText("Movies") and hasClickAction()).assertCountEquals(2)
            compose.onAllNodes(hasText("TV Shows") and hasClickAction()).assertCountEquals(2)
            compose.onAllNodes(hasText("Photos") and hasClickAction()).assertCountEquals(0)
            compose.onAllNodes(hasText("Music") and hasClickAction()).assertCountEquals(0)
            compose.onAllNodesWithText("Movie genres").assertCountEquals(0)
            compose.onAllNodes(hasScrollAction())[0].performScrollToIndex(0)
            compose.onNodeWithContentDescription("My List").assertIsDisplayed()
            waitText("89 min left")
            compose.onNode(SemanticsMatcher.keyIsDefined(SemanticsProperties.ProgressBarRangeInfo) and !hasAnyAncestor(hasClickAction()),
                useUnmergedTree = true).assertIsDisplayed()
            capture("home-header")
        } else {
            val hero = { text: String -> compose.onNode(hasText(text) and !hasAnyAncestor(hasClickAction()), useUnmergedTree = true) }
            hero("Continuing Film").assertIsDisplayed()
            hero("2026 · PG · Drama · Adventure").assertIsDisplayed()
            hero("A synthetic title for a repeatable Android journey.").assertIsDisplayed()
            compose.onNodeWithContentDescription("Next Film").performSemanticsAction(SemanticsActions.RequestFocus)
            compose.waitUntil(5_000) { compose.onAllNodes(hasText("Next Film") and !hasAnyAncestor(hasClickAction()),
                useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty() }
            compose.waitForIdle()
            hero("Next Film").assertIsDisplayed()
            hero("2026 · PG · Drama · Adventure").assertIsDisplayed()
            capture("home-hero-focus")
        }
        compose.onNodeWithText("Continue watching").performScrollTo()
        val card = compose.onNodeWithContentDescription("Next Film")
        card.performScrollTo().assertIsDisplayed()
        val size = card.fetchSemanticsNode().size
        val wide = homeCardWidth(tv, landscape = true).value * context.resources.displayMetrics.density
        assertTrue("Landscape fallback must keep a wide card: $size", size.width >= wide - 1)
        if (!tv) {
            val heading = compose.onNode(hasText("Continue watching") and !hasClickAction()).fetchSemanticsNode().boundsInRoot
            val bounds = card.fetchSemanticsNode().boundsInRoot
            assertEquals("Shelf cards must start at their heading edge, as on iPhone", heading.left, bounds.left, 1f)
            val title = compose.onNode(hasText("Next Film") and hasAnyAncestor(hasContentDescription("Next Film")),
                useUnmergedTree = true).fetchSemanticsNode().boundsInRoot
            assertEquals("Card titles must align with their artwork, as on iPhone", bounds.left, title.left, 1f)
        }
        compose.onNode(hasText("PG · Drama · Adventure") and hasAnyAncestor(hasContentDescription("Next Film")),
            useUnmergedTree = true).assertDoesNotExist()
        compose.onNode(hasText("Resume at 0:30") and hasAnyAncestor(hasContentDescription("Next Film")),
            useUnmergedTree = true).assertDoesNotExist()
        compose.waitUntil(20_000) { compose.onAllNodes(SemanticsMatcher.keyIsDefined(SemanticsProperties.ProgressBarRangeInfo) and
            hasAnyAncestor(hasContentDescription("Next Film")), useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty() }
        compose.onAllNodes(hasText("60 min left") and hasAnyAncestor(hasContentDescription("Next Film")), useUnmergedTree = true)
            .assertCountEquals(if (tv) 1 else 0)
        capture("home-continuation")
        if (tv) {
            val screen = context.resources.displayMetrics.widthPixels.toFloat()
            assertTrue("TV landscape cards must match Apple TV proportions: ${size.width}/$screen", size.width <= screen * 0.21f)
            compose.onAllNodes(hasScrollAction())[0].performScrollToNode(hasContentDescription("New Film"))
            val poster = compose.onAllNodesWithContentDescription("New Film")[0].fetchSemanticsNode().size
            assertTrue("TV posters must match Apple TV proportions: ${poster.width}/$screen", poster.width <= screen * 0.13f)
            compose.onAllNodesWithText("Kinosail").assertCountEquals(0)
            compose.onNode(hasText("Browse") and !hasClickAction()).assertExists()
            compose.onAllNodes(hasScrollAction())[0].performScrollToNode(hasText("Unwatched movies"))
            compose.onNodeWithText("Search").assertIsDisplayed().activate()
            waitText("Search library")
            instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
            compose.waitUntil(10_000) { compose.onAllNodes(hasText("Search") and hasClickAction() and isFocused()).fetchSemanticsNodes().isNotEmpty() }
            compose.onNodeWithText("Unwatched movies").assertIsDisplayed()
            compose.onNodeWithContentDescription("Settings").assertIsDisplayed().activate()
            waitText("Sign out")
            instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
            compose.waitUntil(10_000) { compose.onAllNodes(hasContentDescription("Settings") and isFocused()).fetchSemanticsNodes().isNotEmpty() }
            compose.onNodeWithText("Unwatched movies").assertIsDisplayed()
            capture("home-navigation-return")
        }
    }

    @Test fun homeProgressIgnoresInvalidWatchSummary() = journey(prepare = {
        it.progressBody = """{"seconds":9000,"duration":5400}"""
    }) { fixture ->
        org.junit.Assume.assumeFalse(tv)
        waitText("Watching")
        compose.waitUntil(20_000) { fixture.progressRequests > 0 }
        Thread.sleep(500)
        compose.waitForIdle()
        compose.onAllNodes(hasText("min left", substring = true)).assertCountEquals(0)
        compose.onAllNodes(SemanticsMatcher.keyIsDefined(SemanticsProperties.ProgressBarRangeInfo) and
            !hasAnyAncestor(hasContentDescription("Next Film")), useUnmergedTree = true).assertCountEquals(0)
        compose.onAllNodesWithText("Resume at 1:30")[0].assertIsDisplayed()
        capture("home-progress-rejected")
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
        waitFor(myList(false))
        capture("detail")
        compose.onNode(myList(false)).performScrollTo().activate()
        waitFor(myList(true))
        assertEquals(1, fixture.listWrites)
        instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
        waitText(if (tv) "Movies" else "Watching")
        if (tv) instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
        else compose.onNodeWithText("More").activate()
        if (!tv) {
            waitText("Music")
            val row = compose.onNode(hasText("Music") and hasClickAction()).fetchSemanticsNode().boundsInRoot
            val density = context.resources.displayMetrics.density
            assertTrue("More destinations must be full-width list rows, as on iPhone: $row",
                row.height >= 56 * density && row.width >= 300 * density)
            capture("more")
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
        compose.onNodeWithText("Your library is ready").assertExists()
        assertEquals(0, compose.onAllNodesWithContentDescription("Loading library").fetchSemanticsNodes().size)
        if (!tv) compose.onAllNodes(hasScrollAction())[0].performScrollToIndex(2)
        capture("empty")
        fixture.mode = "ready"
        if (tv) {
            compose.onNodeWithText("Movies").performScrollTo().activate()
            instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
        } else {
            compose.onNodeWithText("Search").activate()
            compose.onNodeWithText("Home").activate()
        }
        waitText(if (tv) "Continue watching" else "Watching")
        capture("recovered")
    }

    @Test fun playbackChromeHidesTogetherAndReturnsWithInput() = journey(accessibility = false) { fixture ->
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
        if (tv) instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
        else compose.onNodeWithContentDescription("Close player").activate()
        waitText(if (tv) "Continue watching" else "Watching")
    }

    @Test fun playbackChromeRemainsVisibleToAccessibilityServices() = journey {
        waitText(if (tv) "Continue watching" else "Watching")
        if (tv) compose.onAllNodesWithContentDescription("Continuing Film")[0].activate()
        else compose.onNodeWithText("Resume").performScrollTo().activate()
        waitText("Speed 1×")
        Thread.sleep(6000)
        capture("playback-accessibility")
        compose.onNodeWithText("Speed 1×").assertIsDisplayed()
        compose.onNodeWithText("Continuing Film").assertIsDisplayed()
        fun labels(node: android.view.accessibility.AccessibilityNodeInfo?): List<String> = if (node == null) emptyList()
            else listOfNotNull(node.contentDescription?.toString()) + (0 until node.childCount).flatMap { labels(node.getChild(it)) }
        var shown = emptyList<String>()
        if ("Show player controls" in labels(uiAutomation().rootInActiveWindow)) {
            if (tv) instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_DPAD_CENTER)
            else compose.onRoot().performTouchInput { click(center) }
        }
        runCatching { compose.waitUntil(10_000) {
            shown = labels(uiAutomation().rootInActiveWindow)
            "Pause" in shown || "Play" in shown
        } }
        assertTrue("Playback controls must be readable: $shown", "Pause" in shown || "Play" in shown)
        assertTrue("Playback must not show Previous or Next track buttons, as on Apple clients: $shown",
            "Previous" !in shown && "Next" !in shown)
        assertTrue("Playback must skip ten seconds each way, as on Apple clients: $shown",
            "Rewind 10 seconds" in shown && "Fast forward 10 seconds" in shown)
        assertTrue("Speed and tracks have one control each, so the player's own settings menu stays hidden: $shown",
            "Settings" !in shown)
        assertEquals(0, compose.onAllNodesWithText("Done").fetchSemanticsNodes().size)
        assertEquals(0, compose.onAllNodesWithText("Google Cast", substring = true).fetchSemanticsNodes().size)
        assertEquals(if (tv) 0 else 1, compose.onAllNodesWithContentDescription("Close player").fetchSemanticsNodes().size)
        compose.onNodeWithText("Continuing Film").assertIsDisplayed()
        compose.onNodeWithText("Speed 1×").activate()
        waitText("Playback speed")
        compose.onNode(hasText("Normal") and isSelected()).assertIsDisplayed()
        assertEquals(1, compose.onAllNodes(isSelected()).fetchSemanticsNodes().size)
        capture("playback-speed-picker")
        compose.onNodeWithText("1.5×").activate()
        waitText("Speed 1.5×")
        compose.waitUntil(5_000) { compose.onAllNodesWithText("Playback speed").fetchSemanticsNodes().isEmpty() }
        compose.onNodeWithText("Speed 1.5×").activate()
        waitText("Playback speed")
        compose.onNode(hasText("1.5×") and isSelected()).assertIsDisplayed()
        compose.onNodeWithText("Normal").activate()
        waitText("Speed 1×")
        compose.waitUntil(5_000) { compose.onAllNodesWithText("Playback speed").fetchSemanticsNodes().isEmpty() }
        instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
        waitText(if (tv) "Continue watching" else "Watching")
    }

    @Test fun episodeMarkersOfferSkipThenNextEpisodeAsOnAppleClients() = journey(prepare = {
        it.episodeMarkers = ""","markers":[{"type":"intro","start":1,"end":12},{"type":"credits","start":15,"end":20}],"autoSkip":[],"next":"episode-two""""
    }) { fixture ->
        playFirstEpisode(fixture)
        waitText("Skip Intro")
        compose.onAllNodesWithText("Next episode").assertCountEquals(0)
        compose.onNodeWithTag("playback-prompt-skip", useUnmergedTree = true).assertIsDisplayed()
        capture("playback-skip-intro")
        compose.onNodeWithText("Skip Intro").activate()
        compose.waitUntil(20_000) { compose.onAllNodesWithText("Skip Intro").fetchSemanticsNodes().isEmpty() }
        compose.waitUntil(20_000) { compose.onAllNodesWithText("Next Episode").fetchSemanticsNodes().isNotEmpty() }
        compose.onNodeWithTag("playback-prompt-next", useUnmergedTree = true).assertIsDisplayed()
        capture("playback-next-episode")
        assertEquals(listOf("episode-one"), fixture.playbackReads.toList())
        compose.waitUntil(30_000) { "episode-two" in fixture.playbackReads }
        waitText("Second episode")
        compose.onAllNodesWithText("Skip Intro").assertCountEquals(0)
        instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
    }

    @Test fun automaticSkipsPassMarkersAndCreditsStartTheNextEpisode() = journey(prepare = {
        it.episodeMarkers = ""","markers":[{"type":"intro","start":0.5,"end":9},{"type":"credits","start":12,"end":20}],"autoSkip":["intro","credits"],"next":"episode-two""""
    }) { fixture ->
        playFirstEpisode(fixture)
        var prompted = false
        compose.waitUntil(30_000) {
            prompted = prompted || compose.onAllNodesWithText("Skip Intro").fetchSemanticsNodes().isNotEmpty() ||
                compose.onAllNodesWithText("Next Episode").fetchSemanticsNodes().isNotEmpty()
            "episode-two" in fixture.playbackReads
        }
        assertFalse("Markers set to skip automatically must not ask the viewer", prompted)
        waitText("Second episode")
        capture("playback-auto-next")
        instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
    }

    @Test fun tvLibraryDetailsAndShowsFollowAppleTvLayout() = journey {
        org.junit.Assume.assumeTrue(tv)
        val screen = context.resources.displayMetrics.widthPixels.toFloat()
        waitText("Continue watching")
        compose.onNodeWithText("Movies").performScrollTo().activate()
        waitText("New Film")
        compose.onAllNodes(hasText("Home") and hasClickAction()).assertCountEquals(0)
        compose.onAllNodes(hasText("Settings") and hasClickAction()).assertCountEquals(0)
        compose.onAllNodes(hasTestTag("unwatched") and hasAnyAncestor(hasContentDescription("New Film")), true).assertCountEquals(1)
        compose.onAllNodes(hasTestTag("unwatched") and hasAnyAncestor(hasContentDescription("Continuing Film")), true).assertCountEquals(0)
        capture("library-header")
        compose.onNodeWithContentDescription("New Film").activate()
        waitFor(myList(false))
        val backdrop = compose.onNodeWithTag("tv-backdrop", true).fetchSemanticsNode().boundsInRoot
        assertEquals("Details use a full-bleed backdrop, as on Apple TV", 0f, backdrop.left, 1f)
        assertEquals("Details use a full-bleed backdrop, as on Apple TV", screen, backdrop.width, 1f)
        compose.onAllNodes(hasText("Back") and hasClickAction()).assertCountEquals(0)
        compose.onAllNodesWithText("Kinosail").assertCountEquals(0)
        compose.waitUntil(5_000) { compose.onAllNodes(hasText("Play") and hasClickAction() and isFocused()).fetchSemanticsNodes().isNotEmpty() }
        capture("detail-backdrop")
        instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
        compose.waitUntil(10_000) { compose.onAllNodes(hasContentDescription("New Film") and isFocused()).fetchSemanticsNodes().isNotEmpty() }
        capture("library-focus-restored")
        instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
        waitText("Continue watching")
        compose.onNodeWithText("TV Shows").performScrollTo().activate()
        waitText("Fixture Series")
        compose.onNodeWithText("Fixture Series").activate()
        waitText("Resume · S1 E1")
        compose.waitUntil(5_000) { compose.onAllNodes(hasText("Resume · S1 E1") and hasClickAction() and isFocused()).fetchSemanticsNodes().isNotEmpty() }
        compose.waitForIdle()
        val showTitle = compose.onNode(hasText("Fixture Series") and isHeading()).fetchSemanticsNode().boundsInRoot
        assertTrue("The focused show hero keeps its title on screen, as on Apple TV: $showTitle", showTitle.top >= 24 * context.resources.displayMetrics.density)
        capture("show-hero-focus")
        compose.onAllNodes(hasText("Back to Library") and hasClickAction()).assertCountEquals(0)
        val showBackdrop = compose.onNodeWithTag("tv-backdrop", true).fetchSemanticsNode().boundsInRoot
        assertEquals("Show pages use a full-bleed backdrop, as on Apple TV", screen, showBackdrop.width, 1f)
        compose.onAllNodes(hasScrollAction())[0].performScrollToNode(hasText("Episodes"))
        val first = compose.onNode(hasText("Season 1") and hasClickAction()).fetchSemanticsNode().boundsInRoot
        val second = compose.onNode(hasText("Season 2") and hasClickAction()).fetchSemanticsNode().boundsInRoot
        assertEquals("Seasons sit in one row, as on Apple TV", first.top, second.top, 1f)
        assertTrue("Seasons sit in one row, as on Apple TV", second.left > first.right)
        capture("show-backdrop")
    }

    @Test fun tvHomeStartsOnContinueWatchingAndBackReachesSearch() = journey {
        org.junit.Assume.assumeTrue(tv)
        waitText("Continue watching")
        compose.waitUntil(10_000) { compose.onAllNodes(hasContentDescription("Continuing Film") and isFocused()).fetchSemanticsNodes().isNotEmpty() }
        compose.onAllNodesWithText("Movie genres").assertCountEquals(0)
        capture("home-launch-focus")
        instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
        compose.waitUntil(10_000) { compose.onAllNodes(hasText("Search") and hasClickAction() and isFocused()).fetchSemanticsNodes().isNotEmpty() }
        capture("home-back-search")
        compose.onAllNodes(hasScrollAction())[0].performScrollToNode(hasText("Recently added movies"))
        compose.onAllNodes(hasContentDescription("New Film") and hasAnyAncestor(hasScrollToIndexAction()))[0].activate()
        waitFor(myList(false))
        instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
        compose.waitUntil(10_000) { compose.onAllNodes(hasContentDescription("New Film") and isFocused()).fetchSemanticsNodes().isNotEmpty() }
        compose.onAllNodes(hasContentDescription("New Film") and isFocused())[0].assertIsDisplayed()
        capture("home-focus-restored")
    }

    private fun playFirstEpisode(fixture: ParityFixture) {
        waitText(if (tv) "Continue watching" else "Watching")
        if (tv) compose.onNodeWithText("TV Shows").performScrollTo().activate()
        else compose.onNodeWithText("TV Shows").activate()
        waitText("Fixture Series")
        compose.onNodeWithText("Fixture Series").activate()
        waitText("Resume · S1 E1")
        compose.onNodeWithText("Resume · S1 E1").performScrollTo().activate()
        compose.waitUntil(20_000) { "episode-one" in fixture.playbackReads }
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
        if (tv) compose.waitUntil(10_000) { compose.onAllNodes(hasText("Try again") and isFocused()).fetchSemanticsNodes().isNotEmpty() }
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
        waitText(if (tv) "Second episode" else "S2 E1 · Second episode")
        capture("show-season-two")
    }

    @Test fun catalogGridKeepsPendingFailedEmptyAndLoadedStatesDistinct() = journey { fixture ->
        waitText(if (tv) "Continue watching" else "Watching")
        fixture.mode = "pending"
        if (tv) {
            compose.onAllNodes(hasScrollAction())[0].performScrollToNode(hasText("Browse"))
            compose.onNodeWithText("Movies").performScrollTo().activate()
        }
        else compose.onNodeWithText("Movies").activate()
        compose.waitUntil(20_000) { compose.onAllNodesWithContentDescription("Loading library").fetchSemanticsNodes().isNotEmpty() }
        capture("catalog-pending")
        if (!tv && Build.VERSION.SDK_INT >= 26) assertReadableHeading(compose.onNode(hasText("Movies") and !hasClickAction()))
        fixture.mode = "failed"
        waitText("Could not load your library. Try again.")
        capture("catalog-failed")
        fixture.mode = "empty"
        compose.onNodeWithText("Try again").activate()
        waitText("Nothing in your library yet.")
        capture("catalog-empty")
        fixture.mode = "ready"
        if (tv) instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK) else compose.onNodeWithText("Home").activate()
        waitText(if (tv) "Continue watching" else "Watching")
        if (tv) {
            compose.onAllNodes(hasScrollAction())[0].performScrollToNode(hasText("Browse"))
            compose.onNodeWithText("Movies").performScrollTo().activate()
        }
        else compose.onNodeWithText("Movies").activate()
        waitText("New Film")
        if (!tv) {
            val first = compose.onNodeWithContentDescription("New Film").fetchSemanticsNode().boundsInRoot
            val second = compose.onNodeWithContentDescription("Continuing Film").fetchSemanticsNode().boundsInRoot
            assertEquals("Grid cards must share a row", first.top, second.top, 1f)
            assertTrue("Phone grids must use three iPhone-sized columns: $first",
                first.width < 130 * context.resources.displayMetrics.density)
            compose.onAllNodes(hasTestTag("unwatched") and hasAnyAncestor(hasContentDescription("New Film")), true).assertCountEquals(1)
            compose.onAllNodes(hasTestTag("unwatched") and hasAnyAncestor(hasContentDescription("Continuing Film")), true).assertCountEquals(0)
        }
        capture("catalog-loaded")
    }

    @Test fun settingsFollowIphoneSectionsAndSavePlaybackDefaults() = journey { fixture ->
        org.junit.Assume.assumeFalse(tv)
        waitText("Watching")
        compose.onNodeWithText("More").activate()
        compose.onNodeWithText("Settings").performScrollTo().activate()
        waitText("Server & Viewer Profile")
        compose.onNodeWithContentDescription("Viewer Profile: Alex. Server: Fixture library").assertExists()
        compose.onNodeWithText("Playback & library").performScrollTo().assertIsDisplayed()
        compose.onNodeWithText("Personalize").performScrollTo().assertIsDisplayed()
        compose.onNodeWithText("About").performScrollTo().assertIsDisplayed()
        capture("settings")
        fixture.preferencesMode = "pending"
        compose.onNodeWithText("Playback").performScrollTo().activate()
        compose.waitUntil(20_000) { compose.onAllNodesWithContentDescription("Loading preferences").fetchSemanticsNodes().isNotEmpty() }
        capture("playback-defaults-pending")
        fixture.preferencesMode = "failed"
        waitText("Could not load your playback preferences. Try again.")
        compose.onAllNodesWithContentDescription("Loading preferences").assertCountEquals(0)
        capture("playback-defaults-failed")
        fixture.preferencesMode = "ready"
        compose.onNodeWithText("Try again").activate()
        waitText("Boost Dialog")
        compose.onNode(hasText("Subtitles") and hasText("Off") and hasClickAction()).assertExists()
        compose.onNode(hasText("Audio language") and hasText("Automatic") and hasClickAction()).assertExists()
        capture("playback-defaults")
        compose.onNode(hasText("Boost Dialog") and hasClickAction()).performScrollTo().activate()
        compose.waitUntil(20_000) { fixture.preferenceWrites.size == 1 }
        val boosted = JSONObject(fixture.preferenceWrites[0])
        assertTrue(boosted.getJSONObject("playback").getBoolean("dialogueBoost"))
        assertEquals("off", boosted.getJSONObject("playback").getString("subtitleLanguage"))
        assertEquals(30, boosted.getInt("downloadLimitGiB"))
        assertEquals("dark", boosted.getString("readerTheme"))
        compose.onNode(hasText("Speed") and hasClickAction()).performScrollTo().activate()
        waitText("1.5×")
        compose.onNodeWithText("1.5×").activate()
        compose.waitUntil(20_000) { fixture.preferenceWrites.size == 2 }
        assertEquals(1.5, JSONObject(fixture.preferenceWrites[1]).getJSONObject("playback").getDouble("rate"), 0.0)
        compose.waitUntil(20_000) { compose.onAllNodes(hasText("Speed") and hasText("1.5×") and hasClickAction()).fetchSemanticsNodes().isNotEmpty() }
        capture("playback-defaults-saved")
        instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
        waitText("Server & Viewer Profile")
        compose.onNodeWithText("Sign out").performScrollTo().activate()
        waitText("Sign out of this device?")
        capture("sign-out-confirm")
        compose.onNodeWithText("Cancel").activate()
        waitText("Server & Viewer Profile")
        instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
        waitText("Customize tabs")
    }

    @Test fun connectATvReviewsTheCodeBeforeApproving() = journey { fixture ->
        org.junit.Assume.assumeFalse(tv)
        waitText("Watching")
        compose.onNodeWithText("More").activate()
        compose.onNodeWithText("Settings").performScrollTo().activate()
        compose.onNodeWithText("Connect a TV").performScrollTo().activate()
        waitText("Six-digit code")
        compose.onNodeWithText("Review device").assertIsNotEnabled()
        compose.onNode(hasSetTextAction()).performTextInput("111111")
        compose.onNodeWithText("Review device").activate()
        waitText("That code expired or was already used. Check the code on your TV.")
        assertEquals(0, fixture.approvals)
        compose.onNode(hasSetTextAction()).performTextReplacement("482913")
        compose.onNodeWithText("Review device").activate()
        waitText("Living room TV")
        compose.onNodeWithText("Approve only if this code matches the device you want to connect.").assertExists()
        assertEquals(0, fixture.approvals)
        capture("connect-tv-review")
        compose.onNodeWithText("Approve device").performScrollTo().activate()
        waitText("Device connected")
        assertEquals(1, fixture.approvals)
        capture("connect-tv-approved")
        compose.onNodeWithText("Done").activate()
        waitText("Server & Viewer Profile")
    }

    @Test fun detailRemovesTitleFromContinueWatching() = journey { fixture ->
        org.junit.Assume.assumeFalse(tv)
        waitText("Watching")
        compose.onNodeWithText("Details").performScrollTo().activate()
        waitFor(myList(false))
        compose.onNodeWithText("Remove from Continue watching").performScrollTo().activate()
        compose.waitUntil(20_000) { fixture.dismissals == 1 }
        waitText("Removed from Continue watching.")
        compose.onAllNodesWithText("Remove from Continue watching").assertCountEquals(0)
        capture("detail-continue-removed")
        fixture.mode = "pending"
        instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
        waitFor(hasText("Next Film") and isHeading())
        compose.onAllNodes(hasText("Continuing Film") and isHeading()).assertCountEquals(0)
        capture("home-continue-removed")
    }

    @Test fun showItemsWithoutShowIdOpenTheShowPage() = journey(prepare = { it.bareShow = true }) {
        waitText(if (tv) "Continue watching" else "Watching")
        if (tv) compose.onNodeWithText("TV Shows").performScrollTo().activate()
        else compose.onNodeWithText("TV Shows").activate()
        waitText("Fixture Series")
        compose.onNodeWithText("Fixture Series").activate()
        waitText("Resume · S1 E1")
        capture("bare-show")
    }

    @Test fun tvEpisodeSpotlightNamesTheShow() = journey(prepare = { it.episodeHistory = true }) {
        org.junit.Assume.assumeTrue(tv)
        waitText("Continue watching")
        compose.onNodeWithContentDescription("S01E01 · First episode").performSemanticsAction(SemanticsActions.RequestFocus)
        val hero = { text: String -> hasText(text) and !hasAnyAncestor(hasClickAction()) }
        compose.waitUntil(5_000) { compose.onAllNodes(hero("Fixture Series"), true).fetchSemanticsNodes().isNotEmpty() }
        compose.onNode(hero("Fixture Series"), true).assertIsDisplayed()
        compose.onNode(hero("S1 E1 · First episode · PG"), true).assertIsDisplayed()
        capture("home-episode-hero")
    }

    @Test fun tvSearchShowsResultsAsTheViewerTypes() = journey {
        org.junit.Assume.assumeTrue(tv)
        waitText("Continue watching")
        compose.onNodeWithText("Search").activate()
        compose.waitUntil(10_000) { compose.onAllNodes(hasSetTextAction() and isFocused()).fetchSemanticsNodes().isNotEmpty() }
        compose.onAllNodes(hasText("Search") and hasClickAction()).assertCountEquals(0)
        capture("search-focused")
        compose.onNode(hasSetTextAction()).performTextInput("New")
        compose.waitUntil(10_000) { compose.onAllNodes(hasText("“New”", substring = true)).fetchSemanticsNodes().isNotEmpty() }
        waitFor(hasContentDescription("New Film") and hasClickAction())
        capture("search-live-results")
        instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
        compose.waitUntil(10_000) { compose.onAllNodes(hasText("Search") and hasClickAction() and isFocused()).fetchSemanticsNodes().isNotEmpty() }
    }

    @Test fun tvSettingsFollowAppleTvGroupsAndAskBeforeSignOut() = journey { fixture ->
        org.junit.Assume.assumeTrue(tv)
        waitText("Continue watching")
        compose.onNodeWithContentDescription("Settings").activate()
        waitText("Server & Viewer Profile")
        compose.onNodeWithContentDescription("Viewer Profile: Alex. Server: Fixture library").assertExists()
        val brand = compose.onNode(hasText("Settings") and isHeading()).fetchSemanticsNode().boundsInRoot
        val group = compose.onNodeWithText("Server & Viewer Profile").fetchSemanticsNode().boundsInRoot
        assertTrue("Settings put the title beside the grouped list, as on Apple TV: $brand $group", brand.right <= group.left)
        compose.onNodeWithText("Playback & library").performScrollTo().assertIsDisplayed()
        compose.onNodeWithText("About").performScrollTo().assertIsDisplayed()
        capture("settings")
        compose.onNodeWithText("Playback").performScrollTo().activate()
        waitText("Boost Dialog")
        capture("settings-playback")
        instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
        waitText("Server & Viewer Profile")
        compose.onNodeWithText("Sign out").performScrollTo().activate()
        waitText("Sign out of this device?")
        capture("settings-sign-out")
        compose.onNodeWithText("Cancel").activate()
        compose.waitUntil(10_000) { compose.onAllNodesWithText("Sign out of this device?").fetchSemanticsNodes().isEmpty() }
        assertEquals(0, fixture.signOuts)
        instrumentation.sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
        compose.waitUntil(10_000) { compose.onAllNodes(hasContentDescription("Settings") and isFocused()).fetchSemanticsNodes().isNotEmpty() }
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

    private fun journey(mode: String = "ready", accessibility: Boolean = true, prepare: (ParityFixture) -> Unit = {},
                        check: (ParityFixture) -> Unit) {
        org.junit.Assume.assumeTrue("Disabling UiAutomation accessibility requires API 31", accessibility || Build.VERSION.SDK_INT >= 31)
        automationFlags = if (accessibility) 0 else android.app.UiAutomation.FLAG_DONT_USE_ACCESSIBILITY
        uiAutomation()
        val manager = context.getSystemService(android.content.Context.ACCESSIBILITY_SERVICE)
            as android.view.accessibility.AccessibilityManager
        compose.waitUntil(10_000) { manager.isEnabled == accessibility }
        ParityFixture(mode).use { fixture ->
            prepare(fixture)
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
    private fun myList(listed: Boolean) =
        hasText(if (listed) "In My List" else "My List") and hasClickAction() and !hasContentDescription("My List")
    private fun waitFor(matcher: SemanticsMatcher) {
        compose.waitUntil(20_000) { compose.onAllNodes(matcher).fetchSemanticsNodes().isNotEmpty() }
        compose.waitForIdle()
    }
    private fun waitText(text: String) {
        compose.waitUntil(20_000) { compose.onAllNodesWithText(text, ignoreCase = true).fetchSemanticsNodes().isNotEmpty() }
        compose.waitForIdle()
    }
    private fun capture(name: String) {
        compose.waitForIdle()
        instrumentation.waitForIdleSync()
        Thread.sleep(300)
        val prefix = InstrumentationRegistry.getArguments().getString("capture") ?: if (tv) "tv" else "phone"
        val directory = File(context.filesDir, "parity-evidence").apply { mkdirs() }
        File(directory, "$prefix-$name-semantics.txt").writeText(compose.onAllNodes(isRoot()).printToString())
        if (!tv && !name.startsWith("playback") && Build.VERSION.SDK_INT >= 34) {
            compose.enableAccessibilityChecks()
            compose.onRoot().tryPerformAccessibilityChecks()
            compose.disableAccessibilityChecks()
        }
        (uiAutomation().takeScreenshot() ?: run { Thread.sleep(1_000); uiAutomation().takeScreenshot() })?.let { bitmap ->
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

    private fun uiAutomation() = if (Build.VERSION.SDK_INT >= 31) {
        instrumentation.getUiAutomation(automationFlags)
    } else instrumentation.getUiAutomation()
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
    @Volatile var progressBody = """{"seconds":90,"duration":5400}"""
    @Volatile var progressRequests = 0
    @Volatile var readerSaveInFlight = false
    @Volatile var readerWrites = 0
    @Volatile var listWrites = 0
    @Volatile var preferencesMode = "ready"
    val preferenceWrites = java.util.concurrent.CopyOnWriteArrayList<String>()
    @Volatile var approvals = 0
    @Volatile var dismissals = 0
    @Volatile private var preferences = """{"playback":{"audioTrack":"","subtitleTrack":"","rate":1,"audioLanguage":"auto","subtitleLanguage":"off","nightMode":false,"dialogueBoost":false,"volumeBoost":1},"autoDownloadNext":1,"removeWatched":true,"downloadLimitGiB":30,"wifiOnly":false,"readerFontSize":22,"readerTheme":"dark"}"""
    @Volatile var episodeMarkers = ""
    @Volatile var bareShow = false
    @Volatile var episodeHistory = false
    @Volatile var signOuts = 0
    val playbackReads = java.util.concurrent.CopyOnWriteArrayList<String>()
    private var listed = false
    private val movie = item("film", "Continuing Film", progress = 90)
    private val newMovie = item("new-film", "New Film")
    private val nextMovie = item("next-film", "Next Film", progress = 30)
        .replace("\"backdrop\":\"/backdrop/next-film\"", "\"backdrop\":\"\"")
    private val series = item("0123456789abcdef", "Fixture Series", kind = "show")
        .replace("\"genres\"", "\"showId\":\"0123456789abcdef\",\"genres\"")
    private val episodes = listOf(item("episode-one", "S01E01 · First episode", progress = 12), item("episode-two", "S02E01 · Second episode"))
        .mapIndexed { index, item -> item.replace("\"genres\"", "\"show\":\"Fixture Series\",\"showId\":\"0123456789abcdef\",\"season\":${index+1},\"episode\":1,\"genres\"") }
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
                            path == "/api/v1/session" && parts[0] == "DELETE" -> { signOuts++; code = 204; ByteArray(0) }
                            path == "/api/v1/me" -> """{"server":"Fixture library","serverId":"parity-fixture","viewer":{"id":"viewer","name":"Alex"}}""".toByteArray()
                            path == "/api/v1/library" -> {
                                while (mode == "pending" && running.get()) Thread.sleep(20)
                                if (mode == "failed") { code = 503; "{}".toByteArray() }
                                else {
                                    val query = uri.query.split('&').associate { val p = it.split('=', limit = 2); p[0] to p.getOrElse(1) { "" } }
                                    val view = query["view"] ?: "all"
                                    val items = if (mode == "empty") emptyList() else when (view) {
                                        "history" -> listOf(movie, nextMovie, song).filter { dismissals == 0 || it != movie } +
                                            if (episodeHistory) listOf(episodes[0]) else emptyList()
                                        "movies" -> listOf(newMovie, movie)
                                        "shows" -> listOf(if (bareShow) series.replace("\"showId\":\"0123456789abcdef\",", "") else series)
                                        "music" -> listOf(song)
                                        "books" -> listOf(book)
                                        "list" -> if (listed) listOf(movie) else emptyList()
                                        "all" -> listOf(movie, newMovie, song)
                                        else -> emptyList()
                                    }
                                    """{"items":[${items.joinToString(",")}],"total":${items.size},"offset":0,"limit":${query["limit"] ?: "24"},"view":"$view","sort":"${query["sort"] ?: "title"}","query":"${query["q"] ?: ""}","letters":null}""".toByteArray()
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
                            path == "/api/v1/me/media-preferences" -> {
                                while (preferencesMode == "pending" && running.get()) Thread.sleep(20)
                                if (preferencesMode == "failed") { code = 503; "{}".toByteArray() }
                                else {
                                    if (parts[0] == "PUT") { preferences = requestBody; preferenceWrites += requestBody }
                                    preferences.toByteArray()
                                }
                            }
                            path == "/api/v1/quick-connect/482913" -> if (parts[0] == "POST") { approvals++; code = 204; ByteArray(0) }
                                else """{"code":"482913","device":"Living room TV","expiresAt":"${java.time.Instant.now().plusSeconds(600)}"}""".toByteArray()
                            path.startsWith("/api/v1/quick-connect/") -> { code = 400; """{"error":"Quick Connect code is invalid or expired"}""".toByteArray() }
                            path.endsWith("/continue-watching") && parts[0] == "DELETE" -> { dismissals++; code = 204; ByteArray(0) }
                            path.endsWith("/playback-preferences") -> """{"playback":{"audioTrack":"","subtitleTrack":"","rate":1,"audioLanguage":"auto","subtitleLanguage":"off","nightMode":false,"dialogueBoost":false,"volumeBoost":1},"overridden":false}""".toByteArray()
                            path.endsWith("/progress/sync") -> JSONObject(requestBody).getJSONObject("progress").toString().toByteArray()
                            path.startsWith("/api/v1/items/episode-") && path.endsWith("/playback") -> {
                                val id = path.removePrefix("/api/v1/items/").removeSuffix("/playback")
                                playbackReads += id
                                val extra = if (id == "episode-one") episodeMarkers else ""
                                """{"policy":"automatic","plan":{"allowed":true,"mode":"direct","reason":"direct-preferred"},"directAllowed":true,"direct":"/media/$id","directType":"video/mp4","duration":20,"start":0$extra}""".toByteArray()
                            }
                            path.startsWith("/api/v1/items/episode-") && path.endsWith("/watch-progress") -> """{"seconds":0,"duration":20}""".toByteArray()
                            path.endsWith("/playback") -> """{"policy":"automatic","plan":{"allowed":true,"mode":"direct","reason":"direct-preferred"},"directAllowed":true,"direct":"/media/film","directType":"video/mp4","duration":20,"start":0}""".toByteArray()
                            path == "/media/film" || path.startsWith("/media/episode-") -> {
                                type = "video/mp4"
                                InstrumentationRegistry.getInstrumentation().context.assets.open("parity-video.mp4").use { it.readBytes() }
                            }
                            path == "/api/v1/items/film/watch-progress" -> { progressRequests++; progressBody.toByteArray() }
                            path == "/api/v1/items/next-film/watch-progress" -> """{"seconds":30,"duration":3600}""".toByteArray()
                            path.startsWith("/api/v1/items/") -> {
                                val selected = if (path.endsWith("new-film")) newMovie else if (path.endsWith("/book")) book
                                    else if (path.endsWith("/episode-one")) episodes[0].replace("\"seconds\":12", "\"seconds\":0")
                                    else if (path.endsWith("/episode-two")) episodes[1] else movie
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
