package com.kinosail.player.mobile

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.CheckCircle
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import com.kinosail.player.core.*
import com.kinosail.player.design.KinoColor
import java.time.Instant
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

private const val USED_CODE = "That code expired or was already used. Check the code on your TV."

@Composable
internal fun ConnectTvScreen(viewer: Viewer, catalog: CatalogModel, close: () -> Unit) {
    val connection = catalog.connection
    var code by remember { mutableStateOf("") }
    var approval by remember { mutableStateOf<DeviceApproval?>(null) }
    var approved by remember { mutableStateOf(false) }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()
    val usedCode = interfaceText(USED_CODE)
    val approveFailed = interfaceText("Could not approve this device. Try again.")
    val checkFailed = interfaceText("Could not check that code. Try again.")
    fun submit(failure: String, action: suspend (AccountApi, String, String) -> Unit) {
        val (saved, viewerId) = connection ?: run { error = failure; return }
        busy = true
        error = null
        scope.launch {
            try { action(AccountApi(saved.server), saved.token, viewerId) }
            catch (cancelled: CancellationException) { throw cancelled }
            catch (_: Exception) { error = failure }
            finally { busy = false }
        }
    }
    SettingsPage(interfaceText("Connect a TV")) {
        val pending = approval
        when {
            approved -> MoreGroup {
                Row(Modifier.fillMaxWidth().padding(18.dp), verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                    Icon(Icons.Filled.CheckCircle, contentDescription = null, tint = KinoColor.signal)
                    Text(interfaceText("Device connected"), color = KinoColor.text, style = MaterialTheme.typography.titleMedium)
                }
                MoreDivider(icon = false)
                MoreRow(interfaceText("Done"), color = KinoColor.signal, chevron = false, onClick = close)
            }
            pending != null -> {
                MoreSection(interfaceText("Review device"))
                MoreGroup {
                    Detail(interfaceText("Device"), pending.device.ifEmpty { interfaceText("Kinosail device") })
                    MoreDivider(icon = false)
                    Detail(interfaceText("Code"), pending.code)
                    MoreDivider(icon = false)
                    Detail(interfaceText("Viewer Profile"), viewer.name)
                }
                Text(interfaceText("Approve only if this code matches the device you want to connect."), color = KinoColor.text,
                    modifier = Modifier.padding(horizontal = 18.dp))
                FlowRow(horizontalArrangement = Arrangement.spacedBy(12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    Button(onClick = {
                        submit(approveFailed) { api, token, id ->
                            val accepted = withContext(Dispatchers.IO) { api.approve(pending.code, token, id) }
                            if (accepted) approved = true else error = usedCode
                        }
                    }, enabled = !busy && pending.expires.isAfter(Instant.now())) {
                        Text(interfaceText(if (busy) "Approving…" else "Approve device"))
                    }
                    OutlinedButton(onClick = close, enabled = !busy) { Text(interfaceText("Not now")) }
                }
            }
            else -> {
                MoreSection(interfaceText("Enter the code shown on your TV"))
                OutlinedTextField(code, onValueChange = { code = it.filter(Char::isDigit).take(6); error = null },
                    label = { Text(interfaceText("Six-digit code")) }, singleLine = true,
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.NumberPassword),
                    modifier = Modifier.fillMaxWidth())
                Button(onClick = {
                    submit(checkFailed) { api, token, id ->
                        val found = withContext(Dispatchers.IO) { api.previewApproval(code, token, id) }
                        if (found == null) error = usedCode else approval = found
                    }
                }, enabled = !busy && code.length == 6) { Text(interfaceText(if (busy) "Checking…" else "Review device")) }
            }
        }
        error?.let { Text(it, color = KinoColor.danger, modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite }) }
    }
}

@Composable
private fun Detail(label: String, value: String) {
    Row(Modifier.fillMaxWidth().heightIn(min = 56.dp).padding(horizontal = 18.dp).semantics(mergeDescendants = true) { },
        verticalAlignment = Alignment.CenterVertically) {
        Text(label, color = KinoColor.text, modifier = Modifier.weight(1f))
        Text(value, color = KinoColor.muted)
    }
}
