package com.kinosail.player.core

import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
import java.net.HttpURLConnection
import java.net.URL
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
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

    @Test fun browsesShowsAndRejectsInvalidViewOrShowMetadata() {
        val episode = item.replace("\"artwork\":\"/art/film-1\"",
            "\"artwork\":\"/art/film-1?variant=episode\",\"showId\":\"0123456789abcdef\",\"season\":2,\"episode\":3")
        val body = page(episode).replace("\"view\":\"all\"", "\"view\":\"shows\"")
        val response = CatalogResponse(200, body)
        val result = CatalogApi(server) { url -> response.also { it.requestedURL = url } }
            .list("token", "alex", view = "shows")
        assertEquals("0123456789abcdef", result.items.single().showId)
        assertEquals(2, result.items.single().season)
        assertTrue(response.requestedURL!!.query.contains("view=shows"))
        var opens = 0
        val api = CatalogApi(server) { opens++; CatalogResponse(200, body) }
        assertThrows(IllegalArgumentException::class.java) { api.list("token", "alex", view = "unknown") }
        assertEquals(0, opens)
        listOf(body.replace("\"view\":\"shows\"", "\"view\":\"all\""),
            body.replace("0123456789abcdef", "bad"),
            body.replace("\"season\":2", "\"season\":-1"),
            body.replace("\"episode\":3", "\"episode\":\"3\"")).forEach { invalid ->
            assertThrows(Exception::class.java) {
                CatalogApi(server) { CatalogResponse(200, invalid) }.list("token", "alex", view = "shows")
            }
        }
    }

    @Test fun browsesEachMediaCategoryAndRejectsUnknownViewsBeforeNetwork() {
        for (view in listOf("movies", "music", "audiobooks", "books", "photos", "list")) {
            val response = CatalogResponse(200, page().replace("\"view\":\"all\"", "\"view\":\"$view\""))
            val result = CatalogApi(server) { url -> response.also { it.requestedURL = url } }
                .list("token", "alex", view = view)
            assertEquals("Arrival", result.items.single().title)
            assertTrue(response.requestedURL!!.query.contains("view=$view"))
            assertEquals("alex", response.getRequestProperty("X-Kinosail-Viewer-Profile"))
            assertThrows(Exception::class.java) {
                CatalogApi(server) { CatalogResponse(200, page()) }.list("token", "alex", view = view)
            }
        }
        var opens = 0
        val api = CatalogApi(server) { opens++; CatalogResponse(200, page()) }
        for (view in listOf("unknown", "movies&sort=added", "", "MOVIES")) {
            assertThrows(IllegalArgumentException::class.java) { api.list("token", "alex", view = view) }
        }
        assertEquals(0, opens)
    }

    @Test fun acceptsOnlyTheItemsOwnBoundedMediaPath() {
        val photo = """{"id":"photo-1","kind":"photo","title":"Harbor","stream":"/media/photo-1"}"""
        val result = CatalogApi(server) { CatalogResponse(200, page(photo)) }.list("token", "alex")
        assertEquals("/media/photo-1", result.items.single().stream)
        for (path in listOf("https://other.example/media/photo-1", "//other.example/media/photo-1",
            "/media/other", "/media/photo-1?token=secret", "/media/" + "x".repeat(257))) {
            val invalid = photo.replace("/media/photo-1", path)
            assertThrows(Exception::class.java) {
                CatalogApi(server) { CatalogResponse(200, page(invalid)) }.list("token", "alex")
            }
        }
    }

    @Test fun loadsHomeShelvesWithValidatedViewAndSort() {
        val history = CatalogResponse(200, page().replace("\"view\":\"all\"", "\"view\":\"history\""))
        val recent = CatalogResponse(200, page().replace("\"sort\":\"title\"", "\"sort\":\"added\""))
        CatalogApi(server) { url -> history.also { it.requestedURL = url } }
            .list("token", "alex", view = "history")
        CatalogApi(server) { url -> recent.also { it.requestedURL = url } }
            .list("token", "alex", sort = "added")
        assertTrue(history.requestedURL!!.query.contains("view=history"))
        assertTrue(recent.requestedURL!!.query.contains("sort=added"))
        var opens = 0
        assertThrows(IllegalArgumentException::class.java) {
            CatalogApi(server) { opens++; recent }.list("token", "alex", sort = "unknown")
        }
        assertEquals(0, opens)
        assertThrows(IllegalArgumentException::class.java) {
            CatalogApi(server) { recent }.list("token", "alex", sort = "added", view = "history")
        }
    }

    @Test fun browsesMyListAndSavesViewerScopedMembership() {
        val page = CatalogResponse(200, page().replace("\"view\":\"all\"", "\"view\":\"list\""))
        val items = CatalogApi(server) { url -> page.also { it.requestedURL = url } }
            .list("token", "alex", view = "list")
        assertEquals(1, items.total)
        assertTrue(page.requestedURL!!.query.contains("view=list"))
        val saved = CatalogResponse(200, """{"listed":true}""")
        assertTrue(CatalogApi(server) { url -> saved.also { it.requestedURL = url } }
            .setListed("film-1", "token", "alex", true))
        assertEquals("/api/v1/items/film-1/list", saved.requestedURL?.path)
        assertEquals("PUT", saved.requestMethod)
        assertEquals("""{"listed":true}""", saved.sent.toString(Charsets.UTF_8))
        assertEquals("alex", saved.getRequestProperty("X-Kinosail-Viewer-Profile"))
        assertTrue(saved.closed)
        var opens = 0
        val api = CatalogApi(server) { opens++; saved }
        assertThrows(IllegalArgumentException::class.java) { api.setListed("../other", "token", "alex", true) }
        assertThrows(IllegalArgumentException::class.java) { api.setListed("film-1", "bad token", "alex", true) }
        assertThrows(IllegalArgumentException::class.java) { api.setListed("film-1", "token", "bad viewer", true) }
        assertEquals(0, opens)
        listOf("{}", """{"listed":false}""", """{"listed":"true"}""",
            """{"listed":true,"extra":1}""").forEach { invalid ->
            assertThrows(Exception::class.java) {
                CatalogApi(server) { CatalogResponse(200, invalid) }
                    .setListed("film-1", "token", "alex", true)
            }
        }
    }

    @Test fun loadsTheValidatedNextItemForTheSameViewer() {
        val body = """{"item":$item,"listed":true,"profileId":"alex"}"""
        val response = CatalogResponse(200, body)
        val loaded = CatalogApi(server) { url -> response.also { it.requestedURL = url } }
            .item("film-1", "token", "alex")
        assertEquals("film-1", loaded.id)
        assertEquals(false, CatalogApi(server) { CatalogResponse(200,
            body.replace("\"listed\":true", "\"listed\":false")) }
            .details("film-1", "token", "alex").listed)
        assertEquals("/api/v1/items/film-1", response.requestedURL?.path)
        assertEquals("alex", response.getRequestProperty("X-Kinosail-Viewer-Profile"))
        assertEquals("film-1", CatalogApi(server) { CatalogResponse(200,
            body.replace("\"listed\":true", "\"listed\":false")) }.item("film-1", "token", "alex").id)
        var opens = 0
        assertThrows(IllegalArgumentException::class.java) {
            CatalogApi(server) { opens++; response }.item("../other", "token", "alex")
        }
        assertEquals(0, opens)
        listOf(body.replace("\"film-1\"", "\"film-2\""),
            body.replace("\"listed\":true", "\"listed\":\"true\""),
            body.replace(",\"profileId\":\"alex\"", ""),
            body.replace("\"alex\"", "\"other\""),
            body.replace("\"item\":", "\"extra\":1,\"item\":"))
            .forEach { invalid -> assertThrows(Exception::class.java) {
                CatalogApi(server) { CatalogResponse(200, invalid) }.item("film-1", "token", "alex")
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
    val sent = ByteArrayOutputStream()
    override fun connect() = Unit
    override fun disconnect() { closed = true }
    override fun usingProxy() = false
    override fun getResponseCode() = status
    override fun getContentType() = type
    override fun getContentLengthLong() = body.toByteArray().size.toLong()
    override fun getInputStream() = ByteArrayInputStream(body.toByteArray())
    override fun getOutputStream() = sent
}
