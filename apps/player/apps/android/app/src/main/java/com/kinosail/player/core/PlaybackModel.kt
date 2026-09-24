package com.kinosail.player.core

import android.app.Application
import android.net.Uri
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import androidx.media3.common.util.UnstableApi
import androidx.media3.common.AudioAttributes
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.MediaMetadata
import androidx.media3.common.MimeTypes
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.common.Tracks
import androidx.media3.datasource.ResolvingDataSource
import androidx.media3.datasource.okhttp.OkHttpDataSource
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import java.io.IOException
import java.net.URL
import java.util.UUID
import java.util.concurrent.CancellationException
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
    private var activeTitle = ""
    private var server: ServerAddress? = null
    private var policy: MediaUriPolicy? = null
    private val syncLock = Mutex()
    private var journal: ProgressJournal? = null
    private var activeSession: SavedSession? = null
    private var activeViewer: Viewer? = null
    private var progressSession = ""
    private var progressRevision = 0L
    private var progressJob: Job? = null
    private var rateSave: Job? = null
    private var rateChoice = 0
    private var preferences: PlaybackPreferences? = null
    private val trackChoices = PlaybackTracks()
    private var lastRecorded: Pair<Double, Boolean>? = null
    var player by mutableStateOf<ExoPlayer?>(null)
        private set
    var loading by mutableStateOf(false)
        private set
    var usingCompatible by mutableStateOf(false)
        private set
    var message by mutableStateOf<String?>(null)
        private set
    var retryable by mutableStateOf(false)
        private set
    var progressNotice by mutableStateOf<String?>(null)
        private set
    var progressConflict by mutableStateOf(false)
        private set
    var nextItemId by mutableStateOf<String?>(null)
        private set
    var nextBusy by mutableStateOf(false)
        private set
    val captionsAvailable get() = trackChoices.text.isNotEmpty()
    val captionsEnabled get() = trackChoices.captionsEnabled
    internal val audioTracks get() = trackChoices.audio
    internal val textTracks get() = trackChoices.text
    var playbackSpeed by mutableStateOf(1f)
        private set
    var preferenceNotice by mutableStateOf<String?>(null)
        private set
    internal var onPlayerChanged: ((ExoPlayer?) -> Unit)? = null
    internal val activeItemId get() = source?.itemId
    private var activeAudioItem by mutableStateOf<CatalogItem?>(null)

    internal fun nowPlayingFor(viewer: Viewer): CatalogItem? = activeAudioItem?.takeIf {
        activeViewer?.id == viewer.id && activeViewer?.serverId == viewer.serverId && player != null
    }

    @androidx.annotation.OptIn(markerClass = [UnstableApi::class])
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
                rateSave?.join()
                val loadedPreferences = try {
                    withContext(Dispatchers.IO) {
                        PlaybackPreferencesApi(saved.server).load(item.id, saved.token, viewer.id)
                    }
                } catch (error: CancellationException) { throw error }
                catch (_: Exception) { null }
                if (attempt != generation) return@launch
                val allowed = MediaUriPolicy(saved.server, item.id)
                plan.direct?.let { allowed.requireAllowed(URL(saved.server.url, it).toString()) }
                plan.compatible?.let { allowed.requireAllowed(URL(saved.server.url, it).toString()) }
                plan.subtitles.forEach { allowed.requireAllowed(URL(saved.server.url, it.path).toString()) }
                server = saved.server
                policy = allowed
                source = plan
                activeTitle = item.title
                preferences = loadedPreferences
                playbackSpeed = loadedPreferences?.rate?.toFloat() ?: 1f
                preferenceNotice = if (loadedPreferences == null)
                    "Playback speed is available on this device; Server preference could not be loaded." else null
                nextItemId = plan.nextItemId
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
                engine.setPlaybackSpeed(playbackSpeed)
                engine.addListener(object : Player.Listener {
                    override fun onTracksChanged(tracks: Tracks) { trackChoices.update(tracks) }
                    override fun onPlaybackStateChanged(state: Int) {
                        loading = state == Player.STATE_BUFFERING || state == Player.STATE_IDLE && message == null
                        if (state == Player.STATE_ENDED) {
                            checkpoint()
                            activeAudioItem = null
                        }
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
                            retryable = true
                        }
                    }
                })
                player = engine
                try { onPlayerChanged?.invoke(engine) }
                catch (error: Exception) { player = null; engine.release(); throw error }
                val path = plan.direct ?: requireNotNull(plan.compatible)
                val resume = pending?.progress?.takeIf { !it.watched }?.seconds
                    ?: current.seconds.takeIf { !current.watched }
                val start = resume?.takeIf { it < plan.duration } ?: plan.start
                install(engine, path, if (plan.direct == null) MimeTypes.APPLICATION_M3U8 else plan.directType,
                    ((if (plan.direct == null) plan.compatibleTimeline.presentationTime(start) else start) * 1000).toLong(),
                    plan.direct == null)
                activeAudioItem = item.takeIf { it.kind == "music" || it.kind == "audiobook" }
                progressJob = viewModelScope.launch {
                    while (attempt == generation) { delay(15_000); checkpoint() }
                }
            } catch (error: ServerHttpException) {
                if (attempt == generation) fail(if (error.status == 401 || error.status == 403)
                    "This connection cannot play this title. Reconnect or check the Viewer permissions."
                else "Could not load this title from the Server.", error.status != 401 && error.status != 403)
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
        val tracks = source?.subtitles.orEmpty().takeIf {
            !compatible || source?.compatibleTimeline?.omitted?.isEmpty() == true
        }.orEmpty().map { track ->
            MediaItem.SubtitleConfiguration.Builder(Uri.parse(URL(target.url, track.path).toString()))
                .setMimeType(MimeTypes.TEXT_VTT).setLabel(track.label).setLanguage(track.language)
                .setSelectionFlags((if (track.isDefault) C.SELECTION_FLAG_DEFAULT else 0) or
                    (if (track.forced) C.SELECTION_FLAG_FORCED else 0)).build()
        }
        trackChoices.prepare(engine, source?.subtitles?.any(PlaybackSubtitle::isDefault) == true)
        engine.setMediaItem(MediaItem.Builder().setUri(URL(target.url, path).toString()).setMimeType(type)
            .setMediaId(requireNotNull(source).itemId)
            .setMediaMetadata(MediaMetadata.Builder().setTitle(activeTitle).build())
            .setSubtitleConfigurations(tracks).build())
        engine.prepare()
        if (at > 0) engine.seekTo(at)
        engine.playWhenReady = true
    }

    private fun fail(text: String, canRetry: Boolean = true) {
        loading = false
        message = text
        retryable = canRetry
    }

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

    fun playNext(open: (CatalogItem) -> Unit) {
        val id = nextItemId ?: return
        val saved = activeSession ?: return
        val viewer = activeViewer ?: return
        if (nextBusy) return
        nextBusy = true
        val attempt = generation
        viewModelScope.launch {
            try {
                val item = withContext(Dispatchers.IO) {
                    CatalogApi(saved.server).item(id, saved.token, viewer.id)
                }
                if (attempt == generation) {
                    require(item.kind in setOf("video", "music", "audiobook")) { "Invalid next title." }
                    open(item)
                }
            } catch (_: Exception) {
                if (attempt == generation) message = "Could not load the next episode. Try again."
            } finally {
                if (attempt == generation) nextBusy = false
            }
        }
    }

    fun toggleCaptions() {
        val engine = player ?: return
        if (!captionsAvailable) return
        trackChoices.toggleCaptions(engine)
    }

    internal fun selectAudio(choice: PlaybackTrack) { player?.let { trackChoices.selectAudio(it, choice) } }
    internal fun selectText(choice: PlaybackTrack?) { player?.let { trackChoices.selectText(it, choice) } }

    fun changeSpeed(rate: Float) {
        val selected = checkedSpeed(rate)
        val engine = player ?: return
        engine.setPlaybackSpeed(selected)
        playbackSpeed = selected
        val current = preferences ?: run {
            preferenceNotice = "Speed changed on this device; Server preference could not be saved."
            return
        }
        val saved = activeSession ?: return
        val viewer = activeViewer ?: return
        val itemId = source?.itemId ?: return
        val next = current.copy(rate = selected.toDouble())
        preferences = next
        val previous = rateSave
        val choice = ++rateChoice
        val attempt = generation
        rateSave = viewModelScope.launch {
            previous?.join()
            if (choice != rateChoice) return@launch
            try {
                withContext(Dispatchers.IO) {
                    PlaybackPreferencesApi(saved.server).save(itemId, saved.token, viewer.id, next)
                }
                if (attempt == generation && playbackSpeed == selected) preferenceNotice = null
            } catch (error: CancellationException) { throw error }
            catch (_: Exception) {
                if (attempt == generation && playbackSpeed == selected)
                    preferenceNotice = "Speed changed on this device; Server preference could not be saved."
            }
        }
    }

    fun stop() {
        checkpoint()
        generation++
        progressJob?.cancel()
        progressJob = null
        runCatching { onPlayerChanged?.invoke(null) }
        player?.release()
        player = null
        source = null
        activeAudioItem = null
        activeTitle = ""
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
        retryable = false
        progressNotice = null
        progressConflict = false
        nextItemId = null
        nextBusy = false
        trackChoices.clear()
        playbackSpeed = 1f
        preferences = null
        preferenceNotice = null
    }

    override fun onCleared() { stop(); super.onCleared() }

    companion object {
        internal val SPEEDS = listOf(0.5f, 0.75f, 1f, 1.25f, 1.5f, 1.75f, 2f, 2.5f, 3f)
        internal fun checkedSpeed(rate: Float): Float {
            require(rate in SPEEDS) { "Choose a supported playback speed." }
            return rate
        }
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
