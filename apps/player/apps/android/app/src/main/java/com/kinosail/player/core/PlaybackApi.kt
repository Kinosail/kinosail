package com.kinosail.player.core

import java.net.HttpURLConnection
import java.net.URL
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.booleanOrNull
import kotlinx.serialization.json.doubleOrNull

data class PlaybackSource(
    val itemId: String,
    val direct: String?,
    val compatible: String?,
    val directType: String,
    val duration: Double,
    val start: Double,
    val compatibleStart: Double,
    val progressToken: String,
    val compatibleTimeline: MediaTimeline,
    val nextItemId: String?,
) {
    fun sourceTime(position: Double, compatible: Boolean): Double =
        if (compatible) compatibleTimeline.sourceTime(position) else position
}

class PlaybackApi(
    server: ServerAddress,
    open: (URL) -> HttpURLConnection = { it.openConnection() as HttpURLConnection },
) {
    private val api = ServerApi(server, open)

    fun source(itemId: String, token: String, viewerId: String, capabilities: PlaybackCapabilities): PlaybackSource {
        require(itemId.matches(ID)) { "Invalid playback request." }
        val value = api.playback("/api/v1/items/$itemId/playback?${capabilities.query}", token, viewerId)
            .fields(PLAYBACK_KEYS, setOf("plan", "directAllowed", "duration", "start"))
        val plan = value.getValue("plan").fields(PLAN_KEYS, setOf("allowed", "mode", "reason"))
        require(plan.flag("allowed") == (plan.text("mode", 32) != "denied") &&
            plan.text("mode", 32) in MODES && plan.text("reason", 128).isNotEmpty() &&
            plan.text("markerMode", 32) in MARKER_MODES) { INVALID_RESPONSE }
        val directPath = value.text("direct", 16_384)
        val directAllowed = value.flag("directAllowed")
        require(directAllowed == directPath.isNotEmpty() &&
            (!directAllowed || directPath == "/media/$itemId")) { INVALID_RESPONSE }
        val compatiblePath = value.text("compatible", 16_384)
        var timeline: MediaTimeline? = null
        if (compatiblePath.isNotEmpty()) {
            require(compatiblePath.matches(Regex("/hls/${Regex.escape(itemId)}/p/[A-Za-z0-9_.-]{1,8192}/index\\.m3u8"))) {
                INVALID_RESPONSE
            }
            val fallback = value.getValue("compatiblePlan").fields(PLAN_KEYS, setOf("allowed", "mode", "reason"))
            require(fallback.flag("allowed") && fallback.text("mode", 32) in MODES - "direct" - "denied" &&
                fallback.text("reason", 128).isNotEmpty() &&
                fallback.text("markerMode", 32) in MARKER_MODES) { INVALID_RESPONSE }
            if (fallback.text("markerMode", 32) == "server") {
                timeline = MediaTimeline.parse(fallback.getValue("timeline"))
                val compatibleDuration = value.number("compatibleDuration", 0.0..31_536_000.0)
                require(kotlin.math.abs(timeline.duration - compatibleDuration) < 0.01 &&
                    value.text("compatibleProgressToken", 8192).isNotEmpty()) { INVALID_RESPONSE }
            }
        } else require(value["compatiblePlan"] == null) { INVALID_RESPONSE }
        require(directAllowed || compatiblePath.isNotEmpty()) { INVALID_RESPONSE }
        val duration = value.number("duration", 0.0..1_000_000_000.0)
        val start = value.number("start", 0.0..1_000_000_000.0)
        val type = value.text("directType", 128)
        require(directAllowed == type.isNotEmpty()) { INVALID_RESPONSE }
        val progressToken = value.text("progressToken", 8192)
        val next = value.text("next", 128)
        require(next.isEmpty() || next.matches(ID) && next != itemId) { INVALID_RESPONSE }
        val safeStart = if (start < duration) start else 0.0
        val compatibleTimeline = timeline ?: MediaTimeline(duration, duration)
        return PlaybackSource(itemId, directPath.ifEmpty { null }, compatiblePath.ifEmpty { null },
            type, compatibleTimeline.sourceDuration, safeStart, compatibleTimeline.presentationTime(safeStart),
            progressToken, compatibleTimeline, next.ifEmpty { null })
    }

    companion object {
        private const val INVALID_RESPONSE = "The Server returned an invalid playback plan."
        private val ID = Regex("[A-Za-z0-9_-]{1,128}")
        private val MODES = setOf("direct", "remux", "audio-transcode", "transcode", "denied")
        private val MARKER_MODES = setOf("", "unavailable", "server")
        private val PLAYBACK_KEYS = setOf("media", "plan", "compatiblePlan", "compatibleLabel", "compatibleDescription",
            "qualities", "directAllowed", "direct", "compatibleDuration", "compatibleProgressToken", "compatible",
            "download", "directType", "summary", "duration", "start", "audio", "chapters", "markers", "autoSkip",
            "subtitles", "next", "downloadNext", "trickplay", "progressToken", "replayGain")
        private val PLAN_KEYS = setOf("allowed", "mode", "reason", "container", "videoCodec", "audioCodec", "subtitleMode",
            "colorMode", "audioIndex", "subtitleIndex", "subtitleSourceIndex", "subtitleText", "subtitleExternal",
            "subtitleExternalIndex", "maxBitrate", "width", "height", "adaptive", "qualities", "markerMode", "timeline")

        private fun JsonElement.fields(allowed: Set<String>, required: Set<String>): JsonObject {
            val value = this as? JsonObject
            require(value != null && value.keys.all(allowed::contains) && value.keys.containsAll(required)) {
                INVALID_RESPONSE
            }
            return value
        }

        private fun JsonObject.text(key: String, maximum: Int): String {
            val value = this[key] ?: return ""
            val text = value as? JsonPrimitive
            require(text != null && text.isString && text.content.toByteArray(Charsets.UTF_8).size <= maximum &&
                text.content.none(Char::isISOControl)) { INVALID_RESPONSE }
            return text.content
        }

        private fun JsonObject.flag(key: String): Boolean {
            val value = (this[key] as? JsonPrimitive)?.booleanOrNull
            require(value != null) { INVALID_RESPONSE }
            return value
        }

        private fun JsonObject.number(key: String, range: ClosedFloatingPointRange<Double>): Double {
            val primitive = this[key] as? JsonPrimitive
            val value = if (primitive?.isString == false) primitive.doubleOrNull else null
            require(value != null && value.isFinite() && value in range) { INVALID_RESPONSE }
            return value
        }
    }
}
