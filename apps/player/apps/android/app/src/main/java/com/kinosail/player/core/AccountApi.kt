package com.kinosail.player.core

import java.net.HttpURLConnection
import java.net.URL
import java.time.Instant
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive

data class MediaPreferences(val playback: PlaybackPreferences, internal val other: JsonObject) {
    fun json() = JsonObject(other + ("playback" to playback.json()))

    companion object {
        private val KEYS = setOf("playback", "autoDownloadNext", "removeWatched", "downloadLimitGiB",
            "wifiOnly", "readerFontSize", "readerTheme")

        internal fun parse(raw: JsonElement): MediaPreferences {
            val value = raw as? JsonObject
            require(value != null && value.keys.all(KEYS::contains) && "playback" in value &&
                value.all { (key, field) -> key == "playback" || field is JsonPrimitive && field.content.length <= 32 }) {
                "The Server returned invalid playback preferences."
            }
            return MediaPreferences(PlaybackPreferences.parse(value.getValue("playback")), JsonObject(value - "playback"))
        }
    }
}

data class DeviceApproval(val code: String, val device: String, val expires: Instant)

class AccountApi(server: ServerAddress, open: (URL) -> HttpURLConnection = { it.openConnection() as HttpURLConnection }) {
    private val api = ServerApi(server, open)

    fun mediaPreferences(token: String, viewerId: String): MediaPreferences =
        MediaPreferences.parse(api.account(PREFERENCES, "GET", token, viewerId).second)

    fun saveMediaPreferences(token: String, viewerId: String, value: MediaPreferences): MediaPreferences =
        MediaPreferences.parse(api.account(PREFERENCES, "PUT", token, viewerId, value.json()).second)

    fun previewApproval(code: String, token: String, viewerId: String): DeviceApproval? {
        val (status, raw) = api.account("/api/v1/quick-connect/${checkedCode(code)}", "GET", token, viewerId,
            expected = setOf(200, 400, 404))
        if (status != 200) return null
        val value = raw as? JsonObject
        require(value != null && value.keys.all { it in setOf("code", "device", "expiresAt") }) { INVALID }
        fun text(key: String, max: Int) = (value[key] as? JsonPrimitive)?.takeIf { it.isString }?.content
            ?.takeIf { it.toByteArray(Charsets.UTF_8).size <= max && it.none(Char::isISOControl) }
        val returned = text("code", 6)
        val expires = text("expiresAt", 40)?.let { runCatching { Instant.parse(it) }.getOrNull() }
        require(returned == code && expires != null && (value["device"] == null || text("device", 80) != null)) { INVALID }
        return DeviceApproval(returned, text("device", 80).orEmpty(), expires)
    }

    fun approve(code: String, token: String, viewerId: String): Boolean =
        api.account("/api/v1/quick-connect/${checkedCode(code)}", "POST", token, viewerId,
            expected = setOf(204, 400, 404)).first == 204

    fun dismissContinueWatching(itemId: String, token: String, viewerId: String) {
        api.account("/api/v1/items/$itemId/continue-watching", "DELETE", token, viewerId, expected = setOf(204))
    }

    private fun checkedCode(code: String): String {
        require(code.matches(Regex("[0-9]{6}"))) { "Enter the six-digit code shown on your TV." }
        return code
    }

    private companion object {
        const val PREFERENCES = "/api/v1/me/media-preferences"
        const val INVALID = "The Server returned an invalid approval."
    }
}
