package com.kinosail.player.core

import androidx.activity.compose.BackHandler
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.platform.LocalContext
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

@Composable
internal fun BookReaderScreen(item: CatalogItem, viewer: Viewer, close: () -> Unit) {
    val context = LocalContext.current
    var revision by remember(item.id) { mutableIntStateOf(0) }
    var format by remember(item.id, revision) { mutableStateOf<String?>(null) }
    var notice by remember(item.id, revision) { mutableStateOf<String?>(null) }
    BackHandler(onBack = close)
    LaunchedEffect(item.id, viewer, revision) {
        try {
            format = withContext(Dispatchers.IO) {
                val saved = SessionStore(context).load() ?: error("No saved connection")
                ReaderApi(saved.server).format(item.id, saved.token, viewer.id)
            }
        } catch (failure: CancellationException) {
            throw failure
        } catch (_: Exception) {
            notice = "Could not open this book. Check your Server connection and try again."
        }
    }
    when (format) {
        "pdf" -> PdfReaderScreen(item, viewer, close)
        "comic" -> ComicReaderScreen(item, viewer, close)
        else -> ReaderPageView(item.title, null, 0, 0,
            if (format == "epub") "This book format is not available on Android yet." else notice,
            null, close, previous = {}, next = {}, retry = { revision++ }, retrySync = {})
    }
}
