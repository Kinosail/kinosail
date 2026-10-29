package com.kinosail.player.core

import androidx.compose.material3.Button
import androidx.compose.material3.Text
import androidx.compose.runtime.*

@Composable
internal fun RetryAction(tv: Boolean = false, enabled: Boolean = true, modifier: androidx.compose.ui.Modifier = androidx.compose.ui.Modifier,
                         onClick: () -> Unit) {
    if (tv) androidx.tv.material3.Button(onClick = onClick, enabled = enabled, modifier = modifier) { androidx.tv.material3.Text(interfaceText("Try again")) }
    else Button(onClick = onClick, enabled = enabled) { Text(interfaceText("Try again")) }
}

private val healDelays = listOf(2_000L, 5_000L, 15_000L, 30_000L, 60_000L)

@Composable
internal fun AutoRetry(failed: Boolean, retry: () -> Unit) {
    var attempt by remember { mutableIntStateOf(0) }
    val current by rememberUpdatedState(retry)
    val lifecycle = androidx.lifecycle.compose.LocalLifecycleOwner.current.lifecycle
    LaunchedEffect(failed, attempt) {
        if (!failed) { attempt = 0; return@LaunchedEffect }
        kotlinx.coroutines.delay(healDelays[minOf(attempt, healDelays.lastIndex)])
        current()
        attempt++
    }
    OnReturnToForeground(lifecycle) { if (failed) current() }
}

@Composable
internal fun OnReturnToForeground(lifecycle: androidx.lifecycle.Lifecycle, action: () -> Unit) {
    val current by rememberUpdatedState(action)
    DisposableEffect(lifecycle) {
        var paused = false
        val observer = androidx.lifecycle.LifecycleEventObserver { _, event ->
            when (event) {
                androidx.lifecycle.Lifecycle.Event.ON_PAUSE -> paused = true
                androidx.lifecycle.Lifecycle.Event.ON_RESUME -> if (paused) { paused = false; current() }
                else -> {}
            }
        }
        lifecycle.addObserver(observer)
        onDispose { lifecycle.removeObserver(observer) }
    }
}
