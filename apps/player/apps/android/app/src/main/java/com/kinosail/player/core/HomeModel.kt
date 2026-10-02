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
                     val notice: String? = null, val listening: Boolean = false) {
    private fun includes(item: CatalogItem) = if (listening) item.kind in setOf("music", "audiobook")
        else item.kind in setOf("video", "show")
    private val continuation get() = continueWatching.filter {
        includes(it) && !it.progress.watched && !it.progress.dismissed && it.progress.seconds > 0
    }
    val featured: CatalogItem? get() = continuation.firstOrNull() ?: recent.firstOrNull(::includes)
    val watchShelf get() = continuation.filterNot { it.id == featured?.id }.take(15)
    val tvWatchingRail get() = continuation.take(15)
    val recentShelf get() = recent.filter(::includes)
    val movies get() = recent.filter { it.kind == "video" && it.showId.isEmpty() }
    val shows get() = recent.filter { it.kind == "show" || it.kind == "video" && it.showId.isNotEmpty() }
    val unwatchedMovies get() = movies.filterNot { it.progress.watched }
    val unwatchedShows get() = shows.filterNot { it.progress.watched }
    val movieGenres get() = movies.flatMap { movie ->
        movie.genres.split(" · ").map(String::trim).filter(String::isNotEmpty).distinct().map { it to movie }
    }.groupBy({ it.first }, { it.second }).toList()
        .sortedWith(compareByDescending<Pair<String, List<CatalogItem>>> { it.second.size }.thenBy { it.first }).take(4)
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
                val history = async(Dispatchers.IO) { api.list(saved.token, viewer.id, view = "history", limit = 200) }
                val categories = listOf("movies", "shows", "music", "audiobooks")
                val added = categories.map { view -> async(Dispatchers.IO) {
                    api.list(saved.token, viewer.id, view = view, sort = "added", limit = 36)
                } }
                history.await() to added.map { it.await() }
            }
            if (attempt == generation) {
                state = HomeState(history.items, recent.flatMap { it.items }.distinctBy(CatalogItem::id))
                withContext(Dispatchers.IO) {
                    runCatching { sessions.saveCatalog(saved.server, viewer, "home-history",
                        history) }
                    runCatching { sessions.saveCatalog(saved.server, viewer, "home-recent",
                        CatalogPage(state.recent, state.recent.size, 0, 200)) }
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
