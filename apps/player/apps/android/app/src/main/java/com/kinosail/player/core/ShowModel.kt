package com.kinosail.player.core

import android.app.Application
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

data class ShowState(val detail: ShowDetail? = null, val loading: Boolean = false, val notice: String? = null)

class ShowModel(application: Application) : AndroidViewModel(application) {
    private val sessions = SessionStore(application)
    private var generation = 0
    private var activeId = ""
    private var activeViewer = ""
    var state by mutableStateOf(ShowState())
        private set

    fun open(showId: String, viewerId: String) {
        activeId = showId
        activeViewer = viewerId
        val attempt = ++generation
        state = ShowState(loading = true)
        viewModelScope.launch { fetch(attempt, showId, viewerId) }
    }

    fun retry() {
        if (activeId.isEmpty()) return
        state = state.copy(loading = true, notice = null)
        val attempt = generation
        val id = activeId
        val viewer = activeViewer
        viewModelScope.launch { fetch(attempt, id, viewer) }
    }

    fun reset() { generation++; activeId = ""; activeViewer = ""; state = ShowState() }

    private suspend fun fetch(attempt: Int, showId: String, viewerId: String) {
        try {
            val detail = withContext(Dispatchers.IO) {
                val saved = sessions.load() ?: throw IllegalStateException("No saved connection")
                ShowApi(saved.server).detail(showId, saved.token, viewerId)
            }
            if (attempt == generation) state = ShowState(detail = detail)
        } catch (error: ServerHttpException) {
            if (attempt == generation) state = state.copy(loading = false,
                notice = if (error.status in setOf(401, 403)) "Reconnect to open this show."
                else "Could not load this show. Try again.")
        } catch (_: Exception) {
            if (attempt == generation) state = state.copy(loading = false,
                notice = "Could not load this show. Try again.")
        }
    }
}
