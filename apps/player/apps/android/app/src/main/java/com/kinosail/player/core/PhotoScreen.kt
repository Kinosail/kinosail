package com.kinosail.player.core

import android.view.KeyEvent
import androidx.activity.compose.BackHandler
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.focusable
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.gestures.detectTransformGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.material3.Button
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.draw.clipToBounds
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.input.key.onKeyEvent
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.unit.dp

@Composable
internal fun PhotoScreen(item: CatalogItem, catalog: CatalogModel, tv: Boolean, close: () -> Unit) {
    var revision by remember(item.id) { mutableIntStateOf(0) }
    var image by remember(item.id, revision) { mutableStateOf<android.graphics.Bitmap?>(null) }
    var loaded by remember(item.id, revision) { mutableStateOf(false) }
    var zoom by remember(item.id) { mutableFloatStateOf(1f) }
    var offset by remember(item.id) { mutableStateOf(Offset.Zero) }
    var size by remember { mutableStateOf(IntSize.Zero) }
    val photoFocus = remember { FocusRequester() }
    val zoomFocus = remember { FocusRequester() }
    val fit = { zoom = 1f; offset = Offset.Zero }
    BackHandler { if (zoom > 1f) fit() else close() }
    LaunchedEffect(item.id, revision) {
        image = if (item.stream.isNotEmpty()) catalog.photo(item) else null
        loaded = true
    }
    LaunchedEffect(zoom, tv, image) {
        if (tv && image != null) {
            if (zoom > 1f) photoFocus.requestFocus() else zoomFocus.requestFocus()
        }
    }
    Box(Modifier.fillMaxSize().background(Color.Black).safeDrawingPadding().clipToBounds()) {
        image?.let { bitmap ->
            Image(bitmap.asImageBitmap(), contentDescription = item.title,
                contentScale = ContentScale.Fit,
                modifier = Modifier.fillMaxSize().onSizeChanged { size = it }
                    .then(if (tv) Modifier.focusRequester(photoFocus).focusable().onKeyEvent { event ->
                        if (event.nativeKeyEvent.action != KeyEvent.ACTION_DOWN) return@onKeyEvent false
                        when (event.nativeKeyEvent.keyCode) {
                            KeyEvent.KEYCODE_MEDIA_PLAY_PAUSE, KeyEvent.KEYCODE_DPAD_CENTER -> {
                                if (zoom > 1f) fit() else zoom = 2f
                                true
                            }
                            KeyEvent.KEYCODE_DPAD_LEFT, KeyEvent.KEYCODE_DPAD_RIGHT,
                            KeyEvent.KEYCODE_DPAD_UP, KeyEvent.KEYCODE_DPAD_DOWN -> {
                                if (zoom <= 1f) false else {
                                    val move = 80f
                                    val delta = when (event.nativeKeyEvent.keyCode) {
                                        KeyEvent.KEYCODE_DPAD_LEFT -> Offset(move, 0f)
                                        KeyEvent.KEYCODE_DPAD_RIGHT -> Offset(-move, 0f)
                                        KeyEvent.KEYCODE_DPAD_UP -> Offset(0f, move)
                                        else -> Offset(0f, -move)
                                    }
                                    offset = photoOffset(offset + delta, size, bitmap, zoom)
                                    true
                                }
                            }
                            else -> false
                        }
                    } else Modifier.pointerInput(bitmap) {
                        detectTransformGestures { _, pan, change, _ ->
                            zoom = (zoom * change).coerceIn(1f, 4f)
                            offset = photoOffset(offset + pan, size, bitmap, zoom)
                        }
                    }.pointerInput(bitmap) {
                        detectTapGestures(onDoubleTap = {
                            if (zoom > 1f) fit() else zoom = 2f
                        })
                    })
                    .graphicsLayer(scaleX = zoom, scaleY = zoom,
                        translationX = offset.x, translationY = offset.y))
        }
        Column(Modifier.fillMaxWidth().padding(if (tv) 40.dp else 16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Text(item.title, color = Color.White)
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                if (tv) {
                    androidx.tv.material3.Button(onClick = { if (zoom > 1f) fit() else zoom = 2f },
                        enabled = image != null, modifier = Modifier.focusRequester(zoomFocus)) {
                        androidx.tv.material3.Text(if (zoom > 1f) "Fit photo" else "Zoom in")
                    }
                    androidx.tv.material3.Button(onClick = close) { androidx.tv.material3.Text("Done") }
                } else {
                    Button(onClick = { if (zoom > 1f) fit() else zoom = 2f }, enabled = image != null) {
                        Text(if (zoom > 1f) "Fit photo" else "Zoom in")
                    }
                    Button(onClick = close) { Text("Done") }
                }
            }
            if (loaded && image == null) {
                Text(if (item.stream.isEmpty()) "This Viewer cannot open the photo."
                    else "Could not open the photo. Check your Server connection and try again.",
                    color = Color.White)
                if (item.stream.isNotEmpty()) {
                    if (tv) androidx.tv.material3.Button(onClick = { revision++ }) {
                        androidx.tv.material3.Text("Try again")
                    } else Button(onClick = { revision++ }) { Text("Try again") }
                }
            } else if (!loaded) Text("Opening photo…", color = Color.White)
        }
    }
}

private fun photoOffset(offset: Offset, size: IntSize, image: android.graphics.Bitmap, zoom: Float): Offset {
    if (zoom <= 1f) return Offset.Zero
    val fit = minOf(size.width / image.width.toFloat(), size.height / image.height.toFloat())
    val x = ((image.width * fit * zoom - size.width) / 2f).coerceAtLeast(0f)
    val y = ((image.height * fit * zoom - size.height) / 2f).coerceAtLeast(0f)
    return Offset(offset.x.coerceIn(-x, x), offset.y.coerceIn(-y, y))
}
