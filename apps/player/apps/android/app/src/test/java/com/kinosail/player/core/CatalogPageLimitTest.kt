package com.kinosail.player.core

import java.io.ByteArrayInputStream
import java.net.HttpURLConnection
import java.net.URL
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

/** Home requests 36 category rows and 200 history rows; the small parity fixture misses these limits. */
class CatalogPageLimitTest {
    private val server = ServerAddress("https://example.com")

    @Test fun acceptsFullHomePagesAtTheirRequestedLimits() {
        for ((view, limit) in listOf("movies" to 25, "movies" to 36, "history" to 200)) {
            val response = PageLimitResponse(page(limit, limit, view))
            val result = CatalogApi(server) { response }.list("token", "alex", view = view, limit = limit)
            assertEquals(limit, result.items.size)
            assertEquals(limit, result.limit)
            assertEquals("film-1", result.items.first().id)
            assertEquals("film-$limit", result.items.last().id)
            assertTrue(response.closed)
            assertEquals("alex", response.getRequestProperty("X-Kinosail-Viewer-Profile"))
        }
    }

    @Test fun rejectsRowsBeyondTheRequestedLimitAndAllExistingPageBounds() {
        for (limit in listOf(1, 24, 36, 200)) {
            rejects(page(limit + 1, limit), limit)
            rejects(page(1, limit + 1), limit)
            rejects(page(2, limit, total = 1), limit)
            rejects(page(0, limit, total = 1), limit)
            rejects(page(2, limit).replace("film-2", "film-1"), limit)
            rejects(page(1, limit).replace("\"id\":", "\"unexpected\":true,\"id\":"), limit)
            rejects(page(1, limit).replace("\"title\":\"Fictional 1\"",
                "\"title\":\"Fictional 1\",\"stream\":\"https://other.example/media/film-1\""), limit)
        }
    }

    private fun rejects(body: String, limit: Int) {
        val response = PageLimitResponse(body)
        assertThrows(body, Exception::class.java) {
            CatalogApi(server) { response }.list("token", "alex", limit = limit)
        }
        assertTrue(response.closed)
    }

    private fun page(count: Int, limit: Int, view: String = "all", total: Int = count): String {
        val items = (1..count).joinToString(",") {
            """{"id":"film-$it","kind":"video","title":"Fictional $it"}"""
        }
        return """{"items":[$items],"total":$total,"offset":0,"limit":$limit,"view":"$view","sort":"title","query":""}"""
    }
}

private class PageLimitResponse(private val body: String) :
    HttpURLConnection(URL("https://example.com/api/v1/library")) {
    var closed = false
    override fun connect() = Unit
    override fun disconnect() { closed = true }
    override fun usingProxy() = false
    override fun getResponseCode() = 200
    override fun getContentType() = "application/json"
    override fun getInputStream() = ByteArrayInputStream(body.toByteArray(Charsets.UTF_8))
}
