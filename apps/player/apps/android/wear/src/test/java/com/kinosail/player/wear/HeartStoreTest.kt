package com.kinosail.player.wear

import android.content.Context
import com.kinosail.player.watchcore.WatchPlayer
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment

@RunWith(RobolectricTestRunner::class)
class HeartStoreTest {
    private val context get() = RuntimeEnvironment.getApplication() as Context
    private val player = WatchPlayer("phone", "This phone", "Scary Movie", "", "movie-1", "playing", 10.0, 120.0, false)

    @Test fun trackingRequiresOptInAndPersistsOnlyMappedReadings() {
        val now = System.currentTimeMillis() - 10_000
        assertNull(HeartStore.load(context, now))
        assertNull(HeartStore.reading(context, now, 100.0, now))
        HeartStore.start(context, player, now)
        HeartStore.reading(context, now + 1_000, 110.0, now + 1_000)
        assertEquals(11.0, HeartStore.load(context, now + 1_000)!!.points.single().position, 0.001)
        HeartStore.stop(context, now + 2_000)
        assertFalse(HeartStore.load(context, now + 2_000)!!.tracking)
        assertEquals(1, HeartStore.reading(context, now + 3_000, 120.0, now + 3_000)!!.points.size)
    }

    @Test fun invalidStartDoesNotReplaceExistingGraph() {
        val now = System.currentTimeMillis() - 10_000
        val first = HeartStore.start(context, player, now)
        assertThrows(IllegalArgumentException::class.java) { HeartStore.start(context, player.copy(audio = true), now + 1) }
        assertEquals(first, HeartStore.load(context, now + 1))
    }
}
