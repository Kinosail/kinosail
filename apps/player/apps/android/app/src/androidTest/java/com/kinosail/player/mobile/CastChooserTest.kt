package com.kinosail.player.mobile

import android.view.ViewGroup
import androidx.fragment.app.DialogFragment
import androidx.mediarouter.app.MediaRouteButton
import androidx.test.ext.junit.rules.ActivityScenarioRule
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import com.google.android.gms.cast.framework.CastButtonFactory
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class CastChooserTest {
    @get:Rule val activity = ActivityScenarioRule(MobileActivity::class.java)

    @Test fun castChooserOpensUnderPhoneTheme() {
        activity.scenario.onActivity { phone ->
            val button = MediaRouteButton(phone)
            CastButtonFactory.setUpMediaRouteButton(phone.applicationContext, button)
            phone.findViewById<ViewGroup>(android.R.id.content).addView(button)
            assertTrue(button.showDialog())
        }
        InstrumentationRegistry.getInstrumentation().waitForIdleSync()
        activity.scenario.onActivity { phone ->
            val chooser = phone.supportFragmentManager.findFragmentByTag(
                "android.support.v7.mediarouter:MediaRouteChooserDialogFragment",
            ) as? DialogFragment
            assertTrue(chooser?.dialog?.isShowing == true)
        }
    }
}
