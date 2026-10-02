package com.kinosail.player.core

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.selected
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
    var chosenSeason by remember(showId) { mutableStateOf<Int?>(null) }
    val featured = detail?.next
    val season = chosenSeason?.takeIf { it in (detail?.seasons ?: emptyList()) }
        ?: featured?.season ?: detail?.seasons?.firstOrNull()
    LaunchedEffect(showId, viewer.id) { model.open(showId, viewer.id) }
    DisposableEffect(Unit) { onDispose { model.reset() } }
    Box(Modifier.fillMaxSize().background(KinoColor.background)) {
        SailBackdrop()
        LazyColumn(Modifier.fillMaxSize().safeDrawingPadding(), contentPadding = PaddingValues(if (tv) 48.dp else 20.dp),
            verticalArrangement = Arrangement.spacedBy(24.dp)) {
            item {
                if (tv) androidx.tv.material3.Button(onClick = close) { androidx.tv.material3.Text(interfaceText("Back to Library")) }
                else TextButton(onClick = close) { Text(interfaceText("Back")) }
            }
            if (state.loading && detail == null) item { LibraryLoading(home = true, tv = tv, show = true) }
            state.notice?.let { notice -> item {
                Text(notice, color = KinoColor.text, modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite })
                if (tv) androidx.tv.material3.Button(onClick = model::retry) { androidx.tv.material3.Text(interfaceText("Try again")) }
                else TextButton(onClick = model::retry) { Text(interfaceText("Try again")) }
            } }
            if (featured != null) {
                item {
                    MediaHero(featured.copy(title = detail.title, plot = detail.plot, year = detail.year), catalog, tv) {
                        val label = "${featured.playLabel} · S${featured.season} E${featured.episode}"
                        if (tv) androidx.tv.material3.Button(onClick = { play(featured) }) { androidx.tv.material3.Text(label) }
                        else Button(onClick = { play(featured) }) { Text(label) }
                    }
                }
                val visible = detail.episodes.filter { it.season == season }
                if (tv) item {
                    Row(horizontalArrangement = Arrangement.spacedBy(24.dp)) {
                        Column(Modifier.width(180.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                            Text(interfaceText("Seasons"), color = KinoColor.text, style = MaterialTheme.typography.titleLarge)
                            detail.seasons.forEach { number ->
                                androidx.tv.material3.Button(onClick = { chosenSeason = number }, modifier = Modifier.semantics { selected = number == season }) {
                                    androidx.tv.material3.Text(if (number == 0) "Specials" else "Season $number")
                                }
                            }
                        }
                        LazyRow(Modifier.weight(1f), horizontalArrangement = Arrangement.spacedBy(18.dp), contentPadding = PaddingValues(12.dp)) {
                            items(visible, key = CatalogItem::id) { episode ->
                                androidx.tv.material3.Card(onClick = { play(episode) }, modifier = Modifier.width(300.dp).semantics { contentDescription = episode.title }) {
                                    Column {
                                        MediaArtwork(episode, catalog, Modifier.fillMaxWidth())
                                        EpisodeDescription(episode, Modifier.padding(12.dp))
                                    }
                                }
                            }
                        }
                    }
                } else {
                    item { Text(interfaceText("Seasons"), color = KinoColor.text, style = MaterialTheme.typography.titleLarge) }
                    item {
                        LazyRow(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                            items(detail.seasons, key = { it }) { number ->
                                FilterChip(selected = number == season, onClick = { chosenSeason = number },
                                    label = { Text(if (number == 0) "Specials" else "Season $number") })
                            }
                        }
                    }
                    items(visible, key = CatalogItem::id) { episode ->
                        Card(onClick = { play(episode) }, modifier = Modifier.semantics { contentDescription = episode.title }, colors = CardDefaults.cardColors(containerColor = KinoColor.surface)) {
                            BoxWithConstraints(Modifier.fillMaxWidth().padding(12.dp)) {
                                if (maxWidth < 320.dp || androidx.compose.ui.platform.LocalDensity.current.fontScale >= 1.5f)
                                    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                                        MediaArtwork(episode, catalog, Modifier.fillMaxWidth())
                                        EpisodeDescription(episode)
                                    }
                                else Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                                    MediaArtwork(episode, catalog, Modifier.width(128.dp))
                                    EpisodeDescription(episode, Modifier.weight(1f))
                                }
                            }
                        }
                    }
                }
            } else if (detail != null) item {
                Text(detail.title, color = KinoColor.text, style = MaterialTheme.typography.headlineLarge)
                Text(interfaceText("This show has no available episodes."), color = KinoColor.muted)
            }
        }
    }
}

@Composable
private fun EpisodeDescription(item: CatalogItem, modifier: Modifier = Modifier) {
    Column(modifier, verticalArrangement = Arrangement.spacedBy(6.dp)) {
        Text("S${item.season} E${item.episode} · ${item.title}", color = KinoColor.text, style = MaterialTheme.typography.titleMedium)
        if (item.plot.isNotEmpty()) Text(item.plot, color = KinoColor.muted, style = MaterialTheme.typography.bodyMedium)
    }
}
