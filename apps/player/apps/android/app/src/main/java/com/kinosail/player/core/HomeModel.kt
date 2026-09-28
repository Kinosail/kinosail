package com.kinosail.player.core

import android.app.Application
import java.io.IOException
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

data class HomeState(val continueWatching: List<CatalogItem> = emptyList(),
                     val recent: List<CatalogItem> = emptyList(), val loading: Boolean = false,
                     val notice: String? = null) {
    val featured: CatalogItem? get() = continueWatching.firstOrNull { it.kind in PLAYABLE_KINDS }
        ?: recent.firstOrNull { it.kind in PLAYABLE_KINDS }
    val watchShelf: List<CatalogItem> get() = continueWatching.filterNot { it.id == featured?.id }.take(12)
    val recentShelf: List<CatalogItem> get() = recent.filterNot { it.id == featured?.id }.take(24)

    private companion object { val PLAYABLE_KINDS = setOf("video", "music", "audiobook") }
}

class HomeModel(application: Application) : AndroidViewModel(application) {
    private val sessions = SessionStore(application)
    private var generation = 0
    private var viewer: Viewer? = null
    var state by mutableStateOf(HomeState())
        private set

    fun open(viewer: Viewer) {
        if (this.viewer?.id != viewer.id || this.viewer?.serverId != viewer.serverId) state = HomeState(loading = true)
        this.viewer = viewer
        val attempt = ++generation
        viewModelScope.launch { fetch(attempt, viewer) }
    }

    fun retry() {
        val selected = viewer ?: return
        state = state.copy(loading = state.continueWatching.isEmpty() && state.recent.isEmpty(), notice = null)
        val attempt = ++generation
        viewModelScope.launch { fetch(attempt, selected) }
    }

    fun reset() { generation++; viewer = null; state = HomeState() }

    private suspend fun fetch(attempt: Int, viewer: Viewer) {
        try {
            val saved = withContext(Dispatchers.IO) { sessions.load() }
                ?: throw IllegalStateException("No saved connection")
            if (state.continueWatching.isEmpty() && state.recent.isEmpty()) {
                val cached = withContext(Dispatchers.IO) {
                    sessions.loadCatalog(saved.server, viewer, "home-history") to
                        sessions.loadCatalog(saved.server, viewer, "home-recent")
                }
                if (attempt == generation) cached.first?.let { history ->
                    cached.second?.let { recent -> state = HomeState(history.items, recent.items) }
                }
            }
            val (history, recent) = coroutineScope {
                val api = CatalogApi(saved.server)
                val history = async(Dispatchers.IO) { api.list(saved.token, viewer.id, view = "history") }
                val recent = async(Dispatchers.IO) { api.list(saved.token, viewer.id, sort = "added") }
                history.await() to recent.await()
            }
            if (attempt == generation) {
                state = HomeState(history.items, recent.items)
                withContext(Dispatchers.IO) {
                    runCatching { sessions.saveCatalog(saved.server, viewer, "home-history", history) }
                    runCatching { sessions.saveCatalog(saved.server, viewer, "home-recent", recent) }
                }
            }
        } catch (error: CancellationException) { throw error } catch (error: ServerHttpException) {
            if (attempt == generation) state = state.copy(loading = false,
                notice = if (error.status in setOf(401, 403)) "Reconnect to load your home."
                    else if (state.continueWatching.isEmpty() && state.recent.isEmpty()) "Could not load your home. Try again." else null)
            if (error.status in setOf(408, 429, 500, 502, 503, 504)) retryLater(attempt, viewer)
        } catch (_: IOException) {
            if (attempt == generation) state = state.copy(loading = false,
                notice = if (state.continueWatching.isEmpty() && state.recent.isEmpty()) "Could not load your home. Try again." else null)
            retryLater(attempt, viewer)
        } catch (_: Exception) {
            if (attempt == generation) state = state.copy(loading = false,
                notice = if (state.continueWatching.isEmpty() && state.recent.isEmpty()) "Could not load your home. Try again." else null)
        }
    }

    private fun retryLater(attempt: Int, viewer: Viewer) {
        viewModelScope.launch {
            delay(15_000)
            if (attempt == generation) fetch(attempt, viewer)
        }
    }
}
