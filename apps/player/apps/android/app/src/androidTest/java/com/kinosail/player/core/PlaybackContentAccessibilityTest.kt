package com.kinosail.player.core

import androidx.activity.ComponentActivity
import androidx.compose.ui.test.junit4.accessibility.enableAccessibilityChecks
import androidx.compose.ui.test.junit4.v2.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.tryPerformAccessibilityChecks
import androidx.test.ext.junit.runners.AndroidJUnit4
import com.kinosail.player.design.TvKinoTheme
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class PlaybackContentAccessibilityTest {
    @get:Rule val compose = createAndroidComposeRule<ComponentActivity>()

    @Test fun playbackErrorPassesAccessibilityChecks() {
        val item = CatalogItem("book-1", "book", "Example book", "", "", "")
        val viewer = Viewer("https://example.test", "server-1", "viewer-1", "Viewer")
        compose.setContent {
            TvKinoTheme { PlaybackScreen(item, viewer, tv = true, close = {}, onNext = {}) }
        }
        compose.onNodeWithText("This title cannot be played here.").assertExists()
        compose.enableAccessibilityChecks()
        compose.onRoot().tryPerformAccessibilityChecks()
    }

    @Test fun readerErrorsPassAccessibilityChecks() {
        compose.setContent {
            ReaderPageView("Example", null, 0, 1, "Could not render this page.",
                "Reading position was not saved.", {}, {}, {}, {}, {})
        }
        compose.onNodeWithText("Reading position was not saved.").assertExists()
        compose.enableAccessibilityChecks()
        compose.onRoot().tryPerformAccessibilityChecks()
    }
}
