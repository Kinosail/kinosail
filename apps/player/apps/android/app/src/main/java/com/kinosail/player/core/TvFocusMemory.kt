package com.kinosail.player.core

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Modifier
import androidx.compose.ui.composed
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import com.kinosail.player.design.KinoColor

internal object TvFocusMemory {
    const val LAUNCH = "launch"
    var id by mutableStateOf<String?>(null)
}

internal fun Modifier.restoresTvFocus(key: String): Modifier = composed {
    val requester = remember { FocusRequester() }
    LaunchedEffect(key, TvFocusMemory.id) {
        if (TvFocusMemory.id != key) return@LaunchedEffect
        withFrameNanos { }
        if (TvFocusMemory.id == key && runCatching { requester.requestFocus() }.getOrDefault(false)) TvFocusMemory.id = null
    }
    focusRequester(requester)
}

internal fun Modifier.initialTvFocus(key: Any?): Modifier = composed {
    val requester = remember { FocusRequester() }
    val manager = LocalFocusManager.current
    var focused by remember { mutableStateOf(false) }
    LaunchedEffect(key) {
        withFrameNanos { }
        if (focused) manager.clearFocus(force = true)
        runCatching { requester.requestFocus() }
    }
    focusRequester(requester).onFocusChanged { focused = it.isFocused }
}

internal val CatalogItem.unwatched get() = (kind == "video" || kind == "show") && !progress.watched && progress.seconds <= 0

@Composable
internal fun UnwatchedMark(modifier: Modifier = Modifier) {
    Box(modifier.padding(7.dp).testTag("unwatched").size(12.dp)
        .shadow(6.dp, CircleShape, ambientColor = KinoColor.signal, spotColor = KinoColor.signal)
        .background(KinoColor.signal, CircleShape).border(2.dp, Color.Black.copy(alpha = 0.55f), CircleShape))
}
