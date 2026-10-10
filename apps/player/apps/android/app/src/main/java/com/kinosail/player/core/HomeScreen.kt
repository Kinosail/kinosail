package com.kinosail.player.core

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.compose.ui.text.style.TextOverflow
import androidx.lifecycle.viewmodel.compose.viewModel
import com.kinosail.player.design.KinoColor
import com.kinosail.player.design.SailBackdrop

@Composable
internal fun HomeScreen(viewer: Viewer, catalog: CatalogModel, tv: Boolean, nowPlaying: CatalogItem?,
                        browse: (String) -> Unit, open: (CatalogItem) -> Unit, play: (CatalogItem) -> Unit,
                        listening: Boolean = false, settings: () -> Unit = {}) {
    val model: HomeModel = viewModel()
    val state = model.state.copy(listening = listening)
    LaunchedEffect(viewer.serverId, viewer.id) { model.open(viewer) }
    DisposableEffect(Unit) { onDispose { model.reset() } }
    Box(Modifier.fillMaxSize().background(KinoColor.background)) {
        SailBackdrop()
        Column(if (tv) Modifier.fillMaxSize().safeDrawingPadding() else Modifier.fillMaxSize()) {
            if (tv) HomeHeader(listening, browse, settings)
            LazyColumn(Modifier.weight(1f), contentPadding = PaddingValues(if (tv) 48.dp else 20.dp),
                verticalArrangement = Arrangement.spacedBy(32.dp)) {
                if (!tv) item {
                    FlowRow(horizontalArrangement = Arrangement.spacedBy(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        Text(if (listening) interfaceText("Listen") else "Kinosail",
                            style = MaterialTheme.typography.titleLarge,
                            color = KinoColor.text, modifier = Modifier.weight(1f))
                        TextButton(onClick = { browse("list") }) { Text(interfaceText("My List")) }
                    }
                }
                if (nowPlaying != null) item {
                    HomeAction("Now playing · ${nowPlaying.title}", tv) { play(nowPlaying) }
                }
                if (state.loading && state.featured == null) item { LibraryLoading(home = true, tv = tv) }
                state.notice?.let { notice -> item {
                    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        Text(notice, color = KinoColor.text, modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite })
                        HomeAction(interfaceText("Try again"), tv, model::retry)
                    }
                } }
                if (tv && state.tvWatchingRail.isNotEmpty()) item {
                    HomeShelf(if (listening) "Continue listening" else "Continue watching", state.tvWatchingRail,
                        catalog, tv, landscape = true, open = play)
                }
                if (!tv) state.featured?.let { featured -> item {
                    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                        Text(interfaceText(if (featured.progress.seconds > 0) if (listening) "Listening" else "Watching" else "For you"),
                            style = MaterialTheme.typography.titleLarge, color = KinoColor.text)
                        MediaHero(featured.copy(plot = ""), catalog, tv = false) {
                            FlowRow(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                                HomeAction(interfaceText(featured.playLabel), false) {
                                    if (featured.kind == "show") open(featured) else play(featured)
                                }
                                OutlinedButton(onClick = { open(featured) }) { Text(interfaceText("Details")) }
                            }
                        }
                    }
                } }
                if (!tv && state.watchShelf.isNotEmpty()) item {
                    HomeShelf(if (listening) "Continue listening" else "Continue watching", state.watchShelf,
                        catalog, tv, landscape = true, open = play)
                }
                if (tv && !(state.loading && state.featured == null)) item { BrowseLinks(browse, tv, listening) }
                val shelves = if (listening) listOf("Recently added music" to state.recentShelf.filter { it.kind == "music" },
                    "Recently added audiobooks" to state.recentShelf.filter { it.kind == "audiobook" })
                else listOf("Recently added movies" to state.movies, "Recently added TV shows" to state.shows,
                    "Unwatched TV shows" to state.unwatchedShows, "Unwatched movies" to state.unwatchedMovies)
                shelves.filter { it.second.isNotEmpty() }.forEach { (title, items) -> item {
                    HomeShelf(title, items, catalog, tv, open = open)
                } }
                if (!listening && state.movieGenres.isNotEmpty()) {
                    item { Text(interfaceText("Movie genres"), style = MaterialTheme.typography.headlineMedium, color = KinoColor.text) }
                    state.movieGenres.forEach { (genre, movies) -> item { HomeShelf(genre, movies, catalog, tv, open = open) } }
                }
                if (!state.loading && state.notice == null && state.featured == null) item {
                    Text(interfaceText(if (listening) "Add music or audiobooks to your Server to see them here."
                        else "Media added to your Server will appear here."), color = KinoColor.muted)
                }
                if (!tv) item { BrowseLinks(browse, tv, listening) }
            }
        }
    }
}

@Composable
private fun HomeHeader(listening: Boolean, browse: (String) -> Unit, settings: () -> Unit) {
    FlowRow(Modifier.fillMaxWidth().padding(horizontal = 48.dp, vertical = 16.dp),
        horizontalArrangement = Arrangement.spacedBy(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(if (listening) interfaceText("Listen") else "Kinosail", style = MaterialTheme.typography.headlineLarge,
            color = KinoColor.text, modifier = Modifier.weight(1f))
        HomeAction(interfaceText(if (listening) "Watch" else "Listen"), true) { browse(if (listening) "home" else "listen") }
        HomeAction(interfaceText("Search"), true) { browse("search") }
        HomeAction(interfaceText("Settings"), true, settings)
    }
}

internal fun homeCardWidth(tv: Boolean, landscape: Boolean) =
    if (landscape) if (tv) 390.dp else 260.dp else if (tv) 230.dp else 164.dp

@Composable
private fun BrowseLinks(browse: (String) -> Unit, tv: Boolean, listening: Boolean) {
    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Text(interfaceText("Browse library"), style = MaterialTheme.typography.titleLarge, color = KinoColor.text,
            modifier = Modifier.semantics { heading() })
        val views = if (listening) LIBRARY_VIEWS.filter { it.first in setOf("music", "audiobooks") }
            else LIBRARY_VIEWS.filter { it.first in setOf("movies", "shows", "music", "audiobooks", "photos", "list", "all") }
        LazyRow(horizontalArrangement = Arrangement.spacedBy(18.dp), contentPadding = PaddingValues(8.dp)) {
            items(views, key = { it.first }) { (view, title) ->
                HomeAction(interfaceText(if (view == "all") "Library" else title), tv) { browse(view) }
            }
        }
    }
}

@Composable
private fun HomeAction(label: String, tv: Boolean, onClick: () -> Unit) {
    if (tv) androidx.tv.material3.Button(onClick = onClick) { androidx.tv.material3.Text(label) }
    else Button(onClick = onClick) { Text(label) }
}

@Composable
private fun HomeShelf(title: String, items: List<CatalogItem>, catalog: CatalogModel, tv: Boolean,
                      landscape: Boolean = false, open: (CatalogItem) -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Text(interfaceText(title), style = MaterialTheme.typography.titleLarge, color = KinoColor.text,
            modifier = Modifier.semantics { heading() })
        LazyRow(horizontalArrangement = Arrangement.spacedBy(18.dp), contentPadding = PaddingValues(12.dp)) {
            items(items, key = CatalogItem::id) { item ->
                val width = homeCardWidth(tv, landscape)
                val body: @Composable () -> Unit = {
                    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        MediaArtwork(item, catalog, Modifier.fillMaxWidth(), landscape, progress = false)
                        Column(Modifier.padding(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                            val enlarged = androidx.compose.ui.platform.LocalDensity.current.fontScale >= 1.5f
                            Text(item.title, style = MaterialTheme.typography.titleMedium, color = KinoColor.text,
                                maxLines = if (enlarged) Int.MAX_VALUE else 2, overflow = TextOverflow.Ellipsis)
                            val metadata = if (item.kind in setOf("music", "audiobook")) listOf(item.artist, item.album)
                                else listOf(if (item.showId.isNotEmpty()) "S${item.season} E${item.episode}" else "", item.rating, item.genres)
                            metadata.filter(String::isNotEmpty).joinToString(" · ").takeIf(String::isNotEmpty)?.let {
                                Text(it, style = MaterialTheme.typography.bodySmall, color = KinoColor.muted,
                                    maxLines = if (enlarged) Int.MAX_VALUE else 2, overflow = TextOverflow.Ellipsis)
                            }
                            if (item.positionLabel.isNotEmpty()) Text(interfaceText(item.positionLabel),
                                style = MaterialTheme.typography.bodySmall, color = KinoColor.muted)
                        }
                    }
                }
                if (tv) androidx.tv.material3.Card(onClick = { open(item) }, modifier = Modifier.width(width).semantics { contentDescription = item.title },
                    colors = androidx.tv.material3.CardDefaults.colors(containerColor = Color.Transparent,
                        focusedContainerColor = Color.Transparent), content = { body() })
                else Card(onClick = { open(item) }, modifier = Modifier.width(width).semantics { contentDescription = item.title },
                    colors = CardDefaults.cardColors(containerColor = Color.Transparent), content = { body() })
            }
        }
    }
}
