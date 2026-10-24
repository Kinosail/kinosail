package com.kinosail.player.core

import androidx.compose.material3.Button
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable

@Composable
internal fun RetryAction(tv: Boolean = false, enabled: Boolean = true, onClick: () -> Unit) {
    if (tv) androidx.tv.material3.Button(onClick = onClick, enabled = enabled) { androidx.tv.material3.Text(interfaceText("Try again")) }
    else Button(onClick = onClick, enabled = enabled) { Text(interfaceText("Try again")) }
}
