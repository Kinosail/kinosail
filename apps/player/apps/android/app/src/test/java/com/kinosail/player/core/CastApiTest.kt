package com.kinosail.player.core

import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
import java.net.HttpURLConnection
import java.net.URL
import java.time.Instant
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class CastApiTest {
    private val server = ServerAddress("https://example.com")
    private val id = "a".repeat(32)
    private val ticket = "b".repeat(64)
    private val mediaURL = "https://example.com/cast/$id/media?ticket=$ticket"
    private val expires = Instant.now().plusSeconds(3600).toString()
    private val response get() = """{"id":"$id","url":"$mediaURL","contentType":"audio/mpeg","title":"Song","position":12,"duration":120,"expiresAt":"$expires","protocol":"google-cast","tracks":[{"id":1,"url":"https://example.com/cast/$id/subtitles/1?ticket=$ticket","label":"English","language":"en","default":true}]}"""

    @Test fun parsesScopedAudioAndReceiverConfiguration() {
        val configuration = CastResponse(200, """{"appId":"deadbeef"}""")
        assertEquals("DEADBEEF", CastApi(server) { configuration }.receiverAppId("token", "alex"))
        assertEquals("Bearer token", configuration.getRequestProperty("Authorization"))
        assertEquals("alex", configuration.getRequestProperty("X-Kinosail-Viewer-Profile"))
        assertTrue(configuration.closed)
        val cast = CastApi(server) { CastResponse(201, response) }.start("song-1", 12.0, "token", "alex")
        assertEquals(mediaURL, cast.url)
        assertEquals(1, cast.tracks.size)
        assertTrue(cast.tracks.single().isDefault)
    }

    @Test fun rejectsInvalidInputBeforeNetwork() {
        var opens = 0
        val api = CastApi(server) { opens++; CastResponse(201, response) }
        for (item in listOf("../song", "", "x".repeat(129))) assertThrows(IllegalArgumentException::class.java) {
            api.start(item, 0.0, "token", "alex")
        }
        for (position in listOf(-1.0, Double.NaN, Double.POSITIVE_INFINITY, 31_536_001.0))
            assertThrows(IllegalArgumentException::class.java) {
                api.start("song-1", position, "token", "alex")
            }
        assertThrows(IllegalArgumentException::class.java) { api.end("bad", "token", "alex") }
        assertThrows(IllegalArgumentException::class.java) { api.receiverAppId("bad token", "alex") }
        assertEquals(0, opens)
    }

    @Test fun rejectsMalformedOrExternalReceiverResponses() {
        val invalid = listOf("{}", response.replace(mediaURL, "https://other.example/cast/$id/media?ticket=$ticket"),
            response.replace("ticket=$ticket", "ticket=short"),
            response.replace("/cast/$id/media", "/cast/$id/%6dedia"),
            response.replace("ticket=$ticket", "ticket=$ticket&extra=1"),
            response.replace("/subtitles/1", "/subtitles/2"),
            response.replace("\"default\":true", "\"default\":\"true\""),
            response.replace("\"protocol\":\"google-cast\"", "\"protocol\":\"dlna\""),
            response.replace("\"duration\":120", "\"duration\":-1"),
            response.replace("\"title\":\"Song\"", "\"title\":\"Song\",\"unknown\":1"),
            response.replace(expires, Instant.now().minusSeconds(60).toString()),
            response.replace("\"position\":12", "\"position\":12,\"position\":13"))
        invalid.forEach { body -> assertThrows(body, Exception::class.java) {
            CastApi(server) { CastResponse(201, body) }.start("song-1", 12.0, "token", "alex")
        } }
        for (body in listOf("{}", """{"appId":"NOTHEX!!"}""", """{"appId":null}"""))
            assertThrows(body, Exception::class.java) {
                CastApi(server) { CastResponse(200, body) }.receiverAppId("token", "alex")
            }
    }
}

private class CastResponse(private val status: Int, private val body: String) :
    HttpURLConnection(URL("https://example.com/api/v1/cast/config")) {
    var closed = false
    private val sent = ByteArrayOutputStream()
    override fun connect() = Unit
    override fun disconnect() { closed = true }
    override fun usingProxy() = false
    override fun getResponseCode() = status
    override fun getContentType() = "application/json"
    override fun getInputStream() = ByteArrayInputStream(body.toByteArray())
    override fun getOutputStream() = sent
}
