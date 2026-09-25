package com.kinosail.player.core

import android.app.Application
import java.util.UUID
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.google.android.gms.cast.CastDevice
import com.google.android.gms.cast.CastMediaControlIntent
import com.google.android.gms.cast.MediaInfo
import com.google.android.gms.cast.HlsSegmentFormat
import com.google.android.gms.cast.HlsVideoSegmentFormat
import com.google.android.gms.cast.MediaLoadRequestData
import com.google.android.gms.cast.MediaMetadata
import com.google.android.gms.cast.MediaTrack
import com.google.android.gms.cast.framework.CastContext
import com.google.android.gms.cast.framework.CastSession
import com.google.android.gms.cast.framework.SessionManagerListener
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

class AndroidCastModel(application: Application) : AndroidViewModel(application) {
    private val sessions = SessionStore(application)
    private var context: CastContext? = null
    private var saved: SavedSession? = null
    private var viewer: Viewer? = null
    private var selected: CatalogItem? = null
    private var localPosition: () -> Double = { 0.0 }
    private var pauseLocal: () -> Unit = {}
    private var generation = 0
    private var playGeneration = 0
    private var progressJob: Job? = null

    var ready by mutableStateOf(false)
        private set
    var connected by mutableStateOf(false)
        private set
    var busy by mutableStateOf(false)
        private set
    var active by mutableStateOf<CastMedia?>(null)
        private set
    var notice by mutableStateOf<String?>(null)
        private set

    private val listener = object : SessionManagerListener<CastSession> {
        override fun onSessionStarted(session: CastSession, sessionId: String) {
            connected = true
            playSelected()
        }
        override fun onSessionResumed(session: CastSession, wasSuspended: Boolean) { connected = true }
        override fun onSessionEnded(session: CastSession, error: Int) { connected = false; ++playGeneration; revoke() }
        override fun onSessionStartFailed(session: CastSession, error: Int) { notice = "Could not connect to that Cast device." }
        override fun onSessionResumeFailed(session: CastSession, error: Int) { connected = false; ++playGeneration; revoke() }
        override fun onSessionStarting(session: CastSession) {}
        override fun onSessionEnding(session: CastSession) {}
        override fun onSessionResuming(session: CastSession, sessionId: String) {}
        override fun onSessionSuspended(session: CastSession, reason: Int) { notice = "Cast connection interrupted." }
    }

    fun configure(item: CatalogItem, viewer: Viewer, position: () -> Double, pause: () -> Unit) {
        if (this.viewer != null && (this.viewer?.id != viewer.id || this.viewer?.serverId != viewer.serverId)) stop()
        selected = item
        localPosition = position
        pauseLocal = pause
        if (this.viewer?.id == viewer.id && this.viewer?.serverId == viewer.serverId && ready) return
        val attempt = ++generation
        ready = false
        this.viewer = viewer
        viewModelScope.launch {
            try {
                val pair = withContext(Dispatchers.IO) {
                    val session = sessions.load() ?: throw IllegalStateException("Connect to your Server first.")
                    session to CastApi(session.server).receiverAppId(session.token, viewer.id)
                }
                if (attempt != generation) return@launch
                saved = pair.first
                val cast = context ?: CastContext.getSharedInstance(getApplication()).also {
                    it.sessionManager.addSessionManagerListener(listener, CastSession::class.java)
                    context = it
                }
                cast.setReceiverApplicationId(pair.second.ifEmpty {
                    CastMediaControlIntent.DEFAULT_MEDIA_RECEIVER_APPLICATION_ID
                })
                connected = cast.sessionManager.currentCastSession?.isConnected == true
                ready = true
                notice = null
            } catch (_: Exception) { if (attempt == generation) notice = "Google Cast is unavailable on this device or Server." }
        }
    }

    fun playSelected() {
        val item = selected ?: return
        val identity = viewer ?: return
        val session = saved ?: return
        val cast = context?.sessionManager?.currentCastSession ?: return
        if (busy || !cast.isConnected) return
        if (item.kind !in setOf("video", "music", "audiobook")) return
        if (cast.castDevice?.hasCapability(if (item.kind == "video") CastDevice.CAPABILITY_VIDEO_OUT
                else CastDevice.CAPABILITY_AUDIO_OUT) != true) {
            notice = if (item.kind == "video") "Choose a Cast TV for video." else "Choose a Cast speaker or TV for audio."
            return
        }
        val position = localPosition().takeIf { it.isFinite() && it in 0.0..31_536_000.0 }
            ?: item.progress.seconds.coerceIn(0.0, 31_536_000.0)
        val playAttempt = ++playGeneration
        busy = true
        viewModelScope.launch {
            var issued: CastMedia? = null
            try {
                if (active != null) { cast.remoteMediaClient?.stop(); revoke() }
                val media = withContext(Dispatchers.IO) {
                    CastApi(session.server).start(item.id, position, session.token, identity.id)
                }
                issued = media
                if (playAttempt != playGeneration) throw IllegalStateException("Cast session ended.")
                val remote = cast.remoteMediaClient ?: throw IllegalStateException("Cast receiver is unavailable.")
                if (context?.sessionManager?.currentCastSession !== cast) throw IllegalStateException("Cast receiver changed.")
                val metadata = MediaMetadata(when (item.kind) {
                    "music" -> MediaMetadata.MEDIA_TYPE_MUSIC_TRACK
                    "audiobook" -> MediaMetadata.MEDIA_TYPE_AUDIOBOOK_CHAPTER
                    else -> MediaMetadata.MEDIA_TYPE_MOVIE
                }).apply {
                    putString(MediaMetadata.KEY_TITLE, media.title)
                    if (item.kind == "music") {
                        putString(MediaMetadata.KEY_ARTIST, item.artist)
                        putString(MediaMetadata.KEY_ALBUM_TITLE, item.album)
                    }
                }
                val infoBuilder = MediaInfo.Builder(media.url).setContentType(media.contentType)
                    .setStreamType(MediaInfo.STREAM_TYPE_BUFFERED).setMetadata(metadata)
                    .setStreamDuration((media.duration * 1000).toLong())
                    .setMediaTracks(media.tracks.map { track ->
                        MediaTrack.Builder(track.id, MediaTrack.TYPE_TEXT).setContentId(track.url)
                            .setContentType("text/vtt").setName(track.label).setLanguage(track.language)
                            .setSubtype(MediaTrack.SUBTYPE_SUBTITLES).build()
                    })
                if (media.contentType == "application/vnd.apple.mpegurl") infoBuilder
                    .setHlsSegmentFormat(HlsSegmentFormat.FMP4)
                    .setHlsVideoSegmentFormat(HlsVideoSegmentFormat.FMP4)
                val info = infoBuilder.build()
                val load = MediaLoadRequestData.Builder().setMediaInfo(info)
                    .setCurrentTime((media.position * 1000).toLong())
                    .setActiveTrackIds(media.tracks.filter(CastTrack::isDefault).map(CastTrack::id).toLongArray())
                    .build()
                remote.load(load).setResultCallback { result ->
                    if (playAttempt != playGeneration || this@AndroidCastModel.viewer != identity ||
                        context?.sessionManager?.currentCastSession !== cast) {
                        if (remote.mediaInfo?.contentId == media.url) remote.stop()
                        viewModelScope.launch(Dispatchers.IO) {
                            runCatching { CastApi(session.server).end(media.id, session.token, identity.id) }
                        }
                    } else if (result.status.isSuccess) {
                        active = media
                        pauseLocal()
                        notice = "Playing on ${cast.castDevice?.friendlyName ?: "Cast device"}."
                        trackProgress(item.id, media, session, identity)
                    } else {
                        notice = "The Cast receiver could not play this title."
                        viewModelScope.launch(Dispatchers.IO) {
                            runCatching { CastApi(session.server).end(media.id, session.token, identity.id) }
                        }
                    }
                    if (playAttempt == playGeneration) busy = false
                }
            } catch (_: Exception) {
                issued?.let { media -> viewModelScope.launch(Dispatchers.IO) {
                    runCatching { CastApi(session.server).end(media.id, session.token, identity.id) }
                } }
                if (playAttempt == playGeneration) {
                    busy = false
                    notice = "Could not start playback on that Cast device."
                }
            }
        }
    }

    fun playPause() {
        val remote = context?.sessionManager?.currentCastSession?.remoteMediaClient ?: return
        if (remote.isPlaying) remote.pause() else remote.play()
    }

    fun seek(seconds: Double) {
        val media = active ?: return
        if (!seconds.isFinite() || seconds !in 0.0..media.duration) return
        context?.sessionManager?.currentCastSession?.remoteMediaClient?.seek((seconds * 1000).toLong())
    }

    fun skip(delta: Double) {
        val remote = context?.sessionManager?.currentCastSession?.remoteMediaClient ?: return
        val duration = active?.duration ?: return
        seek((remote.approximateStreamPosition / 1000.0 + delta).coerceIn(0.0, duration))
    }

    fun stop() {
        ++playGeneration
        busy = false
        context?.sessionManager?.currentCastSession?.remoteMediaClient?.stop()
        context?.sessionManager?.endCurrentSession(true)
        revoke()
    }

    fun signOut() {
        stop()
        selected = null
        saved = null
        viewer = null
        ready = false
        connected = false
        context?.setReceiverApplicationId("")
    }

    private fun revoke() {
        val media = active ?: return
        progressJob?.cancel()
        progressJob = null
        active = null
        val session = saved ?: return
        val identity = viewer ?: return
        viewModelScope.launch(Dispatchers.IO) {
            runCatching { CastApi(session.server).end(media.id, session.token, identity.id) }
        }
    }

    private fun trackProgress(itemId: String, media: CastMedia, session: SavedSession, identity: Viewer) {
        progressJob?.cancel()
        progressJob = viewModelScope.launch {
            val api = ProgressApi(session.server)
            var expected = try { withContext(Dispatchers.IO) { api.current(itemId, session.token, identity.id) } }
                catch (_: Exception) { notice = "Cast playback started; watch position could not sync."; return@launch }
            val progressSession = UUID.randomUUID().toString()
            var revision = 0L
            while (active?.id == media.id) {
                delay(15_000)
                val remote = context?.sessionManager?.currentCastSession?.remoteMediaClient ?: break
                if (remote.mediaInfo?.contentId != media.url) break
                val seconds = (remote.approximateStreamPosition / 1000.0).coerceIn(0.0, media.duration)
                val watched = media.duration > 0 && seconds >= media.duration - 2
                if (seconds <= 0 || revision >= 9_007_199_254_740_990L) continue
                val next = WatchProgress(seconds, watched, progressSession, ++revision)
                try {
                    val result = withContext(Dispatchers.IO) {
                        api.sync(itemId, session.token, identity.id, next, expected)
                    }
                    if (result.conflict) {
                        notice = "Watch position changed on another device. Open this title to resolve it."
                        break
                    }
                    expected = result.progress
                } catch (_: Exception) { notice = "Cast playback continues; watch position could not sync." }
            }
        }
    }

    override fun onCleared() {
        context?.sessionManager?.removeSessionManagerListener(listener, CastSession::class.java)
        super.onCleared()
    }
}
