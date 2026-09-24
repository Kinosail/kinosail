package com.kinosail.player.core

import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
import java.net.HttpURLConnection
import java.net.URL
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class ProgressApiTest {
    private val server = ServerAddress("https://example.com")
    private val expected = WatchProgress(12.0, false, "old", 2)
    private val update = WatchProgress(35.0, false, "new", 1)
    private val snapshot = """{"seconds":35,"watched":false,"session":"new","revision":1}"""

    @Test fun readsCurrentViewerProgressAndSynchronizes() {
        val current = ProgressResponse(200, """{"item":{"id":"film-1","progress":{"seconds":12,"session":"old","revision":2}},"listed":false,"profileId":"alex"}""")
        assertEquals(expected, ProgressApi(server) { current }.current("film-1", "token", "alex"))
        val response = ProgressResponse(200, snapshot)
        val result = ProgressApi(server) { response }.sync("film-1", "token", "alex", update, expected)
        assertFalse(result.conflict)
        assertEquals(update, result.progress)
        assertEquals("PUT", response.requestMethod)
        assertEquals("Bearer token", response.getRequestProperty("Authorization"))
        assertEquals("alex", response.getRequestProperty("X-Kinosail-Viewer-Profile"))
        assertTrue(response.body.toString(Charsets.UTF_8).contains("\"expected\":{\"seconds\":12.0"))
        assertTrue(response.closed && !response.instanceFollowRedirects)
    }

    @Test fun reportsConflictWithoutOverwritingTheRemotePosition() {
        val response = ProgressResponse(409, """{"error":"changed","progress":{"seconds":50,"watched":true,"session":"other","revision":4}}""")
        val result = ProgressApi(server) { response }.sync("film-1", "token", "alex", update, expected)
        assertTrue(result.conflict)
        assertEquals(50.0, result.progress.seconds, 0.0)
        assertEquals("other", result.progress.session)
    }

    @Test fun rejectsInvalidRequestsBeforeNetworkOrWrites() {
        var opens = 0
        val api = ProgressApi(server) { opens++; ProgressResponse(200, snapshot) }
        listOf(WatchProgress(-1.0, false, "s", 1), WatchProgress(1.0, false, "", 1),
            WatchProgress(1.0, false, "s", 0), WatchProgress(1.0, false, "s".repeat(129), 1),
            WatchProgress(1.0, false, "bad\n", 1), WatchProgress(Double.NaN, false, "s", 1),
            WatchProgress(31_536_001.0, false, "s", 1),
            WatchProgress(1.0, false, "s", 9_007_199_254_740_992)).forEach {
            assertThrows(IllegalArgumentException::class.java) { api.sync("film-1", "token", "alex", it, expected) }
        }
        assertThrows(IllegalArgumentException::class.java) { api.sync("../x", "token", "alex", update, expected) }
        assertThrows(IllegalArgumentException::class.java) { api.sync("film-1", "bad token", "alex", update, expected) }
        assertThrows(IllegalArgumentException::class.java) { api.sync("film-1", "token", "bad viewer", update, expected) }
        assertEquals(0, opens)
    }

    @Test fun rejectsMalformedRemoteProgressAndViewerMismatch() {
        val invalid = listOf(snapshot.replace("\"seconds\":35", "\"seconds\":-1"),
            snapshot.replace("\"revision\":1", "\"revision\":1.5"),
            snapshot.replace("\"watched\":false", "\"watched\":\"false\""),
            snapshot.replace("\"session\":\"new\"", "\"session\":\"bad\\n\""),
            snapshot.replace("\"revision\":1", "\"revision\":1,\"extra\":1"),
            snapshot.replace("\"seconds\":35", "\"seconds\":35,\"seconds\":36"))
        invalid.forEach { body ->
            val response = ProgressResponse(200, body)
            assertThrows(body, Exception::class.java) {
                ProgressApi(server) { response }.sync("film-1", "token", "alex", update, expected)
            }
            assertTrue(response.closed)
        }
        val wrongViewer = ProgressResponse(200,
            """{"item":{"id":"film-1","progress":{}},"listed":false,"profileId":"other"}""")
        assertThrows(IllegalArgumentException::class.java) {
            ProgressApi(server) { wrongViewer }.current("film-1", "token", "alex")
        }
    }
}

private class ProgressResponse(private val status: Int, private val content: String) :
    HttpURLConnection(URL("https://example.com/api/v1/items/film-1/progress/sync")) {
    val body = ByteArrayOutputStream()
    var closed = false
    override fun connect() = Unit
    override fun disconnect() { closed = true }
    override fun usingProxy() = false
    override fun getResponseCode() = status
    override fun getContentType() = "application/json"
    override fun getContentLengthLong() = content.toByteArray().size.toLong()
    override fun getInputStream() = if (status >= 400) throw java.io.FileNotFoundException() else
        ByteArrayInputStream(content.toByteArray())
    override fun getErrorStream() = if (status >= 400) ByteArrayInputStream(content.toByteArray()) else null
    override fun getOutputStream() = body
}
