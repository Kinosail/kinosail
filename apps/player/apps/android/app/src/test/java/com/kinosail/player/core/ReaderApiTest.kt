package com.kinosail.player.core

import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
import java.net.HttpURLConnection
import java.net.URL
import java.nio.file.Files
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class ReaderApiTest {
    private val server = ServerAddress("https://example.com")
    private val metadata = """{"id":"book-1","title":"The Book","type":"pdf","pages":[{"number":1,"title":"Document","url":"/read/book-1/file"}]}"""
    private val comic = """{"id":"comic-1","title":"Panels","type":"comic","pages":[{"number":1,"title":"One","url":"/read/comic-1/asset/Page%201.jpg"},{"number":2,"title":"Two","url":"/read/comic-1/asset/Page%202.jpg"}]}"""

    @Test fun validatesTheReaderRouteAndViewerBeforeOpeningAConnection() {
        var opens = 0
        val api = ReaderApi(server) { opens++; Reply(200, metadata) }
        listOf("", "bad/id", "x".repeat(129)).forEach { id ->
            assertThrows(IllegalArgumentException::class.java) { api.book(id, "token", "viewer") }
        }
        assertThrows(IllegalArgumentException::class.java) { api.position("book-1", "bad token", "viewer") }
        assertThrows(IllegalArgumentException::class.java) { api.save("book-1", "token", "viewer", Double.NaN) }
        assertThrows(IllegalArgumentException::class.java) { api.save("book-1", "token", "viewer", 1.1) }
        assertThrows(IllegalArgumentException::class.java) { api.download(
            ReaderBook("book-1", "Book", "/read/book-2/file"), "token", "viewer", Files.createTempDirectory("pdf-test").toFile()) }
        assertEquals(0, opens)
    }

    @Test fun readsAndSavesAViewerScopedPosition() {
        val responses = ArrayDeque(listOf(Reply(200, metadata),
            Reply(200, """{"page":1,"total":1,"offset":0.5}"""),
            Reply(200, """{"page":1,"total":1,"offset":0.75}""")))
        val api = ReaderApi(server) { responses.removeFirst() }
        assertEquals(ReaderBook("book-1", "The Book", "/read/book-1/file"),
            api.book("book-1", "token", "viewer"))
        assertEquals(ReaderPosition(1, 1, 0.5), api.position("book-1", "token", "viewer"))
        assertEquals(ReaderPosition(1, 1, 0.75), api.save("book-1", "token", "viewer", 0.75))
        assertTrue(responses.isEmpty())
    }

    @Test fun rejectsMalformedMetadataBeforeFetchingAFile() {
        val bad = listOf("{}", metadata.replace("\"id\":\"book-1\"", "\"id\":\"book-2\""),
            metadata.replace("book-1/file", "other/file"),
            metadata.replace("/read/book-1/file", "https://outside.test/read/book-1/file"),
            metadata.replace("\"number\":1", "\"number\":2"),
            metadata.replace("\"type\":\"pdf\"", "\"type\":\"epub\""),
            metadata.replace("\"type\":\"pdf\"", "\"type\":\"unknown\""),
            metadata.replace("\"title\":\"The Book\"", "\"title\":\"The Book\",\"extra\":1"),
            metadata.replace("\"pages\":[", "\"pages\":[{\"number\":1,\"title\":\"Duplicate\",\"url\":\"/read/book-1/file\"},"))
        bad.forEach { body ->
            var opens = 0
            val api = ReaderApi(server) { opens++; Reply(200, body) }
            assertThrows(body, Exception::class.java) { api.book("book-1", "token", "viewer") }
            assertEquals(1, opens)
        }
    }

    @Test fun rejectsBadPositionsAndConflictingSaveResponses() {
        listOf("{}", """{"page":2,"total":1,"offset":0.5}""",
            """{"page":1,"total":1,"offset":-0.1}""", """{"page":1,"total":1,"offset":"0.5"}""",
            """{"page":1,"total":1,"offset":0.5,"extra":true}""").forEach { body ->
            assertThrows(body, Exception::class.java) {
                ReaderApi(server) { Reply(200, body) }.position("book-1", "token", "viewer")
            }
        }
        assertThrows(IllegalArgumentException::class.java) {
            ReaderApi(server) { Reply(200, """{"page":1,"total":1,"offset":0.2}""") }
                .save("book-1", "token", "viewer", 0.5)
        }
    }

    @Test fun readsComicPagesAndSavesTheServerPageNumber() {
        val saved = Reply(200, """{"page":2,"total":2,"offset":0.0}""")
        val responses = ArrayDeque(listOf(Reply(200, comic),
            Reply(200, """{"page":2,"total":2,"offset":0.0}"""), saved))
        val api = ReaderApi(server) { responses.removeFirst() }
        assertEquals(ComicBook("comic-1", "Panels", listOf("/read/comic-1/asset/Page%201.jpg",
            "/read/comic-1/asset/Page%202.jpg")), api.comic("comic-1", "token", "viewer"))
        assertEquals(ReaderPosition(2, 2, 0.0), api.position("comic-1", "token", "viewer", 2))
        assertEquals(ReaderPosition(2, 2, 0.0), api.save("comic-1", "token", "viewer", 0.0, 2, 2))
        assertEquals("PUT", saved.requestMethod)
        assertTrue(saved.output.toString().contains("\"page\":2"))
        assertTrue(responses.isEmpty())
    }

    @Test fun formatSelectionRequiresARecognizedBookAndBoundedPageList() {
        assertEquals("pdf", ReaderApi(server) { Reply(200, metadata) }.format("book-1", "token", "viewer"))
        assertEquals("comic", ReaderApi(server) { Reply(200, comic) }.format("comic-1", "token", "viewer"))
        listOf(comic.replace("\"type\":\"comic\"", "\"type\":\"other\""),
            """{"id":"comic-1","title":"Panels","type":"comic","pages":[]}""",
            comic.replace("\"id\":\"comic-1\"", "\"id\":\"other\"")).forEach { body ->
            assertThrows(body, Exception::class.java) {
                ReaderApi(server) { Reply(200, body) }.format("comic-1", "token", "viewer")
            }
        }
    }

    @Test fun rejectsUnsafeComicPathsAndMalformedPagesBeforeFetchingAssets() {
        assertTrue(ReaderApi.validComicPath("/read/comic-1/asset/Chapter%201/Page%2001.jpg", "comic-1"))
        val invalid = listOf("https://outside.test/read/comic-1/asset/a.jpg",
            "//outside.test/read/comic-1/asset/a.jpg", "/read/comic-2/asset/a.jpg",
            "/read/comic-1/asset/../a.jpg", "/read/comic-1/asset/%2E%2E/a.jpg",
            "/read/comic-1/asset/a%2Fb.jpg", "/read/comic-1/asset/a%5Cb.jpg",
            "/read/comic-1/asset/a.jpg?x=1", "/read/comic-1/asset/a.jpg#part",
            "/read/comic-1/asset/a%GG.jpg", "/read/comic-1/asset/")
        invalid.forEach { assertFalse(ReaderApi.validComicPath(it, "comic-1")) }
        var opens = 0
        val api = ReaderApi(server) { opens++; Reply(200, comic) }
        assertThrows(IllegalArgumentException::class.java) { api.comic("bad/id", "token", "viewer") }
        assertThrows(IllegalArgumentException::class.java) { api.save("comic-1", "token", "viewer", 0.0, 3, 2) }
        assertEquals(0, opens)
        listOf(comic.replace("Page%202.jpg", "Page%201.jpg"),
            comic.replace("\"number\":2", "\"number\":3"),
            comic.replace("Page%202.jpg", "../Page%202.jpg"),
            comic.replace("\"type\":\"comic\"", "\"type\":\"epub\"")).forEach { body ->
            assertThrows(body, Exception::class.java) {
                ReaderApi(server) { Reply(200, body) }.comic("comic-1", "token", "viewer")
            }
        }
    }

    @Test fun acceptsAComicPageListLargerThanTheDefaultJsonResponseLimit() {
        val pages = (1..900).joinToString(",") { number ->
            """{"number":$number,"title":"Page $number","url":"/read/comic-1/asset/Page%20$number.png"}"""
        }
        val body = """{"id":"comic-1","title":"Panels","type":"comic","pages":[$pages]}"""
        assertTrue(body.length > 65_536)
        assertEquals(900, ReaderApi(server) { Reply(200, body) }.comic("comic-1", "token", "viewer").pages.size)
    }

    @Test fun downloadsOnlyBoundedPdfBytesAndDeletesRejectedFiles() {
        val dir = Files.createTempDirectory("kinosail-reader-test").toFile()
        try {
            val pdf = Reply(200, "%PDF-hello", "application/pdf")
            val api = ReaderApi(server) { pdf }
            val file = api.download(ReaderBook("book-1", "Book", "/read/book-1/file"), "token", "viewer", dir)
            assertEquals("%PDF-hello", file.readText())
            assertEquals("Bearer token", pdf.getRequestProperty("Authorization"))
            assertEquals("viewer", pdf.getRequestProperty("X-Kinosail-Viewer-Profile"))
            assertFalse(pdf.instanceFollowRedirects)
            file.delete()
            listOf(Reply(302, "%PDF-hello", "application/pdf"),
                Reply(200, "%PDF-hello", "text/html"), Reply(200, "not-a-pdf", "application/pdf"),
                Reply(200, "%PDF-hello", "application/pdf", (129L * 1024 * 1024).toString()),
                Reply(200, "%PDF-hello", "application/pdf", "2147483648"),
                Reply(200, "%PDF-hello", "application/pdf", "1, 2"),
                Reply(200, "%PDF-hello", "application/pdf", "-1")).forEach { response ->
                assertThrows(Exception::class.java) {
                    ReaderApi(server) { response }.download(
                        ReaderBook("book-1", "Book", "/read/book-1/file"), "token", "viewer", dir)
                }
                assertTrue(response.closed)
                assertTrue(dir.listFiles().isNullOrEmpty())
            }
        } finally {
            dir.deleteRecursively()
        }
    }
}

private class Reply(
    private val status: Int,
    private val body: String,
    private val type: String = "application/json",
    private val declaredLength: String? = body.toByteArray().size.toString(),
) : HttpURLConnection(URL("https://example.com")) {
    var closed = false
    val output = ByteArrayOutputStream()
    override fun connect() = Unit
    override fun disconnect() { closed = true }
    override fun usingProxy() = false
    override fun getResponseCode() = status
    override fun getContentType() = type
    override fun getHeaderField(name: String?): String? =
        if (name.equals("Content-Length", ignoreCase = true)) declaredLength else null
    override fun getInputStream() = ByteArrayInputStream(body.toByteArray())
    override fun getOutputStream() = output
}
