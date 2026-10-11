package com.kinosail.player.core

import android.app.Application
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

internal data class CollectionsState(val name: String? = null, val names: List<String> = emptyList(),
    val items: List<CatalogItem> = emptyList(), val loading: Boolean = true, val notice: String? = null)

internal class CollectionsModel(application: Application) : AndroidViewModel(application) {
    private var viewer: Viewer? = null
    private var task: Job? = null
    private var generation = 0
    var state by mutableStateOf(CollectionsState())
        private set

    fun open(viewer: Viewer) {
        if (this.viewer?.let { it.serverId == viewer.serverId && it.id == viewer.id } == true) return
        this.viewer = viewer.validated()
        state = CollectionsState()
        refresh()
    }

    fun select(name: String) {
        CollectionsApi.checkedName(name)
        state = state.copy(name = name, items = emptyList(), loading = true, notice = null)
        refresh()
    }

    fun back() {
        generation++; task?.cancel()
        state = state.copy(name = null, items = emptyList(), loading = false, notice = null)
    }

    fun refresh() {
        val identity = viewer ?: return
        val name = state.name
        val attempt = ++generation
        task?.cancel()
        state = state.copy(loading = true, notice = null)
        task = viewModelScope.launch {
            try {
                val result = withContext(Dispatchers.IO) {
                    val saved = SessionStore(getApplication()).load() ?: error("No saved connection")
                    require(saved.viewer?.let { it.serverId == identity.serverId && it.id == identity.id } == true)
                    val api = CollectionsApi(saved.server)
                    if (name == null) CollectionsState(names = api.names(saved.token, identity.id), loading = false)
                    else state.copy(items = api.items(name, saved.token, identity.id), loading = false, notice = null)
                }
                if (attempt == generation) state = result
            } catch (cancelled: CancellationException) { throw cancelled }
            catch (failure: Exception) {
                if (attempt == generation) state = state.copy(loading = false,
                    notice = if (failure is ServerHttpException && failure.status in setOf(401, 403))
                        "Reconnect to open collections." else "Could not load collections. Try again.")
            }
        }
    }
}
