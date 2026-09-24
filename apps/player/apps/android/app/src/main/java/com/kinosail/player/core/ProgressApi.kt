package com.kinosail.player.core

import java.net.HttpURLConnection
import java.net.URL
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.booleanOrNull
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put

data class ProgressResult(val progress: WatchProgress, val conflict: Boolean)

class ProgressApi(server: ServerAddress,
                  open: (URL) -> HttpURLConnection = { it.openConnection() as HttpURLConnection }) {
    private val api = ServerApi(server, open)

    fun current(itemId: String, token: String, viewerId: String): WatchProgress {
        val response = api.item(itemId, token, viewerId) as? JsonObject
        require(response != null && response.keys == setOf("item", "listed", "profileId") &&
            (response["listed"] as? JsonPrimitive)?.let { !it.isString && it.booleanOrNull != null } == true &&
            (response["profileId"] as? JsonPrimitive)?.let { it.isString && it.content == viewerId } == true) {
            "Invalid item response."
        }
        val item = response["item"] as? JsonObject
        require(item != null && (item["id"] as? JsonPrimitive)?.let { it.isString && it.content == itemId } == true &&
            item.containsKey("progress")) { "Invalid item response." }
        return WatchProgress.parse(item.getValue("progress"))
    }

    fun sync(itemId: String, token: String, viewerId: String, progress: WatchProgress,
             expected: WatchProgress): ProgressResult {
        progress.validated(required = true)
        expected.validated()
        val body = buildJsonObject {
            put("progress", progress.json()); put("expected", expected.json()); put("playbackToken", "")
        }
        val (status, raw) = api.syncProgress(itemId, token, viewerId, body)
        val snapshot = if (status == 409) {
            val fields = raw as? JsonObject
            require(fields != null && fields.keys == setOf("error", "progress") &&
                (fields["error"] as? JsonPrimitive)?.let { it.isString && it.content.length <= 512 } == true) {
                "Invalid progress response."
            }
            fields.getValue("progress")
        } else raw
        val fields = snapshot as? JsonObject
        require(fields != null && fields.keys.all { it in setOf("seconds", "watched", "session", "revision",
            "updated", "dismissed", "readerPage", "readerOffset") }) { "Invalid progress response." }
        return ProgressResult(WatchProgress.parse(snapshot), status == 409)
    }
}
