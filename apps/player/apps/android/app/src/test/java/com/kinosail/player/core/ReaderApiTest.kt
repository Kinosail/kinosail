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
        assertEquals(ReaderPosition(0.5), api.position("book-1", "token", "viewer"))
        assertEquals(ReaderPosition(0.75), api.save("book-1", "token", "viewer", 0.75))
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
                Reply(200, "%PDF-hello", "application/pdf", 129L * 1024 * 1024)).forEach { response ->
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
    private val declaredLength: Long = body.toByteArray().size.toLong(),
) : HttpURLConnection(URL("https://example.com")) {
    var closed = false
    private val output = ByteArrayOutputStream()
    override fun connect() = Unit
    override fun disconnect() { closed = true }
    override fun usingProxy() = false
    override fun getResponseCode() = status
    override fun getContentType() = type
    override fun getContentLengthLong() = declaredLength
    override fun getInputStream() = ByteArrayInputStream(body.toByteArray())
    override fun getOutputStream() = output
}
