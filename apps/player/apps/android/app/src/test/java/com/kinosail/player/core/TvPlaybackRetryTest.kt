package com.kinosail.player.core

import android.app.Application
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.input.key.Key as ComposeKey
import androidx.compose.ui.test.assertIsFocused
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performKeyInput
import androidx.compose.ui.test.pressKey
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.ViewModelStore
import androidx.lifecycle.ViewModelStoreOwner
import androidx.lifecycle.viewmodel.compose.LocalViewModelStoreOwner
import androidx.test.core.app.ApplicationProvider
import com.kinosail.player.design.TvKinoTheme
import com.kinosail.player.watchcore.WatchRequest
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35], qualifiers = "w960dp-h540dp")
class TvPlaybackRetryTest {
    @get:Rule val compose = createComposeRule()

    @Test fun actualTvRetryRetainsRemoteRegistrationFirstRun() = retryJourney()
    @Test fun actualTvRetryRetainsRemoteRegistrationSecondRun() = retryJourney()

    private fun retryJourney() {
        val application = ApplicationProvider.getApplicationContext<Application>()
        val models = ViewModelStore()
        AndroidRecoveryFixture(application).use { fixture ->
            val owner = object : ViewModelStoreOwner { override val viewModelStore = models }
            val model = ViewModelProvider(models, ViewModelProvider.AndroidViewModelFactory(application))[PlaybackModel::class.java]
            val item = CatalogItem("film-1", "video", "Fictional 1", "", "", "")
            try {
                compose.setContent {
                    CompositionLocalProvider(LocalViewModelStoreOwner provides owner) {
                        TvKinoTheme { PlaybackScreen(item, fixture.viewer, tv = true, close = {}, onNext = {}) }
                    }
                }
                await { model.retryable && fixture.playbackRequests.get() == 1 }
                fixture.playbackFailure = false
                compose.onNodeWithText("Try again").assertIsFocused()
                    .performKeyInput { pressKey(ComposeKey.DirectionCenter) }
                await { fixture.playbackRequests.get() >= 2 && model.player != null }
                println("R13 TV retry: playbackRequests=${fixture.playbackRequests.get()} phoneOwnership=${PlaybackModel.currentPhone() != null}")
                assertNull("TV retry must not claim phone playback ownership", PlaybackModel.currentPhone())
                await { fixture.remoteUpdates.isNotEmpty() }
                val remoteId = fixture.remoteUpdates.last().first
                assertEquals("Retry must preserve the saved TV identifier", fixture.tvId, remoteId)
                assertEquals("film-1", fixture.remoteUpdates.last().second)
                compose.runOnIdle {
                    assertTrue(model.applyRemote(WatchRequest(remoteId, "film-1", "pause")))
                    assertFalse(model.applyRemote(WatchRequest("phone", "film-1", "pause")))
                    assertFalse(model.applyRemote(WatchRequest(remoteId, "other", "pause")))
                }
            } catch (error: Throwable) {
                println("R13 fixture state: retryable=${model.retryable} message=${model.message} requests=${fixture.requestPaths.take(12)}")
                throw error
            } finally { compose.runOnIdle { models.clear() } }
        }
    }

    private fun await(condition: () -> Boolean) = compose.waitUntil(8_000) {
        compose.waitForIdle()
        condition()
    }
}
