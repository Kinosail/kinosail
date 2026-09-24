package com.kinosail.player.core

import java.net.HttpURLConnection
import java.net.URL
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.booleanOrNull
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.doubleOrNull
import kotlinx.serialization.json.put

data class PlaybackPreferences(
    val audioTrack: String,
    val subtitleTrack: String,
    val rate: Double,
    val audioLanguage: String,
    val subtitleLanguage: String,
    val nightMode: Boolean,
    val dialogueBoost: Boolean,
    val volumeBoost: Double,
) {
    init {
        require(listOf(audioTrack, subtitleTrack).all {
            it.toByteArray(Charsets.UTF_8).size <= 256 && it.none(Char::isISOControl)
        } && rate.isFinite() && rate in 0.5..3.0 && volumeBoost.isFinite() && volumeBoost in 1.0..2.0 &&
            validLanguage(audioLanguage, false) && validLanguage(subtitleLanguage, true)) {
            "Invalid playback preferences."
        }
    }

    fun json(): JsonObject = buildJsonObject {
        put("audioTrack", audioTrack)
        put("subtitleTrack", subtitleTrack)
        put("rate", rate)
        put("audioLanguage", audioLanguage)
        put("subtitleLanguage", subtitleLanguage)
        put("nightMode", nightMode)
        put("dialogueBoost", dialogueBoost)
        put("volumeBoost", volumeBoost)
    }

    companion object {
        private val LANGUAGE = Regex("[a-z]{2,3}(?:-[A-Za-z0-9]{2,8}){0,3}")
        private fun validLanguage(value: String, subtitle: Boolean): Boolean =
            value == "auto" || subtitle && value == "off" ||
                value.length <= 32 && value.matches(LANGUAGE)

        internal fun parse(raw: JsonElement): PlaybackPreferences {
            val value = raw as? JsonObject
            require(value != null && value.keys == KEYS) { INVALID_RESPONSE }
            return PlaybackPreferences(value.text("audioTrack"), value.text("subtitleTrack"),
                value.number("rate"), value.text("audioLanguage"), value.text("subtitleLanguage"),
                value.flag("nightMode"), value.flag("dialogueBoost"), value.number("volumeBoost"))
        }

        private val KEYS = setOf("audioTrack", "subtitleTrack", "rate", "audioLanguage",
            "subtitleLanguage", "nightMode", "dialogueBoost", "volumeBoost")
    }
}

class PlaybackPreferencesApi(
    server: ServerAddress,
    open: (URL) -> HttpURLConnection = { it.openConnection() as HttpURLConnection },
) {
    private val api = ServerApi(server, open)

    fun load(itemId: String, token: String, viewerId: String): PlaybackPreferences =
        parse(api.playbackPreferences(itemId, token, viewerId)).first

    fun save(itemId: String, token: String, viewerId: String, value: PlaybackPreferences): PlaybackPreferences {
        val (saved, overridden) = parse(api.playbackPreferences(itemId, token, viewerId, value.json()))
        require(overridden && saved == value) { INVALID_RESPONSE }
        return saved
    }

    private fun parse(raw: JsonElement): Pair<PlaybackPreferences, Boolean> {
        val value = raw as? JsonObject
        require(value != null && value.keys == setOf("playback", "overridden")) { INVALID_RESPONSE }
        return PlaybackPreferences.parse(value.getValue("playback")) to value.flag("overridden")
    }
}

private const val INVALID_RESPONSE = "The Server returned invalid playback preferences."

private fun JsonObject.text(key: String): String {
    val value = this[key] as? JsonPrimitive
    require(value != null && value.isString && value.content.toByteArray(Charsets.UTF_8).size <= 256 &&
        value.content.none(Char::isISOControl)) { INVALID_RESPONSE }
    return value.content
}

private fun JsonObject.flag(key: String): Boolean {
    val value = this[key] as? JsonPrimitive
    require(value != null && !value.isString && value.booleanOrNull != null) { INVALID_RESPONSE }
    return value.booleanOrNull!!
}

private fun JsonObject.number(key: String): Double {
    val value = this[key] as? JsonPrimitive
    require(value != null && !value.isString && value.doubleOrNull?.isFinite() == true) { INVALID_RESPONSE }
    return value.doubleOrNull!!
}
