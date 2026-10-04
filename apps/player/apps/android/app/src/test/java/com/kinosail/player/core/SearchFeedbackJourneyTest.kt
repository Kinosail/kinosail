package com.kinosail.player.core

import android.app.Application
import android.graphics.Bitmap
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.input.key.Key as ComposeKey
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.ViewModelStore
import androidx.lifecycle.ViewModelStoreOwner
import androidx.lifecycle.viewmodel.compose.LocalViewModelStoreOwner
import androidx.test.core.app.ApplicationProvider
import com.kinosail.player.design.KinoTheme
import com.kinosail.player.design.TvKinoTheme
import com.kinosail.player.mobile.MobileLibrary
import com.kinosail.player.tv.TvLibrary
import java.io.File
import java.util.concurrent.CountDownLatch
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35], qualifiers = "w390dp-h844dp-mdpi")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
class SearchFeedbackJourneyTest {
    @get:Rule val compose = createComposeRule()
    @Test fun phoneSubmittedResultsAndClearAction() = resultsJourney("phone")
    @Test @Config(qualifiers = "w834dp-h1210dp-mdpi")
    fun tabletSubmittedResultsAndClearAction() = resultsJourney("tablet")
    @Test @Config(qualifiers = "w960dp-h540dp-mdpi")
    fun tvSubmittedResultsAndClearActionRetainFocus() = resultsJourney("tv", tv = true)

    @Test fun phoneHasOneExplicitClearActionAfterSubmission() = screen { fixture, catalog, field ->
        field.performTextInput("Fictional")
        field.performImeAction()
        await { !catalog.state.loading && catalog.hasActiveSearch }
        capture("phone-clear-baseline")
        compose.onNodeWithText("Clear search").assertIsDisplayed().performClick()
        await { !catalog.state.loading && !catalog.hasActiveSearch }
        assertTrue(catalog.searchInput.isEmpty())
        assertEquals("", fixture.catalogQueries.last())
    }

    @Test @Config(qualifiers = "w960dp-h540dp-mdpi")
    fun tvHasOneFocusedClearActionAfterSubmission() = screen(tv = true) { fixture, catalog, field ->
        field.performTextInput("Fictional")
        field.performImeAction()
        await { !catalog.state.loading && catalog.hasActiveSearch }
        capture("tv-clear-baseline")
        val clear = compose.onNodeWithText("Clear search").assertIsDisplayed()
        focus(clear)
        clear.performKeyInput { pressKey(ComposeKey.DirectionCenter) }
        await { !catalog.state.loading && !catalog.hasActiveSearch }
        assertTrue(catalog.searchInput.isEmpty())
        assertEquals("", fixture.catalogQueries.last())
        clear.assertIsFocused()
    }

    @Test fun pendingAndFailedQueriesDoNotAnnounceOlderResults() = screen { fixture, catalog, field ->
        field.performTextInput("Fictional")
        field.performImeAction()
        await { !catalog.state.loading && catalog.hasActiveSearch }
        compose.onNodeWithText("Results: 96 · “Fictional”").assertIsDisplayed()
            .assert(SemanticsMatcher.expectValue(SemanticsProperties.LiveRegion, LiveRegionMode.Polite))
        fixture.catalogGates["new query"] = CountDownLatch(1)
        fixture.catalogFailures["new query"] = 400
        field.performTextReplacement("new query")
        field.performImeAction()
        await { fixture.catalogQueries.last() == "new query" }
        assertTrue(catalog.state.loading)
        compose.onNodeWithText("Results: 96 · “Fictional”").assertDoesNotExist()
        compose.onNode(hasText("Results:", substring = true)).assertDoesNotExist()
        capture("phone-search-pending")
        fixture.catalogGates.getValue("new query").countDown()
        await { !catalog.state.loading && catalog.state.notice != null }
        compose.onNode(hasText("Results:", substring = true)).assertDoesNotExist()
        compose.onNodeWithText("Could not load your library. Try again.").assertIsDisplayed()
        capture("phone-search-failed")
    }

    @Test fun zeroMatchesShowSubmittedUnicodeQueryAndKeepClearAvailable() = screen { fixture, catalog, field ->
        val query = "Álbum \"Ecos\""
        fixture.catalogTotals[query] = 0
        field.performTextInput(query)
        field.performImeAction()
        await { !catalog.state.loading && fixture.catalogQueries.last() == query }
        assertEquals(0, catalog.state.total)
        assertTrue(catalog.state.items.isEmpty())
        compose.onNodeWithText("Results: 0 · “$query”").assertIsDisplayed()
        compose.onNodeWithText("No results. Try another search.").assertIsDisplayed()
        compose.onNodeWithText("Clear search").assertIsDisplayed()
        capture("phone-search-zero")
    }

    @Test fun anOlderResponseCannotRelabelNewerSubmittedResults() = screen { fixture, catalog, field ->
        fixture.catalogGates["slow"] = CountDownLatch(1)
        fixture.catalogTotals["new"] = 2
        field.performTextInput("slow")
        field.performImeAction()
        await { fixture.catalogQueries.last() == "slow" }
        field.performTextReplacement("new")
        field.performImeAction()
        await { !catalog.state.loading && catalog.state.total == 2 }
        val latest = catalog.state.items
        compose.onNodeWithText("Results: 2 · “new”").assertIsDisplayed()
        fixture.catalogGates.getValue("slow").countDown()
        await { fixture.completedCatalogQueries.contains("slow") }
        assertEquals(latest, catalog.state.items)
        assertEquals(2, catalog.state.total)
        compose.onNodeWithText("Results: 2 · “new”").assertIsDisplayed()
        compose.onNodeWithText("Results: 96 · “slow”").assertDoesNotExist()
    }

    private fun resultsJourney(name: String, tv: Boolean = false) = screen(tv) { fixture, catalog, field ->
        val requests = fixture.catalogQueries.size
        field.performTextInput("Fictional")
        compose.waitForIdle()
        assertEquals("Editing must not submit a query", requests, fixture.catalogQueries.size)
        capture("$name-search-entered")
        field.performImeAction()
        await { !catalog.state.loading && fixture.catalogQueries.last() == "Fictional" }
        compose.onNodeWithText("Results: 96 · “Fictional”").assertIsDisplayed()
        capture("$name-search-results")
        val submittedRequests = fixture.catalogQueries.size
        field.performTextReplacement("draft")
        compose.waitForIdle()
        assertEquals(submittedRequests, fixture.catalogQueries.size)
        compose.onNodeWithText("Results: 96 · “Fictional”").assertIsDisplayed()
        val clear = compose.onNodeWithText("Clear search").assertIsDisplayed()
        if (tv) {
            focus(clear)
            clear.performKeyInput { pressKey(ComposeKey.DirectionCenter) }
        } else clear.performClick()
        await { !catalog.state.loading && !catalog.hasActiveSearch && fixture.catalogQueries.last().isEmpty() }
        assertTrue(catalog.searchInput.isEmpty())
        assertEquals("One Clear action must send one new query", submittedRequests + 1, fixture.catalogQueries.size)
        await { compose.onAllNodesWithText("Results: 96").fetchSemanticsNodes().isNotEmpty() }
        compose.onNodeWithText("Results: 96").assertIsDisplayed()
        if (tv) clear.assertIsFocused()
        capture("$name-search-cleared")
    }

    private fun screen(tv: Boolean = false,
                       action: (AndroidRecoveryFixture, CatalogModel, SemanticsNodeInteraction) -> Unit) {
        val application = ApplicationProvider.getApplicationContext<Application>()
        val models = ViewModelStore()
        AndroidRecoveryFixture(application).use { fixture ->
            val owner = object : ViewModelStoreOwner { override val viewModelStore = models }
            val factory = ViewModelProvider.AndroidViewModelFactory(application)
            val catalog = ViewModelProvider(models, factory)[CatalogModel::class.java]
            val home = ViewModelProvider(models, factory)[HomeModel::class.java]
            val connection = ViewModelProvider(models, factory)[ConnectionModel::class.java]
            try {
                compose.setContent {
                    CompositionLocalProvider(LocalViewModelStoreOwner provides owner) {
                        if (tv) TvKinoTheme { TvLibrary(connection, fixture.viewer) }
                        else KinoTheme { MobileLibrary(connection, fixture.viewer) }
                    }
                }
                await { catalog.state.items.isNotEmpty() && !home.state.loading && home.state.recent.isNotEmpty() }
                val searchDestination = compose.onAllNodesWithText("Search")[0]
                if (tv) {
                    searchDestination.assertIsDisplayed()
                    searchDestination.performSemanticsAction(androidx.compose.ui.semantics.SemanticsActions.RequestFocus) { it() }
                    searchDestination.assertIsFocused().performKeyInput { pressKey(ComposeKey.DirectionCenter) }
                } else searchDestination.performClick()
                val field = compose.onNodeWithText("Search library")
                field.assertIsDisplayed()
                await { !catalog.state.loading && catalog.state.view == "all" }
                action(fixture, catalog, field)
            } finally { compose.runOnIdle { models.clear() } }
        }
    }

    private fun await(condition: () -> Boolean) = compose.waitUntil(8_000) {
        compose.waitForIdle()
        condition()
    }

    private fun focus(node: SemanticsNodeInteraction) {
        node.performSemanticsAction(androidx.compose.ui.semantics.SemanticsActions.RequestFocus) { it() }
        node.assertIsFocused()
    }

    private fun capture(name: String) {
        val directory = System.getenv("KINOSAIL_ANDROID_SEARCH_EVIDENCE") ?: return
        val output = File(directory).apply { mkdirs() }
        File(output, "$name-semantics.txt").writeText(compose.onRoot().printToString())
        File(output, "$name.png").outputStream().use { stream ->
            compose.onRoot().captureToImage().asAndroidBitmap().compress(Bitmap.CompressFormat.PNG, 100, stream)
        }
    }
}
