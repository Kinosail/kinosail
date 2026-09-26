package com.kinosail.player.core

import com.kinosail.player.watchcore.WatchPlayer
import com.kinosail.player.watchcore.WatchRequest
import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
import java.net.HttpURLConnection
import java.net.URL
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test

class RemotePlayersApiTest {
    private val server = ServerAddress("https://example.com")
    private val id = "11111111-2222-3333-4444-555555555555"
    private val movie = WatchPlayer(id, "Android TV", "Scary Movie", "", "movie-1", "playing", 20.0, 120.0, false)
    private val row = """{"id":"$id","name":"Android TV","title":"Scary Movie","artist":"","itemId":"movie-1","state":"playing","position":20,"duration":120,"audio":false}"""

    @Test fun listsViewerScopedPlayerAndPostsScopedCommand() {
        val list = RemoteResponse(200, """{"players":[$row]}""")
        assertEquals(listOf(movie), RemotePlayersApi(server) { list }.list("token", "alex"))
        assertEquals("alex", list.getRequestProperty("X-Kinosail-Viewer-Profile"))
        val response = RemoteResponse(202, """{"status":"queued"}""")
        RemotePlayersApi(server) { url -> response.also { it.requestedURL = url } }
            .command(movie, WatchRequest(id, "movie-1", "pause"), "token", "alex")
        assertEquals("POST", response.requestMethod)
        assertEquals("/api/v1/remote-players/$id/commands", response.requestedURL?.path)
        assertEquals(true, response.sent.toString().contains("\"itemId\":\"movie-1\""))
    }

    @Test fun publishesTvStateAndParsesQueuedCommand() {
        val response = RemoteResponse(200, """{"command":{"command":"seek","itemId":"movie-1","position":75}}""")
        val command = RemotePlayersApi(server) { url -> response.also { it.requestedURL = url } }
            .update(movie, "token", "alex")
        assertEquals(WatchRequest(id, "movie-1", "seek", 75.0), command)
        assertEquals("PUT", response.requestMethod)
        assertEquals("/api/v1/remote-players/$id", response.requestedURL?.path)
        assertEquals(true, response.sent.toString().contains("\"state\":\"playing\""))
        assertEquals("alex", response.getRequestProperty("X-Kinosail-Viewer-Profile"))
    }

    @Test fun rejectsInvalidInputBeforeNetworkSideEffects() {
        var opens = 0
        val api = RemotePlayersApi(server) { opens++; RemoteResponse(202, "{}") }
        assertThrows(IllegalArgumentException::class.java) {
            api.command(movie, WatchRequest(id, "other", "pause"), "token", "alex")
        }
        assertThrows(IllegalArgumentException::class.java) {
            api.command(movie, WatchRequest(id, "movie-1", "seek", 121.0), "token", "alex")
        }
        assertThrows(IllegalArgumentException::class.java) {
            api.command(movie.copy(id = "../bad"), WatchRequest(id, "movie-1", "pause"), "token", "alex")
        }
        assertThrows(IllegalArgumentException::class.java) { api.list("bad token", "alex") }
        assertThrows(IllegalArgumentException::class.java) { api.list("token", "bad/id") }
        assertThrows(IllegalArgumentException::class.java) { api.update(movie.copy(title = ""), "token", "alex") }
        assertEquals(0, opens)
    }

    @Test fun rejectsMalformedRemoteServerState() {
        for (body in listOf("{}", """{"players":[${row.replace("\"position\":20", "\"position\":200")}]}""",
            """{"players":[$row,$row]}""", """{"players":[$row],"unknown":1}""",
            """{"players":[$row],"players":[]}""")) {
            assertThrows(body, Exception::class.java) { RemotePlayersApi(server) { RemoteResponse(200, body) }.list("token", "alex") }
        }
        for (body in listOf("{}", """{"command":{"command":"seek","itemId":"movie-1"}}""",
            """{"command":{"command":"play","itemId":"other"}}""",
            """{"command":{"command":"play","itemId":"movie-1","extra":1}}""")) {
            assertThrows(body, Exception::class.java) { RemotePlayersApi(server) { RemoteResponse(200, body) }.update(movie, "token", "alex") }
        }
    }
}

private class RemoteResponse(private val status: Int, private val body: String) :
    HttpURLConnection(URL("https://example.com/")) {
    var requestedURL: URL? = null
    val sent = ByteArrayOutputStream()
    override fun connect() = Unit
    override fun disconnect() = Unit
    override fun usingProxy() = false
    override fun getResponseCode() = status
    override fun getContentType() = "application/json"
    override fun getInputStream() = ByteArrayInputStream(body.toByteArray())
    override fun getOutputStream() = sent
}
