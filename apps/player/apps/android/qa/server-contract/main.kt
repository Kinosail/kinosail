package com.kinosail.player.core

import java.net.HttpURLConnection
import java.net.URL
import java.nio.file.Files
import java.nio.file.Path
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put

/** Real loopback Server contract journey; no Android UI or codec invocation. */
fun main(args: Array<String>) {
    val server = ServerAddress(args[0])
    val output = Path.of(args[1])
    val results = mutableListOf<JsonObject>()
    val token = "disposable-fixture"
    val viewer = "local-owner"
    fun get(path: String): String {
        val connection = URL(server.url, path).openConnection() as HttpURLConnection
        return try {
            connection.connectTimeout = 5_000
            connection.readTimeout = 10_000
            check(connection.responseCode == 200)
            connection.inputStream.use { String(it.readNBytes(2 * 1024 * 1024 + 1), Charsets.UTF_8) }
                .also { check(it.toByteArray().size <= 2 * 1024 * 1024) }
        } finally { connection.disconnect() }
    }
    fun checkCase(name: String, action: () -> Unit) {
        var error = ""
        try { action() } catch (failure: Exception) { error = failure.javaClass.simpleName }
        results += buildJsonObject { put("case", name); put("passed", error.isEmpty()); put("errorClass", error) }
        println("$name: ${if (error.isEmpty()) "PASS" else "FAIL ($error)"}")
    }
    val all = get("/api/v1/library?limit=200")
    Files.writeString(output.resolve("server-library-200.json"), all)
    val rawItems = (StrictJson.parse(all) as JsonObject)["items"] as JsonArray
    check(rawItems.size == 200)
    val itemId = ((rawItems.first() as JsonObject)["id"] as JsonPrimitive).content
    val capabilities = PlaybackCapabilities(listOf("h264"), listOf("aac"), listOf("sdr"), 2)
    val playback = get("/api/v1/items/$itemId/playback?${capabilities.query}")
    Files.writeString(output.resolve("server-playback.json"), playback)
    val progress = ProgressApi(server)
    rawItems.forEach { raw ->
        val id = ((raw as JsonObject)["id"] as JsonPrimitive).content
        val saved = progress.sync(id, token, viewer, WatchProgress(1.0, session = "fixture", revision = 1),
            WatchProgress())
        check(!saved.conflict && saved.progress.seconds == 1.0)
    }
    val history = get("/api/v1/library?view=history&limit=200")
    Files.writeString(output.resolve("server-history-200.json"), history)
    check(((StrictJson.parse(history) as JsonObject)["items"] as JsonArray).size == 200)
    for (attempt in 1..2) {
        checkCase("R01-real-server-playback-$attempt") {
            val source = PlaybackApi(server).source(itemId, token, viewer, capabilities)
            check(source.direct == "/media/$itemId" && source.duration > 0)
            val connection = URL(server.url, source.direct!!).openConnection() as HttpURLConnection
            try {
                check(connection.responseCode == 200)
                check(connection.inputStream.use { it.readNBytes(1_048_577) }.size in 1..1_048_576)
            } finally { connection.disconnect() }
        }
        checkCase("R02-real-server-category-36-$attempt") {
            val page = CatalogApi(server).list(token, viewer, view = "movies", sort = "added", limit = 36)
            check(page.items.size == 36 && page.limit == 36 && page.total == 200)
        }
        checkCase("R02-real-server-library-200-$attempt") {
            val page = CatalogApi(server).list(token, viewer, limit = 200)
            check(page.items.size == 200 && page.limit == 200 && page.total == 200)
        }
        checkCase("R02-real-server-history-200-$attempt") {
            val page = CatalogApi(server).list(token, viewer, view = "history", limit = 200)
            check(page.items.size == 200 && page.limit == 200 && page.total == 200)
            check(page.items.all { it.progress.seconds == 1.0 })
        }
    }
    Files.writeString(output.resolve("android-result.json"), JsonArray(results).toString())
    check(results.all { (it["passed"] as JsonPrimitive).content == "true" }) {
        "Android Server contract journey failed."
    }
}
