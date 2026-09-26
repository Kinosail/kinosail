package com.kinosail.player.design

import androidx.compose.material3.MaterialTheme
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.test.junit4.createComposeRule
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
class KinoThemeTest {
    @get:Rule val compose = createComposeRule()

    @Test @Config(qualifiers = "notnight") fun phoneShellStaysDarkWhenSystemIsLight() {
        var background: Color? = null
        var primary: Color? = null
        compose.setContent {
            KinoTheme {
                background = MaterialTheme.colorScheme.background
                primary = MaterialTheme.colorScheme.primary
            }
        }
        compose.waitForIdle()
        assertEquals(KinoColor.background, background)
        assertEquals(KinoColor.signal, primary)
    }
}
