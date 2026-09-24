package com.kinosail.player.core

import android.content.Intent
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.Robolectric
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class AudioPlaybackServiceTest {
    @Test fun onlyLocalActionExposesThePlaybackModel() {
        val service = Robolectric.buildService(AudioPlaybackService::class.java).create().get()
        try {
            assertNull(service.onBind(Intent("unexpected")))
            val local = service.onBind(Intent(AudioPlaybackService.ACTION_LOCAL))
            assertNotNull(local)
            assertFalse((local as AudioPlaybackService.LocalBinder).model.player?.isPlaying == true)
        } finally { service.onDestroy() }
    }
}
