package com.kinosail.player.core

import android.content.res.Configuration
import android.os.LocaleList
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
    @Test fun navigationUsesSharedTranslations() {
        val app = RuntimeEnvironment.getApplication()
        for ((tag, home, shows) in listOf(
            Triple("es", "Inicio", "Series"),
            Triple("fr", "Accueil", "Émissions"),
            Triple("ar", "الصفحة الرئيسية", "العروض"),
        )) {
            val configuration = Configuration(app.resources.configuration)
            configuration.setLocale(Locale.forLanguageTag(tag))
            val context = app.createConfigurationContext(configuration)
            assertEquals(home, InterfaceCopy.translate(context, "Home"))
            assertEquals(shows, InterfaceCopy.translate(context, "Shows"))
        }
    }

    @Test fun unsupportedLanguageFallsBackToEnglish() {
        val app = RuntimeEnvironment.getApplication()
        val configuration = Configuration(app.resources.configuration)
        configuration.setLocale(Locale.forLanguageTag("zu"))
        val context = app.createConfigurationContext(configuration)
        assertEquals("Try again", InterfaceCopy.translate(context, "Try again"))
    }

    @Test @Config(sdk = [23]) fun olderPhonesUseTheirConfiguredLanguage() {
        val app = RuntimeEnvironment.getApplication()
        val configuration = Configuration(app.resources.configuration)
        configuration.setLocale(Locale.forLanguageTag("es"))
        val context = app.createConfigurationContext(configuration)
        assertEquals("Inicio", InterfaceCopy.translate(context, "Home"))
    }

    @Test fun emptyLocaleListFallsBackWithoutCrashing() {
        val app = RuntimeEnvironment.getApplication()
        val configuration = Configuration(app.resources.configuration)
        configuration.setLocales(LocaleList())
        val context = app.createConfigurationContext(configuration)
        assertEquals("Home", InterfaceCopy.translate(context, "Home"))
    }

}
