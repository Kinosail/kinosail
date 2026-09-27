package com.kinosail.player.core

import android.app.Application
import android.os.SystemClock
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

sealed interface ConnectionPhase {
    data object Restoring : ConnectionPhase
    data object Setup : ConnectionPhase
    data class Pairing(val code: String, val server: String) : ConnectionPhase
    data class Connected(val viewer: Viewer) : ConnectionPhase
}

class ConnectionModel(application: Application) : AndroidViewModel(application) {
    private val sessions = SessionStore(application)
    private var generation = 0
    private var pending: Pair<ServerAddress, ConnectChallenge>? = null
    private var current: SavedSession? = null

    var phase by mutableStateOf<ConnectionPhase>(ConnectionPhase.Restoring)
        private set
    var address by mutableStateOf("")
    var busy by mutableStateOf(false)
        private set
    var notice by mutableStateOf<String?>(null)
        private set

    init {
        viewModelScope.launch {
            val saved = try { withContext(Dispatchers.IO) { sessions.load() } }
                catch (_: Exception) {
                    AudioPlaybackService.stopIfRunning(getApplication())
                    withContext(Dispatchers.IO) { runCatching { sessions.clear() } }
                    notice = "Saved connection could not be restored. Connect again."
                    null
                }
            if (saved != null) {
                address = saved.server.url.toString()
                current = saved
                saved.viewer?.let { phase = ConnectionPhase.Connected(it) }
                revalidate(saved)
                return@launch
            }
            phase = ConnectionPhase.Setup
        }
    }

    private suspend fun revalidate(saved: SavedSession) {
        val attempt = generation
        var wait = 2_000L
        while (attempt == generation) {
            try {
                val viewer = withContext(Dispatchers.IO) { ServerApi(saved.server).viewer(saved.token) }
                if (attempt != generation) return
                require(saved.viewer == null || saved.viewer.serverId == viewer.serverId && saved.viewer.id == viewer.id) {
                    "The saved Viewer identity changed."
                }
                val refreshed = saved.copy(viewer = viewer)
                withContext(Dispatchers.IO) { sessions.save(refreshed) }
                if (attempt != generation) return
                current = refreshed
                phase = ConnectionPhase.Connected(viewer)
                notice = null
                return
            } catch (error: CancellationException) { throw error }
            catch (error: ServerHttpException) {
                if (error.status == 401 || error.status == 403) {
                    forgetSaved(attempt, "This connection expired. Connect again.")
                    return
                }
            } catch (error: IllegalArgumentException) {
                forgetSaved(attempt, "The saved connection changed. Connect again.")
                return
            } catch (_: Exception) { /* Keep the last verified Viewer while the Server restarts. */ }
            if (attempt != generation) return
            if (saved.viewer == null) notice = "Waiting for your Server to return."
            delay(wait)
            wait = (wait * 2).coerceAtMost(15_000)
        }
    }

    private suspend fun forgetSaved(attempt: Int, message: String) {
        if (attempt != generation) return
        AudioPlaybackService.stopIfRunning(getApplication())
        withContext(Dispatchers.IO) { sessions.clear() }
        if (attempt != generation) return
        current = null
        phase = ConnectionPhase.Setup
        notice = message
    }

    fun connect(device: String) {
        if (phase != ConnectionPhase.Setup || busy) return
        busy = true
        notice = null
        val requested = address
        val attempt = ++generation
        viewModelScope.launch {
            try {
                val (server, challenge) = withContext(Dispatchers.IO) {
                    val found = ServerProbe().check(requested)
                    found to ServerApi(found).start(device)
                }
                if (attempt != generation) {
                    withContext(Dispatchers.IO) { runCatching { ServerApi(server).cancel(challenge.secret) } }
                    return@launch
                }
                address = server.url.toString()
                pending = server to challenge
                phase = ConnectionPhase.Pairing(challenge.code, server.url.toString().trimEnd('/'))
                busy = false
                awaitApproval(server, challenge, attempt)
            } catch (error: IllegalArgumentException) {
                if (attempt == generation) notice = error.message ?: "Could not connect to this Server."
            } catch (_: Exception) {
                if (attempt == generation) notice = "Could not reach that Server. Check its address and try again."
            } finally {
                if (attempt == generation) busy = false
            }
        }
    }

    private suspend fun awaitApproval(server: ServerAddress, challenge: ConnectChallenge, attempt: Int) {
        val api = ServerApi(server)
        val deadline = SystemClock.elapsedRealtime() + 300_000
        try {
            while (attempt == generation && SystemClock.elapsedRealtime() < deadline) {
                delay(2_000)
                if (attempt != generation) break
                val token = withContext(Dispatchers.IO) { api.poll(challenge.secret) } ?: continue
                if (attempt == generation) {
                    try {
                        val viewer = withContext(Dispatchers.IO) { api.viewer(token) }
                        if (attempt == generation) {
                            val saved = SavedSession(server, token, viewer)
                            withContext(Dispatchers.IO) { sessions.save(saved) }
                            if (attempt == generation) {
                                current = saved
                                phase = ConnectionPhase.Connected(viewer)
                                return
                            }
                            withContext(Dispatchers.IO) {
                                if (runCatching { sessions.load() }.getOrNull()?.token == token) sessions.clear()
                            }
                        }
                    } catch (_: Exception) {
                        if (attempt == generation) notice = "Approval succeeded, but this session could not be verified. Try again."
                    }
                }
                withContext(Dispatchers.IO) { runCatching { api.signOut(token) } }
                break
            }
            if (attempt == generation && notice == null) notice = "This code expired. Request a new one."
        } catch (error: IllegalArgumentException) {
            if (attempt == generation) notice = error.message ?: "Pairing failed. Try again."
        } catch (_: Exception) {
            if (attempt == generation) notice = "Could not complete pairing. Try again."
        } finally {
            if (attempt == generation) {
                withContext(Dispatchers.IO) { runCatching { api.cancel(challenge.secret) } }
                pending = null
                if (phase is ConnectionPhase.Pairing) phase = ConnectionPhase.Setup
            }
        }
    }

    fun cancelPairing() {
        val challenge = pending ?: return
        generation++
        pending = null
        phase = ConnectionPhase.Setup
        notice = null
        viewModelScope.launch(Dispatchers.IO) { runCatching { ServerApi(challenge.first).cancel(challenge.second.secret) } }
    }

    fun signOut() {
        val session = current ?: return
        if (busy) return
        generation++
        busy = true
        notice = null
        viewModelScope.launch {
            try {
                AudioPlaybackService.stopIfRunning(getApplication())
                withContext(Dispatchers.IO) { sessions.clear() }
                current = null
                phase = ConnectionPhase.Setup
                try { withContext(Dispatchers.IO) { ServerApi(session.server).signOut(session.token) } }
                    catch (_: Exception) { notice = "Disconnected here. The Server could not revoke this session." }
            } catch (_: Exception) {
                notice = "Could not remove the saved connection. Try again."
            } finally {
                busy = false
            }
        }
    }
}
