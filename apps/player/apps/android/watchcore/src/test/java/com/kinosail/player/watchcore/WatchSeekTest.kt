package com.kinosail.player.watchcore

import org.junit.Assert.*
import org.junit.Test

class WatchSeekTest {
    private val movie = WatchPlayer("phone", "This phone", "Film", "", "film", "playing", 20.0, 120.0, false)
    @Test fun seekUsesTheSelectedTitleAndBoundsAtBothEnds() {
        for (position in listOf(0.0, 120.0)) {
            assertEquals(WatchRequest("phone", "film", "seek", position), movie.seekRequest(position))
        }
    }
    @Test fun rejectsMissingPlaybackUnknownDurationNonfiniteAndOutOfRangeBeforeCommandEncoding() {
        for (value in listOf(-1.0, 121.0, Double.NaN, Double.POSITIVE_INFINITY))
            assertThrows(IllegalArgumentException::class.java) { movie.seekRequest(value) }
        assertThrows(IllegalArgumentException::class.java) { movie.copy(duration = 0.0, position = 0.0).seekRequest(0.0) }
        assertThrows(IllegalArgumentException::class.java) {
            WatchPlayer("phone", "Phone", "", "", "", "idle", 0.0, 0.0, false).seekRequest(0.0)
        }
    }
}
