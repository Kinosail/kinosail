package com.kinosail.player.core

import android.app.Application
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.*
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock

internal data class EpubState(val book: EpubBook? = null, val session: SavedSession? = null,
    val position: ReaderPosition = ReaderPosition(1, 1, 0.0), val loading: Boolean = true,
    val chapterLoading: Boolean = true, val notice: String? = null, val syncNotice: String? = null,
    val revision: Int = 0, val conflict: ReaderPosition? = null)

internal class EpubModel(application: Application) : AndroidViewModel(application) {
    var state by mutableStateOf(EpubState())
        private set
    private var item: CatalogItem? = null
    private var viewer: Viewer? = null
    private var generation = 0
    private var load: Job? = null
    private var save: Job? = null
    private val writer = Mutex()
    private var changed = false
    private var baseline = ReaderPosition(1, 1, 0.0)
    private var journal: ReadingJournal? = null
    private var decision = 0

    fun open(item: CatalogItem, viewer: Viewer) {
        flush()
        this.item = item; this.viewer = viewer
        val attempt = ++generation
        load?.cancel()
        changed = false
        state = EpubState()
        load = viewModelScope.launch {
            try {
                val result = writer.withLock { withContext(Dispatchers.IO) {
                    val saved = SessionStore(getApplication()).load() ?: error("No saved connection")
                    val api = ReaderApi(saved.server)
                    val book = api.epub(item.id, saved, viewer)
                    val position = api.position(item.id, saved.token, viewer.id, book.chapters.size)
                    val store = ReadingJournal(getApplication(), saved, viewer, item.id)
                    val pending = store.load(book.chapters.size)
                    if (pending == null) store.store(PendingReading(position, position))
                    Triple(EpubState(book, saved, pending?.position ?: position, loading = false,
                        conflict = position.takeIf { pending != null && it != pending.position && it != pending.baseline }), store, position)
                } }
                if (attempt == generation) { state = result.first; journal = result.second; baseline = result.third }
            } catch (cancelled: CancellationException) { throw cancelled }
            catch (_: Exception) {
                if (attempt == generation) state = EpubState(loading = false, chapterLoading = false,
                    notice = "Could not open this book. Check your Server connection and try again.")
            }
        }
    }

    fun turn(page: Int) {
        val book = state.book ?: return
        if (page !in 1..book.chapters.size || page == state.position.page) return
        flush()
        state = state.copy(position = ReaderPosition(page, book.chapters.size, 0.0),
            chapterLoading = true, notice = null, syncNotice = null)
        changed = true
    }

    fun chapterResult(page: Int, success: Boolean) {
        if (page != state.position.page) return
        if (!success) flush()
        state = state.copy(chapterLoading = false,
            notice = if (success) null else "Could not load this chapter. Try again.")
        if (success) { if (changed) stage(); persist(false) }
    }

    fun offset(page: Int, offset: Double) {
        if (state.chapterLoading || state.notice != null || page != state.position.page ||
            !offset.isFinite() || offset !in 0.0..1.0 || kotlin.math.abs(state.position.offset - offset) < 0.001) return
        state = state.copy(position = state.position.copy(offset = offset))
        changed = true
        persist(true)
    }

    fun retry() {
        if (state.book == null) { item?.let { value -> viewer?.let { open(value, it) } }; return }
        flush()
        state = state.copy(chapterLoading = true, notice = null, revision = state.revision + 1)
    }

    fun flush() = persist(false)
    fun retrySync() = persist(false)

    private fun stage() {
        val store = journal ?: return
        try {
            store.stage(state.position, baseline)
            changed = false
        } catch (_: Exception) {
            android.util.Log.e("KinosailReader", "Position failed operation=reader-progress kind=storage outcome=unsaved")
            state = state.copy(syncNotice = "Reading position could not be saved on this device.")
        }
    }

    fun resolve(useDevice: Boolean) {
        val remote = state.conflict ?: return
        val store = journal ?: return
        if (useDevice) { stageDisplayed(); if (changed) return }
        try {
            decision++
            if (useDevice) {
                val pending = store.load(remote.total, includeAcknowledged = true) ?: return
                store.store(pending.copy(baseline = remote))
                state = state.copy(conflict = null)
                persist(false)
            } else {
                store.store(PendingReading(remote, remote)); baseline = remote; changed = false
                state = state.copy(position = remote, conflict = null, syncNotice = null,
                    chapterLoading = true, notice = null, revision = state.revision + 1)
            }
        } catch (_: Exception) { state = state.copy(syncNotice = "Reading position could not be saved on this device.") }
    }

    private fun persist(debounce: Boolean) {
        val saved = state.session ?: return
        val identity = viewer ?: return
        val book = state.book ?: return
        val store = journal ?: return
        if (!debounce) { stageDisplayed(); if (changed && !state.chapterLoading) return }
        val attempt = generation
        val choice = decision
        save?.cancel()
        save = viewModelScope.launch scheduling@ {
            if (debounce) delay(600)
            if (attempt != generation || choice != decision) return@scheduling
            if (debounce) { stageDisplayed(); if (changed && !state.chapterLoading) return@scheduling }
            viewModelScope.launch {
            try {
                writer.withLock {
                    if (attempt != generation || choice != decision || state.conflict != null) return@withLock
                    val outcome = withContext(NonCancellable + Dispatchers.IO) {
                        val pending = store.load(book.chapters.size) ?: return@withContext null
                        val current = SessionStore(getApplication()).load()
                        require(current != null && current.server.url.toString() == saved.server.url.toString() &&
                            current.token == saved.token && current.viewer?.id == identity.id &&
                            current.viewer.serverId == identity.serverId)
                        val api = ReaderApi(saved.server)
                        val remote = api.position(book.id, saved.token, identity.id, book.chapters.size)
                        if (store.load(book.chapters.size) != pending) return@withContext null
                        if (remote != pending.position && remote != pending.baseline) remote to true
                        else {
                            if (remote != pending.position) api.save(book.id, saved.token, identity.id,
                                pending.position.offset, pending.position.page, pending.position.total)
                            store.acknowledge(pending)
                            pending.position to false
                        }
                    }
                    if (attempt == generation && choice == decision && outcome != null) {
                        if (outcome.second) state = state.copy(conflict = outcome.first, syncNotice = null)
                        else { baseline = outcome.first; state = state.copy(syncNotice = null) }
                    }
                }
            } catch (cancelled: CancellationException) { throw cancelled }
            catch (_: Exception) {
                if (attempt == generation && choice == decision) state = state.copy(syncNotice = "Reading position was not saved.")
            }
            }
        }
    }

    private fun stageDisplayed() {
        if (changed && !state.chapterLoading && state.notice == null) stage()
    }
}
