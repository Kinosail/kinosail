package com.kinosail.player.tv

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.isImeVisible
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.tv.material3.Button
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import com.kinosail.player.core.ConnectionModel
import com.kinosail.player.core.ConnectionPhase
import com.kinosail.player.design.KinoColor
import com.kinosail.player.design.SailBackdrop
import com.kinosail.player.design.TvKinoTheme

class TvActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent { TvKinoTheme { TvStart() } }
    }
}

@Composable
@OptIn(ExperimentalLayoutApi::class)
private fun TvStart() {
    val connection: ConnectionModel = viewModel()
    val phase = connection.phase
    if (phase is ConnectionPhase.Connected) {
        TvLibrary(connection, phase.viewer)
        return
    }
    val firstFocus = remember { FocusRequester() }
    val editing = WindowInsets.isImeVisible
    BackHandler(phase is ConnectionPhase.Pairing) { connection.cancelPairing() }
    LaunchedEffect(phase, editing) {
        if (phase != ConnectionPhase.Restoring && !editing) firstFocus.requestFocus()
    }
    Box(Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        SailBackdrop()
        Column(Modifier.fillMaxSize().safeDrawingPadding().padding(64.dp),
            verticalArrangement = if (editing) Arrangement.Top else Arrangement.SpaceBetween) {
            if (!editing) Text("Kinosail", style = MaterialTheme.typography.headlineLarge,
                color = MaterialTheme.colorScheme.onBackground)
            Column(Modifier.widthIn(max = 800.dp), verticalArrangement = Arrangement.spacedBy(20.dp)) {
                when (phase) {
                    ConnectionPhase.Restoring -> {
                        Text("Restoring your Server", style = MaterialTheme.typography.displayMedium,
                            color = MaterialTheme.colorScheme.onBackground)
                        Text("Checking your saved connection…", style = MaterialTheme.typography.titleLarge,
                            color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    ConnectionPhase.Setup -> {
                        if (!editing) Text("Your library, on the big screen", style = MaterialTheme.typography.displayMedium,
                            color = MaterialTheme.colorScheme.onBackground)
                        if (!editing) Text("Enter your Kinosail Server address to connect this TV.",
                            style = MaterialTheme.typography.titleLarge,
                            color = MaterialTheme.colorScheme.onSurfaceVariant)
                        OutlinedTextField(value = connection.address,
                            onValueChange = { if (it.length <= 2048) connection.address = it },
                            label = { androidx.compose.material3.Text("Server address") },
                            placeholder = { androidx.compose.material3.Text("https://your-server") },
                            singleLine = true, enabled = !connection.busy,
                            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri, imeAction = ImeAction.Go),
                            keyboardActions = KeyboardActions(onGo = { connection.connect("Kinosail Android TV") }),
                            colors = OutlinedTextFieldDefaults.colors(
                                focusedTextColor = KinoColor.text, unfocusedTextColor = KinoColor.text,
                                focusedLabelColor = KinoColor.signal, unfocusedLabelColor = KinoColor.muted,
                                focusedBorderColor = KinoColor.signal, unfocusedBorderColor = KinoColor.muted,
                                focusedPlaceholderColor = KinoColor.muted, unfocusedPlaceholderColor = KinoColor.muted),
                            modifier = Modifier.fillMaxWidth())
                        if (!editing) Button(onClick = { connection.connect("Kinosail Android TV") },
                            enabled = !connection.busy, modifier = Modifier.focusRequester(firstFocus)) {
                            Text(if (connection.busy) "Connecting…" else "Connect")
                        }
                    }
                    is ConnectionPhase.Pairing -> {
                        Text("Approve this TV", style = MaterialTheme.typography.displayMedium,
                            color = MaterialTheme.colorScheme.onBackground)
                        Text("On a signed in device, open ${phase.server}/quick-connect and enter:",
                            style = MaterialTheme.typography.titleLarge,
                            color = MaterialTheme.colorScheme.onSurfaceVariant)
                        Text(phase.code, style = MaterialTheme.typography.displayLarge,
                            fontFamily = FontFamily.Monospace, letterSpacing = 8.sp,
                            color = MaterialTheme.colorScheme.primary)
                        Text("Waiting for approval…", style = MaterialTheme.typography.titleLarge,
                            color = MaterialTheme.colorScheme.onSurfaceVariant)
                        Button(onClick = connection::cancelPairing, modifier = Modifier.focusRequester(firstFocus)) {
                            Text("Cancel")
                        }
                    }
                    is ConnectionPhase.Connected -> Unit
                }
                if (!editing) connection.notice?.let { Text(it, color = MaterialTheme.colorScheme.error) }
            }
        }
    }
}
