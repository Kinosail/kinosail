package com.kinosail.player.core

import android.view.KeyEvent
import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.focusable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
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
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.media3.ui.PlayerView
import androidx.compose.ui.viewinterop.AndroidView
import com.kinosail.player.design.KinoColor

@Composable
internal fun PlaybackScreen(item: CatalogItem, viewer: Viewer, tv: Boolean, close: () -> Unit,
                            onNext: (CatalogItem) -> Unit) {
    val playback: PlaybackModel = viewModel()
    val player = playback.player
    var speedPicker by remember { mutableStateOf(false) }
    val speedFocus = remember { FocusRequester() }
    val context = LocalContext.current
    val playerView = remember(player, context) { player?.let { engine -> PlayerView(context).apply {
        this.player = engine
        useController = true
        controllerShowTimeoutMs = if (tv) 5_000 else 3_000
    } } }
    BackHandler { if (speedPicker) speedPicker = false else close() }
    LaunchedEffect(item.id, viewer.id) { playback.start(item, viewer) }
    LaunchedEffect(speedPicker, tv) { if (speedPicker && tv) speedFocus.requestFocus() }
    DisposableEffect(Unit) { onDispose { playback.stop() } }
    Box(Modifier.fillMaxSize().background(Color.Black)) {
        if (playerView != null) AndroidView(
            factory = { playerView.apply { if (tv) post { requestFocus() } } },
            update = { it.player = player },
            modifier = Modifier.fillMaxSize().then(if (tv) Modifier.focusable().onKeyEvent {
                it.nativeKeyEvent.keyCode != KeyEvent.KEYCODE_BACK &&
                    playerView.dispatchKeyEvent(it.nativeKeyEvent)
            } else Modifier),
        )
        Column(Modifier.fillMaxSize().safeDrawingPadding().padding(if (tv) 40.dp else 16.dp),
            verticalArrangement = Arrangement.SpaceBetween) {
            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Text(item.title, color = Color.White, modifier = Modifier.fillMaxWidth(), maxLines = 1,
                    overflow = TextOverflow.Ellipsis)
                Row(Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()),
                    horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    if (player != null) {
                        val label = "Speed ${speedText(playback.playbackSpeed)}"
                        if (tv) androidx.tv.material3.Button(onClick = { speedPicker = !speedPicker }) {
                            androidx.tv.material3.Text(label)
                        } else TextButton(onClick = { speedPicker = !speedPicker }) {
                            Text(label, color = KinoColor.signal)
                        }
                    }
                    if (playback.captionsAvailable) {
                        val label = if (playback.captionsEnabled) "Captions on" else "Captions off"
                        if (tv) androidx.tv.material3.Button(onClick = playback::toggleCaptions) {
                            androidx.tv.material3.Text(label)
                        } else TextButton(onClick = playback::toggleCaptions) {
                            Text(label, color = KinoColor.signal)
                        }
                    }
                    if (playback.nextItemId != null) {
                        if (tv) androidx.tv.material3.Button(onClick = { playback.playNext(onNext) },
                            enabled = !playback.nextBusy) { androidx.tv.material3.Text("Next episode") }
                        else TextButton(onClick = { playback.playNext(onNext) }, enabled = !playback.nextBusy) {
                            Text("Next episode", color = KinoColor.signal)
                        }
                    }
                    if (tv) androidx.tv.material3.Button(onClick = close) {
                        androidx.tv.material3.Text("Done")
                    } else TextButton(onClick = close) { Text("Done", color = KinoColor.signal) }
                }
                if (speedPicker) Column(Modifier.fillMaxWidth().background(Color.Black.copy(alpha = 0.82f))
                    .padding(12.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    Text("Playback speed", color = Color.White)
                    LazyRow(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                        items(PlaybackModel.SPEEDS) { rate ->
                            val label = if (rate == 1f) "Normal" else speedText(rate)
                            val chosen = if (playback.playbackSpeed == rate) "$label · Selected" else label
                            val select = { playback.changeSpeed(rate); speedPicker = false }
                            if (tv) androidx.tv.material3.Button(onClick = select,
                                modifier = if (rate == PlaybackModel.SPEEDS.first())
                                    Modifier.focusRequester(speedFocus) else Modifier) {
                                androidx.tv.material3.Text(chosen)
                            } else TextButton(onClick = select) { Text(chosen, color = KinoColor.signal) }
                        }
                    }
                }
            }
            Column(Modifier.fillMaxWidth().then(if (playback.loading || playback.usingCompatible ||
                    playback.message != null || playback.progressNotice != null)
                    Modifier.background(Color.Black.copy(alpha = 0.82f)).padding(12.dp) else Modifier),
                verticalArrangement = Arrangement.spacedBy(12.dp)) {
                if (playback.loading) CircularProgressIndicator(color = KinoColor.signal)
                if (playback.usingCompatible) Text("Compatible playback", color = Color.White)
                playback.message?.let { Text(it, color = Color.White) }
                playback.progressNotice?.let { Text(it, color = Color.White,
                    modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite }) }
                if (playback.progressConflict) Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    if (tv) {
                        androidx.tv.material3.Button(onClick = { playback.resolveProgress(true) }) {
                            androidx.tv.material3.Text("Use this device")
                        }
                        androidx.tv.material3.Button(onClick = { playback.resolveProgress(false) }) {
                            androidx.tv.material3.Text("Keep other device")
                        }
                    } else {
                        TextButton(onClick = { playback.resolveProgress(true) }) { Text("Use this device") }
                        TextButton(onClick = { playback.resolveProgress(false) }) { Text("Keep other device") }
                    }
                }
            }
        }
    }
}

private fun speedText(rate: Float): String = if (rate % 1f == 0f) "${rate.toInt()}×" else "$rate×"
