package com.kinosail.player.core

import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
import java.net.HttpURLConnection
import java.net.URL
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class ServerApiTest {
    private val server = ServerAddress("https://example.com")

    @Test fun startsAndPollsTheServerContract() {
        val start = FakeResponse(201, """{"code":"123456","secret":"abcDEF123"}""")
        val pending = FakeResponse(202, """{"status":"pending"}""")
        val approved = FakeResponse(201, """{"token":"token-123","expiresIn":28800}""")
        val cancel = FakeResponse(204, "")
        val responses = ArrayDeque(listOf(start, pending, approved, cancel))
        val api = ServerApi(server) { url -> responses.removeFirst().also { it.requestedURL = url } }
        assertEquals(ConnectChallenge("123456", "abcDEF123"), api.start("Kinosail Android TV"))
        assertEquals("/api/v1/quick-connect", start.requestedURL?.path)
        assertTrue(start.output.toString().contains("Kinosail Android TV"))
        assertEquals(null, api.poll("abcDEF123"))
        assertEquals("token-123", api.poll("abcDEF123"))
        api.cancel("abcDEF123")
        assertEquals("/api/v1/quick-connect/cancel", cancel.requestedURL?.path)
        assertTrue(listOf(start, pending, approved, cancel).all { it.closed && !it.instanceFollowRedirects })
    }

    @Test fun viewerAndSignOutRequireAnOriginBoundBearerToken() {
        val profile = FakeResponse(200, """{"server":"Living Room","serverId":"server-1","viewer":{"id":"viewer-1","name":"Alex","owner":false,"downloads":true,"transcode":false,"remote":false,"libraries":[]}}""")
        val signOut = FakeResponse(204, "")
        val responses = ArrayDeque(listOf(profile, signOut))
        val api = ServerApi(server) { url -> responses.removeFirst().also { it.requestedURL = url } }
        assertEquals(Viewer("Living Room", "server-1", "viewer-1", "Alex"), api.viewer("token-123"))
        assertEquals("Bearer token-123", profile.getRequestProperty("Authorization"))
        api.signOut("token-123")
        assertEquals("/api/v1/session", signOut.requestedURL?.path)
        assertEquals("DELETE", signOut.requestMethod)
    }

    @Test fun invalidInputsNeverOpenAConnection() {
        var opens = 0
        val api = ServerApi(server) { opens++; FakeResponse(201, "{}") }
        listOf("", "x".repeat(81), "bad\nname").forEach {
            assertThrows(IllegalArgumentException::class.java) { api.start(it) }
        }
        listOf("", "bad secret", "x".repeat(129)).forEach {
            assertThrows(IllegalArgumentException::class.java) { api.poll(it) }
        }
        assertThrows(IllegalArgumentException::class.java) { api.viewer("bad token") }
        assertEquals(0, opens)
    }

    @Test fun malformedMissingUnknownAndConflictingResponsesAreRejected() {
        val invalid = listOf(
            "{}", """{"code":"123456"}""", """{"code":"12345x","secret":"s"}""",
            """{"code":"123456","secret":"s","other":1}""",
            """{"code":"123456","code":"654321","secret":"s"}""",
            """{"code":"123456","secret":"bad secret"}""", "not json",
        )
        invalid.forEach { body ->
            val response = FakeResponse(201, body)
            assertThrows(body, Exception::class.java) { ServerApi(server) { response }.start("Android TV") }
            assertTrue(response.closed)
        }
        listOf("""{"status":"approved"}""", """{"token":"ok","expiresIn":0}""",
            """{"token":"ok","expiresIn":1.5}""", """{"token":"ok","expiresIn":1,"extra":true}""").forEach { body ->
            assertThrows(Exception::class.java) { ServerApi(server) { FakeResponse(if (body.contains("status")) 202 else 201, body) }.poll("secret") }
        }
    }

    @Test fun redirectsOversizedBodiesAndUnknownViewerFieldsAreRejected() {
        listOf(FakeResponse(302, "{}"), FakeResponse(201, "x".repeat(65_537)),
            FakeResponse(201, "{}", type = "text/html")).forEach { response ->
            assertThrows(Exception::class.java) { ServerApi(server) { response }.start("Android TV") }
            assertTrue(response.closed)
        }
        val extra = FakeResponse(200, """{"server":"S","serverId":"id","viewer":{"id":"v","name":"N","token":"leak"}}""")
        assertThrows(Exception::class.java) { ServerApi(server) { extra }.viewer("token") }
    }
}

private class FakeResponse(
    private val status: Int,
    private val body: String,
    private val type: String = "application/json",
) : HttpURLConnection(URL("https://example.com/api/v1/quick-connect")) {
    var closed = false
    var requestedURL: URL? = null
    val output = ByteArrayOutputStream()
    override fun connect() = Unit
    override fun disconnect() { closed = true }
    override fun usingProxy() = false
    override fun getResponseCode() = status
    override fun getContentType() = type
    override fun getContentLengthLong() = body.toByteArray().size.toLong()
    override fun getInputStream() = ByteArrayInputStream(body.toByteArray())
    override fun getOutputStream() = output
}
