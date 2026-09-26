package com.kinosail.player.core

import com.kinosail.player.watchcore.WatchPlayer
import com.kinosail.player.watchcore.WatchRequest
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.booleanOrNull
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.doubleOrNull
import kotlinx.serialization.json.put

internal class RemotePlayersApi(private val server: ServerAddress,
                                open: (java.net.URL) -> java.net.HttpURLConnection =
                                    { it.openConnection() as java.net.HttpURLConnection }) {
    private val api = ServerApi(server, open)

    fun list(token: String, viewerId: String): List<WatchPlayer> {
        val result = api.remotePlayers(token, viewerId).fields(setOf("players"), setOf("players"))
        val rows = result["players"] as? JsonArray ?: throw IllegalArgumentException(INVALID)
        require(rows.size <= 64) { INVALID }
        val players = rows.map { raw ->
            val value = raw.fields(PLAYER_FIELDS, PLAYER_FIELDS)
            WatchPlayer(value.text("id", 36), value.text("name", 80), value.text("title", 256),
                value.text("artist", 128), value.text("itemId", 128), value.text("state", 16),
                value.number("position"), value.number("duration"), value.flag("audio")).checked()
        }
        require(players.map(WatchPlayer::id).toSet().size == players.size && players.none { it.id == "phone" }) { INVALID }
        return players
    }

    fun update(player: WatchPlayer, token: String, viewerId: String): WatchRequest? {
        player.checked()
        require(player.id != "phone") { INVALID }
        val body = buildJsonObject {
            put("name", player.name); put("title", player.title); put("artist", player.subtitle)
            put("itemId", player.itemId); put("state", player.state)
            put("position", player.position); put("duration", player.duration); put("audio", player.audio)
        }
        val result = api.updateRemotePlayer(player.id, token, viewerId, body).fields(setOf("command"), setOf("command"))
        val command = result["command"]
        if (command == JsonNull) return null
        val value = (command ?: throw IllegalArgumentException(INVALID)).fields(COMMAND_FIELDS,
            setOf("command", "itemId"))
        val request = WatchRequest(player.id, value.text("itemId", 128), value.text("command", 16),
            value["position"]?.let { value.number("position") }).checked()
        require(player.active && request.itemId == player.itemId &&
            (request.position?.let { it <= player.duration } ?: true)) { INVALID }
        return request
    }

    fun command(player: WatchPlayer, request: WatchRequest, token: String, viewerId: String) {
        player.checked(); request.checked()
        val position = request.position
        require(player.id != "phone" && player.active && request.target == player.id &&
            request.itemId == player.itemId && (position == null || position <= player.duration)) { INVALID }
        val body = buildJsonObject {
            put("command", requireNotNull(request.command)); put("itemId", player.itemId)
            request.position?.let { put("position", it) }
        }
        api.commandRemotePlayer(player.id, token, viewerId, body)
    }

    private companion object {
        const val INVALID = "The Server returned an invalid remote player response."
        val PLAYER_FIELDS = setOf("id", "name", "title", "artist", "itemId", "state", "position", "duration", "audio")
        val COMMAND_FIELDS = setOf("command", "itemId", "position")
    }

    private fun JsonElement.fields(allowed: Set<String>, required: Set<String>): JsonObject {
        val value = this as? JsonObject
        require(value != null && value.keys.all(allowed::contains) && value.keys.containsAll(required)) { INVALID }
        return value
    }

    private fun JsonObject.text(key: String, max: Int): String {
        val value = this[key] as? JsonPrimitive
        require(value != null && value.isString && value.content.toByteArray(Charsets.UTF_8).size <= max &&
            value.content.none(Char::isISOControl)) { INVALID }
        return value.content
    }

    private fun JsonObject.number(key: String): Double {
        val value = (this[key] as? JsonPrimitive)?.takeIf { !it.isString }?.doubleOrNull
        require(value != null && value.isFinite()) { INVALID }
        return value
    }

    private fun JsonObject.flag(key: String): Boolean {
        val value = (this[key] as? JsonPrimitive)?.takeIf { !it.isString }?.booleanOrNull
        require(value != null) { INVALID }
        return value
    }
}
