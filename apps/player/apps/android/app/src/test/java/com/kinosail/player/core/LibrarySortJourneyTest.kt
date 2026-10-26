package com.kinosail.player.core

import android.app.Application
import android.graphics.Bitmap
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.input.key.Key as ComposeKey
import androidx.compose.ui.semantics.SemanticsActions
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
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.shadows.ShadowLog

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35], qualifiers = "w390dp-h844dp-mdpi")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
class LibrarySortJourneyTest {
    @get:Rule val compose = createComposeRule()
    private val letters = """[{"label":"A","offset":0,"count":30},{"label":"M","offset":30,"count":40},{"label":"T","offset":70,"count":26}]"""

    @Test fun phoneSortsAndJumpsByLetter() = library(letters = letters) { fixture, catalog ->
        compose.onNodeWithText("96 titles").assertIsDisplayed()
        compose.onNodeWithContentDescription("M, 40 titles").assertIsDisplayed()
        capture("phone-library-index")
        compose.onNodeWithContentDescription("T, 26 titles").performClick()
        await { catalog.state.items.size > 70 && !catalog.state.loading }
        await { compose.onAllNodesWithContentDescription("Fictional 71").fetchSemanticsNodes().isNotEmpty() }
        val jumped = compose.onNodeWithContentDescription("Fictional 71").assertIsDisplayed().fetchSemanticsNode().boundsInRoot
        assertTrue("A letter jump must bring its first title to the top: ${jumped.top}", jumped.top < compose.onRoot().fetchSemanticsNode().size.height / 2f)
        capture("phone-library-jumped")
        chooseSort("Recently added")
        await { !catalog.state.loading && fixture.catalogSorts.last() == "added" && catalog.state.items.firstOrNull()?.title == "Fictional 96" }
        compose.onNode(hasText("Sort: Recently added") and hasClickAction()).assertIsDisplayed()
        compose.onNodeWithContentDescription("Fictional 96").assertIsDisplayed()
        compose.onAllNodesWithContentDescription("M, 40 titles").assertCountEquals(0)
        capture("phone-library-added")
        chooseSort("Year")
        await { !catalog.state.loading && fixture.catalogSorts.last() == "year" }
        compose.onNode(hasText("Sort: Year") and hasClickAction()).assertIsDisplayed()
        chooseSort("Title")
        await { !catalog.state.loading && fixture.catalogSorts.last() == "title" && catalog.state.letters.isNotEmpty() }
        compose.onNodeWithContentDescription("M, 40 titles").assertIsDisplayed()
    }

    @Test fun phoneHidesTheIndexForSixtyTitlesOrFewer() = library(letters = """[{"label":"A","offset":0,"count":30},{"label":"M","offset":30,"count":30}]""",
        total = 60) { _, _ ->
        compose.onNodeWithText("60 titles").assertIsDisplayed()
        compose.onAllNodesWithContentDescription("M, 30 titles").assertCountEquals(0)
    }

    @Test fun phoneRejectsLettersThatSkipTitles() = library(
        letters = """[{"label":"A","offset":0,"count":30},{"label":"M","offset":40,"count":56}]""", loaded = false) { _, catalog ->
        await { !catalog.state.loading && catalog.state.notice != null }
        assertTrue(catalog.state.items.isEmpty())
        assertTrue(catalog.state.letters.isEmpty())
        compose.onNodeWithText("Could not load your library. Try again.").assertIsDisplayed()
        compose.onAllNodesWithContentDescription("M, 56 titles").assertCountEquals(0)
        val logs = ShadowLog.getLogsForTag("KinosailCatalog").map { it.msg }
        assertTrue(logs.toString(), logs.any { it == "operation=library offset=0 failure=invalid_response outcome=rejected" })
        assertTrue(logs.none { it.contains("recovery-fixture") || it.contains("127.0.0.1") })
    }

    @Test @Config(qualifiers = "w960dp-h540dp-mdpi")
    fun tvSortsAndJumpsByLetter() = library(tv = true, letters = letters) { fixture, catalog ->
        compose.onNode(hasText("Movies") and !hasClickAction()).assertIsDisplayed()
        compose.onNodeWithText("96 titles").assertIsDisplayed()
        capture("tv-library-index")
        focus(compose.onNodeWithContentDescription("T, 26 titles"))
        await { catalog.state.items.size > 70 && !catalog.state.loading }
        await { compose.onAllNodesWithContentDescription("Fictional 71").fetchSemanticsNodes().isNotEmpty() }
        compose.onNodeWithContentDescription("Fictional 71").assertIsDisplayed()
        capture("tv-library-jumped")
        val sort = compose.onNode(hasText("Sort: Title") and hasClickAction())
        focus(sort)
        sort.performKeyInput { pressKey(ComposeKey.DirectionCenter) }
        val added = compose.onNode(hasText("Recently added") and hasClickAction())
        focus(added)
        added.performKeyInput { pressKey(ComposeKey.DirectionCenter) }
        await { !catalog.state.loading && fixture.catalogSorts.last() == "added" && catalog.state.items.firstOrNull()?.title == "Fictional 96" }
        compose.onNode(hasText("Sort: Recently added") and hasClickAction()).assertIsDisplayed()
        compose.onAllNodesWithContentDescription("M, 40 titles").assertCountEquals(0)
        capture("tv-library-added")
    }

    private fun chooseSort(label: String) {
        compose.onNode(hasText("Sort:", substring = true) and hasClickAction()).performClick()
        compose.onNode(hasText(label) and hasClickAction()).performClick()
    }

    private fun library(tv: Boolean = false, letters: String? = null, total: Int? = null, loaded: Boolean = true,
                        action: (AndroidRecoveryFixture, CatalogModel) -> Unit) {
        val application = ApplicationProvider.getApplicationContext<Application>()
        val models = ViewModelStore()
        AndroidRecoveryFixture(application).use { fixture ->
            total?.let { fixture.catalogTotals[""] = it }
            fixture.catalogLetters = letters
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
                await { !catalog.state.loading && !home.state.loading && (!loaded || home.state.recent.isNotEmpty()) }
                val movies = if (tv) compose.onNodeWithContentDescription("Movies") else compose.onAllNodes(hasText("Movies") and hasClickAction())[0]
                if (tv) {
                    focus(movies)
                    movies.performKeyInput { pressKey(ComposeKey.DirectionCenter) }
                } else movies.performClick()
                await { !catalog.state.loading && catalog.state.view == "movies" && (!loaded || catalog.state.items.isNotEmpty()) }
                action(fixture, catalog)
            } finally { compose.runOnIdle { models.clear() } }
        }
    }

    private fun await(condition: () -> Boolean) = compose.waitUntil(8_000) {
        compose.waitForIdle()
        condition()
    }

    private fun focus(node: SemanticsNodeInteraction) {
        node.performSemanticsAction(SemanticsActions.RequestFocus) { it() }
        compose.waitForIdle()
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
