package com.kinosail.player.tv

import android.app.Application
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.getUnclippedBoundsInRoot
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.unit.dp
import androidx.test.core.app.ApplicationProvider
import com.kinosail.player.core.CatalogItem
import com.kinosail.player.core.CatalogModel
import com.kinosail.player.design.TvKinoTheme
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35], qualifiers = "w960dp-h540dp")
class TvDetailLayoutTest {
    @get:Rule val compose = createComposeRule()

    @Test fun titleAndActionsShareTheFirstScreen() {
        val catalog = CatalogModel(ApplicationProvider.getApplicationContext<Application>())
        val item = CatalogItem("movie-1", "video", "Arrival", "2016", "A visitor arrives.", "")
        compose.setContent {
            TvKinoTheme {
                Box(Modifier.fillMaxSize().padding(56.dp)) {
                    TvDetail(item, catalog, Modifier, play = {}, viewPhoto = {})
                }
            }
        }

        val title = compose.onNodeWithText("Arrival").getUnclippedBoundsInRoot()
        val play = compose.onNodeWithText("Play").getUnclippedBoundsInRoot()
        val back = compose.onNodeWithText("Back").getUnclippedBoundsInRoot()
        assertTrue("Title should lead the action group", title.top < play.top)
        assertTrue("Play and Back should form one action row",
            play.top - back.top < 8.dp && back.top - play.top < 8.dp)
    }
}
