package com.kinosail.player.core

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
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
        LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(if (tv) 48.dp else 20.dp),
            verticalArrangement = Arrangement.spacedBy(32.dp)) {
            item {
                FlowRow(horizontalArrangement = Arrangement.spacedBy(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    Text(if (listening) interfaceText("Listen") else "Kinosail",
                        style = if (tv) MaterialTheme.typography.headlineLarge else MaterialTheme.typography.titleLarge,
                        color = KinoColor.text, modifier = Modifier.weight(1f))
                    if (tv) {
                        androidx.tv.material3.Button(onClick = { browse(if (listening) "home" else "listen") }) {
                            androidx.tv.material3.Text(interfaceText(if (listening) "Watch" else "Listen"))
                        }
                        androidx.tv.material3.Button(onClick = { browse("search") }) { androidx.tv.material3.Text(interfaceText("Search")) }
                        androidx.tv.material3.Button(onClick = settings) { androidx.tv.material3.Text(interfaceText("Settings")) }
                    } else TextButton(onClick = { browse("list") }) { Text(interfaceText("My List")) }
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
            if (tv) item { BrowseLinks(browse, tv, listening) }
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

@Composable
private fun BrowseLinks(browse: (String) -> Unit, tv: Boolean, listening: Boolean) {
    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Text(interfaceText("Browse library"), style = MaterialTheme.typography.titleLarge, color = KinoColor.text,
            modifier = Modifier.semantics { heading() })
        val views = if (listening) LIBRARY_VIEWS.filter { it.first in setOf("music", "audiobooks") }
            else LIBRARY_VIEWS.filter { it.first in setOf("movies", "shows", "music", "audiobooks", "photos", "list") }
        LazyRow(horizontalArrangement = Arrangement.spacedBy(18.dp), contentPadding = PaddingValues(8.dp)) {
            items(views, key = { it.first }) { (view, title) -> HomeAction(interfaceText(title), tv) { browse(view) } }
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
                val width = if (landscape) if (tv) 320.dp else 240.dp else if (tv) 200.dp else 144.dp
                val body: @Composable () -> Unit = {
                    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        MediaArtwork(item, catalog, Modifier.fillMaxWidth(), landscape)
                        Text(item.title, style = MaterialTheme.typography.titleMedium, color = KinoColor.text,
                            modifier = Modifier.padding(8.dp), maxLines = if (androidx.compose.ui.platform.LocalDensity.current.fontScale >= 1.5f) Int.MAX_VALUE else 2)
                    }
                }
                if (tv) androidx.tv.material3.Card(onClick = { open(item) }, modifier = Modifier.width(width).semantics { contentDescription = item.title }, content = { body() })
                else Card(onClick = { open(item) }, modifier = Modifier.width(width).semantics { contentDescription = item.title },
                    colors = CardDefaults.cardColors(containerColor = KinoColor.surface), content = { body() })
            }
        }
    }
}
