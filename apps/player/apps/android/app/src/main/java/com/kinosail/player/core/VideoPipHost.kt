package com.kinosail.player.core

internal interface VideoPipHost {
    val inPictureInPicture: Boolean
    fun setVideoPipReady(ready: Boolean)
}
