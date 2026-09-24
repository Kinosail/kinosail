package com.kinosail.player.core

import java.io.ByteArrayInputStream
import java.net.HttpURLConnection
import java.net.URL
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class ShowApiTest {
    private val server = ServerAddress("https://example.com")
    private val showId = "0123456789abcdef"
    private val first = """{"id":"episode-1","kind":"video","title":"Pilot","showId":"$showId","season":1,"episode":1,"progress":{"watched":true}}"""
    private val second = """{"id":"episode-2","kind":"video","title":"Next","showId":"$showId","season":1,"episode":2,"progress":{"seconds":45}}"""
    private val body = """{"id":"$showId","title":"The Show","year":"2026","plot":"A family story.","episodes":[$second,$first]}"""

    @Test fun loadsViewerScopedSeasonsAndChoosesNextUnwatched() {
        val response = ShowResponse(200, body)
        val detail = ShowApi(server) { url -> response.also { it.requestedURL = url } }
            .detail(showId, "token", "alex")
        assertEquals("The Show", detail.title)
        assertEquals(listOf("episode-1", "episode-2"), detail.episodes.map(CatalogItem::id))
        assertEquals("episode-2", detail.next?.id)
        assertEquals(listOf(1), detail.seasons)
        assertEquals("/api/v1/shows/$showId", response.requestedURL?.path)
        assertEquals("Bearer token", response.getRequestProperty("Authorization"))
        assertEquals("alex", response.getRequestProperty("X-Kinosail-Viewer-Profile"))
        assertTrue(response.closed && !response.instanceFollowRedirects)
    }

    @Test fun rejectsInvalidInputsBeforeNetwork() {
        var opens = 0
        val api = ShowApi(server) { opens++; ShowResponse(200, body) }
        listOf("bad", "../show", "F123456789abcdef", "a".repeat(17)).forEach {
            assertThrows(IllegalArgumentException::class.java) { api.detail(it, "token", "alex") }
        }
        assertThrows(IllegalArgumentException::class.java) { api.detail(showId, "bad token", "alex") }
        assertThrows(IllegalArgumentException::class.java) { api.detail(showId, "token", "bad viewer") }
        assertEquals(0, opens)
    }

    @Test fun rejectsUnknownDuplicateAndCrossShowEpisodes() {
        val bad = listOf("{}", body.replace(showId, "another-show"),
            body.replace("\"season\":1", "\"season\":-1"),
            body.replace("\"episode\":2", "\"episode\":\"2\""),
            body.replace("\"showId\":\"$showId\"", "\"showId\":\"fedcba9876543210\""),
            body.replace("\"episodes\":[", "\"extra\":1,\"episodes\":["),
            body.replace("$second,$first", "$second,$second"),
            body.replace("\"episodes\":[", "\"episodes\":[\"bad\","),
            body.replace("\"title\":\"The Show\"", "\"title\":\"\""),
            body.replace("\"title\":\"The Show\"", "\"title\":\"${"x".repeat(513)}\""),
            body.replace("\"episodes\":[", "\"episodes\":[\"oops\",\"oops\",\"oops\","))
        bad.forEach { invalid ->
            val response = ShowResponse(200, invalid)
            assertThrows(invalid.take(50), Exception::class.java) {
                ShowApi(server) { response }.detail(showId, "token", "alex")
            }
            assertTrue(response.closed)
        }
    }
}

private class ShowResponse(private val status: Int, private val content: String) :
    HttpURLConnection(URL("https://example.com/api/v1/shows/0123456789abcdef")) {
    var requestedURL: URL? = null
    var closed = false
    override fun connect() = Unit
    override fun disconnect() { closed = true }
    override fun usingProxy() = false
    override fun getResponseCode() = status
    override fun getContentType() = "application/json"
    override fun getContentLengthLong() = content.toByteArray().size.toLong()
    override fun getInputStream() = ByteArrayInputStream(content.toByteArray())
}
