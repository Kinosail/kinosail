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
import java.util.UUID
import kotlin.math.abs
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import okhttp3.OkHttpClient

class PlaybackModel(application: Application) : AndroidViewModel(application) {
    private val sessions = SessionStore(application)
    private var generation = 0
    private var source: PlaybackSource? = null
    private var server: ServerAddress? = null
    private var policy: MediaUriPolicy? = null
    private val syncLock = Mutex()
    private var journal: ProgressJournal? = null
    private var activeSession: SavedSession? = null
    private var activeViewer: Viewer? = null
    private var progressSession = ""
    private var progressRevision = 0L
    private var progressJob: Job? = null
    private var lastRecorded: Pair<Double, Boolean>? = null
    var player by mutableStateOf<ExoPlayer?>(null)
        private set
    var loading by mutableStateOf(false)
        private set
    var usingCompatible by mutableStateOf(false)
        private set
    var message by mutableStateOf<String?>(null)
        private set
    var progressNotice by mutableStateOf<String?>(null)
        private set
    var progressConflict by mutableStateOf(false)
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
                val progressApi = ProgressApi(saved.server)
                val savedJournal = ProgressJournal.forViewer(getApplication(), saved.server, viewer)
                val current = withContext(Dispatchers.IO) {
                    val identity = ServerApi(saved.server).viewer(saved.token)
                    require(identity.id == viewer.id && identity.serverId == viewer.serverId) {
                        "The Viewer Profile changed. Reconnect to continue."
                    }
                    syncPending(savedJournal, saved, viewer)
                    progressApi.current(item.id, saved.token, viewer.id)
                }
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
                journal = savedJournal
                activeSession = saved
                activeViewer = viewer
                baseline = current
                val pending = savedJournal.pending().firstOrNull { it.itemId == item.id }
                progressSession = pending?.progress?.session ?: UUID.randomUUID().toString()
                progressRevision = pending?.progress?.revision ?: 0
                progressConflict = pending?.conflict != null
                progressNotice = if (progressConflict) "Watch position changed on another device." else null
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
                        if (state == Player.STATE_ENDED) checkpoint()
                    }

                    override fun onIsPlayingChanged(isPlaying: Boolean) {
                        if (!isPlaying) checkpoint()
                    }

                    override fun onPlayerError(error: PlaybackException) {
                        val current = source
                        if (!usingCompatible && current?.compatible != null && isFormatFailure(error)) {
                            checkpoint()
                            val at = current.compatibleTimeline.presentationTime(
                                engine.currentPosition / 1000.0).times(1000).toLong()
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
                val resume = pending?.progress?.takeIf { !it.watched }?.seconds
                    ?: current.seconds.takeIf { !current.watched }
                val start = resume?.takeIf { it < plan.duration } ?: plan.start
                install(engine, path, if (plan.direct == null) MimeTypes.APPLICATION_M3U8 else plan.directType,
                    ((if (plan.direct == null) plan.compatibleTimeline.presentationTime(start) else start) * 1000).toLong(),
                    plan.direct == null)
                progressJob = viewModelScope.launch {
                    while (attempt == generation) { delay(15_000); checkpoint() }
                }
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

    private fun checkpoint() {
        val engine = player ?: return
        val plan = source ?: return
        val savedJournal = journal ?: return
        val saved = activeSession ?: return
        val viewer = activeViewer ?: return
        if (engine.currentPosition <= 0 || progressConflict || progressRevision >= 9_007_199_254_740_991) return
        val seconds = plan.sourceTime(engine.currentPosition / 1000.0, usingCompatible)
            .coerceIn(0.0, 31_536_000.0)
        val watched = engine.playbackState == Player.STATE_ENDED ||
            engine.duration > 0 && engine.duration != C.TIME_UNSET &&
            engine.currentPosition >= engine.duration - 2_000
        if (lastRecorded?.let { abs(it.first - seconds) < 1.0 && it.second == watched } == true) return
        try {
            val update = WatchProgress(seconds, watched, progressSession, progressRevision + 1)
            val expected = savedJournal.pending().firstOrNull { it.itemId == plan.itemId }?.expected
                ?: baseline
            savedJournal.record(plan.itemId, update, expected)
            progressRevision = update.revision
            lastRecorded = seconds to watched
            viewModelScope.launch(Dispatchers.IO) {
                try { syncPending(savedJournal, saved, viewer) }
                catch (_: Exception) { withContext(Dispatchers.Main) {
                    if (source?.itemId == plan.itemId) progressNotice = "Watch position saved on this device; sync is pending."
                } }
            }
        } catch (_: Exception) { progressNotice = "Could not save watch position on this device." }
    }

    private var baseline = WatchProgress()

    private suspend fun syncPending(savedJournal: ProgressJournal, saved: SavedSession, viewer: Viewer) {
        syncLock.withLock {
            val api = ProgressApi(saved.server)
            for (entry in savedJournal.pending().filter { it.conflict == null }) {
                val result = try {
                    api.sync(entry.itemId, saved.token, viewer.id, entry.progress, entry.expected)
                } catch (error: ServerHttpException) {
                    if (error.status == 404 || error.status == 410) continue else throw error
                }
                savedJournal.apply(entry.itemId, entry.progress, result)
                if (entry.itemId == source?.itemId) withContext(Dispatchers.Main) {
                    if (result.conflict) {
                        progressConflict = true
                        progressNotice = "Watch position changed on another device."
                    } else progressNotice = null
                }
            }
        }
    }

    fun resolveProgress(useDevice: Boolean) {
        val savedJournal = journal ?: return
        val itemId = source?.itemId ?: return
        val saved = activeSession ?: return
        val viewer = activeViewer ?: return
        viewModelScope.launch(Dispatchers.IO) {
            try {
                val remote = savedJournal.pending().firstOrNull { it.itemId == itemId }?.conflict
                syncLock.withLock { savedJournal.resolve(itemId, useDevice) }
                if (useDevice) syncPending(savedJournal, saved, viewer)
                val conflicted = savedJournal.pending().any { it.itemId == itemId && it.conflict != null }
                withContext(Dispatchers.Main) {
                    progressConflict = conflicted
                    progressNotice = if (conflicted) "Watch position changed again on another device." else null
                    if (!useDevice) {
                        baseline = remote ?: baseline
                        progressSession = UUID.randomUUID().toString()
                        progressRevision = 0
                        lastRecorded = null
                    }
                }
            } catch (_: Exception) { withContext(Dispatchers.Main) {
                progressNotice = "Could not resolve watch position. Try again."
            } }
        }
    }

    fun stop() {
        checkpoint()
        generation++
        progressJob?.cancel()
        progressJob = null
        player?.release()
        player = null
        source = null
        server = null
        policy = null
        journal = null
        activeSession = null
        activeViewer = null
        progressSession = ""
        progressRevision = 0
        lastRecorded = null
        baseline = WatchProgress()
        loading = false
        usingCompatible = false
        message = null
        progressNotice = null
        progressConflict = false
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
