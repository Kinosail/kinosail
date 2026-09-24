package com.kinosail.player.core

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

class ConnectionModel : ViewModel() {
    var address by mutableStateOf("")
    var checking by mutableStateOf(false)
        private set
    var notice by mutableStateOf<String?>(null)
        private set

    fun check() {
        if (checking) return
        checking = true
        notice = null
        viewModelScope.launch {
            notice = try {
                val server = withContext(Dispatchers.IO) { ServerProbe().check(address) }
                "Server found at ${server.url}."
            } catch (error: IllegalArgumentException) {
                error.message ?: ServerAddress.INVALID_ADDRESS
            } catch (_: Exception) {
                "Could not reach that Server. Check its address and try again."
            }
            checking = false
        }
    }
}
