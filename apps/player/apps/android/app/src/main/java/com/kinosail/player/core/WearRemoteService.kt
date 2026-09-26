package com.kinosail.player.core

import com.google.android.gms.tasks.Task
import com.google.android.gms.tasks.TaskCompletionSource
import com.google.android.gms.tasks.Tasks
import com.google.android.gms.wearable.WearableListenerService
import com.kinosail.player.watchcore.WatchPlayer
import com.kinosail.player.watchcore.WatchReply
import com.kinosail.player.watchcore.WatchRequest
import com.kinosail.player.watchcore.WatchWire
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

class WearRemoteService : WearableListenerService() {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main)

    override fun onRequest(nodeId: String, path: String, request: ByteArray): Task<ByteArray> {
        if (path != WatchWire.PATH) return Tasks.forResult(error("Unknown watch request."))
        val source = TaskCompletionSource<ByteArray>()
        scope.launch {
            source.setResult(try { reply(request) }
                catch (_: Exception) { error("The remote is unavailable. Try again.") })
        }
        return source.task
    }

    override fun onDestroy() {
        scope.cancel()
        super.onDestroy()
    }

    private suspend fun reply(bytes: ByteArray): ByteArray {
        val request = try { WatchWire.request(bytes) }
            catch (_: Exception) { return error("The watch command was invalid.") }
        var accepted = true
        var message: String? = null
        val local = PlaybackModel.currentPhone()
        if (request.command != null && request.target == "phone") {
            accepted = local?.applyRemote(request) == true
            if (!accepted) message = "This phone is not playing that title."
        }
        val phone = local?.remoteState("phone", "This phone")
            ?: WatchPlayer("phone", "This phone", "", "", "", "idle", 0.0, 0.0, false)
        var televisions = emptyList<WatchPlayer>()
        try {
            val saved = withContext(Dispatchers.IO) { SessionStore(this@WearRemoteService).load() }
            if (saved != null) {
                televisions = withContext(Dispatchers.IO) {
                    val viewer = ServerApi(saved.server).viewer(saved.token)
                    val api = RemotePlayersApi(saved.server)
                    val available = api.list(saved.token, viewer.id)
                    if (request.command != null && request.target != "phone") {
                        val selected = available.firstOrNull { it.id == request.target && it.active && it.itemId == request.itemId }
                        if (selected == null) throw IllegalArgumentException("Selected player unavailable.")
                        api.command(selected, request, saved.token, viewer.id)
                    }
                    available
                }
            } else if (request.command != null && request.target != "phone") {
                accepted = false
                message = "Connect this phone to your Server to control Android TV."
            }
        } catch (error: CancellationException) { throw error }
        catch (_: Exception) {
            televisions = emptyList()
            if (request.command != null && request.target != "phone") accepted = false
            message = "Android TV is unavailable. Check its Server connection."
        }
        return WatchWire.replyBytes(WatchReply(listOf(phone) + televisions.take(63), accepted, message))
    }

    private fun error(message: String) = WatchWire.replyBytes(WatchReply(emptyList(), false, message))
}
