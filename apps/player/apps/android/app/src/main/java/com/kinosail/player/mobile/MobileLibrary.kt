package com.kinosail.player.mobile

import android.content.Context
import android.content.ContextWrapper
import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.Image
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
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.FilterChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
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
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import com.kinosail.player.core.CatalogItem
import com.kinosail.player.core.CatalogModel
import com.kinosail.player.core.AudioPlaybackService
import com.kinosail.player.core.ConnectionModel
import com.kinosail.player.core.HomeScreen
import com.kinosail.player.core.LIBRARY_VIEWS
import com.kinosail.player.core.PlaybackScreen
import com.kinosail.player.core.PhotoScreen
import com.kinosail.player.core.VideoPipHost
import com.kinosail.player.core.PdfReaderScreen
import com.kinosail.player.core.ShowScreen
import com.kinosail.player.core.Viewer
import com.kinosail.player.design.SailBackdrop

@Composable
internal fun MobileLibrary(connection: ConnectionModel, viewer: Viewer) {
    val catalog: CatalogModel = viewModel()
    val state = catalog.state
    val nowPlaying = AudioPlaybackService.nowPlayingFor(viewer)
    val context = LocalContext.current
    val pipHost = remember(context) { findVideoPipHost(context) }
    var home by remember { mutableStateOf(true) }
    var playingItem by remember { mutableStateOf<CatalogItem?>(null) }
    var photoItem by remember { mutableStateOf<CatalogItem?>(null) }
    var bookItem by remember { mutableStateOf<CatalogItem?>(null) }
    val keyboard = LocalSoftwareKeyboardController.current
    val submitSearch = {
        catalog.search()
        keyboard?.hide()
        Unit
    }
    LaunchedEffect(viewer.serverId, viewer.id) { catalog.open(viewer) }
    DisposableEffect(Unit) { onDispose { catalog.reset() } }
    BackHandler(state.selected != null && playingItem == null && photoItem == null && bookItem == null) { catalog.closeDetail() }
    BackHandler(!home && state.selected == null && playingItem == null && photoItem == null && bookItem == null) { home = true }
    if (playingItem != null) {
        PlaybackScreen(requireNotNull(playingItem), viewer, tv = false, close = { playingItem = null },
            onNext = { playingItem = it }, pipHost = pipHost)
        return
    }
    if (photoItem != null) {
        PhotoScreen(requireNotNull(photoItem), catalog, tv = false, close = { photoItem = null })
        return
    }
    if (bookItem != null) {
        PdfReaderScreen(requireNotNull(bookItem), viewer, close = { bookItem = null })
        return
    }
    if (state.selected?.showId?.isNotEmpty() == true) {
        ShowScreen(state.selected.showId, viewer, catalog, tv = false, catalog::closeDetail) {
            playingItem = it
        }
        return
    }
    if (home && state.selected == null) {
        HomeScreen(viewer, catalog, tv = false, nowPlaying = nowPlaying, browse = { home = false },
            open = catalog::selectHomeItem, play = { playingItem = it })
        return
    }
    Box(Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        if (isSystemInDarkTheme()) SailBackdrop()
        Column(Modifier.fillMaxSize().safeDrawingPadding().padding(20.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically) {
                Text("Kinosail", style = MaterialTheme.typography.headlineMedium,
                    color = MaterialTheme.colorScheme.onBackground)
                TextButton(onClick = connection::signOut, enabled = !connection.busy) { Text("Disconnect") }
            }
            if (nowPlaying != null) Button(onClick = { playingItem = nowPlaying },
                modifier = Modifier.fillMaxWidth()) { Text("Now playing · ${nowPlaying.title}") }
            if (state.selected != null) {
                MobileDetail(state.selected, catalog, play = { playingItem = state.selected },
                    viewPhoto = { photoItem = state.selected }, readBook = { bookItem = state.selected })
            } else {
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically) {
                    Text(if (state.view == "all") "Library" else LIBRARY_VIEWS.first { it.first == state.view }.second,
                        style = MaterialTheme.typography.headlineLarge,
                        color = MaterialTheme.colorScheme.onBackground)
                    TextButton(onClick = { home = true }) { Text("For you") }
                }
                Text("${viewer.name} · ${viewer.server}", color = MaterialTheme.colorScheme.onSurfaceVariant)
                LazyRow(horizontalArrangement = Arrangement.spacedBy(12.dp), modifier = Modifier.fillMaxWidth()) {
                    items(LIBRARY_VIEWS, key = { it.first }) { (view, label) ->
                        FilterChip(selected = state.view == view, onClick = { catalog.changeView(view) },
                            label = { Text(label) })
                    }
                }
                OutlinedTextField(value = catalog.searchInput,
                    onValueChange = { if (it.length <= 512) catalog.searchInput = it },
                    label = { Text("Search your library") }, singleLine = true,
                    keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
                    keyboardActions = KeyboardActions(onSearch = { submitSearch() }),
                    modifier = Modifier.fillMaxWidth())
                Button(onClick = submitSearch, modifier = Modifier.fillMaxWidth()) { Text("Search") }
                state.notice?.let { Text(it, color = MaterialTheme.colorScheme.error) }
                if (state.notice != null) TextButton(onClick = catalog::retry) { Text("Retry") }
                if (state.loading && state.items.isEmpty()) CircularProgressIndicator()
                else if (state.items.isEmpty() && state.notice == null) {
                    Text(if (state.view == "list") "Save a title to keep it in My List."
                        else "Nothing in your library yet.", color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                LazyVerticalGrid(columns = GridCells.Adaptive(144.dp), modifier = Modifier.weight(1f),
                    horizontalArrangement = Arrangement.spacedBy(12.dp),
                    verticalArrangement = Arrangement.spacedBy(16.dp),
                    contentPadding = PaddingValues(bottom = 24.dp)) {
                    items(state.items, key = CatalogItem::id) { item ->
                        Card(onClick = { catalog.select(item) }, modifier = Modifier.fillMaxWidth()) {
                            Column {
                                CatalogPoster(item, catalog, Modifier.fillMaxWidth())
                                Text(item.title, modifier = Modifier.padding(10.dp),
                                    style = MaterialTheme.typography.titleSmall,
                                    maxLines = 2, overflow = TextOverflow.Ellipsis)
                            }
                        }
                    }
                    if (state.items.size < state.total) item(span = { GridItemSpan(maxLineSpan) }) {
                        Button(onClick = catalog::loadMore, enabled = !state.loading,
                            modifier = Modifier.fillMaxWidth()) {
                            Text(if (state.loading) "Loading…" else "Load more")
                        }
                    }
                }
            }
        }
    }
}

private fun findVideoPipHost(context: Context): VideoPipHost? {
    var current: Context = context
    while (current is ContextWrapper) {
        if (current is VideoPipHost) return current
        current = current.baseContext
    }
    return current as? VideoPipHost
}

@Composable
private fun MobileDetail(item: CatalogItem, catalog: CatalogModel, play: () -> Unit,
                         viewPhoto: () -> Unit, readBook: () -> Unit) {
    Column(Modifier.fillMaxWidth().verticalScroll(rememberScrollState()),
        verticalArrangement = Arrangement.spacedBy(16.dp)) {
        TextButton(onClick = catalog::closeDetail) { Text("Back") }
        if (item.kind in setOf("video", "music", "audiobook")) {
            Button(onClick = play, modifier = Modifier.fillMaxWidth()) {
                Text(if (item.progress.seconds > 0 && !item.progress.watched) "Resume" else "Play")
            }
        }
        if (item.kind == "photo") Button(onClick = viewPhoto, enabled = item.stream.isNotEmpty(),
            modifier = Modifier.fillMaxWidth()) { Text("View photo") }
        if (item.kind == "photo" && item.stream.isEmpty()) Text(
            "Photo viewing is unavailable for this Viewer.",
            color = MaterialTheme.colorScheme.onSurfaceVariant)
        if (item.kind == "book") Button(onClick = readBook, modifier = Modifier.fillMaxWidth()) {
            Text("Read book")
        }
        catalog.state.listed?.let { listed ->
            TextButton(onClick = { catalog.setListed(!listed) }, enabled = !catalog.state.listBusy) {
                Text(if (listed) "Remove from My List" else "Add to My List")
            }
        }
        if (catalog.state.listBusy && catalog.state.listed == null) CircularProgressIndicator()
        catalog.state.detailNotice?.let { notice ->
            Text(notice, color = MaterialTheme.colorScheme.error)
            if (catalog.state.listed == null) TextButton(onClick = catalog::retryDetail) { Text("Retry") }
        }
        CatalogPoster(item, catalog, Modifier.width(176.dp), dimension = 800)
        Text(item.title, style = MaterialTheme.typography.headlineLarge,
            color = MaterialTheme.colorScheme.onBackground)
        Text(listOf(item.kind.replaceFirstChar(Char::uppercaseChar), item.year).filter(String::isNotEmpty)
            .joinToString(" · "), color = MaterialTheme.colorScheme.onSurfaceVariant)
        if (item.plot.isNotEmpty()) Text(item.plot, style = MaterialTheme.typography.bodyLarge,
            color = MaterialTheme.colorScheme.onBackground)
    }
}

@Composable
internal fun CatalogPoster(item: CatalogItem, catalog: CatalogModel, modifier: Modifier = Modifier,
                           dimension: Int = 400) {
    val image = produceState<android.graphics.Bitmap?>(null, item.artwork, catalog, dimension) {
        value = catalog.artwork(item.artwork, dimension)
    }.value
    Box(modifier.aspectRatio(2f / 3f).background(MaterialTheme.colorScheme.surfaceVariant),
        contentAlignment = Alignment.Center) {
        Text(item.title.firstOrNull()?.uppercase() ?: "K",
            modifier = Modifier.clearAndSetSemantics { },
            style = MaterialTheme.typography.displayMedium,
            color = MaterialTheme.colorScheme.primary)
        image?.let { Image(it.asImageBitmap(), contentDescription = null,
            contentScale = ContentScale.Crop, modifier = Modifier.fillMaxSize()) }
    }
}
