package com.kinosail.player.core

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.foundation.lazy.rememberLazyListState
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.compose.ui.text.font.FontWeight
import androidx.lifecycle.viewmodel.compose.viewModel
import com.kinosail.player.design.KinoColor
import com.kinosail.player.design.SailBackdrop

@Composable
internal fun HomeScreen(viewer: Viewer, catalog: CatalogModel, tv: Boolean, nowPlaying: CatalogItem?,
                        browse: (String) -> Unit, open: (CatalogItem) -> Unit, play: (CatalogItem) -> Unit,
                        listening: Boolean = false, settings: () -> Unit = {},
                        list: androidx.compose.foundation.lazy.LazyListState = rememberLazyListState()) {
    val model: HomeModel = viewModel()
    val state = model.state.copy(listening = listening)
    LaunchedEffect(viewer.serverId, viewer.id) { model.open(viewer) }
    AutoRetry(state.notice != null && !state.loading, model::retry)
    DisposableEffect(Unit) { onDispose { if (!tv) model.reset() } }
    var heroItem by remember(viewer.id, listening) { mutableStateOf<CatalogItem?>(null) }
    val scope = rememberCoroutineScope()
    val search = remember { androidx.compose.ui.focus.FocusRequester() }
    var searchFocused by remember { mutableStateOf(false) }
    androidx.activity.compose.BackHandler(tv && !searchFocused) { runCatching { search.requestFocus() } }
    val launchTarget = listOf(state.tvWatchingRail, state.movies, state.shows, state.unwatchedShows, state.unwatchedMovies)
        .firstNotNullOfOrNull { it.firstOrNull()?.id }
    LaunchedEffect(launchTarget) { if (tv && launchTarget != null && TvFocusMemory.id == TvFocusMemory.LAUNCH) TvFocusMemory.id = launchTarget }
    val spotlight = !tv && state.featured?.let { spotlightFits(it, androidx.compose.ui.platform.LocalConfiguration.current.screenWidthDp.dp) } == true
    Box(Modifier.fillMaxSize().background(KinoColor.background)) {
        SailBackdrop()
        Column(if (tv) Modifier.fillMaxSize().safeDrawingPadding() else Modifier.fillMaxSize()) {
            if (tv) TvHomeHeader(listening, browse, settings, search) { searchFocused = it }
            LazyColumn(Modifier.weight(1f), state = list,
                contentPadding = if (tv) PaddingValues(start = 48.dp, top = 8.dp, end = 48.dp, bottom = 48.dp) else PaddingValues(20.dp),
                verticalArrangement = Arrangement.spacedBy(32.dp)) {
                val header: @Composable () -> Unit = { PhoneHomeHeader(listening, browse) }
                if (!tv && !spotlight) item { header() }
                if (spotlight) state.featured?.let { featured -> item {
                    MediaSpotlight(featured.homeSubtitleItem, catalog, interfaceText(if (featured.progress.seconds > 0)
                        if (listening) "Listening" else "Watching" else "For you"), bleed = 20.dp, topBleed = 20.dp, top = header) {
                        HomeFeatureActions(featured, myList = { browse("list") },
                            play = { if (featured.kind == "show") open(featured) else play(featured) }) { open(featured) }
                    }
                } }
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
                if (tv && !listening) state.featured?.let { featured -> item { TvHomeHero(heroItem ?: featured) } }
                if (tv && state.tvWatchingRail.isNotEmpty()) item {
                    HomeShelf(if (listening) "Continue listening" else "Continue watching", state.tvWatchingRail,
                        catalog, tv, landscape = true, onFocus = {
                        heroItem = it
                        scope.launch {
                            withFrameNanos {}
                            snapshotFlow { list.isScrollInProgress }.first { !it }
                            list.animateScrollToItem(0)
                        }
                    }, open = play)
                }
                if (!tv && !spotlight) state.featured?.let { featured -> item {
                    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                        FlowRow(horizontalArrangement = Arrangement.spacedBy(28.dp), verticalArrangement = Arrangement.spacedBy(4.dp),
                            itemVerticalAlignment = androidx.compose.ui.Alignment.CenterVertically) {
                            Text(interfaceText(if (featured.progress.seconds > 0) if (listening) "Listening" else "Watching" else "For you"),
                                style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold, color = KinoColor.text,
                                modifier = Modifier.semantics { heading() })
                            TextButton(onClick = { browse("list") }) { Text(interfaceText("My List"), color = KinoColor.muted) }
                        }
                        MediaHero(featured.homeSubtitleItem, catalog, tv = false) {
                            HomeFeatureActions(featured, myList = null,
                                play = { if (featured.kind == "show") open(featured) else play(featured) }) { open(featured) }
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
                    state.movieGenres.forEach { (genre, movies) -> item { HomeShelf(genre, movies, catalog, tv, open = open) } }
                }
                if (!state.loading && state.notice == null && state.featured == null) item {
                    val message = interfaceText(if (listening) "Add music or audiobooks to your Server to see them here."
                        else "Media added to your Server will appear here.")
                    if (tv) Text(message, color = KinoColor.muted) else HomePlaceholder(listening, message)
                }
                if (!tv) item { BrowseLinks(browse, tv, listening) }
            }
        }
    }
}

internal fun homeCardWidth(tv: Boolean, landscape: Boolean) =
    if (landscape) if (tv) 180.dp else 260.dp else if (tv) 115.dp else 164.dp

@Composable
private fun BrowseLinks(browse: (String) -> Unit, tv: Boolean, listening: Boolean) {
    if (tv) return TvBrowseTiles(if (listening) LIBRARY_VIEWS.filter { it.first in setOf("music", "audiobooks") }
        else LIBRARY_VIEWS.filter { it.first in setOf("movies", "shows", "music", "audiobooks", "photos", "list") } +
            listOf("collections" to "Collections", "all" to "Library"), browse)
    val views = LIBRARY_VIEWS.filter { it.first in if (listening) setOf("music", "audiobooks") else setOf("movies", "shows") }
    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Text(interfaceText("Browse library"), style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold, color = KinoColor.text,
            modifier = Modifier.semantics { heading() })
        LazyRow(horizontalArrangement = Arrangement.spacedBy(18.dp), contentPadding = PaddingValues(vertical = 8.dp)) {
            items(views, key = { it.first }) { (view, title) ->
                OutlinedButton(onClick = { browse(view) }, border = androidx.compose.foundation.BorderStroke(1.dp, KinoColor.signal.copy(alpha = 0.6f)),
                    colors = ButtonDefaults.outlinedButtonColors(contentColor = KinoColor.signal)) {
                    Icon(androidx.compose.ui.res.painterResource(browseIcon(view)), contentDescription = null,
                        modifier = Modifier.size(ButtonDefaults.IconSize))
                    Spacer(Modifier.size(ButtonDefaults.IconSpacing))
                    Text(interfaceText(title))
                }
            }
        }
    }
}

@Composable
private fun HomeShelf(title: String, items: List<CatalogItem>, catalog: CatalogModel, tv: Boolean,
                      landscape: Boolean = false, onFocus: (CatalogItem) -> Unit = {}, open: (CatalogItem) -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Text(interfaceText(title), style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold, color = KinoColor.text,
            modifier = Modifier.semantics { heading() })
        LazyRow(if (tv) Modifier.bleed(24.dp, 0.dp) else Modifier, horizontalArrangement = Arrangement.spacedBy(18.dp),
            contentPadding = PaddingValues(horizontal = if (tv) 24.dp else 0.dp, vertical = 12.dp)) {
            items(items, key = CatalogItem::id) { item ->
                val width = homeCardWidth(tv, landscape)
                if (tv) TvMediaCard(item, catalog, Modifier.width(width), landscape, resumes = landscape, onFocus = onFocus) { open(item) }
                else Card(onClick = { open(item) }, modifier = Modifier.width(width).semantics { contentDescription = item.title },
                    colors = CardDefaults.cardColors(containerColor = Color.Transparent)) { MediaCardBody(item, catalog, tv, landscape) }
            }
        }
    }
}

@Composable
private fun HomeAction(label: String, tv: Boolean, onClick: () -> Unit) {
    if (tv) androidx.tv.material3.Button(onClick = onClick) { androidx.tv.material3.Text(label) }
    else Button(onClick = onClick) { Text(label) }
}

private fun browseIcon(view: String) = when (view) {
    "movies" -> com.kinosail.player.R.drawable.nav_movies
    "shows" -> com.kinosail.player.R.drawable.nav_shows
    "music" -> com.kinosail.player.R.drawable.nav_music
    else -> com.kinosail.player.R.drawable.nav_audiobooks
}

@Composable
private fun HomePlaceholder(listening: Boolean, message: String) {
    Column(Modifier.fillMaxWidth().padding(vertical = 32.dp), horizontalAlignment = androidx.compose.ui.Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Icon(androidx.compose.ui.res.painterResource(if (listening) com.kinosail.player.R.drawable.nav_listen
            else com.kinosail.player.R.drawable.nav_movies), contentDescription = null, tint = KinoColor.muted,
            modifier = Modifier.size(48.dp))
        Text(interfaceText(if (listening) "Nothing to listen to yet" else "Your library is ready"),
            style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold, color = KinoColor.text,
            modifier = Modifier.semantics { heading() })
        Text(message, color = KinoColor.muted, textAlign = androidx.compose.ui.text.style.TextAlign.Center)
    }
}
