package com.kinosail.player.core

import android.content.ComponentName
import android.content.Context
import android.content.ServiceConnection
import android.os.IBinder
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.platform.LocalContext

internal data class AudioPlaybackConnection(val model: PlaybackModel? = null, val error: Boolean = false)

@Composable
internal fun rememberAudioPlaybackConnection(enabled: Boolean, tv: Boolean): AudioPlaybackConnection {
    val context = LocalContext.current
    var state by remember(context, enabled, tv) { mutableStateOf(AudioPlaybackConnection()) }
    DisposableEffect(context, enabled, tv) {
        if (!enabled) return@DisposableEffect onDispose { }
        var active = true
        var bound = false
        val connection = object : ServiceConnection {
            override fun onServiceConnected(name: ComponentName, binder: IBinder) {
                if (active) {
                    val model = (binder as? AudioPlaybackService.LocalBinder)?.model
                    state = AudioPlaybackConnection(model, model == null)
                }
            }
            override fun onServiceDisconnected(name: ComponentName) {
                if (active) state = AudioPlaybackConnection(error = true)
            }
        }
        try {
            val intent = AudioPlaybackService.intent(context)
            context.startService(intent)
            bound = context.bindService(intent.setAction(AudioPlaybackService.ACTION_LOCAL)
                .putExtra(AudioPlaybackService.EXTRA_TV, tv),
                connection, Context.BIND_AUTO_CREATE)
            if (!bound) {
                context.stopService(AudioPlaybackService.intent(context))
                state = AudioPlaybackConnection(error = true)
            }
        } catch (_: Exception) {
            context.stopService(AudioPlaybackService.intent(context))
            state = AudioPlaybackConnection(error = true)
        }
        onDispose {
            active = false
            if (bound) context.unbindService(connection)
        }
    }
    return state
}
