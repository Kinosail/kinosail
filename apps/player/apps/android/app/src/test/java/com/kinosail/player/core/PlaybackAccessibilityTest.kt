package com.kinosail.player.core

import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import com.kinosail.player.design.TvKinoTheme
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class PlaybackAccessibilityTest {
    @get:Rule val compose = createComposeRule()

    @Test fun playbackFailureHasPoliteAnnouncement() {
        val item = CatalogItem("book-1", "book", "Example book", "", "", "")
        val viewer = Viewer("https://example.test", "server-1", "viewer-1", "Viewer")
        compose.setContent {
            TvKinoTheme { PlaybackScreen(item, viewer, tv = true, close = {}, onNext = {}) }
        }

        compose.onNodeWithText("This title cannot be played here.")
            .assert(SemanticsMatcher.expectValue(SemanticsProperties.LiveRegion, LiveRegionMode.Polite))
    }
}
