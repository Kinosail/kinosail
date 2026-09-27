package com.kinosail.player.core

import androidx.mediarouter.app.MediaRouteButton
import com.kinosail.player.R
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class CastButtonThemeTest {
    @Test fun playerThemeCanCreateCastButton() {
        val context = RuntimeEnvironment.getApplication()
        context.setTheme(R.style.Theme_Kinosail)
        MediaRouteButton(context)
    }
}
