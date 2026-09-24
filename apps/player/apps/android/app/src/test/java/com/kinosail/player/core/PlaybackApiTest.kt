package com.kinosail.player.core

import java.io.ByteArrayInputStream
import java.net.HttpURLConnection
import java.net.URL
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class PlaybackApiTest {
    private val server = ServerAddress("https://example.com")
    private val capabilities = PlaybackCapabilities(listOf("h264", "hevc"), listOf("aac", "mp3"), listOf("sdr"), 2)
    private val response = """{"plan":{"allowed":true,"mode":"direct","reason":"direct-preferred"},"directAllowed":true,"direct":"/media/film-1","directType":"video/mp4","duration":120,"start":30,"compatible":"/hls/film-1/p/r-a0-s0-none-t0-b0/index.m3u8","compatiblePlan":{"allowed":true,"mode":"remux","reason":"compatibility"},"progressToken":"abc"}"""

    @Test fun fetchesViewerScopedPlanAndSources() {
        val connection = PlaybackResponse(200, response)
        val source = PlaybackApi(server) { url -> connection.also { it.requestedURL = url } }
            .source("film-1", "token", "alex", capabilities)
        assertEquals("/media/film-1", source.direct)
        assertEquals("/hls/film-1/p/r-a0-s0-none-t0-b0/index.m3u8", source.compatible)
        assertEquals(30.0, source.start, 0.0)
        assertEquals("Bearer token", connection.getRequestProperty("Authorization"))
        assertEquals("alex", connection.getRequestProperty("X-Kinosail-Viewer-Profile"))
        assertTrue(connection.requestedURL.toString().contains(capabilities.query))
        assertTrue(connection.closed)
        assertFalse(connection.instanceFollowRedirects)
    }

    @Test fun rejectsInvalidInputsBeforeNetwork() {
        var opens = 0
        val api = PlaybackApi(server) { opens++; PlaybackResponse(200, response) }
        assertThrows(IllegalArgumentException::class.java) { api.source("../x", "token", "alex", capabilities) }
        assertThrows(IllegalArgumentException::class.java) { api.source("film-1", "bad token", "alex", capabilities) }
        assertThrows(IllegalArgumentException::class.java) { api.source("film-1", "token", "bad viewer", capabilities) }
        assertThrows(IllegalArgumentException::class.java) {
            PlaybackCapabilities(listOf("h264", "h264"), listOf("aac"), listOf("sdr"), 2)
        }
        assertThrows(IllegalArgumentException::class.java) {
            PlaybackCapabilities(listOf("unknown"), listOf("aac"), listOf("sdr"), 2)
        }
        assertThrows(IllegalArgumentException::class.java) {
            PlaybackCapabilities(listOf("h264"), listOf("aac"), listOf("sdr"), 9)
        }
        assertEquals(0, opens)
    }

    @Test fun rejectsMalformedConflictingAndExternalPlans() {
        val invalid = listOf("{}", response.replace("/media/film-1", "https://evil.example/media/film-1"),
            response.replace("/hls/film-1/", "/hls/another/"),
            response.replace("\"directAllowed\":true", "\"directAllowed\":false"),
            response.replace("\"mode\":\"remux\"", "\"mode\":\"denied\""),
            response.replace("\"start\":30", "\"start\":-1"),
            response.replace("\"mode\":\"remux\"", "\"mode\":\"remux\",\"markerMode\":\"unknown\""),
            response.replace("\"duration\":120", "\"duration\":\"120\""),
            response.replace("\"directType\":\"video/mp4\"", "\"directType\":\"\""),
            response.replace("\"directType\":\"video/mp4\"", "\"unknown\":1"),
            response.replace("\"duration\":120", "\"duration\":120,\"duration\":121"))
        invalid.forEach { body ->
            val connection = PlaybackResponse(200, body)
            assertThrows(body, Exception::class.java) {
                PlaybackApi(server) { connection }.source("film-1", "token", "alex", capabilities)
            }
            assertTrue(connection.closed)
        }
    }

    @Test fun rejectsRedirectWrongContentTypeAndOversizedPlan() {
        listOf(PlaybackResponse(302, response), PlaybackResponse(200, response, "text/html"),
            PlaybackResponse(200, "x".repeat(2 * 1024 * 1024 + 1))).forEach { connection ->
            assertThrows(Exception::class.java) {
                PlaybackApi(server) { connection }.source("film-1", "token", "alex", capabilities)
            }
            assertTrue(connection.closed)
        }
    }
}

private class PlaybackResponse(private val status: Int, private val body: String,
                               private val type: String = "application/json") :
    HttpURLConnection(URL("https://example.com/api/v1/items/film-1/playback")) {
    lateinit var requestedURL: URL
    var closed = false
    override fun connect() = Unit
    override fun disconnect() { closed = true }
    override fun usingProxy() = false
    override fun getResponseCode() = status
    override fun getContentType() = type
    override fun getContentLengthLong() = body.toByteArray().size.toLong()
    override fun getInputStream() = ByteArrayInputStream(body.toByteArray())
}
