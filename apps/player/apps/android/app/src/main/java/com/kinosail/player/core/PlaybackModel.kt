package com.kinosail.player.core

import android.app.Application
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import androidx.media3.common.AudioAttributes
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.MimeTypes
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.datasource.ResolvingDataSource
import androidx.media3.datasource.okhttp.OkHttpDataSource
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import java.io.IOException
import java.net.URL
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import okhttp3.OkHttpClient

class PlaybackModel(application: Application) : AndroidViewModel(application) {
    private val sessions = SessionStore(application)
    private var generation = 0
    private var source: PlaybackSource? = null
    private var server: ServerAddress? = null
    private var policy: MediaUriPolicy? = null
    var player by mutableStateOf<ExoPlayer?>(null)
        private set
    var loading by mutableStateOf(false)
        private set
    var usingCompatible by mutableStateOf(false)
        private set
    var message by mutableStateOf<String?>(null)
        private set

    fun start(item: CatalogItem, viewer: Viewer) {
        stop()
        if (item.kind !in setOf("video", "music", "audiobook")) {
            message = "This title cannot be played here."
            return
        }
        val attempt = generation
        loading = true
        viewModelScope.launch {
            try {
                val capabilities = PlaybackCapabilities.detect(getApplication())
                val saved = withContext(Dispatchers.IO) { sessions.load() }
                    ?: throw IllegalStateException("No saved connection")
                val plan = withContext(Dispatchers.IO) {
                    PlaybackApi(saved.server).source(item.id, saved.token, viewer.id, capabilities)
                }
                if (attempt != generation) return@launch
                val allowed = MediaUriPolicy(saved.server, item.id)
                plan.direct?.let { allowed.requireAllowed(URL(saved.server.url, it).toString()) }
                plan.compatible?.let { allowed.requireAllowed(URL(saved.server.url, it).toString()) }
                server = saved.server
                policy = allowed
                source = plan
                val http = OkHttpDataSource.Factory(MEDIA_CLIENT).setDefaultRequestProperties(mapOf(
                    "Authorization" to "Bearer ${saved.token}", "X-Kinosail-Viewer-Profile" to viewer.id))
                val guarded = ResolvingDataSource.Factory(http) { request ->
                    try { allowed.requireAllowed(request.uri.toString()); request }
                    catch (_: IllegalArgumentException) { throw IOException("Blocked media request.") }
                }
                val engine = ExoPlayer.Builder(getApplication<Application>())
                    .setMediaSourceFactory(DefaultMediaSourceFactory(guarded))
                    .build()
                engine.setAudioAttributes(AudioAttributes.Builder().setUsage(C.USAGE_MEDIA)
                    .setContentType(if (item.kind == "video") C.AUDIO_CONTENT_TYPE_MOVIE else C.AUDIO_CONTENT_TYPE_MUSIC)
                    .build(), true)
                engine.addListener(object : Player.Listener {
                    override fun onPlaybackStateChanged(state: Int) {
                        loading = state == Player.STATE_BUFFERING || state == Player.STATE_IDLE
                    }

                    override fun onPlayerError(error: PlaybackException) {
                        val current = source
                        if (!usingCompatible && current?.compatible != null && isFormatFailure(error)) {
                            val at = if (current.compatibleStart == 0.0) 0 else
                                maxOf(engine.currentPosition, (current.compatibleStart * 1000).toLong())
                            install(engine, current.compatible, MimeTypes.APPLICATION_M3U8,
                                at, true)
                        } else {
                            loading = false
                            message = "Playback stopped. Check this title and your Server connection."
                        }
                    }
                })
                player = engine
                val path = plan.direct ?: requireNotNull(plan.compatible)
                install(engine, path, if (plan.direct == null) MimeTypes.APPLICATION_M3U8 else plan.directType,
                    ((if (plan.direct == null) plan.compatibleStart else plan.start) * 1000).toLong(),
                    plan.direct == null)
            } catch (error: ServerHttpException) {
                if (attempt == generation) fail(if (error.status == 401 || error.status == 403)
                    "This connection cannot play this title. Reconnect or check the Viewer permissions."
                else "Could not load this title from the Server.")
            } catch (_: Exception) {
                if (attempt == generation) fail("Could not start playback. Try again.")
            }
        }
    }

    private fun install(engine: ExoPlayer, path: String, type: String, at: Long, compatible: Boolean) {
        val target = requireNotNull(server)
        requireNotNull(policy).requireAllowed(URL(target.url, path).toString())
        usingCompatible = compatible
        message = null
        engine.setMediaItem(MediaItem.Builder().setUri(URL(target.url, path).toString()).setMimeType(type).build())
        engine.prepare()
        if (at > 0) engine.seekTo(at)
        engine.playWhenReady = true
    }

    private fun fail(text: String) { loading = false; message = text }

    fun stop() {
        generation++
        player?.release()
        player = null
        source = null
        server = null
        policy = null
        loading = false
        usingCompatible = false
        message = null
    }

    override fun onCleared() { stop(); super.onCleared() }

    companion object {
        private val MEDIA_CLIENT = OkHttpClient.Builder().followRedirects(false).followSslRedirects(false).build()
        internal fun isFormatFailure(error: PlaybackException): Boolean = error.errorCode in setOf(
            PlaybackException.ERROR_CODE_PARSING_CONTAINER_UNSUPPORTED,
            PlaybackException.ERROR_CODE_DECODER_INIT_FAILED,
            PlaybackException.ERROR_CODE_DECODER_QUERY_FAILED,
            PlaybackException.ERROR_CODE_DECODING_FAILED,
            PlaybackException.ERROR_CODE_DECODING_FORMAT_EXCEEDS_CAPABILITIES,
            PlaybackException.ERROR_CODE_DECODING_FORMAT_UNSUPPORTED)
    }
}
