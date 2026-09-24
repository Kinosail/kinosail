package com.kinosail.player.core

import androidx.activity.compose.BackHandler
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
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.draw.clipToBounds
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.awaitCancellation
import kotlinx.coroutines.withContext
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock

@Composable
internal fun PdfReaderScreen(item: CatalogItem, viewer: Viewer, close: () -> Unit) {
    val context = LocalContext.current
    var revision by remember(item.id) { mutableIntStateOf(0) }
    var document by remember(item.id, revision) { mutableStateOf<PdfDocument?>(null) }
    var page by remember(item.id, revision) { mutableIntStateOf(0) }
    var bitmap by remember(item.id, revision) { mutableStateOf<android.graphics.Bitmap?>(null) }
    var notice by remember(item.id, revision) { mutableStateOf<String?>(null) }
    var savingNotice by remember(item.id, revision) { mutableStateOf<String?>(null) }
    var zoom by remember(item.id, revision) { mutableFloatStateOf(1f) }
    var pan by remember(item.id, revision) { mutableStateOf(Offset.Zero) }
    var imageSize by remember(item.id, revision) { mutableStateOf(IntSize.Zero) }
    var session by remember(item.id, revision) { mutableStateOf<SavedSession?>(null) }
    var navigated by remember(item.id, revision) { mutableStateOf(false) }
    var saveRevision by remember(item.id, revision) { mutableIntStateOf(0) }
    val saveMutex = remember(item.id, revision) { Mutex() }
    BackHandler(onBack = close)

    LaunchedEffect(item.id, viewer, revision) {
        var opened: PdfDocument? = null
        try {
            val result = withContext(Dispatchers.IO) {
                val saved = SessionStore(context).load() ?: error("No saved connection")
                val api = ReaderApi(saved.server)
                val book = api.book(item.id, saved.token, viewer.id)
                val position = api.position(item.id, saved.token, viewer.id)
                val file = api.download(book, saved.token, viewer.id, context.cacheDir)
                val pdf = try { PdfDocument(file) } catch (failure: Exception) { file.delete(); throw failure }
                opened = pdf
                Triple(pdf, position, saved)
            }
            page = (result.second.offset * (result.first.pageCount - 1)).toInt()
                .coerceIn(0, result.first.pageCount - 1)
            session = result.third
            document = opened
            awaitCancellation()
        } catch (failure: CancellationException) {
            throw failure
        } catch (failure: Exception) {
            notice = if (failure.message == "This book format is not available on Android yet.") failure.message
                else "Could not open this PDF. Check your Server connection and try again."
        } finally {
            val finalPage = page
            val saved = session
            val finalDocument = opened
            document = null
            withContext(NonCancellable + Dispatchers.IO) {
                try {
                    if (navigated && saved != null && finalDocument != null) saveMutex.withLock {
                        ReaderApi(saved.server).save(item.id, saved.token, viewer.id,
                            if (finalDocument.pageCount == 1) 0.0 else finalPage.toDouble() / (finalDocument.pageCount - 1))
                    }
                } catch (_: Exception) {
                    // The visible reader already reports save failures; closing must still release the file.
                } finally {
                    finalDocument?.close()
                }
            }
        }
    }

    LaunchedEffect(document, page, saveRevision) {
        val pdf = document ?: return@LaunchedEffect
        bitmap = null
        zoom = 1f
        pan = Offset.Zero
        try {
            bitmap = withContext(Dispatchers.IO) { pdf.render(page) }
            val saved = session
            if (navigated && saved != null) {
                saveMutex.withLock {
                    withContext(Dispatchers.IO) {
                        ReaderApi(saved.server).save(item.id, saved.token, viewer.id,
                            if (pdf.pageCount == 1) 0.0 else page.toDouble() / (pdf.pageCount - 1))
                    }
                }
                savingNotice = null
            }
        } catch (failure: CancellationException) {
            throw failure
        } catch (_: Exception) {
            if (bitmap == null) notice = "Could not render this PDF page."
            else savingNotice = "Reading position was not saved."
        }
    }

    Box(Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background).safeDrawingPadding()) {
        bitmap?.let { image ->
            Box(Modifier.fillMaxSize().padding(top = 80.dp, bottom = 84.dp).clipToBounds()) {
                Image(image.asImageBitmap(), contentDescription = "${item.title}, page ${page + 1}",
                    contentScale = ContentScale.Fit,
                    modifier = Modifier.fillMaxSize().onSizeChanged { imageSize = it }
                        .pointerInput(image) { detectTransformGestures { _, movement, change, _ ->
                            zoom = (zoom * change).coerceIn(1f, 4f)
                            pan = readerPan(pan + movement, imageSize, image, zoom)
                        } }.graphicsLayer(scaleX = zoom, scaleY = zoom,
                            translationX = pan.x, translationY = pan.y))
            }
        }
        Column(Modifier.fillMaxSize().padding(16.dp), verticalArrangement = Arrangement.SpaceBetween) {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically) {
                Text(item.title, style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onBackground,
                    modifier = Modifier.weight(1f).padding(end = 12.dp))
                TextButton(onClick = { zoom = if (zoom > 1f) 1f else 2f; pan = Offset.Zero },
                    enabled = bitmap != null) { Text(if (zoom > 1f) "Fit" else "Zoom") }
                TextButton(onClick = close) { Text("Done") }
            }
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                if (document == null && notice == null) CircularProgressIndicator()
                notice?.let {
                    Text(it, color = MaterialTheme.colorScheme.error)
                    if (it != "This book format is not available on Android yet.") {
                        TextButton(onClick = { revision++ }) { Text("Try again") }
                    }
                }
                savingNotice?.let { Text(it, color = MaterialTheme.colorScheme.error) }
                if (savingNotice != null) TextButton(onClick = { saveRevision++ }) { Text("Retry sync") }
                document?.let { pdf ->
                    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically) {
                        Button(onClick = { page--; navigated = true; savingNotice = null }, enabled = page > 0) {
                            Text("Previous")
                        }
                        Text("${page + 1} of ${pdf.pageCount}", color = MaterialTheme.colorScheme.onBackground)
                        Button(onClick = { page++; navigated = true; savingNotice = null },
                            enabled = page + 1 < pdf.pageCount) {
                            Text("Next")
                        }
                    }
                }
            }
        }
    }
}

private fun readerPan(pan: Offset, size: IntSize, image: android.graphics.Bitmap, zoom: Float): Offset {
    if (zoom <= 1f || size.width == 0 || size.height == 0) return Offset.Zero
    val fit = minOf(size.width / image.width.toFloat(), size.height / image.height.toFloat())
    val x = ((image.width * fit * zoom - size.width) / 2f).coerceAtLeast(0f)
    val y = ((image.height * fit * zoom - size.height) / 2f).coerceAtLeast(0f)
    return Offset(pan.x.coerceIn(-x, x), pan.y.coerceIn(-y, y))
}
