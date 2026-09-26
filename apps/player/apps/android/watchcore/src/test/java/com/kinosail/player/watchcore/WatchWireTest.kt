package com.kinosail.player.watchcore

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Test

class WatchWireTest {
    private val movie = WatchPlayer("phone", "This phone", "Scary Movie", "", "movie-1", "playing", 30.0, 120.0, false)

    @Test fun roundTripsRemoteStateAndScopedCommand() {
        assertEquals(WatchRequest(), WatchWire.request(WatchWire.requestBytes(WatchRequest())))
        val seek = WatchRequest("phone", "movie-1", "seek", 90.0)
        assertEquals(seek, WatchWire.request(WatchWire.requestBytes(seek)))
        assertEquals(WatchReply(listOf(movie), true), WatchWire.reply(WatchWire.replyBytes(WatchReply(listOf(movie), true))))
    }

    @Test fun rejectsMalformedUnknownOversizedAndConflictingRequests() {
        val invalid = listOf("", "[]", "{", "{\"unknown\":1}", "{\"command\":\"play\"}",
            "{\"target\":\"phone\"}", "{\"target\":\"phone\",\"itemId\":\"movie-1\",\"command\":\"play\",\"position\":1}",
            "{\"target\":\"phone\",\"itemId\":\"movie-1\",\"command\":\"seek\",\"position\":-1}",
            "{\"target\":\"phone\",\"itemId\":\"movie-1\",\"command\":\"seek\",\"position\":1e100}",
            "{\"target\":\"phone\",\"target\":\"phone\"}", " ".repeat(65_537))
        invalid.forEach { assertThrows(it.take(60), Exception::class.java) { WatchWire.request(it.toByteArray()) } }
        assertThrows(Exception::class.java) {
            WatchWire.request(byteArrayOf(0xC3.toByte(), 0x28))
        }
    }

    @Test fun rejectsInvalidPlayersAndKeepsExplicitSelectionWhenUnavailable() {
        assertThrows(IllegalArgumentException::class.java) { movie.copy(position = 121.0).checked() }
        assertThrows(IllegalArgumentException::class.java) { movie.copy(state = "idle").checked() }
        assertThrows(IllegalArgumentException::class.java) { WatchReply(listOf(movie, movie), true).checked() }
        assertThrows(IllegalArgumentException::class.java) { WatchWire.reply("{\"players\":[],\"accepted\":true,\"extra\":1}".toByteArray()) }
        assertEquals("missing", listOf(movie).selectedId("missing"))
        assertEquals("phone", listOf(movie).selectedId(null))
    }
}
