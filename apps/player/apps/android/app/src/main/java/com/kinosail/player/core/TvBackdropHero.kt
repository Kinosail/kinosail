package com.kinosail.player.core

import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.kinosail.player.design.KinoColor

@Composable
internal fun TvBackdrop(item: CatalogItem, catalog: CatalogModel) {
    val image by rememberArtwork(catalog, item.spotlightPath, 1600)
    Box(Modifier.fillMaxSize().testTag("tv-backdrop").clearAndSetSemantics { }) {
        image?.let { Image(it.asImageBitmap(), contentDescription = null, contentScale = ContentScale.Crop,
            alignment = Alignment.TopEnd, modifier = Modifier.fillMaxWidth().fillMaxHeight(0.86f)) }
        Box(Modifier.fillMaxSize().background(Brush.horizontalGradient(
            0f to KinoColor.background.copy(alpha = 0.92f), 0.42f to KinoColor.background.copy(alpha = 0.55f), 0.7f to Color.Transparent)))
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0.45f to Color.Transparent, 0.86f to KinoColor.background)))
    }
}

@Composable
internal fun TvHeroInformation(item: CatalogItem, catalog: CatalogModel, metadata: String, summary: String = "",
                               actions: @Composable () -> Unit) {
    val height = LocalConfiguration.current.screenHeightDp.dp * 0.62f
    Box(Modifier.fillMaxWidth().heightIn(min = height), contentAlignment = Alignment.BottomStart) {
        Column(Modifier.widthIn(max = 540.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Text(item.title, color = KinoColor.text, fontWeight = FontWeight.Bold, style = MaterialTheme.typography.displaySmall,
                modifier = Modifier.semantics { heading() })
            if (metadata.isNotEmpty()) Text(metadata, color = KinoColor.muted, style = MaterialTheme.typography.titleMedium)
            if (item.plot.isNotEmpty()) Text(item.plot, color = KinoColor.text, style = MaterialTheme.typography.bodyLarge,
                maxLines = 4, overflow = TextOverflow.Ellipsis)
            if (summary.isNotEmpty()) Text(summary, color = KinoColor.muted, fontWeight = FontWeight.SemiBold,
                style = MaterialTheme.typography.titleMedium)
            Box(Modifier.widthIn(max = 420.dp)) { WatchPosition(item, catalog) }
            actions()
        }
    }
}
