package com.kinosail.player.core

import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.os.Binder
import android.os.IBinder
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.ViewModelStore
import androidx.media3.session.MediaSession
import androidx.media3.session.MediaSessionService
import com.kinosail.player.mobile.MobileActivity
import com.kinosail.player.tv.TvActivity

internal class AudioPlaybackService : MediaSessionService() {
    private val models = ViewModelStore()
    private lateinit var playback: PlaybackModel
    private var mediaSession: MediaSession? = null
    private var television = false

    override fun onCreate() {
        super.onCreate()
        playback = ViewModelProvider(models,
            ViewModelProvider.AndroidViewModelFactory.getInstance(application))[PlaybackModel::class.java]
        playback.onPlayerChanged = { player ->
            mediaSession?.let { removeSession(it); it.release() }
            mediaSession = player?.let {
                val target = if (television) TvActivity::class.java else MobileActivity::class.java
                val launch = PendingIntent.getActivity(this, if (television) 1 else 0,
                    Intent(this, target).addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP),
                    PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE)
                MediaSession.Builder(this, it).setSessionActivity(launch).build().also(::addSession)
            }
        }
        running = this
    }

    override fun onBind(intent: Intent): IBinder? {
        if (intent.action != ACTION_LOCAL) return super.onBind(intent)
        television = intent.getBooleanExtra(EXTRA_TV, false)
        return LocalBinder()
    }

    override fun onGetSession(controllerInfo: MediaSession.ControllerInfo): MediaSession? = mediaSession

    override fun onDestroy() {
        if (running === this) running = null
        models.clear()
        mediaSession?.let { removeSession(it); it.release() }
        mediaSession = null
        super.onDestroy()
    }

    inner class LocalBinder : Binder() { val model: PlaybackModel get() = playback }

    companion object {
        const val ACTION_LOCAL = "com.kinosail.player.BIND_AUDIO"
        const val EXTRA_TV = "com.kinosail.player.TV"
        private var running: AudioPlaybackService? = null

        fun intent(context: Context) = Intent(context, AudioPlaybackService::class.java)
        fun nowPlayingFor(viewer: Viewer): CatalogItem? = running?.playback?.nowPlayingFor(viewer)
        fun stopIfRunning(context: Context) {
            running?.playback?.stop()
            context.stopService(intent(context))
        }
    }
}
