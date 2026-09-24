package com.kinosail.player.core

import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test

class PlaybackSpeedTest {
    @Test fun acceptsTheFamilyPlaybackRates() {
        assertEquals(listOf(0.5f, 0.75f, 1f, 1.25f, 1.5f, 1.75f, 2f, 2.5f, 3f), PlaybackModel.SPEEDS)
        PlaybackModel.SPEEDS.forEach { assertEquals(it, PlaybackModel.checkedSpeed(it)) }
    }

    @Test fun rejectsUnknownAndNonFiniteRatesBeforeChangingThePlayer() {
        listOf(-1f, 0f, 0.6f, 3.5f, Float.NaN, Float.POSITIVE_INFINITY, Float.NEGATIVE_INFINITY)
            .forEach { rate -> assertThrows(IllegalArgumentException::class.java) {
                PlaybackModel.checkedSpeed(rate)
            } }
    }
}
