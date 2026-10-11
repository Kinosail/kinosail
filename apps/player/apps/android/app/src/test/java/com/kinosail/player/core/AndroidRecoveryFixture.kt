package com.kinosail.player.core

import android.app.Application
import com.sun.net.httpserver.HttpServer
import java.net.InetSocketAddress
import java.net.URLDecoder
import java.security.Provider
import java.security.Security
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive

/** A real HTTP stand-in, with only Android's JVM-unavailable key lookup substituted. */
internal class AndroidRecoveryFixture(private val application: Application) : AutoCloseable {
    val viewer = Viewer("Recovery fixture", "recovery-server", "recovery-viewer", "Viewer")
    val catalogOffsets = CopyOnWriteArrayList<Int>()
    val catalogQueries = CopyOnWriteArrayList<String>()
    val completedCatalogQueries = CopyOnWriteArrayList<String>()
    val catalogTotals = ConcurrentHashMap<String, Int>()
    val catalogFailures = ConcurrentHashMap<String, Int>()
    val catalogGates = ConcurrentHashMap<String, CountDownLatch>()
    val remoteUpdates = CopyOnWriteArrayList<Pair<String, String>>()
    val playbackRequests = AtomicInteger()
    val requestPaths = CopyOnWriteArrayList<String>()
    val tvId = "454bcb89-1e30-4c48-9b9e-ce2b965e381b"
    @Volatile var failingOffset: Int? = null
    @Volatile var catalogFailure = 400
    @Volatile var playbackFailure = true
    @Volatile var duplicatePage = false
    @Volatile var catalogGate = CountDownLatch(0)
    @Volatile var playbackGate = CountDownLatch(0)
    @Volatile var progressResponse: ProgressResponse? = null

    class ProgressResponse(val status: Int, val remote: WatchProgress? = null) {
        val entered = CountDownLatch(1)
        val release = CountDownLatch(1)
        val written = CountDownLatch(1)
    }
    private val executor = Executors.newFixedThreadPool(2) { task -> Thread(task).apply { isDaemon = true } }
    private val server = HttpServer.create(InetSocketAddress("127.0.0.1", 0), 0).apply {
        executor = this@AndroidRecoveryFixture.executor
    }

    init {
        check(Security.getProvider("AndroidKeyStore") == null)
        Security.addProvider(object : Provider("AndroidKeyStore", 1.0, "JVM recovery key fixture") {
            init { put("KeyStore.AndroidKeyStore", PhotoScreenTest.FixtureKeyStore::class.java.name) }
        })
        SessionStore(application).clear()
        check(application.getSharedPreferences("kinosail_remote_player", 0).edit().putString("id", tvId).commit())
        server.createContext("/") { request ->
            val path = request.requestURI.path
            requestPaths += path
            val input = request.requestBody.use { String(it.readNBytes(4097), Charsets.UTF_8) }
            check(input.toByteArray().size <= 4096)
            val authorized = request.requestHeaders.getFirst("Authorization") == "Bearer recovery-fixture" &&
                (path == "/api/v1/me" || request.requestHeaders.getFirst("X-Kinosail-Viewer-Profile") == viewer.id)
            var status = if (authorized) 200 else 401
            var catalogQuery: String? = null
            var progressReply: ProgressResponse? = null
            val body = when {
                path == "/api/v1/me" ->
                    """{"server":"${viewer.server}","serverId":"${viewer.serverId}","viewer":{"id":"${viewer.id}","name":"${viewer.name}"}}"""
                path == "/api/v1/library" -> {
                    val query = request.requestURI.rawQuery.orEmpty().split('&').associate { value ->
                        val parts = value.split('=', limit = 2)
                        parts[0] to URLDecoder.decode(parts.getOrElse(1) { "" }, Charsets.UTF_8)
                    }
                    val offset = query["offset"]?.toInt() ?: 0
                    val limit = query["limit"]?.toInt() ?: 24
                    val search = query["q"].orEmpty()
                    catalogQuery = search
                    catalogOffsets += offset; catalogQueries += search
                    check((catalogGates[search] ?: catalogGate).await(5, TimeUnit.SECONDS))
                    if (offset == failingOffset) status = catalogFailure
                    catalogFailures[search]?.let { status = it }
                    val total = catalogTotals[search] ?: 96
                    check(total in 0..96)
                    val begin = if (duplicatePage && offset > 0) 0 else offset
                    val items = (begin until minOf(begin + limit, total)).joinToString(",") { item(it + 1) }
                    """{"items":[$items],"total":$total,"offset":$offset,"limit":$limit,"query":${JsonPrimitive(search)}}"""
                }
                path.endsWith("/playback") -> {
                    playbackRequests.incrementAndGet()
                    check(playbackGate.await(10, TimeUnit.SECONDS))
                    if (playbackFailure) status = 400
                    """{"policy":"automatic","plan":{"allowed":true,"mode":"direct","reason":"direct-preferred"},"directAllowed":true,"direct":"/media/film-1","directType":"video/mp4","duration":120}"""
                }
                path.startsWith("/api/v1/remote-players/") -> {
                    val value = StrictJson.parse(input) as JsonObject
                    remoteUpdates += path.substringAfterLast('/') to (value["itemId"] as JsonPrimitive).content
                    """{"command":null}"""
                }
                path.endsWith("/progress/sync") -> {
                    val reply = progressResponse
                    progressReply = reply
                    reply?.entered?.countDown()
                    check(reply == null || reply.release.await(10, TimeUnit.SECONDS))
                    if (reply != null) status = reply.status
                    when (reply?.status) {
                        409 -> """{"error":"Changed elsewhere","progress":${requireNotNull(reply.remote).json()}}"""
                        503 -> "{}"
                        else -> (StrictJson.parse(input) as JsonObject).getValue("progress").toString()
                    }
                }
                path.endsWith("/playback-preferences") -> "{}"
                path == "/api/v1/items/film-1" -> """{"item":${item(1)},"listed":false,"profileId":"${viewer.id}"}"""
                else -> { status = 404; "{}" }
            }.toByteArray(Charsets.UTF_8)
            request.responseHeaders.set("Content-Type", "application/json")
            request.sendResponseHeaders(status, body.size.toLong())
            request.responseBody.use { it.write(body) }
            progressReply?.written?.countDown()
            catalogQuery?.let { completedCatalogQueries += it }
        }
        server.start()
        SessionStore(application).save(SavedSession(ServerAddress("http://127.0.0.1:${server.address.port}"),
            "recovery-fixture", viewer))
    }

    private fun item(id: Int): String =
        """{"id":"film-$id","kind":"video","title":"Fictional $id","progress":{}}"""

    override fun close() {
        catalogGate.countDown()
        catalogGates.values.forEach { it.countDown() }
        playbackGate.countDown()
        progressResponse?.release?.countDown()
        server.stop(0)
        executor.shutdownNow()
        SessionStore(application).clear()
        Security.removeProvider("AndroidKeyStore")
    }
}
