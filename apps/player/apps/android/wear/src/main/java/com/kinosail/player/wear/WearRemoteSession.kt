package com.kinosail.player.wear

import android.content.Context
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import com.google.android.gms.wearable.Wearable
import com.kinosail.player.watchcore.WatchPlayer
import com.kinosail.player.watchcore.WatchRequest
import com.kinosail.player.watchcore.WatchWire
import com.kinosail.player.watchcore.selectedId
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.tasks.await
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeout

internal class WearRemoteSession(private val context: Context) {
    var players by mutableStateOf(emptyList<WatchPlayer>())
        private set
    var selectedId by mutableStateOf<String?>(null)
        private set
    var message by mutableStateOf<String?>(null)
        private set
    var busy by mutableStateOf(false)
        private set

    val selected: WatchPlayer? get() = players.firstOrNull { it.id == selectedId }

    fun select(id: String) {
        if (players.any { it.id == id }) selectedId = id
    }

    suspend fun refresh() = exchange(WatchRequest())

    suspend fun command(command: String, position: Double? = null) {
        val player = selected ?: return
        if (!player.active || busy) return
        val request = WatchRequest(player.id, player.itemId, command, position).checked()
        busy = true
        try { exchange(request) } finally { busy = false }
    }

    private suspend fun exchange(request: WatchRequest) {
        try {
            val reply = withTimeout(8_000) {
                withContext(Dispatchers.IO) {
                    val nodes = Wearable.getNodeClient(context).connectedNodes.await()
                    val phone = nodes.firstOrNull { it.isNearby } ?: nodes.firstOrNull()
                        ?: throw IllegalStateException("Connect the watch to your Android phone.")
                    val bytes = Wearable.getMessageClient(context)
                        .sendRequest(phone.id, WatchWire.PATH, WatchWire.requestBytes(request)).await()
                    WatchWire.reply(bytes)
                }
            }
            players = reply.players
            selectedId = players.selectedId(selectedId)
            message = if (reply.accepted) reply.message else reply.message ?: "The command was not accepted."
            HeartStore.load(context)?.let { timeline ->
                players.firstOrNull { it.id == timeline.targetId }?.let {
                    HeartStore.note(context, it, System.currentTimeMillis())
                }
            }
        } catch (_: Exception) {
            message = "Connect your watch and Android phone to use the remote."
        }
    }
}
