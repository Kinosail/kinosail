package com.kinosail.player.mobile

import android.app.Application
import android.graphics.Bitmap
import androidx.activity.ComponentActivity
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.v2.createAndroidComposeRule
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.ViewModelStore
import androidx.lifecycle.ViewModelStoreOwner
import androidx.lifecycle.viewmodel.compose.LocalViewModelStoreOwner
import androidx.test.core.app.ApplicationProvider
import com.kinosail.player.core.CatalogModel
import com.kinosail.player.core.ConnectionModel
import com.kinosail.player.core.HomeModel
import com.kinosail.player.core.PersonalTabs
import com.kinosail.player.design.KinoTheme
import java.io.File
import org.json.JSONObject
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
class PhoneParityJourneyTest {
    @get:Rule val compose = createAndroidComposeRule<ComponentActivity>()

    @Test fun settingsFollowIphoneSectionsAndSavePlaybackDefaults() = phone { fixture ->
        openSettings()
        compose.onNodeWithContentDescription("Viewer Profile: Alex. Server: Parity library").assertExists()
        for (section in listOf("Playback & library", "Personalize", "About")) compose.onNodeWithText(section).performScrollTo().assertIsDisplayed()
        capture("settings")
        fixture.preferencesMode = "failed"
        compose.onNode(hasText("Playback") and hasClickAction()).performScrollTo().performClick()
        await { compose.onAllNodesWithText("Could not load your playback preferences. Try again.").fetchSemanticsNodes().isNotEmpty() }
        compose.onAllNodesWithContentDescription("Loading preferences").assertCountEquals(0)
        capture("playback-defaults-failed")
        fixture.preferencesMode = "ready"
        compose.onNodeWithText("Try again").performClick()
        await { compose.onAllNodesWithText("Boost Dialog").fetchSemanticsNodes().isNotEmpty() }
        compose.onNode(hasText("Subtitles") and hasText("Off") and hasClickAction()).assertExists()
        compose.onNode(hasText("Audio language") and hasText("Automatic") and hasClickAction()).assertExists()
        compose.onNode(hasText("Speed") and hasText("1×") and hasClickAction()).assertExists()
        capture("playback-defaults")
        compose.onNode(hasText("Boost Dialog") and hasClickAction()).performScrollTo().performClick()
        await { fixture.preferenceWrites.size == 1 }
        val boosted = JSONObject(fixture.preferenceWrites[0])
        assertTrue(boosted.getJSONObject("playback").getBoolean("dialogueBoost"))
        assertEquals("off", boosted.getJSONObject("playback").getString("subtitleLanguage"))
        assertEquals(30, boosted.getInt("downloadLimitGiB"))
        assertEquals("dark", boosted.getString("readerTheme"))
        compose.onNode(hasText("Speed") and hasClickAction()).performScrollTo().performClick()
        compose.onNode(hasText("1.5×") and hasClickAction()).performClick()
        await { fixture.preferenceWrites.size == 2 }
        assertEquals(1.5, JSONObject(fixture.preferenceWrites[1]).getJSONObject("playback").getDouble("rate"), 0.0)
        await { compose.onAllNodes(hasText("Speed") and hasText("1.5×") and hasClickAction()).fetchSemanticsNodes().isNotEmpty() }
        capture("playback-defaults-saved")
        compose.activityRule.scenario.onActivity { it.onBackPressedDispatcher.onBackPressed() }
        await { compose.onAllNodesWithText("Server & Viewer Profile").fetchSemanticsNodes().isNotEmpty() }
        compose.onNode(hasText("Sign out") and hasClickAction()).performScrollTo().performClick()
        await { compose.onAllNodesWithText("Sign out of this device?").fetchSemanticsNodes().isNotEmpty() }
        compose.onNodeWithText("Cancel").performClick()
        await { compose.onAllNodesWithText("Sign out of this device?").fetchSemanticsNodes().isEmpty() }
        assertFalse(fixture.requests.contains("DELETE /api/v1/session"))
        compose.activityRule.scenario.onActivity { it.onBackPressedDispatcher.onBackPressed() }
        await { compose.onAllNodesWithText("Customize tabs").fetchSemanticsNodes().isNotEmpty() }
    }

    @Test fun connectATvReviewsTheCodeBeforeApproving() = phone { fixture ->
        openSettings()
        compose.onNode(hasText("Connect a TV") and hasClickAction()).performScrollTo().performClick()
        await { compose.onAllNodes(hasSetTextAction()).fetchSemanticsNodes().isNotEmpty() }
        compose.onNode(hasText("Review device") and hasClickAction()).assertIsNotEnabled()
        compose.onNode(hasSetTextAction()).performTextInput("111111")
        compose.onNode(hasText("Review device") and hasClickAction()).performClick()
        await { compose.onAllNodesWithText("That code expired or was already used. Check the code on your TV.").fetchSemanticsNodes().isNotEmpty() }
        assertEquals(0, fixture.approvals)
        compose.onNode(hasSetTextAction()).performTextReplacement("482913")
        compose.onNode(hasText("Review device") and hasClickAction()).performClick()
        await { compose.onAllNodesWithText("Living room TV").fetchSemanticsNodes().isNotEmpty() }
        compose.onNodeWithText("Approve only if this code matches the device you want to connect.").assertExists()
        assertEquals(0, fixture.approvals)
        capture("connect-tv-review")
        compose.onNode(hasText("Approve device") and hasClickAction()).performScrollTo().performClick()
        await { compose.onAllNodesWithText("Device connected").fetchSemanticsNodes().isNotEmpty() }
        assertEquals(1, fixture.approvals)
        capture("connect-tv-approved")
        compose.onNode(hasText("Done") and hasClickAction()).performClick()
        await { compose.onAllNodesWithText("Server & Viewer Profile").fetchSemanticsNodes().isNotEmpty() }
    }

    @Test fun detailUsesIphoneMyListAndRemovesContinueWatching() = phone { fixture ->
        compose.onNode(hasText("Details") and hasClickAction()).performScrollTo().performClick()
        await { compose.onAllNodes(hasText("My List") and hasClickAction()).fetchSemanticsNodes().isNotEmpty() }
        compose.onNode(hasText("My List") and hasClickAction()).performScrollTo().performClick()
        await { compose.onAllNodes(hasText("In My List") and hasClickAction()).fetchSemanticsNodes().isNotEmpty() }
        assertTrue(fixture.listed)
        compose.onNode(hasText("Remove from Continue watching") and hasClickAction()).performScrollTo().performClick()
        await { fixture.dismissals == 1 }
        await { compose.onAllNodesWithText("Removed from Continue watching.").fetchSemanticsNodes().isNotEmpty() }
        compose.onAllNodes(hasText("Remove from Continue watching") and hasClickAction()).assertCountEquals(0)
        capture("detail-continue-removed")
    }

    @Test fun homeBrowsesLikeIphoneAndBackReturnsToMore() = phone { fixture ->
        val list = compose.onAllNodes(hasScrollAction())[0]
        list.performScrollToNode(hasText("Browse library"))
        compose.onAllNodes(hasText("Movies") and hasClickAction()).assertCountEquals(2)
        compose.onAllNodes(hasText("TV Shows") and hasClickAction()).assertCountEquals(2)
        compose.onNode(hasScrollAction() and hasAnyDescendant(hasText("TV Shows") and hasClickAction()) and
            !hasAnyDescendant(hasText("Browse library"))).performTouchInput { swipeLeft() }
        compose.onAllNodes(hasText("Photos") and hasClickAction()).assertCountEquals(0)
        compose.onAllNodes(hasText("Library") and hasClickAction()).assertCountEquals(0)
        compose.onAllNodesWithText("Movie genres").assertCountEquals(0)
        capture("home-browse")
        compose.onNode(hasText("More") and hasClickAction()).performClick()
        compose.onNode(hasText("Music") and hasClickAction()).performScrollTo().performClick()
        await { compose.onAllNodes(hasText("Music") and !hasClickAction()).fetchSemanticsNodes().isNotEmpty() }
        compose.activityRule.scenario.onActivity { it.onBackPressedDispatcher.onBackPressed() }
        await { compose.onAllNodesWithText("Customize tabs").fetchSemanticsNodes().isNotEmpty() }
        compose.onNode(hasText("Customize tabs") and hasClickAction()).performClick()
        compose.onNode(hasText("Add Settings") and hasClickAction()).performScrollTo().assertIsNotEnabled()
        assertEquals(PersonalTabs.defaults, PersonalTabs(ApplicationProvider.getApplicationContext()).load(fixture.viewer))
    }

    @Test fun emptyHomeShowsIphoneFeaturePlaceholder() = phone(empty = true) { _ ->
        await { compose.onAllNodesWithText("Your library is ready").fetchSemanticsNodes().isNotEmpty() }
        compose.onNodeWithText("Media added to your Server will appear here.").assertExists()
        capture("home-empty")
    }

    private fun openSettings() {
        compose.onNode(hasText("More") and hasClickAction()).performClick()
        compose.onNode(hasText("Settings") and hasClickAction()).performScrollTo().performClick()
        await { compose.onAllNodesWithText("Server & Viewer Profile").fetchSemanticsNodes().isNotEmpty() }
    }

    private fun phone(empty: Boolean = false, action: (PhoneParityFixture) -> Unit) {
        val application = ApplicationProvider.getApplicationContext<Application>()
        val models = ViewModelStore()
        PhoneParityFixture(application).use { fixture ->
            fixture.empty = empty
            val owner = object : ViewModelStoreOwner { override val viewModelStore = models }
            val factory = ViewModelProvider.AndroidViewModelFactory(application)
            ViewModelProvider(models, factory)[CatalogModel::class.java]
            val home = ViewModelProvider(models, factory)[HomeModel::class.java]
            val connection = ViewModelProvider(models, factory)[ConnectionModel::class.java]
            try {
                compose.setContent {
                    CompositionLocalProvider(LocalViewModelStoreOwner provides owner) {
                        KinoTheme { MobileLibrary(connection, fixture.viewer) }
                    }
                }
                await { !home.state.loading && (empty || home.state.recent.isNotEmpty()) }
                action(fixture)
            } finally { compose.runOnIdle { models.clear() } }
        }
    }

    private fun await(condition: () -> Boolean) = compose.waitUntil(8_000) {
        compose.waitForIdle()
        condition()
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
