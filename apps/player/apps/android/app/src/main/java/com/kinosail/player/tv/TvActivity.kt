package com.kinosail.player.tv

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.isImeVisible
import androidx.compose.foundation.layout.ExperimentalLayoutApi
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
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import com.kinosail.player.R
import com.kinosail.player.core.ConnectionModel
import com.kinosail.player.design.KinoColor
import com.kinosail.player.design.SailBackdrop
import com.kinosail.player.design.TvKinoTheme
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import androidx.tv.material3.Button

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
    val firstFocus = remember { FocusRequester() }
    val editing = WindowInsets.isImeVisible
    LaunchedEffect(Unit) { firstFocus.requestFocus() }
    Box(Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        SailBackdrop()
        Column(
            Modifier.fillMaxSize().safeDrawingPadding().padding(64.dp),
            verticalArrangement = if (editing) Arrangement.Top else Arrangement.SpaceBetween,
        ) {
            if (!editing) Text(
                stringResource(R.string.app_name),
                style = MaterialTheme.typography.headlineLarge,
                color = MaterialTheme.colorScheme.onBackground,
            )
            Column(Modifier.widthIn(max = 760.dp), verticalArrangement = Arrangement.spacedBy(20.dp)) {
                if (!editing) Text(
                    stringResource(R.string.tv_start_title),
                    style = MaterialTheme.typography.displayMedium,
                    color = MaterialTheme.colorScheme.onBackground,
                )
                if (!editing) Text(
                    stringResource(R.string.tv_start_body),
                    style = MaterialTheme.typography.titleLarge,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                OutlinedTextField(
                    value = connection.address,
                    onValueChange = { if (it.length <= 2048) connection.address = it },
                    label = { androidx.compose.material3.Text(stringResource(R.string.server_address)) },
                    placeholder = { androidx.compose.material3.Text("https://your-server") },
                    singleLine = true,
                    enabled = !connection.checking,
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri, imeAction = ImeAction.Go),
                    keyboardActions = KeyboardActions(onGo = { connection.check() }),
                    colors = OutlinedTextFieldDefaults.colors(
                        focusedTextColor = KinoColor.text,
                        unfocusedTextColor = KinoColor.text,
                        focusedLabelColor = KinoColor.signal,
                        unfocusedLabelColor = KinoColor.muted,
                        focusedBorderColor = KinoColor.signal,
                        unfocusedBorderColor = KinoColor.muted,
                        focusedPlaceholderColor = KinoColor.muted,
                        unfocusedPlaceholderColor = KinoColor.muted,
                    ),
                    modifier = Modifier.fillMaxWidth(),
                )
                if (!editing) {
                    Button(onClick = connection::check, enabled = !connection.checking,
                        modifier = Modifier.focusRequester(firstFocus)) {
                        Text(stringResource(if (connection.checking) R.string.checking_server else R.string.check_server))
                    }
                }
                if (!editing) connection.notice?.let { Text(it, color = MaterialTheme.colorScheme.onSurfaceVariant) }
            }
        }
    }
}
