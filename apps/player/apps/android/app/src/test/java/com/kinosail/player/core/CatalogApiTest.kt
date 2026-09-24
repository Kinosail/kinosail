package com.kinosail.player.core

import java.io.ByteArrayInputStream
import java.net.HttpURLConnection
import java.net.URL
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class CatalogApiTest {
    private val server = ServerAddress("https://example.com")
    private val item = """{"id":"film-1","kind":"video","title":"Arrival","year":"2016","plot":"A visitor arrives.","artwork":"/art/film-1"}"""
    private fun page(items: String = item, total: Int = 1, offset: Int = 0) =
        """{"items":[$items],"total":$total,"offset":$offset,"limit":24,"view":"all","sort":"title","query":""}"""

    @Test fun listsAViewerScopedPageAndEncodesSearch() {
        val response = CatalogResponse(200, page().replace("\"query\":\"\"", "\"query\":\"star wars\""))
        val api = CatalogApi(server) { url -> response.also { it.requestedURL = url } }
        val result = api.list("token-1", "alex", " star wars ")
        assertEquals(listOf(CatalogItem("film-1", "video", "Arrival", "2016", "A visitor arrives.",
            "/art/film-1")), result.items)
        assertEquals(1, result.total)
        assertEquals("/api/v1/library", response.requestedURL?.path)
        assertTrue(response.requestedURL!!.query.contains("q=star+wars"))
        assertEquals("Bearer token-1", response.getRequestProperty("Authorization"))
        assertEquals("alex", response.getRequestProperty("X-Kinosail-Viewer-Profile"))
        assertTrue(response.closed && !response.instanceFollowRedirects)
    }

    @Test fun rejectsInvalidInputsBeforeOpeningAConnection() {
        var opens = 0
        val api = CatalogApi(server) { opens++; CatalogResponse(200, page()) }
        listOf("x".repeat(513), "bad\nquery").forEach {
            assertThrows(IllegalArgumentException::class.java) { api.list("token", "alex", it) }
        }
        assertThrows(IllegalArgumentException::class.java) { api.list("token", "alex", offset = -1) }
        assertThrows(IllegalArgumentException::class.java) { api.list("token", "bad viewer") }
        assertThrows(IllegalArgumentException::class.java) { api.list("bad token", "alex") }
        assertEquals(0, opens)
    }

    @Test fun acceptsTheLastPartialPageAtTheRequestedOffset() {
        val response = CatalogResponse(200, page(item, total = 25, offset = 24))
        val result = CatalogApi(server) { response }.list("token", "alex", offset = 24)
        assertEquals(24, result.offset)
        assertEquals(25, result.total)
        assertEquals(1, result.items.size)
    }

    @Test fun parsesResumePositionAndRejectsInvalidProgress() {
        val withProgress = item.replace("\"artwork\":\"/art/film-1\"",
            "\"artwork\":\"/art/film-1\",\"progress\":{\"seconds\":45,\"session\":\"previous\",\"revision\":2}")
        val result = CatalogApi(server) { CatalogResponse(200, page(withProgress)) }.list("token", "alex")
        assertEquals(45.0, result.items.single().progress.seconds, 0.0)
        listOf(withProgress.replace("\"seconds\":45", "\"seconds\":-1"),
            withProgress.replace("\"revision\":2", "\"revision\":\"2\""),
            withProgress.replace("\"session\":\"previous\"", "\"session\":\"bad\\n\""))
            .forEach { invalid -> assertThrows(Exception::class.java) {
                CatalogApi(server) { CatalogResponse(200, page(invalid)) }.list("token", "alex")
            } }
    }

    @Test fun rejectsMalformedUnknownAndConflictingPages() {
        val invalid = listOf(
            "{}", page().replace("\"limit\":24", "\"limit\":25"),
            page().replace("\"total\":1", "\"total\":0"),
            page().replace("\"title\":\"Arrival\"", "\"title\":\"\""),
            page().replace("/art/film-1", "https://other.example/art/film-1"),
            page().replace("\"kind\":\"video\"", "\"kind\":\"unknown\""),
            page().replace("\"query\":\"\"", "\"query\":\"different\""),
            page().replace("\"items\":[", "\"extra\":1,\"items\":["),
            page("$item,$item", 2),
            page().replace("\"title\":\"Arrival\"", "\"title\":\"Arrival\",\"title\":\"Other\""),
        )
        invalid.forEach { body ->
            val response = CatalogResponse(200, body)
            assertThrows(body, Exception::class.java) { CatalogApi(server) { response }.list("token", "alex") }
            assertTrue(response.closed)
        }
    }

    @Test fun rejectsRedirectAndOversizedResponses() {
        listOf(CatalogResponse(302, page()), CatalogResponse(200, "x".repeat(2 * 1024 * 1024 + 1)),
            CatalogResponse(200, page(), "text/html")).forEach { response ->
            assertThrows(Exception::class.java) { CatalogApi(server) { response }.list("token", "alex") }
            assertTrue(response.closed)
        }
    }
}

private class CatalogResponse(private val status: Int, private val body: String,
                              private val type: String = "application/json") :
    HttpURLConnection(URL("https://example.com/api/v1/library")) {
    var requestedURL: URL? = null
    var closed = false
    override fun connect() = Unit
    override fun disconnect() { closed = true }
    override fun usingProxy() = false
    override fun getResponseCode() = status
    override fun getContentType() = type
    override fun getContentLengthLong() = body.toByteArray().size.toLong()
    override fun getInputStream() = ByteArrayInputStream(body.toByteArray())
}
