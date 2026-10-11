package com.kinosail.player.tv

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.grid.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.tv.material3.*
import com.kinosail.player.core.*
import com.kinosail.player.design.KinoColor
import com.kinosail.player.design.SailBackdrop

@Composable
internal fun TvLibrary(connection: ConnectionModel, viewer: Viewer) {
    val catalog: CatalogModel = viewModel()
    val state = catalog.state
    val nowPlaying = AudioPlaybackService.nowPlayingFor(viewer)
    var destination by remember { mutableStateOf("home") }
    var playingItem by remember { mutableStateOf<CatalogItem?>(null) }
    var photoItem by remember { mutableStateOf<CatalogItem?>(null) }
    val detailFocus = remember { FocusRequester() }
    val grid = rememberLazyGridState()
    val keyboard = LocalSoftwareKeyboardController.current
    val navigate: (String) -> Unit = { chosen ->
        catalog.closeDetail(); destination = chosen
        if (chosen !in setOf("home", "listen", "settings"))
            catalog.changeView(if (chosen == "search") "all" else chosen, refreshIfEmpty = true)
    }
    LaunchedEffect(viewer.serverId, viewer.id) { catalog.open(viewer) }
    LaunchedEffect(destination, state.loading) {
        if (!state.loading && destination !in setOf("home", "listen", "settings"))
            catalog.changeView(if (destination == "search") "all" else destination)
    }
    LaunchedEffect(grid, destination, state.selected, state.items.size, state.loading, state.notice) {
        if (destination !in setOf("home", "listen", "settings") && state.selected == null && !state.loading &&
            state.notice == null && state.items.size < state.total) {
            snapshotFlow { grid.layoutInfo.visibleItemsInfo.lastOrNull()?.index ?: -1 }.collect { last ->
                if (last >= state.items.size - 8) catalog.loadMore()
            }
        }
    }
    DisposableEffect(Unit) { onDispose { catalog.reset() } }
    BackHandler(state.selected != null && playingItem == null && photoItem == null) { catalog.closeDetail() }
    BackHandler(destination != "home" && state.selected == null && playingItem == null && photoItem == null) { destination = "home" }
    if (playingItem != null) {
        PlaybackScreen(requireNotNull(playingItem), viewer, true, close = { playingItem = null }, onNext = { playingItem = it }); return
    }
    if (photoItem != null) { PhotoScreen(requireNotNull(photoItem), catalog, true) { photoItem = null }; return }
    if (state.selected?.showId?.isNotEmpty() == true) {
        ShowScreen(state.selected.showId, viewer, catalog, true, catalog::closeDetail) { playingItem = it }; return
    }
    if (destination in setOf("home", "listen") && state.selected == null) {
        HomeScreen(viewer, catalog, true, nowPlaying, navigate, catalog::selectHomeItem, play = { playingItem = it },
            listening = destination == "listen", settings = { navigate("settings") }); return
    }
    LaunchedEffect(state.selected?.id) {
        if (state.selected != null) { withFrameNanos { }; detailFocus.requestFocus() }
    }
    Box(Modifier.fillMaxSize().background(KinoColor.background)) {
        SailBackdrop()
        Column(Modifier.fillMaxSize().safeDrawingPadding().padding(48.dp), verticalArrangement = Arrangement.spacedBy(24.dp)) {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(20.dp)) {
                Text(if (state.selected != null) "Kinosail" else if (destination == "settings") interfaceText("Settings")
                    else interfaceText(if (destination == "search") "Search" else LIBRARY_VIEWS.first { it.first == destination }.second),
                    style = MaterialTheme.typography.headlineLarge, color = KinoColor.text, modifier = Modifier.weight(1f))
                Button(onClick = { navigate("home") }) { Text(interfaceText("Home")) }
                if (destination != "search") Button(onClick = { navigate("search") }) { Text(interfaceText("Search")) }
                if (destination != "settings") Button(onClick = { navigate("settings") }) { Text(interfaceText("Settings")) }
            }
            if (state.selected != null) {
                TvDetail(state.selected, catalog, Modifier.focusRequester(detailFocus), play = { playingItem = state.selected },
                    viewPhoto = { photoItem = state.selected })
            } else if (destination == "settings") {
                TvSettings(viewer, connection::signOut)
            } else {
                if (destination == "search") {
                    Row(horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                        OutlinedTextField(catalog.searchInput, onValueChange = { if (it.length <= 512) catalog.searchInput = it },
                            label = { androidx.compose.material3.Text(interfaceText("Search library")) }, singleLine = true,
                            keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
                            keyboardActions = KeyboardActions(onSearch = { catalog.search(); keyboard?.hide() }),
                            colors = OutlinedTextFieldDefaults.colors(focusedTextColor = KinoColor.text, unfocusedTextColor = KinoColor.text,
                                focusedLabelColor = KinoColor.signal, unfocusedLabelColor = KinoColor.muted,
                                focusedBorderColor = KinoColor.signal, unfocusedBorderColor = KinoColor.muted), modifier = Modifier.weight(1f))
                        Button(onClick = { catalog.search(); keyboard?.hide() }) { Text(interfaceText("Search")) }
                        Button(onClick = { catalog.clearSearch(); keyboard?.hide() }) { Text(interfaceText("Clear search")) }
                    }
                    state.resultQuery?.let { query ->
                        Text("${interfaceText("Results")}: ${state.total}" + if (query.isEmpty()) "" else " · “$query”",
                            color = KinoColor.text, maxLines = 2, overflow = TextOverflow.Ellipsis,
                            modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite })
                    }
                }
                if ((state.failedOffset ?: 0) == 0 || state.connectionExpired) {
                    state.notice?.let { Text(it, color = KinoColor.text, modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite }) }
                    if (state.notice != null) Button(onClick = catalog::retry) { Text(interfaceText("Try again")) }
                }
                if (state.loading && state.items.isEmpty()) LibraryLoading(tv = true, view = state.view)
                else if (state.items.isEmpty() && state.notice == null) Text(interfaceText(catalogEmptyMessage(state.view, catalog.hasActiveSearch)), color = KinoColor.muted)
                LazyVerticalGrid(GridCells.Adaptive(if (state.view == "photos") 280.dp else 160.dp),
                    modifier = Modifier.weight(1f), state = grid, contentPadding = PaddingValues(12.dp),
                    horizontalArrangement = Arrangement.spacedBy(20.dp), verticalArrangement = Arrangement.spacedBy(28.dp)) {
                    items(state.items, key = CatalogItem::id) { item ->
                        Card(onClick = { catalog.select(item) }, modifier = Modifier.fillMaxWidth().semantics { contentDescription = item.title }) {
                            Column {
                                MediaArtwork(item, catalog, Modifier.fillMaxWidth())
                                Text(item.title, maxLines = 2, modifier = Modifier.padding(8.dp), color = KinoColor.text)
                            }
                        }
                    }
                    if (state.loading && state.items.isNotEmpty()) item(span = { GridItemSpan(maxLineSpan) }) {
                        Text(interfaceText("Loading more…"), color = KinoColor.muted)
                    }
                    if (!state.loading && (state.failedOffset ?: 0) > 0 && !state.connectionExpired) item(key = "page-retry", span = { GridItemSpan(maxLineSpan) }) {
                        Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                            state.notice?.let { Text(it, color = KinoColor.text,
                                modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite }) }
                            Button(onClick = catalog::retry) { Text(interfaceText("Retry loading more")) }
                        }
                    }
                }
            }
        }
    }
}

@Composable
internal fun TvDetail(item: CatalogItem, catalog: CatalogModel, firstModifier: Modifier,
                      play: () -> Unit, viewPhoto: () -> Unit) {
    Column(Modifier.fillMaxWidth().verticalScroll(rememberScrollState()), verticalArrangement = Arrangement.spacedBy(16.dp)) {
        MediaHero(item, catalog, tv = true) {
            Row(horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                if (item.kind in setOf("video", "music", "audiobook")) Button(onClick = play, modifier = firstModifier) {
                    Text(interfaceText(item.playLabel))
                } else if (item.kind == "photo") Button(onClick = viewPhoto, enabled = item.stream.isNotEmpty(), modifier = firstModifier) {
                    Text(interfaceText("View photo"))
                }
                Button(onClick = catalog::closeDetail) { Text(interfaceText("Back")) }
            }
            catalog.state.listed?.let { listed ->
                Button(onClick = { catalog.setListed(!listed) }, enabled = !catalog.state.listBusy) {
                    Text(interfaceText(if (listed) "Remove from My List" else "Add to My List"))
                }
            }
        }
        if (item.kind == "book") Text(interfaceText("Read this book on an Android phone or tablet."), color = KinoColor.muted)
        if (item.kind == "photo" && item.stream.isEmpty()) Text(interfaceText("Photo viewing is unavailable for this Viewer."), color = KinoColor.muted)
        if (catalog.state.listBusy && catalog.state.listed == null) Text(interfaceText("Loading My List status…"), color = KinoColor.muted)
        catalog.state.detailNotice?.let {
            Text(it, color = KinoColor.text, modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite })
            if (catalog.state.listed == null) Button(onClick = catalog::retryDetail) { Text(interfaceText("Try again")) }
        }
    }
}

@Composable
private fun TvSettings(viewer: Viewer, signOut: () -> Unit) {
    var thanks by remember { mutableStateOf(false) }
    var notices by remember { mutableStateOf(false) }
    val context = androidx.compose.ui.platform.LocalContext.current
    Column(Modifier.fillMaxWidth().verticalScroll(rememberScrollState()), verticalArrangement = Arrangement.spacedBy(16.dp)) {
        Text(viewer.name, style = MaterialTheme.typography.titleLarge, color = KinoColor.text)
        Text(viewer.server, color = KinoColor.muted)
        Button(onClick = { thanks = !thanks }) { Text("Made possible by") }
        if (thanks) {
            Text("Thank you to the people behind Jetpack Compose, Media3, and the Android libraries that bring Kinosail to this device. FFmpeg and Jellyfin FFmpeg power media tools on your Server.", color = KinoColor.text)
            Text("This product uses the TMDB API but is not endorsed or certified by TMDB.", color = KinoColor.muted)
            Button(onClick = { notices = !notices }) { Text("Third-party notices") }
            if (notices) Text(remember { runCatching { context.assets.open("THIRD_PARTY_NOTICES.md").bufferedReader().use { it.readText() } }
                .getOrDefault("Notices could not be opened. Reinstall Kinosail and try again.") }, color = KinoColor.text)
        }
        Button(onClick = signOut) { Text(interfaceText("Sign out")) }
    }
}
