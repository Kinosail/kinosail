package com.kinosail.player.core

import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.unit.dp
import com.kinosail.player.design.KinoColor

internal val CatalogItem.landscapePath get() = backdrop.ifEmpty { artwork }
internal val CatalogItem.artworkRatio get() = when {
    kind in setOf("music", "audiobook") -> 1f
    showId.isNotEmpty() && kind == "video" || kind == "photo" -> 16f / 9f
    else -> 2f / 3f
}
internal val CatalogItem.playLabel get() = if (progress.seconds > 0 && !progress.watched) "Resume" else "Play"
internal val CatalogItem.positionLabel get() = when {
    progress.watched -> "Watched"
    progress.seconds > 0 -> "Resume at ${mediaClock(progress.seconds)}"
    else -> ""
}
internal fun mediaClock(seconds: Double): String {
    val value = seconds.toLong().coerceAtLeast(0)
    return if (value >= 3600) "%d:%02d:%02d".format(value / 3600, value / 60 % 60, value % 60)
        else "%d:%02d".format(value / 60, value % 60)
}

@Composable
internal fun MediaArtwork(item: CatalogItem, catalog: CatalogModel, modifier: Modifier = Modifier,
                          landscape: Boolean = false, dimension: Int = 400, progress: Boolean = true) {
    val path = if (landscape) item.landscapePath else item.artwork
    val ratio = if (landscape) 16f / 9f else item.artworkRatio
    val image by produceState<android.graphics.Bitmap?>(null, path, catalog, dimension) {
        value = null
        value = catalog.artwork(path, dimension)
    }
    Box(modifier.aspectRatio(ratio).clip(RoundedCornerShape(12.dp)).background(KinoColor.raised),
        contentAlignment = Alignment.Center) {
        Text(item.title.firstOrNull()?.uppercase() ?: "K", color = KinoColor.signal,
            style = MaterialTheme.typography.displayMedium, modifier = Modifier.clearAndSetSemantics { })
        image?.let { Image(it.asImageBitmap(), contentDescription = null,
            contentScale = ContentScale.Fit, modifier = Modifier.fillMaxSize()) }
        if (progress && item.positionLabel.isNotEmpty()) Text(interfaceText(item.positionLabel), color = KinoColor.text,
            style = MaterialTheme.typography.labelMedium,
            modifier = Modifier.align(Alignment.BottomStart).padding(8.dp)
                .clip(RoundedCornerShape(5.dp)).background(Color.Black.copy(alpha = 0.88f)).padding(6.dp))
    }
}

@Composable
internal fun MediaHero(item: CatalogItem, catalog: CatalogModel, tv: Boolean,
                       actions: @Composable () -> Unit) {
    BoxWithConstraints(Modifier.fillMaxWidth()) {
        val wide = maxWidth >= 600.dp && androidx.compose.ui.platform.LocalDensity.current.fontScale < 1.5f
        val picture: @Composable () -> Unit = {
            MediaArtwork(item, catalog, if (wide) Modifier.width(if (item.backdrop.isNotEmpty()) maxWidth * 0.46f else 200.dp) else
                if (item.backdrop.isNotEmpty()) Modifier.fillMaxWidth() else Modifier.width(200.dp),
                landscape = item.backdrop.isNotEmpty(), dimension = 800, progress = false)
        }
        val information: @Composable () -> Unit = {
            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Text(item.title, style = if (tv) MaterialTheme.typography.displayMedium
                    else MaterialTheme.typography.headlineLarge, color = KinoColor.text)
                val metadata = listOf(item.year, item.rating, item.genres,
                    if (item.showId.isNotEmpty()) "S${item.season} E${item.episode}" else "")
                    .filter(String::isNotEmpty).joinToString(" · ")
                if (metadata.isNotEmpty()) Text(metadata, color = KinoColor.muted)
                if (item.positionLabel.isNotEmpty()) Text(interfaceText(item.positionLabel), color = KinoColor.muted)
                if (item.plot.isNotEmpty()) Text(item.plot, color = KinoColor.text, style = MaterialTheme.typography.bodyLarge)
                actions()
            }
        }
        if (wide) Row(horizontalArrangement = Arrangement.spacedBy(32.dp), verticalAlignment = Alignment.CenterVertically) {
            picture()
            Box(Modifier.weight(1f)) { information() }
        } else Column(verticalArrangement = Arrangement.spacedBy(12.dp)) { picture(); information() }
    }
}
