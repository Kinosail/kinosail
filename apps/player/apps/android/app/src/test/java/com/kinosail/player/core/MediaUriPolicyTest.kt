package com.kinosail.player.core

import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test

class MediaUriPolicyTest {
    private val policy = MediaUriPolicy(ServerAddress("https://example.com:8443"), "film-1")

    @Test fun permitsOnlyThisItemsDirectAndHlsResourcesOnTheServerOrigin() {
        for (path in listOf("/media/film-1", "/hls/film-1/p/r-a0-s0-none-t0-b0/index.m3u8",
            "/hls/film-1/p/r-a0-s0-none-t0-b0/1080p/segment-00001.m4s")) {
            val address = "https://example.com:8443$path"
            assertEquals(address, policy.requireAllowed(address))
        }
    }

    @Test fun rejectsOtherOriginsAndPathEscapes() {
        listOf("https://evil.example:8443/media/film-1", "https://example.com/media/film-1",
            "http://example.com:8443/media/film-1", "https://user@example.com:8443/media/film-1",
            "https://example.com:8443/media/film-2", "https://example.com:8443/hls/film-2/index.m3u8",
            "https://example.com:8443/hls/film-1/../film-2/index.m3u8",
            "https://example.com:8443/hls/film-1/%2e%2e/index.m3u8",
            "https://example.com:8443/hls/film-1//index.m3u8",
            "https://example.com:8443/media/film-1?api_key=leak",
            "https://example.com:8443/media/film-1#fragment", "file:///media/film-1",
            "https://example.com:8443/other/film-1").forEach { address ->
            assertThrows(address, IllegalArgumentException::class.java) { policy.requireAllowed(address) }
        }
    }

    @Test fun rejectsInvalidItemIdBeforeUsingThePolicy() {
        assertThrows(IllegalArgumentException::class.java) {
            MediaUriPolicy(ServerAddress("https://example.com"), "../../secret")
        }
    }
}
