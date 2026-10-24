package com.kinosail.player.core

import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.layout.layout
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.kinosail.player.design.KinoColor

internal val CatalogItem.spotlightPath get() = backdrop.ifEmpty { if (showId.isNotEmpty() && kind == "video") artwork else "" }

@Composable
internal fun spotlightFits(item: CatalogItem, width: Dp) =
    item.spotlightPath.isNotEmpty() && width < 600.dp && LocalDensity.current.fontScale < 1.5f

@Composable
internal fun MediaSpotlight(item: CatalogItem, catalog: CatalogModel, eyebrow: String?, bleed: Dp, topBleed: Dp = 0.dp,
                            top: @Composable () -> Unit = {}, actions: @Composable () -> Unit) {
    val path = item.spotlightPath
    val image by rememberArtwork(catalog, path, 1600)
    Column(Modifier.fillMaxWidth()) {
        Box(Modifier.bleed(bleed, topBleed).fillMaxWidth().aspectRatio(1f / 0.86f)) {
            Box(Modifier.fillMaxSize().background(KinoColor.surface).clearAndSetSemantics { }) {
                image?.let { Image(it.asImageBitmap(), contentDescription = null, contentScale = ContentScale.Crop,
                    modifier = Modifier.fillMaxSize()) }
                Box(Modifier.fillMaxSize().background(Brush.verticalGradient(
                    0f to KinoColor.background.copy(alpha = 0.7f), 0.22f to Color.Transparent,
                    0.55f to Color.Transparent, 0.82f to KinoColor.background.copy(alpha = 0.6f), 1f to KinoColor.background)))
            }
            Box(Modifier.align(Alignment.TopStart).fillMaxWidth().padding(start = bleed, end = bleed, top = topBleed)) { top() }
        }
        Column(Modifier.overlapUp(120.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                eyebrow?.let {
                    Text(it, color = KinoColor.signal, fontWeight = FontWeight.Black,
                        letterSpacing = 1.6.sp, style = MaterialTheme.typography.labelMedium)
                }
                Text(item.title, color = KinoColor.text, fontWeight = FontWeight.Bold,
                    style = MaterialTheme.typography.headlineLarge, modifier = Modifier.semantics { heading() })
                val metadata = listOf(item.year, item.rating, item.genres,
                    if (item.showId.isNotEmpty()) "S${item.season} E${item.episode}" else "")
                    .filter(String::isNotEmpty).joinToString(" · ")
                if (metadata.isNotEmpty()) Text(metadata, color = KinoColor.muted, style = MaterialTheme.typography.bodyMedium)
                if (item.positionLabel.isNotEmpty()) Text(interfaceText(item.positionLabel), color = KinoColor.muted,
                    style = MaterialTheme.typography.bodyMedium)
            }
            actions()
            if (item.plot.isNotEmpty()) Text(item.plot, color = KinoColor.text, style = MaterialTheme.typography.bodyLarge)
        }
    }
}

private fun Modifier.bleed(horizontal: Dp, top: Dp) = layout { measurable, constraints ->
    val extra = horizontal.roundToPx() * 2
    val rise = top.roundToPx()
    val width = constraints.maxWidth + extra
    val placeable = measurable.measure(constraints.copy(minWidth = width, maxWidth = width, maxHeight = androidx.compose.ui.unit.Constraints.Infinity))
    layout(constraints.maxWidth, maxOf(0, placeable.height - rise)) { placeable.place(-extra / 2, -rise) }
}

private fun Modifier.overlapUp(amount: Dp) = layout { measurable, constraints ->
    val shift = amount.roundToPx()
    val placeable = measurable.measure(constraints)
    layout(placeable.width, maxOf(0, placeable.height - shift)) { placeable.place(0, -shift) }
}
