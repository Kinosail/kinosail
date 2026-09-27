package com.kinosail.player.core

import android.content.Context
import android.os.Build
import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalContext
import java.util.Locale
import java.util.concurrent.ConcurrentHashMap
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive

internal object InterfaceCopy {
    private val catalogs = ConcurrentHashMap<String, Map<String, String>>()
    private val translated = setOf("es", "de", "fr", "pt-BR", "zh-Hans", "it", "nl", "pl", "ru", "ja",
        "ko", "ar", "tr", "uk", "pt-PT", "zh-Hant", "sv")

    fun translate(context: Context, english: String): String {
        val configuration = context.resources.configuration
        @Suppress("DEPRECATION")
        val locale = if (Build.VERSION.SDK_INT >= 24 && !configuration.locales.isEmpty) configuration.locales[0]
            else configuration.locale ?: Locale.ENGLISH
        val tag = language(locale) ?: return english
        return catalogs.getOrPut(tag) { load(context, tag) }[english] ?: english
    }

    private fun language(locale: Locale): String? {
        val exact = locale.toLanguageTag()
        if (exact in translated) return exact
        if (locale.language == "zh") {
            return if (locale.script == "Hant" || locale.country in setOf("TW", "HK", "MO")) "zh-Hant" else "zh-Hans"
        }
        if (locale.language == "pt") return if (locale.country == "PT") "pt-PT" else "pt-BR"
        return locale.language.takeIf(translated::contains)
    }

    private fun load(context: Context, tag: String): Map<String, String> = try {
        val source = context.assets.open("interface-copy/$tag.json").bufferedReader().use { it.readText() }
        (Json.parseToJsonElement(source) as JsonObject).mapValues { (_, value) -> (value as JsonPrimitive).content }
    } catch (_: Exception) {
        emptyMap()
    }
}

@Composable
internal fun interfaceText(english: String): String = InterfaceCopy.translate(LocalContext.current, english)
