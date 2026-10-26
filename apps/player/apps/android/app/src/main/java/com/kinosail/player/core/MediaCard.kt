package com.kinosail.player.core

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
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
                           progressOverlay: Boolean = false) {
    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
        MediaArtwork(item, catalog, Modifier.fillMaxWidth(), landscape, progress = progressOverlay)
        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            val enlarged = LocalDensity.current.fontScale >= 1.5f
            Text(item.title, style = MaterialTheme.typography.titleMedium, color = KinoColor.text, minLines = if (tv) 2 else 1,
                maxLines = if (enlarged) Int.MAX_VALUE else 2, overflow = TextOverflow.Ellipsis)
            item.cardSubtitle.takeIf(String::isNotEmpty)?.let {
                Text(it, style = MaterialTheme.typography.bodySmall, color = KinoColor.muted,
                    maxLines = if (enlarged) Int.MAX_VALUE else 1, overflow = TextOverflow.Ellipsis)
            }
            if (!progressOverlay && item.positionLabel.isNotEmpty()) Text(interfaceText(item.positionLabel),
                style = MaterialTheme.typography.bodySmall, color = KinoColor.muted)
        }
    }
}
