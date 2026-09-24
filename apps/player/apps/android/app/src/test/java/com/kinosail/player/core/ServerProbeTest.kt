package com.kinosail.player.core

import java.io.ByteArrayInputStream
import java.net.HttpURLConnection
import java.net.URL
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class ServerProbeTest {
    @Test fun addressAcceptsSecureRemoteAndLocalHTTP() {
        assertEquals("https://example.com/", ServerAddress(" HTTPS://Example.com:443/ ").url.toString())
        assertEquals("http://192.168.1.5:8080/", ServerAddress("http://192.168.1.5:8080").url.toString())
        assertEquals("http://localhost/", ServerAddress("http://localhost").url.toString())
        assertEquals("http://[::1]:8080/", ServerAddress("http://[::1]:8080").url.toString())
    }

    @Test fun invalidAddressesCauseNoNetworkSideEffects() {
        val invalid = listOf("", " ", "x".repeat(2049), "ftp://example.com", "http://example.com",
            "http://8.8.8.8", "http://172.32.0.1", "http://192.168.01.2",
            "http://[2001:db8::1]", "http://server.local",
            "https://user:pass@example.com", "https://example.com/path", "https://example.com/?x=1",
            "https://example.com/#x", "https://example.com:0", "https://example.com:65536",
            "https://example.com\\@evil.test", "https://example.com\n.evil.test")
        var opens = 0
        val probe = ServerProbe { opens++; FakeConnection(it) }
        invalid.forEach { assertThrows(it, IllegalArgumentException::class.java) { probe.check(it) } }
        assertEquals(0, opens)
    }

    @Test fun validHealthResponseIsAcceptedAndConnectionClosed() {
        val connection = FakeConnection(URL("https://example.com/healthz"))
        val probe = ServerProbe { url ->
            assertEquals("https://example.com/healthz", url.toString())
            connection
        }
        assertEquals("https://example.com/", probe.check("https://example.com").url.toString())
        assertTrue(connection.closed)
        assertEquals(false, connection.instanceFollowRedirects)
    }

    @Test fun malformedUnknownRedirectAndOversizedResponsesAreRejected() {
        listOf(
            FakeConnection(URL("https://example.com"), status = 302),
            FakeConnection(URL("https://example.com"), body = "{\"status\":\"ok\",\"token\":\"x\"}"),
            FakeConnection(URL("https://example.com"), body = "x".repeat(4097)),
            FakeConnection(URL("https://example.com"), type = "text/html"),
        ).forEach { connection ->
            assertThrows(IllegalArgumentException::class.java) {
                ServerProbe { connection }.check("https://example.com")
            }
            assertTrue(connection.closed)
        }
    }
}

private class FakeConnection(
    url: URL,
    private val status: Int = 200,
    private val body: String = "{\"status\":\"ok\"}\n",
    private val type: String = "application/json",
) : HttpURLConnection(url) {
    var closed = false
    override fun connect() = Unit
    override fun disconnect() { closed = true }
    override fun usingProxy() = false
    override fun getResponseCode() = status
    override fun getContentType() = type
    override fun getContentLengthLong() = body.toByteArray().size.toLong()
    override fun getInputStream() = ByteArrayInputStream(body.toByteArray())
}
