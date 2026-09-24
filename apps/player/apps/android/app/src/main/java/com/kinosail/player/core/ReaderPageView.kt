package com.kinosail.player.core

import android.graphics.Bitmap
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
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
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clipToBounds
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.unit.dp

@Composable
internal fun ReaderPageView(title: String, bitmap: Bitmap?, page: Int, total: Int,
                            notice: String?, savingNotice: String?, close: () -> Unit,
                            previous: () -> Unit, next: () -> Unit, retry: () -> Unit,
                            retrySync: () -> Unit) {
    var zoom by remember(bitmap) { mutableFloatStateOf(1f) }
    var pan by remember(bitmap) { mutableStateOf(Offset.Zero) }
    var size by remember(bitmap) { mutableStateOf(IntSize.Zero) }
    Box(Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background).safeDrawingPadding()) {
        bitmap?.let { image ->
            Box(Modifier.fillMaxSize().padding(top = 80.dp, bottom = 84.dp).clipToBounds()) {
                Image(image.asImageBitmap(), contentDescription = "$title, page ${page + 1}",
                    contentScale = ContentScale.Fit,
                    modifier = Modifier.fillMaxSize().onSizeChanged { size = it }
                        .pointerInput(image) { detectTransformGestures { _, movement, change, _ ->
                            zoom = (zoom * change).coerceIn(1f, 4f)
                            pan = readerPan(pan + movement, size, image, zoom)
                        } }.graphicsLayer(scaleX = zoom, scaleY = zoom,
                            translationX = pan.x, translationY = pan.y))
            }
        }
        Column(Modifier.fillMaxSize().padding(16.dp), verticalArrangement = Arrangement.SpaceBetween) {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically) {
                Text(title, style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onBackground,
                    modifier = Modifier.weight(1f).padding(end = 12.dp))
                TextButton(onClick = { zoom = if (zoom > 1f) 1f else 2f; pan = Offset.Zero },
                    enabled = bitmap != null) { Text(if (zoom > 1f) "Fit" else "Zoom") }
                TextButton(onClick = close) { Text("Done") }
            }
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                if (total == 0 && notice == null) CircularProgressIndicator()
                notice?.let {
                    Text(it, color = MaterialTheme.colorScheme.error)
                    if (it != "This book format is not available on Android yet.") {
                        TextButton(onClick = retry) { Text("Try again") }
                    }
                }
                savingNotice?.let { Text(it, color = MaterialTheme.colorScheme.error) }
                if (savingNotice != null) TextButton(onClick = retrySync) { Text("Retry sync") }
                if (total > 0) Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically) {
                    Button(onClick = previous, enabled = page > 0) { Text("Previous") }
                    Text("${page + 1} of $total", color = MaterialTheme.colorScheme.onBackground)
                    Button(onClick = next, enabled = page + 1 < total) { Text("Next") }
                }
            }
        }
    }
}

private fun readerPan(pan: Offset, size: IntSize, image: Bitmap, zoom: Float): Offset {
    if (zoom <= 1f || size.width == 0 || size.height == 0) return Offset.Zero
    val fit = minOf(size.width / image.width.toFloat(), size.height / image.height.toFloat())
    val x = ((image.width * fit * zoom - size.width) / 2f).coerceAtLeast(0f)
    val y = ((image.height * fit * zoom - size.height) / 2f).coerceAtLeast(0f)
    return Offset(pan.x.coerceIn(-x, x), pan.y.coerceIn(-y, y))
}
