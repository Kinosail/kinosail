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

    ReaderPageView(item.title, bitmap, page, document?.pageCount ?: 0, notice, savingNotice,
        close = close,
        previous = { page--; navigated = true; savingNotice = null },
        next = { page++; navigated = true; savingNotice = null },
        retry = { revision++ }, retrySync = { saveRevision++ })
}
