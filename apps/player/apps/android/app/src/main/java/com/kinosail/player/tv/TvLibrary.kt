package com.kinosail.player.tv

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.grid.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.Search
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import kotlinx.coroutines.delay
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.text.font.FontWeight
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
import com.kinosail.player.mobile.TitleCount
import kotlinx.coroutines.launch

@Composable
internal fun TvLibrary(connection: ConnectionModel, viewer: Viewer) {
    val catalog: CatalogModel = viewModel()
    val state = catalog.state
    val nowPlaying = AudioPlaybackService.nowPlayingFor(viewer)
    var destination by remember { mutableStateOf("home") }
    var playingItem by remember { mutableStateOf<CatalogItem?>(null) }
    var photoItem by remember { mutableStateOf<CatalogItem?>(null) }
    val grid = rememberLazyGridState()
    val homeList = androidx.compose.foundation.lazy.rememberLazyListState()
    LaunchedEffect(Unit) { TvFocusMemory.id = TvFocusMemory.LAUNCH }
    val scope = rememberCoroutineScope()
    var jumpTarget by remember { mutableStateOf<String?>(null) }
    LaunchedEffect(state.view, state.sort) { grid.scrollToItem(0) }
    val keyboard = LocalSoftwareKeyboardController.current
    val navigate: (String) -> Unit = { chosen ->
        catalog.closeDetail(); destination = chosen
        if (chosen !in setOf("home", "listen", "settings", "collections"))
            catalog.changeView(if (chosen == "search") "all" else chosen, refreshIfEmpty = true)
    }
    LaunchedEffect(viewer.serverId, viewer.id) { catalog.open(viewer) }
    AutoRetry(state.notice != null && !state.loading && !state.connectionExpired, catalog::retry)
    AutoRetry(state.detailNotice != null && state.listed == null && !state.listBusy, catalog::retryDetail)
    LaunchedEffect(destination, state.loading) {
        if (!state.loading && destination !in setOf("home", "listen", "settings", "collections"))
            catalog.changeView(if (destination == "search") "all" else destination)
    }
    LaunchedEffect(grid, destination, state.selected, state.items.size, state.loading, state.notice) {
        if (destination !in setOf("home", "listen", "settings", "collections") && state.selected == null && !state.loading &&
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
    if (destination == "settings" && state.selected == null) { TvSettings(viewer, catalog, connection::signOut); return }
    if (destination == "collections" && state.selected == null) {
        CollectionsScreen(viewer, catalog, true) { navigate("home") }; return
    }
    if (state.selected?.showId?.isNotEmpty() == true) {
        ShowScreen(state.selected.showId, viewer, catalog, true, catalog::closeDetail) { playingItem = it }; return
    }
    if (destination in setOf("home", "listen") && state.selected == null) {
        HomeScreen(viewer, catalog, true, nowPlaying, navigate, catalog::selectHomeItem, play = { playingItem = it },
            listening = destination == "listen", settings = { navigate("settings") }, list = homeList); return
    }
    if (state.selected != null) {
        TvDetail(state.selected, catalog, play = { playingItem = state.selected }, viewPhoto = { photoItem = state.selected }); return
    }
    Box(Modifier.fillMaxSize().background(KinoColor.background)) {
        SailBackdrop()
        Column(Modifier.fillMaxSize().safeDrawingPadding().padding(48.dp), verticalArrangement = Arrangement.spacedBy(24.dp)) {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(20.dp), verticalAlignment = Alignment.CenterVertically) {
                Text(interfaceText(if (destination == "search") "Search" else LIBRARY_VIEWS.first { it.first == destination }.second),
                    style = MaterialTheme.typography.headlineLarge, fontWeight = FontWeight.Bold, color = KinoColor.text,
                    modifier = Modifier.semantics { heading() })
                Spacer(Modifier.weight(1f))
                if (destination != "search" && state.items.isNotEmpty()) TitleCount(state.total)
            }
            run {
                if (destination == "search") {
                    val field = remember { FocusRequester() }
                    LaunchedEffect(Unit) { runCatching { field.requestFocus() } }
                    LaunchedEffect(catalog.searchInput) {
                        if (catalog.searchInput.trim() == catalog.activeSearch) return@LaunchedEffect
                        delay(250)
                        catalog.search()
                    }
                    Row(horizontalArrangement = Arrangement.spacedBy(16.dp), verticalAlignment = Alignment.CenterVertically) {
                        OutlinedTextField(catalog.searchInput, onValueChange = { if (it.length <= 512) catalog.searchInput = it },
                            label = { androidx.compose.material3.Text(interfaceText("Search library")) }, singleLine = true,
                            leadingIcon = { Icon(Icons.Filled.Search, contentDescription = null) },
                            keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search, showKeyboardOnFocus = false),
                            keyboardActions = KeyboardActions(onSearch = { catalog.search(); keyboard?.hide() }),
                            colors = OutlinedTextFieldDefaults.colors(focusedTextColor = KinoColor.text, unfocusedTextColor = KinoColor.text,
                                focusedLabelColor = KinoColor.signal, unfocusedLabelColor = KinoColor.muted,
                                focusedBorderColor = KinoColor.signal, unfocusedBorderColor = KinoColor.muted,
                                focusedLeadingIconColor = KinoColor.signal, unfocusedLeadingIconColor = KinoColor.muted),
                            modifier = Modifier.weight(1f).focusRequester(field))
                        Button(onClick = { catalog.clearSearch(); keyboard?.hide() }) { Text(interfaceText("Clear search")) }
                    }
                    state.resultQuery?.let { query ->
                        Text("${interfaceText("Results")}: ${state.total}" + if (query.isEmpty()) "" else " · “$query”",
                            color = KinoColor.text, maxLines = 2, overflow = TextOverflow.Ellipsis,
                            modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite })
                    }
                }
                LibrarySortMenu(state.sort, tv = true, change = catalog::changeSort)
                if ((state.failedOffset ?: 0) == 0 || state.connectionExpired) {
                    state.notice?.let { Text(it, color = KinoColor.text, modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite }) }
                    if (state.notice != null) Button(onClick = catalog::retry) { Text(interfaceText("Try again")) }
                }
                if (state.loading && state.items.isEmpty()) LibraryLoading(tv = true, view = state.view)
                else if (state.items.isEmpty() && state.notice == null) Text(interfaceText(catalogEmptyMessage(state.view, catalog.hasActiveSearch)), color = KinoColor.muted)
                val letters = jumpLetters(state, destination == "search")
                Row(Modifier.weight(1f), horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                LazyVerticalGrid(GridCells.Adaptive(if (state.view == "photos") 180.dp else 115.dp),
                    modifier = Modifier.weight(1f), state = grid, contentPadding = PaddingValues(12.dp),
                    horizontalArrangement = Arrangement.spacedBy(16.dp), verticalArrangement = Arrangement.spacedBy(28.dp)) {
                    items(state.items, key = CatalogItem::id) { item ->
                        TvMediaCard(item, catalog, Modifier.fillMaxWidth()) { catalog.select(item) }
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
                if (letters.isNotEmpty()) LetterIndex(letters, tv = true, Modifier.fillMaxHeight()) { letter ->
                    jumpTarget = letter.label
                    scope.launch { if (catalog.loadThrough(letter.offset) && jumpTarget == letter.label) grid.scrollToLoaded(letter.offset) }
                }
                }
            }
        }
    }
}

@Composable
internal fun TvDetail(item: CatalogItem, catalog: CatalogModel, play: () -> Unit, viewPhoto: () -> Unit) {
    val firstModifier = Modifier.initialTvFocus(item.id)
    Box(Modifier.fillMaxSize().background(KinoColor.background)) {
    TvBackdrop(item, catalog)
    Column(Modifier.fillMaxSize().safeDrawingPadding().verticalScroll(rememberScrollState()).padding(horizontal = 48.dp, vertical = 32.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp)) {
        TvHeroInformation(item, catalog, listOf(item.year, item.rating, item.genres).filter(String::isNotEmpty).joinToString(" · ")) {
            Row(horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                if (item.kind in setOf("video", "music", "audiobook")) Button(onClick = play, modifier = firstModifier) {
                    Text(interfaceText(item.playLabel))
                } else if (item.kind == "photo") Button(onClick = viewPhoto, enabled = item.stream.isNotEmpty(), modifier = firstModifier) {
                    Text(interfaceText("View photo"))
                }
                catalog.state.listed?.let { listed ->
                    Button(onClick = { catalog.setListed(!listed) }, enabled = !catalog.state.listBusy,
                        contentPadding = ButtonDefaults.ButtonWithIconContentPadding) {
                        Icon(if (listed) Icons.Filled.Check else Icons.Filled.Add, contentDescription = null,
                            modifier = Modifier.size(ButtonDefaults.IconSize))
                        Spacer(Modifier.width(ButtonDefaults.IconSpacing))
                        Text(interfaceText(if (listed) "In My List" else "My List"))
                    }
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
}
