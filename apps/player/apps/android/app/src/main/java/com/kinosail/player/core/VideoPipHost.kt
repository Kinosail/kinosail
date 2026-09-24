package com.kinosail.player.core

import android.view.View

internal interface VideoPipHost {
    val inPictureInPicture: Boolean
    fun setVideoPipReady(ready: Boolean)
    fun setVideoPipView(view: View?)
}
