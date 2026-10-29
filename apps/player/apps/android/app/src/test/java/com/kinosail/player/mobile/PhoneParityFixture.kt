package com.kinosail.player.mobile

import android.app.Application
import com.kinosail.player.core.PhotoScreenTest
import com.kinosail.player.core.SavedSession
import com.kinosail.player.core.ServerAddress
import com.kinosail.player.core.SessionStore
import com.kinosail.player.core.Viewer
import com.sun.net.httpserver.HttpServer
import java.net.InetSocketAddress
import java.security.Provider
import java.security.Security
import java.time.Instant
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.Executors

internal class PhoneParityFixture(private val application: Application) : AutoCloseable {
    val viewer = Viewer("Parity library", "parity-server", "parity-viewer", "Alex")
    val preferenceWrites = CopyOnWriteArrayList<String>()
    val requests = CopyOnWriteArrayList<String>()
    @Volatile var empty = false
    @Volatile var preferencesMode = "ready"
    @Volatile var approvals = 0
    @Volatile var dismissals = 0
    @Volatile var listed = false
    @Volatile private var preferences = """{"playback":{"audioTrack":"","subtitleTrack":"","rate":1,"audioLanguage":"auto","subtitleLanguage":"off","nightMode":false,"dialogueBoost":false,"volumeBoost":1},"autoDownloadNext":1,"removeWatched":true,"downloadLimitGiB":30,"wifiOnly":false,"readerFontSize":22,"readerTheme":"dark"}"""
    private val executor = Executors.newFixedThreadPool(2) { task -> Thread(task).apply { isDaemon = true } }
    private val server = HttpServer.create(InetSocketAddress("127.0.0.1", 0), 0).apply {
        executor = this@PhoneParityFixture.executor
    }
    private val continuing = item("film", "Continuing Film", 90)
    private val fresh = item("new-film", "New Film", 0)

    init {
        check(Security.getProvider("AndroidKeyStore") == null)
        Security.addProvider(object : Provider("AndroidKeyStore", 1.0, "JVM parity key fixture") {
            init { put("KeyStore.AndroidKeyStore", PhotoScreenTest.FixtureKeyStore::class.java.name) }
        })
        SessionStore(application).clear()
        application.getSharedPreferences("kinosail_tabs", 0).edit().clear().commit()
        server.createContext("/") { request ->
            val path = request.requestURI.path
            val method = request.requestMethod
            requests += "$method $path"
            val input = request.requestBody.use { String(it.readNBytes(4097), Charsets.UTF_8) }
            val authorized = request.requestHeaders.getFirst("Authorization") == "Bearer parity-fixture"
            var status = if (authorized) 200 else 401
            val body = when {
                path == "/api/v1/me" ->
                    """{"server":"${viewer.server}","serverId":"${viewer.serverId}","viewer":{"id":"${viewer.id}","name":"${viewer.name}"}}"""
                path == "/api/v1/library" -> {
                    val query = request.requestURI.rawQuery.orEmpty()
                    val view = Regex("view=([a-z]+)").find(query)?.groupValues?.get(1) ?: "all"
                    val items = if (empty) emptyList() else when (view) {
                        "history" -> listOf(continuing)
                        "movies", "all" -> listOf(fresh, continuing)
                        else -> emptyList()
                    }
                    val limit = Regex("limit=([0-9]+)").find(query)?.groupValues?.get(1) ?: "24"
                    val sort = Regex("sort=([a-z]+)").find(query)?.groupValues?.get(1) ?: "title"
                    """{"items":[${items.joinToString(",")}],"total":${items.size},"offset":0,"limit":$limit,"sort":"$sort","query":"","letters":null}"""
                }
                path == "/api/v1/me/media-preferences" -> when {
                    preferencesMode == "failed" -> { status = 503; "{}" }
                    method == "PUT" -> { preferences = input; preferenceWrites += input; preferences }
                    else -> preferences
                }
                path == "/api/v1/quick-connect/482913" -> if (method == "POST") { approvals++; status = 204; "" }
                    else """{"code":"482913","device":"Living room TV","expiresAt":"${Instant.now().plusSeconds(600)}"}"""
                path.startsWith("/api/v1/quick-connect/") -> { status = 400; """{"error":"Quick Connect code is invalid or expired"}""" }
                path.endsWith("/continue-watching") && method == "DELETE" -> { dismissals++; status = 204; "" }
                path.endsWith("/list") && method == "PUT" -> { listed = input.contains("true"); """{"listed":$listed}""" }
                path.endsWith("/watch-progress") -> """{"seconds":90,"duration":5400}"""
                path == "/api/v1/items/film" || path == "/api/v1/items/new-film" ->
                    """{"item":${if (path.endsWith("new-film")) fresh else continuing},"listed":$listed,"profileId":"${viewer.id}"}"""
                else -> { status = 404; "{}" }
            }.toByteArray(Charsets.UTF_8)
            request.responseHeaders.set("Content-Type", "application/json")
            request.sendResponseHeaders(status, if (status == 204) -1 else body.size.toLong())
            request.responseBody.use { if (status != 204) it.write(body) }
        }
        server.start()
        SessionStore(application).save(SavedSession(ServerAddress("http://127.0.0.1:${server.address.port}"),
            "parity-fixture", viewer))
    }

    private fun item(id: String, title: String, seconds: Int) =
        """{"id":"$id","kind":"video","title":"$title","year":"2026","plot":"A synthetic title.","genres":"Drama","progress":{"seconds":$seconds}}"""

    override fun close() {
        server.stop(0)
        executor.shutdownNow()
        SessionStore(application).clear()
        Security.removeProvider("AndroidKeyStore")
    }
}
