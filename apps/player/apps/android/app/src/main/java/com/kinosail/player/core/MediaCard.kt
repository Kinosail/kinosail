package com.kinosail.player.core

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.border
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.kinosail.player.design.KinoColor

internal val CatalogItem.cardSubtitle get() = when {
    kind == "video" || kind == "show" -> if (showId.isNotEmpty() && (season > 0 || episode > 0)) "S$season E$episode" else artist
    showId.isNotEmpty() -> rating
    else -> listOf(artist, year, rating).filter(String::isNotEmpty).joinToString(" · ")
}

internal val CatalogItem.episodeTitle get() = "S%02dE%02d · ".format(season, episode).let { prefix ->
    if (title.startsWith(prefix)) title.removePrefix(prefix) else title
}

@Composable
internal fun MediaCardBody(item: CatalogItem, catalog: CatalogModel, tv: Boolean, landscape: Boolean = false,
                           progressOverlay: Boolean = false, resumes: Boolean = false, focused: Boolean = false) {
    val resumable = item.kind == "video" && item.progress.seconds > 0 && !item.progress.watched
    val summary = rememberWatchSummary(item, catalog)
    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
        val lift by animateFloatAsState(if (focused) 1f else 0f, tween(180), label = "card focus")
        Box(Modifier.graphicsLayer {
            scaleX = 1f + 0.06f * lift; scaleY = scaleX
            shadowElevation = 16.dp.toPx() * lift; shape = RoundedCornerShape(12.dp); clip = false
        }.then(if (focused) Modifier.border(2.dp, KinoColor.text, RoundedCornerShape(12.dp)) else Modifier)) {
            MediaArtwork(item, catalog, Modifier.fillMaxWidth(), landscape, progress = progressOverlay && !resumable)
            summary?.let { WatchBar(it, Modifier.align(Alignment.BottomCenter).padding(start = 12.dp, end = 12.dp, bottom = 8.dp)) }
            if (item.unwatched) UnwatchedMark(Modifier.align(Alignment.TopEnd))
        }
        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            val enlarged = LocalDensity.current.fontScale >= 1.5f
            Text(item.title, style = MaterialTheme.typography.titleMedium, color = KinoColor.text, minLines = if (tv) 2 else 1,
                maxLines = if (enlarged) Int.MAX_VALUE else 2, overflow = TextOverflow.Ellipsis)
            item.cardSubtitle.takeIf(String::isNotEmpty)?.let {
                Text(it, style = MaterialTheme.typography.bodySmall, color = KinoColor.muted,
                    maxLines = if (enlarged) Int.MAX_VALUE else 1, overflow = TextOverflow.Ellipsis)
            }
            val position = when {
                resumable && tv && resumes -> summary?.remainingLabel() ?: interfaceText(item.positionLabel)
                resumable || progressOverlay -> ""
                else -> interfaceText(item.positionLabel)
            }
            if (position.isNotEmpty()) Text(position, style = MaterialTheme.typography.bodySmall, color = KinoColor.muted)
        }
    }
}

@Composable
internal fun TvMediaCard(item: CatalogItem, catalog: CatalogModel, modifier: Modifier = Modifier, landscape: Boolean = false,
                         resumes: Boolean = false, onFocus: (CatalogItem) -> Unit = {}, open: () -> Unit) {
    var focused by remember { mutableStateOf(false) }
    Column(modifier.restoresTvFocus(item.id).onFocusChanged {
        focused = it.isFocused
        if (it.isFocused) onFocus(item)
    }.clickable(remember { MutableInteractionSource() }, indication = null, role = Role.Button) {
        TvFocusMemory.id = item.id
        open()
    }
        .semantics { contentDescription = item.title }) {
        MediaCardBody(item, catalog, tv = true, landscape, resumes = resumes, focused = focused)
    }
}
