package com.kinosail.player.mobile

import android.content.Context
import android.content.ContextWrapper
import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.grid.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Check
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import com.kinosail.player.core.*
import com.kinosail.player.design.KinoColor
import com.kinosail.player.design.SailBackdrop
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

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
    var parent by remember(viewer.serverId, viewer.id) { mutableStateOf<String?>(null) }
    var tabNotice by remember { mutableStateOf<String?>(null) }
    var playingItem by remember { mutableStateOf<CatalogItem?>(null) }
    var photoItem by remember { mutableStateOf<CatalogItem?>(null) }
    var bookItem by remember { mutableStateOf<CatalogItem?>(null) }
    val grid = rememberLazyGridState()
    val scope = rememberCoroutineScope()
    var jumpTarget by remember { mutableStateOf<String?>(null) }
    LaunchedEffect(state.view, state.sort) { grid.scrollToItem(0) }
    val keyboard = LocalSoftwareKeyboardController.current
    val navigate: (String) -> Unit = { chosen ->
        catalog.closeDetail()
        parent = if (chosen in tabs || chosen == "more") null
            else if (destination in tabs || destination == "more") destination else parent
        destination = chosen
        if (chosen !in OUTSIDE_CATALOG)
            catalog.changeView(if (chosen == "search") "all" else chosen, refreshIfEmpty = true)
    }
    LaunchedEffect(viewer.serverId, viewer.id) { catalog.open(viewer) }
    AutoRetry(state.notice != null && !state.loading && !state.connectionExpired, catalog::retry)
    AutoRetry(state.detailNotice != null && state.listed == null && !state.listBusy, catalog::retryDetail)
    LaunchedEffect(destination, state.loading) {
        if (!state.loading && destination !in OUTSIDE_CATALOG)
            catalog.changeView(if (destination == "search") "all" else destination)
    }
    LaunchedEffect(grid, destination, state.selected, state.items.size, state.loading, state.notice) {
        if (destination !in OUTSIDE_CATALOG && state.selected == null &&
            !state.loading && state.notice == null && state.items.size < state.total) {
            snapshotFlow { grid.layoutInfo.visibleItemsInfo.lastOrNull()?.index ?: -1 }.collect { last ->
                if (last >= state.items.size - 8) catalog.loadMore()
            }
        }
    }
    DisposableEffect(Unit) { onDispose { catalog.reset() } }
    BackHandler(state.selected != null && playingItem == null && photoItem == null && bookItem == null) { catalog.closeDetail() }
    BackHandler(destination != tabs.first() && state.selected == null && playingItem == null && photoItem == null && bookItem == null) {
        navigate(parent ?: tabs.first())
    }
    val saveTabs: (List<String>) -> Unit = { value ->
        try { preferences.save(viewer, value); tabs = value; tabNotice = null }
        catch (_: Exception) { tabNotice = "Could not save your tabs. Try again." }
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
            "collections" -> CollectionsScreen(viewer, catalog, false) { navigate(tabs.first()) }
            "home", "listen" -> HomeScreen(viewer, catalog, false, nowPlaying, browse = navigate,
                open = catalog::selectHomeItem, play = { playingItem = it }, listening = destination == "listen")
            "more" -> Column {
                tabNotice?.let { Text(it, modifier = Modifier.padding(20.dp).semantics { liveRegion = LiveRegionMode.Polite }) }
                MoreScreen(tabs, saveTabs, navigate)
            }
            "settings" -> SettingsScreen(viewer, catalog, tabs, saveTabs, connection::signOut)
            else -> Box(Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
                SailBackdrop()
                Column(Modifier.fillMaxSize().padding(20.dp), verticalArrangement = Arrangement.spacedBy(16.dp)) {
                    Text(tabTitle(destination), style = MaterialTheme.typography.headlineLarge)
                    if (nowPlaying != null) TextButton(onClick = { playingItem = nowPlaying }) { Text("Now playing · ${nowPlaying.title}") }
                    if (destination == "search") SearchField(catalog) { keyboard?.hide() }
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Box(Modifier.weight(1f)) { if (state.items.isNotEmpty()) TitleCount(state.total) }
                        LibrarySortMenu(state.sort, tv = false, change = catalog::changeSort)
                    }
                    if ((state.failedOffset ?: 0) == 0 || state.connectionExpired) {
                        state.notice?.let { Text(it, color = KinoColor.text,
                            modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite }) }
                        if (state.notice != null) RetryAction(onClick = catalog::retry)
                    }
                    if (state.loading && state.items.isEmpty()) LibraryLoading(view = state.view)
                    else if (state.items.isEmpty() && state.notice == null) Text(interfaceText(catalogEmptyMessage(state.view, catalog.hasActiveSearch)),
                        color = MaterialTheme.colorScheme.onSurfaceVariant)
                    val letters = jumpLetters(state, destination == "search")
                    Box(Modifier.weight(1f)) {
                    LazyVerticalGrid(GridCells.Adaptive(if (state.view == "photos") 240.dp else 96.dp),
                        modifier = Modifier.fillMaxSize(), state = grid,
                        horizontalArrangement = Arrangement.spacedBy(12.dp), verticalArrangement = Arrangement.spacedBy(28.dp),
                        contentPadding = PaddingValues(end = if (letters.isEmpty()) 0.dp else 32.dp, bottom = 24.dp)) {
                        items(state.items, key = CatalogItem::id) { item ->
                            Card(onClick = { catalog.select(item) }, modifier = Modifier.semantics { contentDescription = item.title },
                                colors = CardDefaults.cardColors(containerColor = androidx.compose.ui.graphics.Color.Transparent)) {
                                MediaCardBody(item, catalog, tv = false, progressOverlay = true)
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
                    if (letters.isNotEmpty()) LetterIndex(letters, tv = false, Modifier.align(Alignment.CenterEnd).offset(x = 8.dp)) { letter ->
                        jumpTarget = letter.label
                        scope.launch { if (catalog.loadThrough(letter.offset) && jumpTarget == letter.label) grid.scrollToLoaded(letter.offset) }
                    }
                    }
                }
            }
        }
    }
}

private val OUTSIDE_CATALOG = setOf("home", "listen", "more", "collections", "settings")

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
        val back: @Composable () -> Unit = { TextButton(onClick = catalog::closeDetail) { Text(interfaceText("Back")) } }
        val spotlight = spotlightFits(item, androidx.compose.ui.platform.LocalConfiguration.current.screenWidthDp.dp)
        if (!spotlight) back()
        val actions: @Composable () -> Unit = {
            FlowRow(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                when (item.kind) {
                    "video", "music", "audiobook" -> Button(onClick = play) { Text(interfaceText(item.playLabel)) }
                    "photo" -> Button(onClick = viewPhoto, enabled = item.stream.isNotEmpty()) { Text(interfaceText("View photo")) }
                    "book" -> Button(onClick = readBook) { Text(interfaceText("Read book")) }
                }
                catalog.state.listed?.let { listed ->
                    FilledTonalButton(onClick = { catalog.setListed(!listed) }, enabled = !catalog.state.listBusy) {
                        Icon(if (listed) Icons.Filled.Check else Icons.Filled.Add, contentDescription = null,
                            modifier = Modifier.size(ButtonDefaults.IconSize))
                        Spacer(Modifier.size(ButtonDefaults.IconSpacing))
                        Text(interfaceText(if (listed) "In My List" else "My List"))
                    }
                }
            }
        }
        if (spotlight) MediaSpotlight(item, catalog, null, bleed = 20.dp, topBleed = 20.dp, top = back, actions = actions)
        else MediaHero(item, catalog, tv = false, actions = actions)
        if (item.kind == "photo" && item.stream.isEmpty()) Text(interfaceText("Photo viewing is unavailable for this Viewer."))
        ContinueWatchingRemoval(item, catalog)
        if (catalog.state.listBusy && catalog.state.listed == null) Text(interfaceText("Loading My List status…"))
        catalog.state.detailNotice?.let {
            Text(it, color = KinoColor.text, modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite })
            if (catalog.state.listed == null) RetryAction(onClick = catalog::retryDetail)
        }
    }
}

@Composable
private fun ContinueWatchingRemoval(item: CatalogItem, catalog: CatalogModel) {
    if (item.kind != "video" || item.progress.seconds <= 0 && !item.progress.watched) return
    var state by remember(item.id) { mutableStateOf("ready") }
    val scope = rememberCoroutineScope()
    val removed = interfaceText("Removed from Continue watching.")
    val failed = interfaceText("Could not remove this title from Continue watching. Try again.")
    if (state != "removed") TextButton(enabled = state != "busy", onClick = {
        val (saved, viewerId) = catalog.connection ?: return@TextButton
        state = "busy"
        scope.launch {
            state = try {
                withContext(Dispatchers.IO) { AccountApi(saved.server).dismissContinueWatching(item.id, saved.token, viewerId) }
                "removed"
            } catch (cancelled: kotlinx.coroutines.CancellationException) { throw cancelled } catch (_: Exception) { "failed" }
        }
    }) { Text(interfaceText("Remove from Continue watching")) }
    when (state) {
        "removed" -> Text(removed, color = KinoColor.text, modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite })
        "failed" -> Text(failed, color = KinoColor.text, modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite })
    }
}
