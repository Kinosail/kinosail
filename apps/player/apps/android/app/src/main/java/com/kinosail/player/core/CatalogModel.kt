package com.kinosail.player.core

import android.app.Application
import android.graphics.Bitmap
import android.util.LruCache
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

data class CatalogState(
    val items: List<CatalogItem> = emptyList(),
    val total: Int = 0,
    val loading: Boolean = false,
    val notice: String? = null,
    val selected: CatalogItem? = null,
)

class CatalogModel(application: Application) : AndroidViewModel(application) {
    private val sessions = SessionStore(application)
    private val artworkCache = object : LruCache<String, Bitmap>(24 * 1024 * 1024) {
        override fun sizeOf(key: String, value: Bitmap): Int = value.byteCount
    }
    private var session: SavedSession? = null
    private var viewerId: String? = null
    private var generation = 0
    private var activeQuery = ""

    var searchInput by mutableStateOf("")
    var state by mutableStateOf(CatalogState())
        private set

    fun open(viewer: Viewer) {
        val attempt = ++generation
        artworkCache.evictAll()
        state = CatalogState(loading = true)
        searchInput = ""
        activeQuery = ""
        viewModelScope.launch {
            try {
                val saved = withContext(Dispatchers.IO) { sessions.load() }
                    ?: throw IllegalStateException("No saved connection")
                if (attempt != generation) return@launch
                session = saved
                viewerId = viewer.id
                fetch(attempt, 0)
            } catch (_: Exception) {
                if (attempt == generation) state = state.copy(loading = false,
                    notice = "Could not open your library. Try again.")
            }
        }
    }

    fun search() {
        if (session == null || viewerId == null) return
        val query = searchInput.trim()
        if (query.toByteArray(Charsets.UTF_8).size > 512 || query.any(Char::isISOControl)) {
            state = state.copy(notice = "Enter a shorter search without control characters.")
            return
        }
        activeQuery = query
        val attempt = ++generation
        state = CatalogState(loading = true)
        viewModelScope.launch { fetch(attempt, 0) }
    }

    fun loadMore() {
        if (state.loading || state.items.size >= state.total || session == null) return
        val attempt = generation
        state = state.copy(loading = true, notice = null)
        viewModelScope.launch { fetch(attempt, state.items.size) }
    }

    fun retry() {
        if (session == null) return
        val attempt = generation
        state = state.copy(loading = true, notice = null)
        viewModelScope.launch { fetch(attempt, state.items.size) }
    }

    fun select(item: CatalogItem) {
        if (state.items.any { it.id == item.id }) state = state.copy(selected = item)
    }

    fun closeDetail() { state = state.copy(selected = null) }

    fun reset() {
        generation++
        session = null
        viewerId = null
        artworkCache.evictAll()
        state = CatalogState()
    }

    suspend fun artwork(path: String, dimension: Int): Bitmap? {
        if (path.isEmpty()) return null
        val saved = session ?: return null
        val viewer = viewerId ?: return null
        val attempt = generation
        val key = "$viewer:$dimension:$path"
        artworkCache.get(key)?.let { return it }
        return try {
            val image = withContext(Dispatchers.IO) {
                ArtworkClient(saved.server).bitmap(path, saved.token, viewer, dimension)
            }
            if (attempt != generation) null else image.also { artworkCache.put(key, it) }
        } catch (_: Exception) { null }
    }

    private suspend fun fetch(attempt: Int, offset: Int) {
        val saved = session ?: return
        val viewer = viewerId ?: return
        try {
            val page = withContext(Dispatchers.IO) {
                CatalogApi(saved.server).list(saved.token, viewer, activeQuery, offset)
            }
            if (attempt != generation) return
            require(offset == 0 || page.items.none { candidate -> state.items.any { it.id == candidate.id } }) {
                "The Server returned a duplicate library item."
            }
            state = state.copy(items = if (offset == 0) page.items else state.items + page.items,
                total = page.total, loading = false, notice = null)
        } catch (error: ServerHttpException) {
            if (attempt == generation) state = state.copy(loading = false,
                notice = if (error.status == 401 || error.status == 403)
                    "This connection expired. Disconnect and connect again."
                else "Could not load your library. Try again.")
        } catch (_: Exception) {
            if (attempt == generation) state = state.copy(loading = false,
                notice = "Could not load your library. Try again.")
        }
    }
}
