package com.kinosail.player.core

import java.io.ByteArrayInputStream
import java.net.HttpURLConnection
import java.net.URL
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class ArtworkClientTest {
    private val server = ServerAddress("https://example.com")

    @Test fun fetchesOnlyViewerScopedArtworkFromTheServerOrigin() {
        val response = ArtworkResponse(200, byteArrayOf(1, 2, 3))
        val client = ArtworkClient(server) { url -> response.also { it.requestedURL = url } }
        assertArrayEquals(byteArrayOf(1, 2, 3), client.bytes("/art/film-1?variant=episode", "token-1", "alex"))
        assertEquals("https://example.com/art/film-1?variant=episode", response.requestedURL.toString())
        assertEquals("Bearer token-1", response.getRequestProperty("Authorization"))
        assertEquals("alex", response.getRequestProperty("X-Kinosail-Viewer-Profile"))
        assertTrue(response.closed && !response.instanceFollowRedirects)
    }

    @Test fun rejectsInvalidPathsAndCredentialsBeforeOpeningAConnection() {
        var opens = 0
        val client = ArtworkClient(server) { opens++; ArtworkResponse(200, byteArrayOf(1)) }
        listOf("", "https://evil.example/art/id", "//evil.example/art/id", "/art/../id",
            "/art/id?other=1", "/media/id", "/art/" + "x".repeat(129)).forEach {
            assertThrows(it, IllegalArgumentException::class.java) { client.bytes(it, "token", "alex") }
        }
        assertThrows(IllegalArgumentException::class.java) { client.bytes("/art/id", "bad token", "alex") }
        assertThrows(IllegalArgumentException::class.java) { client.bytes("/art/id", "token", "bad viewer") }
        assertEquals(0, opens)
    }

    @Test fun rejectsRedirectWrongTypeAndOversizedArtwork() {
        listOf(ArtworkResponse(302, byteArrayOf(1)),
            ArtworkResponse(200, byteArrayOf(1), "text/html"),
            ArtworkResponse(200, ByteArray(16 * 1024 * 1024 + 1))).forEach { response ->
            assertThrows(Exception::class.java) { ArtworkClient(server) { response }.bytes("/art/id", "token", "alex") }
            assertTrue(response.closed)
        }
    }
}

private class ArtworkResponse(private val status: Int, private val body: ByteArray,
                              private val type: String = "image/png") :
    HttpURLConnection(URL("https://example.com/art/id")) {
    var requestedURL: URL? = null
    var closed = false
    override fun connect() = Unit
    override fun disconnect() { closed = true }
    override fun usingProxy() = false
    override fun getResponseCode() = status
    override fun getContentType() = type
    override fun getContentLengthLong() = body.size.toLong()
    override fun getInputStream() = ByteArrayInputStream(body)
}
