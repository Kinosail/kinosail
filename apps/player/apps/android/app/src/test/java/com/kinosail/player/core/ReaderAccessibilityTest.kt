package com.kinosail.player.core

import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class ReaderAccessibilityTest {
    @get:Rule val compose = createComposeRule()

    @Test fun readingAndSavingErrorsAreAnnounced() {
        compose.setContent {
            ReaderPageView("Example", null, 0, 1, "Could not render this page.",
                "Reading position was not saved.", {}, {}, {}, {}, {})
        }

        val polite = SemanticsMatcher.expectValue(SemanticsProperties.LiveRegion, LiveRegionMode.Polite)
        compose.onNodeWithText("Could not render this page.").assert(polite)
        compose.onNodeWithText("Reading position was not saved.").assert(polite)
    }
}
