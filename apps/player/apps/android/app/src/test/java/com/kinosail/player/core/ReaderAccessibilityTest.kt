package com.kinosail.player.core

import android.graphics.Bitmap
import android.graphics.Color
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.mutableStateOf
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onRoot
import com.kinosail.player.design.KinoTheme
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.junit.Assert.assertTrue

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class ReaderAccessibilityTest {
    @get:Rule val compose = createComposeRule()

    @Test fun pendingPageWithKnownCountShowsLoading() {
        compose.setContent { ReaderPageView("Example", null, 0, 2, null, null, {}, {}, {}, {}, {}) }
        compose.onNode(SemanticsMatcher.keyIsDefined(SemanticsProperties.ProgressBarRangeInfo)).assertExists()
    }

    @Test fun loadingFollowsPageAvailabilityAndStopsOnFailure() {
        val image = mutableStateOf<Bitmap?>(null)
        val notice = mutableStateOf<String?>(null)
        compose.setContent { ReaderPageView("Example", image.value, 0, 2, notice.value, null, {}, {}, {}, {}, {}) }
        val loading = compose.onNode(SemanticsMatcher.keyIsDefined(SemanticsProperties.ProgressBarRangeInfo))
        loading.assertExists()
        compose.runOnIdle { image.value = Bitmap.createBitmap(4, 4, Bitmap.Config.ARGB_8888) }
        compose.onNodeWithContentDescription("Example, page 1").assertExists()
        loading.assertDoesNotExist()
        compose.runOnIdle { image.value = null }
        loading.assertExists()
        compose.runOnIdle { notice.value = "Could not load this comic page." }
        loading.assertDoesNotExist()
        compose.onNodeWithText("Could not load this comic page.").assertExists()
        compose.onNodeWithText("Try again").assertExists()
    }

    @Test fun unavailableFormatStopsLoadingWithoutOfferingPageRetry() {
        compose.setContent {
            ReaderPageView("Example", null, 0, 0, "This book format is not available on Android yet.",
                null, {}, {}, {}, {}, {})
        }
        compose.onNodeWithText("This book format is not available on Android yet.").assertExists()
        compose.onNodeWithText("Try again").assertDoesNotExist()
        compose.onNode(SemanticsMatcher.keyIsDefined(SemanticsProperties.ProgressBarRangeInfo)).assertDoesNotExist()
    }

    @Test fun pendingPageBeforeCountLoadsShowsLoading() {
        compose.setContent { ReaderPageView("Example", null, 0, 0, null, null, {}, {}, {}, {}, {}) }
        compose.onNode(SemanticsMatcher.keyIsDefined(SemanticsProperties.ProgressBarRangeInfo)).assertExists()
    }

    @Test fun readyPageStopsLoadingAndPreservesItsImageDuringSaveFailure() {
        val image = Bitmap.createBitmap(4, 4, Bitmap.Config.ARGB_8888)
        compose.setContent { ReaderPageView("Example", image, 0, 2, null, "Reading position was not saved.", {}, {}, {}, {}, {}) }
        compose.onNodeWithContentDescription("Example, page 1").assertExists()
        compose.onNodeWithText("Reading position was not saved.").assertExists()
        compose.onNode(SemanticsMatcher.keyIsDefined(SemanticsProperties.ProgressBarRangeInfo)).assertDoesNotExist()
    }

    @Test @Config(qualifiers = "w390dp-h844dp-xhdpi")
    @GraphicsMode(GraphicsMode.Mode.NATIVE)
    fun savingErrorRemainsReadableOverALightPage() {
        val page = Bitmap.createBitmap(600, 900, Bitmap.Config.ARGB_8888).apply { eraseColor(Color.WHITE) }
        var errorColor = 0
        compose.setContent {
            KinoTheme {
                errorColor = MaterialTheme.colorScheme.error.toArgb()
                ReaderPageView("Example", page, 0, 2, null, "Reading position was not saved.", {}, {}, {}, {}, {})
            }
        }
        val bounds = compose.onNodeWithText("Reading position was not saved.").fetchSemanticsNode().boundsInRoot
        val pixels = compose.onRoot().captureToImage().asAndroidBitmap()
        val foregroundLuminance = Color.luminance(errorColor)
        for (x in listOf(0, bounds.left.toInt() + 1, pixels.width - 1)) {
            val backgroundLuminance = Color.luminance(pixels.getPixel(x, bounds.top.toInt() + 1))
            val contrast = (maxOf(foregroundLuminance, backgroundLuminance) + .05) /
                (minOf(foregroundLuminance, backgroundLuminance) + .05)
            assertTrue("Saving error contrast over the rendered page at x=$x was $contrast", contrast >= 4.5)
        }
    }

    @Test fun readingAndSavingErrorsAreAnnounced() {
        compose.setContent {
            ReaderPageView("Example", null, 0, 1, "Could not render this page.",
                "Reading position was not saved.", {}, {}, {}, {}, {})
        }

        compose.onNode(SemanticsMatcher.keyIsDefined(SemanticsProperties.ProgressBarRangeInfo)).assertDoesNotExist()
        val polite = SemanticsMatcher.expectValue(SemanticsProperties.LiveRegion, LiveRegionMode.Polite)
        compose.onNodeWithText("Could not render this page.").assert(polite)
        compose.onNodeWithText("Reading position was not saved.").assert(polite)
    }
}
