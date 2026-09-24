package com.kinosail.player.core

import android.view.KeyEvent
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
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.key.onKeyEvent
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.media3.ui.PlayerView
import androidx.compose.ui.viewinterop.AndroidView
import com.kinosail.player.design.KinoColor

@Composable
internal fun PlaybackScreen(item: CatalogItem, viewer: Viewer, tv: Boolean, close: () -> Unit) {
    val playback: PlaybackModel = viewModel()
    val player = playback.player
    val context = LocalContext.current
    val playerView = remember(player, context) { player?.let { engine -> PlayerView(context).apply {
        this.player = engine
        useController = true
        controllerShowTimeoutMs = if (tv) 5_000 else 3_000
    } } }
    BackHandler { close() }
    LaunchedEffect(item.id, viewer.id) { playback.start(item, viewer) }
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
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically) {
                Text(item.title, color = Color.White)
                if (tv) androidx.tv.material3.Button(onClick = close) {
                    androidx.tv.material3.Text("Back to Library")
                } else TextButton(onClick = close) { Text("Done", color = KinoColor.signal) }
            }
            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                if (playback.loading) CircularProgressIndicator(color = KinoColor.signal)
                if (playback.usingCompatible) Text("Compatible playback", color = Color.White)
                playback.message?.let { Text(it, color = Color.White) }
            }
        }
    }
}
