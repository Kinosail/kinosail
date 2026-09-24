package com.kinosail.player.core

import android.app.Application
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

data class HomeState(val continueWatching: List<CatalogItem> = emptyList(),
                     val recent: List<CatalogItem> = emptyList(), val loading: Boolean = false,
                     val notice: String? = null)

class HomeModel(application: Application) : AndroidViewModel(application) {
    private val sessions = SessionStore(application)
    private var generation = 0
    private var viewerId = ""
    var state by mutableStateOf(HomeState())
        private set

    fun open(viewer: Viewer) {
        viewerId = viewer.id
        state = HomeState(loading = true)
        val attempt = ++generation
        viewModelScope.launch { fetch(attempt, viewer.id) }
    }

    fun retry() {
        if (viewerId.isEmpty()) return
        state = state.copy(loading = true, notice = null)
        val attempt = ++generation
        viewModelScope.launch { fetch(attempt, viewerId) }
    }

    fun reset() { generation++; viewerId = ""; state = HomeState() }

    private suspend fun fetch(attempt: Int, viewer: String) {
        try {
            val saved = withContext(Dispatchers.IO) { sessions.load() }
                ?: throw IllegalStateException("No saved connection")
            val (history, recent) = coroutineScope {
                val api = CatalogApi(saved.server)
                val history = async(Dispatchers.IO) { api.list(saved.token, viewer, view = "history") }
                val recent = async(Dispatchers.IO) { api.list(saved.token, viewer, sort = "added") }
                history.await() to recent.await()
            }
            if (attempt == generation) state = HomeState(history.items, recent.items)
        } catch (error: ServerHttpException) {
            if (attempt == generation) state = state.copy(loading = false,
                notice = if (error.status in setOf(401, 403)) "Reconnect to load your home."
                    else "Could not load your home. Try again.")
        } catch (_: Exception) {
            if (attempt == generation) state = state.copy(loading = false,
                notice = "Could not load your home. Try again.")
        }
    }
}
