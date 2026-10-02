package com.kinosail.player.core

import java.io.ByteArrayInputStream
import java.net.HttpURLConnection
import java.net.URL
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class) @Config(sdk = [35])
class CatalogLimitsTest {
    private val server = ServerAddress("https://example.com")
    @Test fun acceptsBoundedHomeLimitsAndChecksTheReturnedLimit() {
        for (limit in listOf(36, 200)) {
            var requested = ""
            val api = CatalogApi(server) { url ->
                requested = url.query
                response("""{"items":[],"total":0,"offset":0,"limit":$limit}""")
            }
            assertEquals(limit, api.list("token", "viewer", limit = limit).limit)
            assertTrue(requested.endsWith("limit=$limit"))
        }
        assertThrows(IllegalArgumentException::class.java) {
            CatalogApi(server) { response("""{"items":[],"total":0,"offset":0,"limit":24}""") }
                .list("token", "viewer", limit = 36)
        }
    }
    @Test fun rejectsZeroNegativeAndOversizedLimitsBeforeNetwork() {
        var opens = 0
        val api = CatalogApi(server) { opens++; response("{}") }
        for (limit in listOf(Int.MIN_VALUE, -1, 0, 201, Int.MAX_VALUE))
            assertThrows(IllegalArgumentException::class.java) { api.list("token", "viewer", limit = limit) }
        assertEquals(0, opens)
    }
    private fun response(body: String) = object : HttpURLConnection(URL("https://example.com")) {
        override fun connect() = Unit
        override fun disconnect() = Unit
        override fun usingProxy() = false
        override fun getResponseCode() = 200
        override fun getContentType() = "application/json"
        override fun getInputStream() = ByteArrayInputStream(body.toByteArray())
        override fun getHeaderField(name: String?) = null
    }
}
