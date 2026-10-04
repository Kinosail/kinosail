package com.kinosail.player.core

import android.app.Application
import android.graphics.Bitmap
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.input.key.Key as ComposeKey
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
class CatalogRetryFooterTest {
    @get:Rule val compose = createComposeRule()

    @Test fun phoneRetainsCardsAndShowsRetryAtTheFailedPage() = footerJourney("phone")
    @Test @Config(qualifiers = "w834dp-h1210dp-mdpi")
    fun tabletRetainsCardsAndShowsRetryAtTheFailedPage() = footerJourney("tablet")
    @Test @Config(qualifiers = "w960dp-h540dp-mdpi")
    fun tvRetainsCardsAndShowsRemoteFocusableRetryAtTheFailedPage() = footerJourney("tv", tv = true)

    private fun footerJourney(name: String, tv: Boolean = false) {
        val application = ApplicationProvider.getApplicationContext<Application>()
        val models = ViewModelStore()
        AndroidRecoveryFixture(application).use { fixture ->
            val owner = object : ViewModelStoreOwner { override val viewModelStore = models }
            val factory = ViewModelProvider.AndroidViewModelFactory(application)
            val catalog = ViewModelProvider(models, factory)[CatalogModel::class.java]
            val connection = ViewModelProvider(models, factory)[ConnectionModel::class.java]
            val home = ViewModelProvider(models, factory)[HomeModel::class.java]
            fixture.failingOffset = 48
            try {
                compose.setContent {
                    CompositionLocalProvider(LocalViewModelStoreOwner provides owner) {
                        if (tv) TvKinoTheme { TvLibrary(connection, fixture.viewer) }
                        else KinoTheme { MobileLibrary(connection, fixture.viewer) }
                    }
                }
                await { catalog.state.items.isNotEmpty() && !catalog.state.loading }
                await { home.state.recent.isNotEmpty() && !home.state.loading }
                val movies = compose.onAllNodesWithText("Movies")[0]
                if (tv) {
                    movies.performScrollTo().assertIsDisplayed()
                    movies.performSemanticsAction(androidx.compose.ui.semantics.SemanticsActions.RequestFocus) { it() }
                    movies.assertIsFocused().performKeyInput { pressKey(ComposeKey.DirectionCenter) }
                } else movies.performClick()
                await { catalog.state.view == "movies" && !catalog.state.loading }
                if (catalog.state.items.size == 24) {
                    compose.runOnIdle { catalog.loadMore() }
                    await { catalog.state.items.size == 48 && !catalog.state.loading }
                }
                val loaded = catalog.state.items
                capture("$name-loaded")
                fixture.catalogGate = CountDownLatch(1)
                compose.runOnIdle { catalog.loadMore() }
                await { fixture.catalogOffsets.last() == 48 }
                assertTrue(catalog.state.loading)
                assertEquals(loaded, catalog.state.items)
                val grid = compose.onNode(hasScrollAction())
                grid.performScrollToIndex(48)
                compose.onNodeWithText("Loading more…").assertIsDisplayed()
                capture("$name-pending")
                fixture.catalogGate.countDown()
                await { !catalog.state.loading && fixture.catalogOffsets.last() == 48 }
                assertEquals(loaded, catalog.state.items)
                grid.performScrollToIndex(48)
                val retry = compose.onNodeWithText("Retry loading more")
                retry.assertIsDisplayed()
                if (tv) {
                    retry.performSemanticsAction(androidx.compose.ui.semantics.SemanticsActions.RequestFocus) { it() }
                    retry.assertIsFocused()
                }
                capture("$name-failed")
                fixture.failingOffset = null
                if (tv) retry.performKeyInput { pressKey(ComposeKey.DirectionCenter) }
                else retry.performClick()
                await { !catalog.state.loading && catalog.state.items.size >= 72 }
                assertEquals(loaded, catalog.state.items.take(48))
                assertEquals(48, fixture.catalogOffsets.last())
                compose.onNodeWithText("Retry loading more").assertDoesNotExist()
                capture("$name-recovered")
            } catch (error: Throwable) {
                println("R09 footer $name: view=${catalog.state.view} loading=${catalog.state.loading} rows=${catalog.state.items.size} offsets=${fixture.catalogOffsets.takeLast(8)}")
                runCatching { capture("$name-diagnostic") }
                throw error
            } finally { compose.runOnIdle { models.clear() } }
        }
    }

    private fun await(condition: () -> Boolean) = compose.waitUntil(8_000) {
        compose.waitForIdle()
        condition()
    }

    private fun capture(name: String) {
        val directory = System.getenv("KINOSAIL_ANDROID_RECOVERY_EVIDENCE") ?: return
        val output = File(directory).apply { mkdirs() }
        File(output, "$name-semantics.txt").writeText(compose.onRoot().printToString())
        File(output, "$name.png").outputStream().use { stream ->
            compose.onRoot().captureToImage().asAndroidBitmap().compress(Bitmap.CompressFormat.PNG, 100, stream)
        }
    }
}
