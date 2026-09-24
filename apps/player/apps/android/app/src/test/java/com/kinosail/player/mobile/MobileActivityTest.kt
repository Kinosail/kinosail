package com.kinosail.player.mobile

import android.content.pm.PackageManager
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.Robolectric
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
class MobileActivityTest {
    @Test @Config(sdk = [28]) fun onlyPlayingVideoEntersPipWhenLeavingOldPhones() {
        val activity = Robolectric.buildActivity(MobileActivity::class.java).setup().get()
        Shadows.shadowOf(activity.packageManager)
            .setSystemFeature(PackageManager.FEATURE_PICTURE_IN_PICTURE, true)
        activity.setVideoPipReady(false)
        leave(activity)
        assertFalse(activity.isInPictureInPictureMode)
        activity.setVideoPipReady(true)
        leave(activity)
        assertTrue(activity.isInPictureInPictureMode)
    }

    @Test @Config(sdk = [24]) fun oldPhonesDoNotEnterPip() {
        val activity = Robolectric.buildActivity(MobileActivity::class.java).setup().get()
        activity.setVideoPipReady(true)
        leave(activity)
    }

    @Test @Config(sdk = [28]) fun phonesWithoutPipSupportStayFullscreen() {
        val activity = Robolectric.buildActivity(MobileActivity::class.java).setup().get()
        Shadows.shadowOf(activity.packageManager)
            .setSystemFeature(PackageManager.FEATURE_PICTURE_IN_PICTURE, false)
        activity.setVideoPipReady(true)
        leave(activity)
        assertFalse(activity.isInPictureInPictureMode)
    }

    private fun leave(activity: MobileActivity) {
        MobileActivity::class.java.getDeclaredMethod("onUserLeaveHint").apply {
            isAccessible = true
        }.invoke(activity)
    }
}
