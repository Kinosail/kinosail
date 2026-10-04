package com.kinosail.player.mobile

import android.content.Context
import android.content.ContextWrapper
import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.grid.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import com.kinosail.player.core.*
import com.kinosail.player.design.SailBackdrop

@Composable
internal fun MobileLibrary(connection: ConnectionModel, viewer: Viewer) {
    val catalog: CatalogModel = viewModel()
    val state = catalog.state
    val nowPlaying = AudioPlaybackService.nowPlayingFor(viewer)
    val context = LocalContext.current
    val pipHost = remember(context) { findVideoPipHost(context) }
    val preferences = remember(context) { PersonalTabs(context) }
    var tabs by remember(viewer.serverId, viewer.id) { mutableStateOf(preferences.load(viewer)) }
    var destination by remember(viewer.serverId, viewer.id) { mutableStateOf(tabs.first()) }
    var tabNotice by remember { mutableStateOf<String?>(null) }
    var playingItem by remember { mutableStateOf<CatalogItem?>(null) }
    var photoItem by remember { mutableStateOf<CatalogItem?>(null) }
    var bookItem by remember { mutableStateOf<CatalogItem?>(null) }
    val grid = rememberLazyGridState()
    val keyboard = LocalSoftwareKeyboardController.current
    val navigate: (String) -> Unit = { chosen ->
        catalog.closeDetail()
        destination = chosen
        if (chosen !in setOf("home", "listen", "more")) catalog.changeView(if (chosen == "search") "all" else chosen)
    }
    LaunchedEffect(viewer.serverId, viewer.id) { catalog.open(viewer) }
    LaunchedEffect(destination, state.loading) {
        if (!state.loading && destination !in setOf("home", "listen", "more"))
            catalog.changeView(if (destination == "search") "all" else destination)
    }
    LaunchedEffect(grid, destination, state.selected, state.items.size, state.loading, state.notice) {
        if (destination !in setOf("home", "listen", "more") && state.selected == null &&
            !state.loading && state.notice == null && state.items.size < state.total) {
            snapshotFlow { grid.layoutInfo.visibleItemsInfo.lastOrNull()?.index ?: -1 }.collect { last ->
                if (last >= state.items.size - 8) catalog.loadMore()
            }
        }
    }
    DisposableEffect(Unit) { onDispose { catalog.reset() } }
    BackHandler(state.selected != null && playingItem == null && photoItem == null && bookItem == null) { catalog.closeDetail() }
    BackHandler(destination != tabs.first() && state.selected == null && playingItem == null && photoItem == null && bookItem == null) {
        navigate(tabs.first())
    }
    if (playingItem != null) {
        PlaybackScreen(requireNotNull(playingItem), viewer, false, close = { playingItem = null },
            onNext = { playingItem = it }, pipHost = pipHost)
        return
    }
    if (photoItem != null) { PhotoScreen(requireNotNull(photoItem), catalog, false) { photoItem = null }; return }
    if (bookItem != null) { BookReaderScreen(requireNotNull(bookItem), viewer) { bookItem = null }; return }
    LibraryNavigation(tabs, if (destination in tabs) destination else "more", navigate) {
        if (state.selected?.showId?.isNotEmpty() == true) {
            ShowScreen(state.selected.showId, viewer, catalog, false, catalog::closeDetail) { playingItem = it }
        } else if (state.selected != null) {
            Box(Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
                SailBackdrop()
                MobileDetail(state.selected, catalog, play = { playingItem = state.selected },
                    viewPhoto = { photoItem = state.selected }, readBook = { bookItem = state.selected })
            }
        } else when (destination) {
            "home", "listen" -> HomeScreen(viewer, catalog, false, nowPlaying, browse = navigate,
                open = catalog::selectHomeItem, play = { playingItem = it }, listening = destination == "listen")
            "more" -> Column {
                tabNotice?.let { Text(it, modifier = Modifier.padding(20.dp).semantics { liveRegion = LiveRegionMode.Polite }) }
                MoreScreen(viewer, tabs, saveTabs = { value ->
                    try { preferences.save(viewer, value); tabs = value; tabNotice = null }
                    catch (_: Exception) { tabNotice = "Could not save your tabs. Try again." }
                }, navigate, connection::signOut)
            }
            else -> Box(Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
                SailBackdrop()
                Column(Modifier.fillMaxSize().padding(20.dp), verticalArrangement = Arrangement.spacedBy(16.dp)) {
                    Text(tabTitle(destination), style = MaterialTheme.typography.headlineLarge)
                    if (nowPlaying != null) TextButton(onClick = { playingItem = nowPlaying }) { Text("Now playing · ${nowPlaying.title}") }
                    if (destination == "search") {
                        OutlinedTextField(catalog.searchInput, onValueChange = { if (it.length <= 512) catalog.searchInput = it },
                            label = { Text(interfaceText("Search library")) }, singleLine = true,
                            keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
                            keyboardActions = KeyboardActions(onSearch = { catalog.search(); keyboard?.hide() }),
                            modifier = Modifier.fillMaxWidth())
                        Button(onClick = { catalog.search(); keyboard?.hide() }) { Text(interfaceText("Search")) }
                    }
                    if ((state.failedOffset ?: 0) == 0 || state.connectionExpired) {
                        state.notice?.let { Text(it, color = MaterialTheme.colorScheme.error,
                            modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite }) }
                        if (state.notice != null) TextButton(onClick = catalog::retry) { Text(interfaceText("Try again")) }
                    }
                    if (state.loading && state.items.isEmpty()) LibraryLoading(view = state.view)
                    else if (state.items.isEmpty() && state.notice == null) Text(interfaceText(catalogEmptyMessage(state.view, catalog.hasActiveSearch)),
                        color = MaterialTheme.colorScheme.onSurfaceVariant)
                    LazyVerticalGrid(GridCells.Adaptive(if (state.view == "photos") 240.dp else 144.dp),
                        modifier = Modifier.weight(1f), state = grid,
                        horizontalArrangement = Arrangement.spacedBy(18.dp), verticalArrangement = Arrangement.spacedBy(24.dp),
                        contentPadding = PaddingValues(bottom = 24.dp)) {
                        items(state.items, key = CatalogItem::id) { item ->
                            Card(onClick = { catalog.select(item) }, modifier = Modifier.semantics { contentDescription = item.title }, colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface)) {
                                MediaArtwork(item, catalog, Modifier.fillMaxWidth())
                                Text(item.title, modifier = Modifier.padding(10.dp), style = MaterialTheme.typography.titleMedium,
                                    maxLines = if (androidx.compose.ui.platform.LocalDensity.current.fontScale >= 1.5f) Int.MAX_VALUE else 2,
                                    overflow = TextOverflow.Ellipsis)
                            }
                        }
                        if (state.loading && state.items.isNotEmpty()) item(span = { GridItemSpan(maxLineSpan) }) {
                            Text(interfaceText("Loading more…"))
                        }
                        if (!state.loading && (state.failedOffset ?: 0) > 0 && !state.connectionExpired) item(key = "page-retry", span = { GridItemSpan(maxLineSpan) }) {
                            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                                state.notice?.let { Text(it, color = MaterialTheme.colorScheme.error,
                                    modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite }) }
                                TextButton(onClick = catalog::retry) { Text(interfaceText("Retry loading more")) }
                            }
                        }
                    }
                }
            }
        }
    }
}

private fun findVideoPipHost(context: Context): VideoPipHost? {
    var current: Context = context
    while (current is ContextWrapper) { if (current is VideoPipHost) return current; current = current.baseContext }
    return current as? VideoPipHost
}

@Composable
private fun MobileDetail(item: CatalogItem, catalog: CatalogModel, play: () -> Unit,
                         viewPhoto: () -> Unit, readBook: () -> Unit) {
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(20.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp)) {
        TextButton(onClick = catalog::closeDetail) { Text(interfaceText("Back")) }
        MediaHero(item, catalog, tv = false) {
            FlowRow(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                when (item.kind) {
                    "video", "music", "audiobook" -> Button(onClick = play) { Text(interfaceText(item.playLabel)) }
                    "photo" -> Button(onClick = viewPhoto, enabled = item.stream.isNotEmpty()) { Text(interfaceText("View photo")) }
                    "book" -> Button(onClick = readBook) { Text(interfaceText("Read book")) }
                }
                catalog.state.listed?.let { listed ->
                    OutlinedButton(onClick = { catalog.setListed(!listed) }, enabled = !catalog.state.listBusy) {
                        Text(interfaceText(if (listed) "Remove from My List" else "Add to My List"))
                    }
                }
            }
        }
        if (item.kind == "photo" && item.stream.isEmpty()) Text(interfaceText("Photo viewing is unavailable for this Viewer."))
        if (catalog.state.listBusy && catalog.state.listed == null) Text(interfaceText("Loading My List status…"))
        catalog.state.detailNotice?.let {
            Text(it, color = MaterialTheme.colorScheme.error, modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite })
            if (catalog.state.listed == null) TextButton(onClick = catalog::retryDetail) { Text(interfaceText("Try again")) }
        }
    }
}
