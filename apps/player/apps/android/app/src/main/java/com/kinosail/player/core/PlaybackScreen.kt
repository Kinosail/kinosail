package com.kinosail.player.core

import android.view.KeyEvent
import android.view.View
import android.view.accessibility.AccessibilityManager
import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.focusable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.input.key.onKeyEvent
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.core.view.ViewCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.doOnAttach
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.media3.common.Player
import androidx.media3.common.util.UnstableApi
import androidx.media3.session.MediaSession
import androidx.media3.ui.PlayerView
import androidx.compose.ui.viewinterop.AndroidView
import androidx.mediarouter.app.MediaRouteButton
import com.google.android.gms.cast.framework.CastButtonFactory
import com.kinosail.player.design.KinoColor

@androidx.annotation.OptIn(markerClass = [UnstableApi::class])
@Composable
internal fun PlaybackScreen(item: CatalogItem, viewer: Viewer, tv: Boolean, close: () -> Unit,
                            onNext: (CatalogItem) -> Unit, pipHost: VideoPipHost? = null) {
    val audio = item.kind == "music" || item.kind == "audiobook"
    val videoPipHost = if (audio) null else pipHost
    val audioConnection = rememberAudioPlaybackConnection(audio, tv)
    val playback = if (audio) audioConnection.model else viewModel<PlaybackModel>()
    if (playback == null) {
        BackHandler(onBack = close)
        Column(Modifier.fillMaxSize().background(Color.Black).safeDrawingPadding().padding(24.dp),
            verticalArrangement = Arrangement.spacedBy(20.dp)) {
            Text(if (audioConnection.error) "Could not start audio. Return to the library and try again."
                else "Starting audio…", color = Color.White,
                modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite })
            if (tv) androidx.tv.material3.Button(onClick = close) {
                androidx.tv.material3.Text(interfaceText("Done"))
            } else TextButton(onClick = close) { Text(interfaceText("Done"), color = KinoColor.signal) }
        }
        return
    }
    val player = playback.player
    val casting = if (tv) null else viewModel<AndroidCastModel>()
    LaunchedEffect(item.id, viewer.id, playback) {
        casting?.configure(item, viewer,
            position = { playback.player?.currentPosition?.div(1000.0) ?: item.progress.seconds },
            pause = { playback.player?.pause() })
    }
    var speedPicker by remember { mutableStateOf(false) }
    var trackPicker by remember { mutableStateOf(false) }
    val speedButtonFocus = remember { FocusRequester() }
    val retryFocus = remember { FocusRequester() }
    val trackButtonFocus = remember { FocusRequester() }
    val promptFocus = remember { FocusRequester() }
    val surfaceFocus = remember { FocusRequester() }
    val context = LocalContext.current
    var controlsVisible by remember(player) { mutableStateOf(true) }
    val accessibleControls = (context.getSystemService(android.content.Context.ACCESSIBILITY_SERVICE) as? AccessibilityManager)?.isEnabled == true
    val closeTracks = { trackPicker = false; if (tv) runCatching { trackButtonFocus.requestFocus() } }
    val closeSpeed = { speedPicker = false; if (tv) runCatching { speedButtonFocus.requestFocus() } }
    val playerView = remember(player, context) { player?.let { engine -> PlayerView(context).apply {
        this.player = engine
        findViewById<View>(androidx.media3.ui.R.id.exo_settings)?.visibility = View.GONE
        useController = true
        setShowPreviousButton(false)
        setShowNextButton(false)
        controllerShowTimeoutMs = if (accessibleControls) 0 else 4_000
        ViewCompat.setOnApplyWindowInsetsListener(this) { _, insets ->
            val safe = insets.getInsets(WindowInsetsCompat.Type.systemBars() or
                WindowInsetsCompat.Type.displayCutout())
            subtitleView?.setPadding(safe.left, safe.top, safe.right, safe.bottom)
            insets
        }
        doOnAttach { ViewCompat.requestApplyInsets(it) }
    } } }
    DisposableEffect(playerView) {
        playerView?.setControllerVisibilityListener(PlayerView.ControllerVisibilityListener { visibility ->
            controlsVisible = visibility == View.VISIBLE
        })
        controlsVisible = playerView?.isControllerFullyVisible == true
        onDispose { playerView?.setControllerVisibilityListener(null as PlayerView.ControllerVisibilityListener?) }
    }
    DisposableEffect(playerView, videoPipHost) {
        videoPipHost?.setVideoPipView(playerView)
        onDispose { videoPipHost?.setVideoPipView(null) }
    }
    val inPip = videoPipHost?.inPictureInPicture == true
    BackHandler { if (speedPicker) closeSpeed() else if (trackPicker) closeTracks() else close() }
    LaunchedEffect(item.id, viewer.id, playback) {
        if (playback.activeItemId != item.id || playback.player?.playbackState == Player.STATE_ENDED)
            playback.start(item, viewer, tv)
    }
    LaunchedEffect(speedPicker, trackPicker) { if (speedPicker || trackPicker) playerView?.hideController() }
    LaunchedEffect(playback.autoNext) { playback.advanceAutomatically(onNext) }
    val prompt = playback.prompt.takeIf { !inPip && !audio }
    LaunchedEffect(prompt?.title, tv) {
        if (!tv || speedPicker || trackPicker) return@LaunchedEffect
        withFrameNanos { }
        runCatching { if (prompt != null) promptFocus.requestFocus() else surfaceFocus.requestFocus() }
    }
    LaunchedEffect(playback.retryable, tv) { if (playback.retryable && tv) retryFocus.requestFocus() }
    DisposableEffect(playback, audio) { onDispose { if (!audio) playback.stop() } }
    DisposableEffect(player, videoPipHost) {
        val session = if (player != null && videoPipHost != null)
            MediaSession.Builder(context, player).build() else null
        val listener = object : Player.Listener {
            override fun onIsPlayingChanged(isPlaying: Boolean) {
                videoPipHost?.setVideoPipReady(isPlaying)
            }
        }
        player?.addListener(listener)
        videoPipHost?.setVideoPipReady(player?.isPlaying == true)
        onDispose {
            player?.removeListener(listener)
            videoPipHost?.setVideoPipReady(false)
            session?.release()
        }
    }
    Box(Modifier.fillMaxSize().background(Color.Black)) {
        if (playerView != null) AndroidView(
            factory = { playerView.apply { if (tv) post { requestFocus() } } },
            update = { it.player = player; it.useController = !inPip },
            modifier = Modifier.fillMaxSize().then(if (tv) Modifier.focusRequester(surfaceFocus).focusable().onKeyEvent {
                it.nativeKeyEvent.keyCode != KeyEvent.KEYCODE_BACK &&
                    playerView.dispatchKeyEvent(it.nativeKeyEvent)
            } else Modifier),
        )
        val overlay = !inPip && (controlsVisible || accessibleControls || audio || speedPicker || trackPicker || playback.loading ||
            playback.message != null || playback.progressNotice != null || playback.preferenceNotice != null)
        if (overlay) PlaybackTopFade(tv)
        if (overlay) Column(Modifier.fillMaxSize().safeDrawingPadding().padding(if (tv) 40.dp else 16.dp),
            verticalArrangement = Arrangement.SpaceBetween) {
            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                PlaybackHeader(item, tv, close) {
                    if (casting?.ready == true) AndroidView(factory = { routeContext ->
                        MediaRouteButton(routeContext).apply {
                            contentDescription = "Choose Google Cast device"
                            CastButtonFactory.setUpMediaRouteButton(routeContext.applicationContext, this)
                        }
                    })
                    if (casting?.connected == true) PlaybackAction(interfaceText("Play on Cast"), tv,
                        casting::playSelected, enabled = !casting.busy)
                    if (player != null) PlaybackAction("Speed ${speedText(playback.playbackSpeed)}", tv,
                        { trackPicker = false; speedPicker = !speedPicker }, focus = speedButtonFocus)
                    if (playback.audioTracks.size > 1 || playback.textTracks.size > 1) PlaybackAction(
                        interfaceText("Audio & captions"), tv, { speedPicker = false; trackPicker = !trackPicker },
                        focus = trackButtonFocus)
                    else if (playback.captionsAvailable) PlaybackAction(
                        if (playback.captionsEnabled) "Captions on" else "Captions off", tv, playback::toggleCaptions)
                }
                if (casting?.active != null) Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    TextButton(onClick = { casting.skip(-30.0) }) { Text(interfaceText("Back 30"), color = KinoColor.signal) }
                    TextButton(onClick = casting::playPause) { Text(interfaceText("Play / pause"), color = KinoColor.signal) }
                    TextButton(onClick = { casting.skip(30.0) }) { Text(interfaceText("Forward 30"), color = KinoColor.signal) }
                    TextButton(onClick = casting::stop) { Text(interfaceText("Stop Cast"), color = KinoColor.signal) }
                }
                casting?.notice?.let { Text(it, color = Color.White,
                    modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite }) }
            }
            Column(Modifier.fillMaxWidth().padding(bottom = if (controlsVisible) PlaybackControllerHeight else 0.dp).then(if (playback.loading || playback.usingCompatible ||
                    playback.message != null || playback.progressNotice != null || playback.preferenceNotice != null)
                    Modifier.background(Color.Black.copy(alpha = 0.82f)).padding(12.dp) else Modifier),
                verticalArrangement = Arrangement.spacedBy(12.dp)) {
                if (playback.loading) CircularProgressIndicator(color = KinoColor.signal)
                if (playback.usingCompatible) Text(interfaceText("Compatible playback"), color = Color.White)
                playback.message?.let { Text(it, color = Color.White,
                    modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite }) }
                if (playback.retryable) {
                    if (tv) androidx.tv.material3.Button(onClick = { playback.start(item, viewer, tv) },
                        modifier = Modifier.focusRequester(retryFocus)) {
                        androidx.tv.material3.Text(interfaceText("Try again"))
                    } else Button(onClick = { playback.start(item, viewer) }) { Text(interfaceText("Try again")) }
                }
                playback.progressNotice?.let { Text(it, color = Color.White,
                    modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite }) }
                playback.preferenceNotice?.let { Text(it, color = Color.White,
                    modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite }) }
                if (playback.progressConflict) Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    if (tv) {
                        androidx.tv.material3.Button(onClick = { playback.resolveProgress(true) }) {
                            androidx.tv.material3.Text(interfaceText("Use this device"))
                        }
                        androidx.tv.material3.Button(onClick = { playback.resolveProgress(false) }) {
                            androidx.tv.material3.Text(interfaceText("Keep other device"))
                        }
                    } else {
                        TextButton(onClick = { playback.resolveProgress(true) }) { Text(interfaceText("Use this device")) }
                        TextButton(onClick = { playback.resolveProgress(false) }) { Text(interfaceText("Keep other device")) }
                    }
                }
            }
        }
        if (prompt != null && !speedPicker && !trackPicker)
            PlaybackPromptButton(prompt, tv, controlsVisible, promptFocus) { playback.choosePrompt(onNext) }
        if (!inPip) ScrubPreview(playerView, playback, tv)
        if (speedPicker && !inPip) PlaybackChoiceSheet(interfaceText("Playback speed"), listOf(PlaybackChoiceGroup(null,
            PlaybackModel.SPEEDS.map { rate -> PlaybackChoice(if (rate == 1f) interfaceText("Normal") else speedText(rate),
                playback.playbackSpeed == rate) { playback.changeSpeed(rate) } })), tv, closeSpeed)
        if (trackPicker && !inPip) PlaybackChoiceSheet(interfaceText("Audio & captions"), listOfNotNull(
            if (playback.audioTracks.size > 1) PlaybackChoiceGroup(interfaceText("Audio"),
                playback.audioTracks.map { PlaybackChoice(it.label, it.selected) { playback.selectAudio(it) } }) else null,
            if (playback.textTracks.size > 1) PlaybackChoiceGroup(interfaceText("Captions"),
                listOf(PlaybackChoice(interfaceText("Off"), !playback.captionsEnabled) { playback.selectText(null) }) +
                    playback.textTracks.map { PlaybackChoice(it.label, it.selected) { playback.selectText(it) } }) else null),
            tv, closeTracks)
    }
}

private fun speedText(rate: Float): String = if (rate % 1f == 0f) "${rate.toInt()}×" else "$rate×"
