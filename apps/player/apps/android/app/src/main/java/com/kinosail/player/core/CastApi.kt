package com.kinosail.player.core

import java.net.URI
import java.net.URL
import java.time.Instant
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.booleanOrNull
import kotlinx.serialization.json.doubleOrNull

data class CastTrack(val id: Long, val url: String, val label: String, val language: String, val isDefault: Boolean)
data class CastMedia(val id: String, val url: String, val contentType: String, val title: String,
                     val position: Double, val duration: Double, val tracks: List<CastTrack>)

class CastApi(private val server: ServerAddress, open: (URL) -> java.net.HttpURLConnection =
    { it.openConnection() as java.net.HttpURLConnection }) {
    private val api = ServerApi(server, open)

    fun receiverAppId(token: String, viewerId: String): String {
        val fields = api.castConfig(token, viewerId).objectFields(setOf("appId"), setOf("appId"))
        val id = fields.text("appId", 8)
        require(id.isEmpty() || id.matches(Regex("[A-Fa-f0-9]{8}"))) { INVALID_RESPONSE }
        return id.uppercase()
    }

    fun start(itemId: String, position: Double, token: String, viewerId: String): CastMedia {
        val fields = api.startCast(itemId, position, token, viewerId).objectFields(
            setOf("id", "url", "contentType", "title", "position", "duration", "expiresAt", "protocol",
                "tracks", "deviceId", "deviceName"),
            setOf("id", "url", "contentType", "title", "position", "duration", "expiresAt", "protocol", "tracks"))
        val id = fields.text("id", 32)
        require(id.matches(Regex("[a-f0-9]{32}")) && fields.text("protocol", 20) == "google-cast" &&
            fields["deviceId"] == null && fields["deviceName"] == null) { INVALID_RESPONSE }
        val url = checkedMediaURL(fields.text("url", 4096), "/cast/$id/media", "/cast/$id/hls/index.m3u8")
        val type = fields.text("contentType", 128)
        require(type.matches(Regex("(?:audio|video)/[a-z0-9.+-]+|application/vnd\\.apple\\.mpegurl"))) {
            INVALID_RESPONSE
        }
        val title = fields.text("title", 1024)
        val mediaPosition = fields.number("position")
        val duration = fields.number("duration")
        val expiry = try { Instant.parse(fields.text("expiresAt", 64)) }
            catch (_: Exception) { throw IllegalArgumentException(INVALID_RESPONSE) }
        require(title.isNotBlank() && mediaPosition in 0.0..31_536_000.0 &&
            duration in 0.0..31_536_000.0 && (duration == 0.0 || mediaPosition <= duration) &&
            expiry.isAfter(Instant.now()) && expiry.isBefore(Instant.now().plusSeconds(25 * 3600))) { INVALID_RESPONSE }
        val rows = fields["tracks"] as? JsonArray
        require(rows != null && rows.size <= 64) { INVALID_RESPONSE }
        val tracks = rows.mapIndexed { index, raw ->
            val track = raw.objectFields(setOf("id", "url", "label", "language", "default"),
                setOf("id", "url", "label", "language", "default"))
            require(track.number("id") == (index + 1).toDouble()) { INVALID_RESPONSE }
            val trackURL = checkedMediaURL(track.text("url", 4096), "/cast/$id/subtitles/${index + 1}")
            require(URI(trackURL).rawQuery == URI(url).rawQuery) { INVALID_RESPONSE }
            val label = track.text("label", 512)
            val language = track.text("language", 32)
            val selected = (track["default"] as? JsonPrimitive)?.takeIf { !it.isString }?.booleanOrNull
            require(label.isNotBlank() && selected != null && language.matches(Regex("[A-Za-z0-9_-]{0,32}"))) {
                INVALID_RESPONSE
            }
            CastTrack((index + 1).toLong(), trackURL, label, language.ifEmpty { "und" }, selected)
        }
        return CastMedia(id, url, type, title, mediaPosition, duration, tracks)
    }

    fun end(id: String, token: String, viewerId: String) = api.endCast(id, token, viewerId)

    private fun checkedMediaURL(raw: String, vararg paths: String): String {
        val uri = try { URI(raw) } catch (_: Exception) { throw IllegalArgumentException(INVALID_RESPONSE) }
        val base = server.url.toURI()
        require(uri.isAbsolute && uri.scheme == base.scheme && uri.host == base.host &&
            uri.port == base.port && uri.userInfo == null && uri.rawFragment == null &&
            uri.path in paths && uri.rawQuery?.matches(Regex("ticket=[a-f0-9]{64}")) == true) {
            INVALID_RESPONSE
        }
        return uri.toString()
    }

    private fun JsonElement.objectFields(allowed: Set<String>, required: Set<String>): JsonObject {
        val fields = this as? JsonObject
        require(fields != null && fields.keys.all(allowed::contains) && fields.keys.containsAll(required)) {
            INVALID_RESPONSE
        }
        return fields
    }

    private fun JsonObject.text(key: String, max: Int): String {
        val value = this[key] as? JsonPrimitive
        require(value != null && value.isString && value.content.toByteArray(Charsets.UTF_8).size <= max &&
            value.content.none(Char::isISOControl)) { INVALID_RESPONSE }
        return value.content
    }

    private fun JsonObject.number(key: String): Double {
        val value = (this[key] as? JsonPrimitive)?.takeIf { !it.isString }?.doubleOrNull
        require(value != null && value.isFinite()) { INVALID_RESPONSE }
        return value
    }

    companion object { private const val INVALID_RESPONSE = "The Server returned an invalid Cast response." }
}
