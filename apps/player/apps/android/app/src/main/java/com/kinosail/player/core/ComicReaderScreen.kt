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
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.awaitCancellation
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext

@Composable
internal fun ComicReaderScreen(item: CatalogItem, viewer: Viewer, close: () -> Unit) {
    val context = LocalContext.current
    var revision by remember(item.id) { mutableIntStateOf(0) }
    var imageRevision by remember(item.id, revision) { mutableIntStateOf(0) }
    var saveRevision by remember(item.id, revision) { mutableIntStateOf(0) }
    var comic by remember(item.id, revision) { mutableStateOf<ComicBook?>(null) }
    var session by remember(item.id, revision) { mutableStateOf<SavedSession?>(null) }
    var page by remember(item.id, revision) { mutableIntStateOf(0) }
    var bitmap by remember(item.id, revision) { mutableStateOf<android.graphics.Bitmap?>(null) }
    var notice by remember(item.id, revision) { mutableStateOf<String?>(null) }
    var savingNotice by remember(item.id, revision) { mutableStateOf<String?>(null) }
    var navigated by remember(item.id, revision) { mutableStateOf(false) }
    val saveMutex = remember(item.id, revision) { Mutex() }
    BackHandler(onBack = close)

    LaunchedEffect(item.id, viewer, revision) {
        try {
            val result = withContext(Dispatchers.IO) {
                val saved = SessionStore(context).load() ?: error("No saved connection")
                val api = ReaderApi(saved.server)
                val book = api.comic(item.id, saved.token, viewer.id)
                Triple(book, api.position(item.id, saved.token, viewer.id, book.pages.size), saved)
            }
            page = result.second.page - 1
            session = result.third
            comic = result.first
            awaitCancellation()
        } catch (failure: CancellationException) {
            throw failure
        } catch (_: Exception) {
            notice = "Could not open this comic. Check your Server connection and try again."
        } finally {
            val saved = session
            val book = comic
            val finalPage = page + 1
            withContext(NonCancellable + Dispatchers.IO) {
                if (navigated && saved != null && book != null) runCatching {
                    saveMutex.withLock {
                        ReaderApi(saved.server).save(item.id, saved.token, viewer.id, 0.0,
                            finalPage, book.pages.size)
                    }
                }
            }
        }
    }

    LaunchedEffect(comic, page, imageRevision, saveRevision) {
        val book = comic ?: return@LaunchedEffect
        val saved = session ?: return@LaunchedEffect
        bitmap = null
        notice = null
        try {
            bitmap = withContext(Dispatchers.IO) {
                ArtworkClient(saved.server).comic(book.pages[page], book.id, saved.token, viewer.id)
            }
            if (navigated) {
                saveMutex.withLock {
                    withContext(Dispatchers.IO) {
                        ReaderApi(saved.server).save(item.id, saved.token, viewer.id, 0.0,
                            page + 1, book.pages.size)
                    }
                }
                savingNotice = null
            }
        } catch (failure: CancellationException) {
            throw failure
        } catch (_: Exception) {
            if (bitmap == null) notice = "Could not load this comic page."
            else savingNotice = "Reading position was not saved."
        }
    }

    ReaderPageView(item.title, bitmap, page, comic?.pages?.size ?: 0, notice, savingNotice,
        close = close,
        previous = { page--; navigated = true; savingNotice = null },
        next = { page++; navigated = true; savingNotice = null },
        retry = { if (comic == null) revision++ else imageRevision++ },
        retrySync = { saveRevision++ })
}
