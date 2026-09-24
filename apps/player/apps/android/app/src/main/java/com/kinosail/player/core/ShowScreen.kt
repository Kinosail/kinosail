package com.kinosail.player.core

import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import com.kinosail.player.design.KinoColor
import com.kinosail.player.design.SailBackdrop

@Composable
internal fun ShowScreen(showId: String, viewer: Viewer, catalog: CatalogModel, tv: Boolean,
                        close: () -> Unit, play: (CatalogItem) -> Unit) {
    val model: ShowModel = viewModel()
    val state = model.state
    val detail = state.detail
    val focus = remember { FocusRequester() }
    var chosenSeason by remember(showId) { mutableStateOf<Int?>(null) }
    val featured = detail?.next
    val season = chosenSeason?.takeIf { it in (detail?.seasons ?: emptyList()) }
        ?: featured?.season ?: detail?.seasons?.firstOrNull()
    LaunchedEffect(showId, viewer.id) { model.open(showId, viewer.id) }
    DisposableEffect(Unit) { onDispose { model.reset() } }
    LaunchedEffect(detail?.id, tv) {
        if (tv && featured != null) { withFrameNanos { }; focus.requestFocus() }
    }
    Box(Modifier.fillMaxSize().background(if (tv || isSystemInDarkTheme()) KinoColor.background
        else MaterialTheme.colorScheme.background)) {
        if (tv || isSystemInDarkTheme()) SailBackdrop()
        LazyColumn(Modifier.fillMaxSize().safeDrawingPadding(),
            contentPadding = PaddingValues(if (tv) 56.dp else 20.dp),
            verticalArrangement = Arrangement.spacedBy(if (tv) 24.dp else 16.dp)) {
            item {
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically) {
                    Text(detail?.title ?: "Seasons & episodes",
                        style = if (tv) MaterialTheme.typography.displayMedium else MaterialTheme.typography.headlineLarge,
                        color = if (tv || isSystemInDarkTheme()) KinoColor.text else MaterialTheme.colorScheme.onBackground,
                        modifier = Modifier.weight(1f))
                    if (tv) androidx.tv.material3.Button(onClick = close) {
                        androidx.tv.material3.Text("Back to Library")
                    } else TextButton(onClick = close) { Text("Back") }
                }
            }
            if (state.loading && detail == null) item { CircularProgressIndicator(color = KinoColor.signal) }
            state.notice?.let { notice -> item {
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    Text(notice, color = if (tv) KinoColor.text else MaterialTheme.colorScheme.error)
                    if (tv) androidx.tv.material3.Button(onClick = model::retry) {
                        androidx.tv.material3.Text("Try again")
                    } else TextButton(onClick = model::retry) { Text("Try again") }
                }
            } }
            if (featured != null) {
                item {
                    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                        Text(featured.title, style = MaterialTheme.typography.titleLarge,
                            color = if (tv || isSystemInDarkTheme()) KinoColor.text else MaterialTheme.colorScheme.onBackground)
                        val label = "${if (featured.progress.seconds > 0 && !featured.progress.watched) "Resume" else "Play"} · S${featured.season} E${featured.episode}"
                        if (tv) androidx.tv.material3.Button(onClick = { play(featured) },
                            modifier = Modifier.focusRequester(focus)) { androidx.tv.material3.Text(label) }
                        else androidx.compose.material3.Button(onClick = { play(featured) }) { Text(label) }
                    }
                }
                item {
                    Text("Seasons", style = MaterialTheme.typography.titleLarge,
                        color = if (tv || isSystemInDarkTheme()) KinoColor.text else MaterialTheme.colorScheme.onBackground)
                }
                item {
                    LazyRow(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                        items(detail.seasons, key = { it }) { number ->
                            val label = if (number == 0) "Specials" else "Season $number"
                            val text = if (number == season) "$label · Selected" else label
                            if (tv) androidx.tv.material3.Button(onClick = { chosenSeason = number }) {
                                androidx.tv.material3.Text(text)
                            } else androidx.compose.material3.FilterChip(selected = number == season,
                                onClick = { chosenSeason = number }, label = { Text(label) })
                        }
                    }
                }
                val visible = detail.episodes.filter { it.season == season }
                items(visible, key = CatalogItem::id) { episode ->
                    if (tv) androidx.tv.material3.Card(onClick = { play(episode) },
                        modifier = Modifier.fillMaxWidth()) {
                        EpisodeRow(episode, catalog, tv = true)
                    } else androidx.compose.material3.Card(onClick = { play(episode) },
                        modifier = Modifier.fillMaxWidth(),
                        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface)) {
                        EpisodeRow(episode, catalog, tv = false)
                    }
                }
            } else if (detail != null) item {
                Text("This show has no available episodes.", color = KinoColor.muted)
            }
        }
    }
}

@Composable
private fun EpisodeRow(item: CatalogItem, catalog: CatalogModel, tv: Boolean) {
    Row(Modifier.fillMaxWidth().padding(if (tv) 16.dp else 12.dp),
        horizontalArrangement = Arrangement.spacedBy(if (tv) 24.dp else 12.dp),
        verticalAlignment = Alignment.CenterVertically) {
        val bitmap by produceState<android.graphics.Bitmap?>(null, item.artwork, catalog) {
            value = catalog.artwork(item.artwork, 400)
        }
        Box(Modifier.width(if (tv) 240.dp else 112.dp).aspectRatio(16f / 9f)
            .background(KinoColor.raised), contentAlignment = Alignment.Center) {
            bitmap?.let { Image(it.asImageBitmap(), contentDescription = null,
                contentScale = ContentScale.Crop, modifier = Modifier.fillMaxSize()) }
        }
        Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text("S${item.season} E${item.episode} · ${item.title}",
                style = if (tv) MaterialTheme.typography.titleLarge else MaterialTheme.typography.titleMedium,
                color = if (tv) KinoColor.text else MaterialTheme.colorScheme.onSurface)
            if (item.progress.seconds > 0 && !item.progress.watched) Text("Resume at ${item.progress.seconds.toInt() / 60}m ${item.progress.seconds.toInt() % 60}s",
                color = if (tv) KinoColor.muted else MaterialTheme.colorScheme.onSurfaceVariant)
            else if (item.progress.watched) Text("Watched",
                color = if (tv) KinoColor.muted else MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}
