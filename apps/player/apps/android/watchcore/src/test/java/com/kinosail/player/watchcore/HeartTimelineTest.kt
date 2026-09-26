package com.kinosail.player.watchcore

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class HeartTimelineTest {
    private val start = 1_700_000_000_000L
    private val movie = WatchPlayer("phone", "This phone", "Scary Movie", "", "movie-1", "playing", 30.0, 120.0, false)

    @Test fun mapsReadingsToMovieTimeAndPreservesGaps() {
        var timeline = HeartTimeline.start(movie, start)
        timeline = timeline.addReading(start + 5_000, 110.0, start + 5_000)
        assertEquals(35.0, timeline.points.single().position, 0.001)
        timeline = timeline.note(movie.copy(state = "paused", position = 40.0), start + 10_000)
        assertEquals(timeline, timeline.addReading(start + 11_000, 140.0, start + 11_000))
        timeline = timeline.note(movie.copy(position = 60.0), start + 20_000)
        timeline = timeline.addReading(start + 23_000, 150.0, start + 23_000)
        assertEquals(63.0, timeline.peak?.position ?: -1.0, 0.001)
        assertEquals(2, timeline.points.size)
        assertEquals(timeline, timeline.addReading(start + 40_000, 120.0, start + 40_000))
    }

    @Test fun rejectsInvalidSamplesAndEndsOnTitleChange() {
        val timeline = HeartTimeline.start(movie, start)
        for (bpm in listOf(24.0, 251.0, Double.NaN, Double.POSITIVE_INFINITY))
            assertEquals(timeline, timeline.addReading(start + 1_000, bpm, start + 1_000))
        assertEquals(timeline, timeline.addReading(start - 1, 100.0, start + 1_000))
        assertEquals(timeline, timeline.addReading(start + 16_000, 100.0, start + 16_000))
        assertTrue(timeline.note(movie.copy(itemId = "other"), start + 10_000).endedMs != null)
        assertTrue(timeline.note(movie.copy(duration = 240.0, position = 150.0), start + 10_000).endedMs != null)
        assertEquals(timeline, timeline.note(movie, start - 1))
        assertThrows(IllegalArgumentException::class.java) { HeartTimeline.start(movie.copy(audio = true), start) }
    }

    @Test fun persistedTimelineIsBoundedAndValidated() {
        val timeline = HeartTimeline.start(movie, start).addReading(start + 1_000, 100.0, start + 1_000)
        assertEquals(timeline, HeartTimeline.decode(HeartTimeline.encode(timeline), System.currentTimeMillis()))
        assertThrows(Exception::class.java) { HeartTimeline.decode("{\"unknown\":1}", System.currentTimeMillis()) }
        assertThrows(Exception::class.java) { HeartTimeline.decode("{\"title\":\"A\",\"title\":\"B\"}", System.currentTimeMillis()) }
        assertThrows(IllegalArgumentException::class.java) { HeartTimeline.decode("x".repeat(1_000_001), System.currentTimeMillis()) }
        assertThrows(IllegalArgumentException::class.java) {
            timeline.copy(points = listOf(HeartPoint(start + 1_000, 100.0, 300.0))).checked(System.currentTimeMillis())
        }
        assertNull(HeartTimeline.start(movie, start).peak)
    }
}
