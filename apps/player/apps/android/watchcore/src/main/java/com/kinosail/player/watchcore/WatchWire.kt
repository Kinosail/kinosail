package com.kinosail.player.watchcore

import com.fasterxml.jackson.core.JsonFactory
import com.fasterxml.jackson.core.StreamReadConstraints
import com.fasterxml.jackson.core.StreamReadFeature
import java.nio.ByteBuffer
import java.nio.charset.CodingErrorAction
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.booleanOrNull
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.doubleOrNull
import kotlinx.serialization.json.put

data class WatchPlayer(
    val id: String, val name: String, val title: String, val subtitle: String, val itemId: String,
    val state: String, val position: Double, val duration: Double, val audio: Boolean,
) {
    val active get() = state != "idle"
    val playing get() = state == "playing"

    fun checked(): WatchPlayer {
        require(WATCH_ID.matches(id) && name.safeText(80) && name.isNotBlank() &&
            title.safeText(256) && subtitle.safeText(128) && position.isFinite() && duration.isFinite() &&
            duration in 0.0..1_000_000_000.0 && position in 0.0..duration && state in STATES) { INVALID }
        if (active) require(title.isNotBlank() && ITEM_ID.matches(itemId)) { INVALID }
        else require(title.isEmpty() && subtitle.isEmpty() && itemId.isEmpty() &&
            position == 0.0 && duration == 0.0 && !audio) { INVALID }
        return this
    }
}

data class WatchRequest(val target: String? = null, val itemId: String? = null,
                        val command: String? = null, val position: Double? = null) {
    fun checked(): WatchRequest {
        if (command == null) require(target == null && itemId == null && position == null) { INVALID }
        else {
            require(target != null && WATCH_ID.matches(target) && itemId != null && ITEM_ID.matches(itemId) &&
                command in COMMANDS) { INVALID }
            if (command == "seek") require(position != null && position.isFinite() && position in 0.0..1_000_000_000.0) { INVALID }
            else require(position == null) { INVALID }
        }
        return this
    }
}

data class WatchReply(val players: List<WatchPlayer>, val accepted: Boolean, val message: String? = null) {
    fun checked(): WatchReply {
        require(players.size <= 64 && players.map { it.checked().id }.toSet().size == players.size &&
            (message == null || message.safeText(256))) { INVALID }
        return this
    }
}

fun List<WatchPlayer>.selectedId(previous: String?): String? =
    previous ?: firstOrNull { it.active }?.id ?: firstOrNull()?.id

object WatchWire {
    const val PATH = "/kinosail/watch-remote/v1"
    private const val MAX_BYTES = 65_536
    private val json = Json { isLenient = false }
    private val parser = JsonFactory.builder()
        .enable(StreamReadFeature.STRICT_DUPLICATE_DETECTION)
        .streamReadConstraints(StreamReadConstraints.builder().maxNestingDepth(8)
            .maxStringLength(4096).maxNumberLength(32).build()).build()

    fun request(bytes: ByteArray): WatchRequest {
        val value = decode(bytes).objectWith(setOf("target", "itemId", "command", "position"))
        return WatchRequest(value.stringOrNull("target", 36), value.stringOrNull("itemId", 128),
            value.stringOrNull("command", 16), value.numberOrNull("position")).checked()
    }

    fun requestBytes(value: WatchRequest): ByteArray {
        value.checked()
        return buildJsonObject {
            value.target?.let { put("target", it) }
            value.itemId?.let { put("itemId", it) }
            value.command?.let { put("command", it) }
            value.position?.let { put("position", it) }
        }.toString().toByteArray(Charsets.UTF_8)
    }

    fun reply(bytes: ByteArray): WatchReply {
        val value = decode(bytes).objectWith(setOf("players", "accepted", "message"), setOf("players", "accepted"))
        val rows = value["players"] as? JsonArray ?: throw IllegalArgumentException(INVALID)
        require(rows.size <= 64) { INVALID }
        val players = rows.map { row ->
            val item = row.objectWith(PLAYER_FIELDS, PLAYER_FIELDS)
            WatchPlayer(item.string("id", 36), item.string("name", 80), item.string("title", 256),
                item.string("subtitle", 128), item.string("itemId", 128), item.string("state", 16),
                item.number("position"), item.number("duration"), item.flag("audio")).checked()
        }
        return WatchReply(players, value.flag("accepted"), value.stringOrNull("message", 256)).checked()
    }

    fun replyBytes(value: WatchReply): ByteArray {
        value.checked()
        val rows = value.players.map { player -> buildJsonObject {
            put("id", player.id); put("name", player.name); put("title", player.title)
            put("subtitle", player.subtitle); put("itemId", player.itemId); put("state", player.state)
            put("position", player.position); put("duration", player.duration); put("audio", player.audio)
        } }
        val bytes = buildJsonObject {
            put("players", JsonArray(rows)); put("accepted", value.accepted)
            value.message?.let { put("message", it) }
        }.toString().toByteArray(Charsets.UTF_8)
        require(bytes.size <= MAX_BYTES) { INVALID }
        return bytes
    }

    private fun decode(bytes: ByteArray): JsonElement {
        require(bytes.isNotEmpty() && bytes.size <= MAX_BYTES) { INVALID }
        val text = Charsets.UTF_8.newDecoder().onMalformedInput(CodingErrorAction.REPORT)
            .onUnmappableCharacter(CodingErrorAction.REPORT).decode(ByteBuffer.wrap(bytes)).toString()
        parser.createParser(bytes).use { stream -> while (stream.nextToken() != null) {} }
        return json.parseToJsonElement(text)
    }

    private val PLAYER_FIELDS = setOf("id", "name", "title", "subtitle", "itemId", "state", "position", "duration", "audio")
}

private const val INVALID = "Invalid watch remote message."
private val WATCH_ID = Regex("phone|[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}")
private val ITEM_ID = Regex("[A-Za-z0-9_-]{1,128}")
private val STATES = setOf("idle", "playing", "paused", "buffering")
private val COMMANDS = setOf("play", "pause", "backward", "forward", "seek")
private fun String.safeText(max: Int) = toByteArray(Charsets.UTF_8).size <= max && none(Char::isISOControl)

private fun JsonElement.objectWith(allowed: Set<String>, required: Set<String> = emptySet()): JsonObject {
    val value = this as? JsonObject
    require(value != null && value.keys.all(allowed::contains) && value.keys.containsAll(required)) { INVALID }
    return value
}
private fun JsonObject.string(key: String, max: Int): String = stringOrNull(key, max) ?: throw IllegalArgumentException(INVALID)
private fun JsonObject.stringOrNull(key: String, max: Int): String? {
    val raw = this[key] ?: return null
    require(raw !== JsonNull) { INVALID }
    val value = raw as? JsonPrimitive
    require(value != null && value.isString && value.content.safeText(max)) { INVALID }
    return value.content
}
private fun JsonObject.number(key: String): Double = numberOrNull(key) ?: throw IllegalArgumentException(INVALID)
private fun JsonObject.numberOrNull(key: String): Double? {
    val raw = this[key] ?: return null
    val value = raw as? JsonPrimitive
    require(value != null && !value.isString && value.doubleOrNull?.isFinite() == true) { INVALID }
    return value.doubleOrNull
}
private fun JsonObject.flag(key: String): Boolean {
    val value = this[key] as? JsonPrimitive
    require(value != null && !value.isString && value.booleanOrNull != null) { INVALID }
    return value.booleanOrNull!!
}
