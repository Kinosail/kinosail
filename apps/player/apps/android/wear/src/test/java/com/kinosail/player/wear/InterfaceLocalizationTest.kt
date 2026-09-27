package com.kinosail.player.wear

import android.content.res.Configuration
import org.junit.Assert.assertEquals
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import org.robolectric.annotation.Config
import java.util.Locale

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class InterfaceLocalizationTest {
    @Test fun playbackActionUsesSharedTranslation() {
        val app = RuntimeEnvironment.getApplication()
        val configuration = Configuration(app.resources.configuration)
        configuration.setLocale(Locale.forLanguageTag("es"))
        assertEquals("Reproducir", InterfaceCopy.translate(app.createConfigurationContext(configuration), "Play"))
    }

    @Test fun unsupportedLanguageKeepsEnglish() {
        val app = RuntimeEnvironment.getApplication()
        val configuration = Configuration(app.resources.configuration)
        configuration.setLocale(Locale.forLanguageTag("zu"))
        assertEquals("Play", InterfaceCopy.translate(app.createConfigurationContext(configuration), "Play"))
    }
}
